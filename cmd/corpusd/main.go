package main

import (
	"context"
	"github.com/yusefmosiah/go-choir/internal/frontimportance"
	"github.com/yusefmosiah/go-choir/internal/hostbuild"
	"github.com/yusefmosiah/go-choir/internal/keyescrow"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// storeOpenTimeout bounds each attempt to open and bootstrap the platform
// store (20 attempts below).
const storeOpenTimeout = 60 * time.Second

func main() {
	// `corpusd front-importance` is the host timer's scoring pass
	// (internal/frontimportance); it never starts the platform service.
	if len(os.Args) > 1 && os.Args[1] == "front-importance" {
		os.Exit(frontimportance.Main(os.Args[2:]))
	}
	cfg, err := platform.LoadConfig()
	if err != nil {
		log.Fatalf("corpusd config: %v", err)
	}
	if err := cfg.EnsureDirs(); err != nil {
		log.Fatalf("corpusd dirs: %v", err)
	}

	var store *platform.Store
	var storeErr error
	for attempt := 1; attempt <= 20; attempt++ {
		// Each attempt is bounded: a database that accepts connections but never
		// answers must not hold this process (and a deploy switch) forever.
		openCtx, cancelOpen := context.WithTimeout(context.Background(), storeOpenTimeout)
		store, storeErr = platform.OpenStore(openCtx, cfg.ControlDSN, cfg.GraphDSN)
		cancelOpen()
		if storeErr == nil {
			break
		}
		log.Printf("corpusd store: attempt %d/20: %v", attempt, storeErr)
		time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
	}
	if storeErr != nil {
		log.Fatalf("corpusd store: %v", storeErr)
	}
	defer func() {
		_ = store.Close()
	}()

	// Retrieval indexes and projections build beside the listener: corpusd
	// serves on the old query shapes until each one reports live
	// (docs/archive/problems/corpus-retrieval-windowed-unindexed-untimed-2026-10-14.md).
	store.StartRetrievalMigrations(context.Background(), 10*time.Minute)

	svc := platform.NewService(store, cfg.ArtifactsRoot, cfg.SigningKeyPath)
	// Published versions' check overlay reads quote checks and support
	// verdicts from Store B, never from a paper computer (Cut 2 2b).
	svc.SetCheckOverlayProvider(platform.NewStoreCheckOverlay(svc))
	// The Autopaper front lists only these owners' auto stories when
	// CORPUSD_AUTOPAPER_OWNERS is set; every owner's when unset.
	svc.SetAutopaperOwners(platform.ParseAutopaperOwners(cfg.AutopaperOwnersRaw))
	svc.SetPublicBaseURL(cfg.PublicBaseURL)
	// Opinion pieces (signed-in users' labelled Texture, listed in the
	// Opinion section) are off unless CORPUSD_OPINION_LISTING=on.
	svc.SetOpinionListing(platform.ParseOpinionListing(cfg.OpinionListingRaw))
	if n, err := strconv.Atoi(strings.TrimSpace(cfg.OpinionDailyLimitRaw)); err == nil {
		svc.SetOpinionDailyLimit(n)
	}
	// Front rows carry importance scores when the host scorer's file is
	// present; absent or unreadable, the front list is unchanged.
	svc.SetFrontImportancePath(cfg.FrontImportancePath)
	// Periodic platform-artifacts reachability GC (dry-run unless explicitly
	// activated). The on-demand endpoint /internal/platform/artifact-gc runs
	// the same sweep, clamped to this policy.
	gcGrace, _ := time.ParseDuration(cfg.ArtifactGCGrace)
	gcInterval, _ := time.ParseDuration(cfg.ArtifactGCInterval)
	gcMax, _ := strconv.Atoi(cfg.ArtifactGCMaxDeletes)
	gcMaxBytes, _ := strconv.ParseInt(cfg.ArtifactGCMaxBytes, 10, 64)
	gcPolicy := platform.ArtifactGCConfig{
		Mode:             cfg.ArtifactGCMode,
		Grace:            gcGrace,
		MaxDeletes:       gcMax,
		MaxBytes:         gcMaxBytes,
		ActiveNamespaces: platform.ParseArtifactGCNamespaces(cfg.ArtifactGCNamespaces),
	}
	svc.SetArtifactGCPolicy(gcPolicy)
	platform.NewGCRunner(svc, gcPolicy, gcInterval).Start(context.Background())
	// Tape sweep (orphan pins, destroyed computers' tapes): dry-run unless
	// PLATFORM_TAPE_SWEEP_MODE=active; on demand only, at
	// /internal/platform/artifact-gc/tape. The destroyed proof needs vmctl.
	svc.SetTapeSweepPolicy(platform.TapeSweepPolicy{Mode: cfg.TapeSweepMode, ArchiveDir: cfg.TapeSweepArchiveDir})
	// Tape segments (stage A2): off unless PLATFORM_TAPE_SEGMENTS=write. The
	// read path is segment first, legacy second whatever this says.
	svc.SetTapeSegmentsMode(cfg.TapeSegments)
	log.Printf("corpusd: tape segments: %s", svc.TapeSegmentsMode())
	handler := platform.NewHandler(svc)
	// Library S1: release reads are owner-scoped unless CHOIR_RELEASE_READS=open
	// (a missing or unknown value is owner); read once, here.
	releaseReads := platform.ParseReleaseReads(cfg.ReleaseReadsRaw)
	handler.SetReleaseReads(releaseReads)
	log.Printf("corpusd: release reads: %s", releaseReads)
	if cfg.VmctlURL != "" {
		svc.SetComputerRegistry(platform.NewVmctlComputerRegistry(vmctl.NewClient(cfg.VmctlURL)))
		if err := handler.ConfigureGuestBinding(vmctl.NewClient(cfg.VmctlURL)); err != nil {
			log.Fatalf("corpusd guest binding: %v", err)
		}
	}
	eventCAS, eventArtifacts, eventAuth, err := svc.ComputerEventRuntime()
	if err != nil {
		log.Fatalf("corpusd computer event runtime: %v", err)
	}
	// S1 owner-act receipts and the effect_accepted admission gate
	// (docs/problems/effect-accepted-admitted-without-authority-2026-10-15.md).
	// CHOIR_OWNER_ACT_MODE: observe (default) logs "would refuse"; enforce refuses.
	ownerActs := platform.LoadOwnerActAuthorityFromEnv()
	log.Printf("corpusd: owner acts: %s", ownerActs.Status())
	eventCAS.ConfigureEffectAdmission(ownerActs)
	// Library S2: a package publish carries the owner-act receipt, verified
	// under the same mode; a base parent is read from the deploy checkout's
	// object store the host build already uses, read only.
	handler.ConfigurePackageOwnerActs(ownerActs)
	handler.ConfigurePackageBase(context.Background(), os.Getenv("CORPUSD_HOST_BUILD_SOURCE_DIR"))
	log.Printf("corpusd: package base store: %q", os.Getenv("CORPUSD_HOST_BUILD_SOURCE_DIR"))
	// Library S3: package reads follow each listing's scope set unless
	// CHOIR_PACKAGE_SHARE_READS=owner (any value but unset, empty or scoped
	// is owner), the read side's rollback; read once, here.
	shareReads := platform.ParsePackageShareReads(os.Getenv(platform.EnvPackageShareReads))
	handler.SetPackageShareReads(shareReads)
	log.Printf("corpusd: package share reads: %s", shareReads)
	if err := handler.ConfigureComputerEvents(eventCAS, eventArtifacts, eventAuth); err != nil {
		log.Fatalf("corpusd computer event routes: %v", err)
	}
	modeCAS, err := svc.SelfDevelopmentModeRuntime()
	if err != nil {
		log.Fatalf("corpusd self-development mode runtime: %v", err)
	}
	modeCAS.ConfigureOwnerActs(ownerActs)
	if err := handler.ConfigureSelfDevelopmentModes(modeCAS); err != nil {
		log.Fatalf("corpusd self-development mode routes: %v", err)
	}
	// P5 host build service: builds a computer's accepted Go change from its
	// base commit plus the frozen patch. Off unless the source checkout is
	// configured; the route then answers host_builder_unconfigured. A
	// misconfigured build service never stops corpusd (H23): it stays
	// unconfigured, and Go-touching applies fail visibly.
	var hostBuilds platform.HostBuildJobs
	if buildCfg, enabled := hostbuild.ConfigFromEnv(os.Getenv); enabled {
		if buildSvc, err := handler.ConfigureHostBuild(buildCfg); err != nil {
			log.Printf("corpusd host build: disabled, configuration failed: %v", err)
		} else {
			buildSvc.Start(context.Background())
			hostBuilds = buildSvc
			log.Printf("corpusd host build: enabled (source %s, installable %s)", buildCfg.SourceDir, buildCfg.Installable)
		}
	}
	// P4 decision poster (docs/archive/problems/p4-standing-consensus-autonomy-2026-10-15.md):
	// under an owner's standing arm it mints a ConsensusDecisionReceipt from
	// the host's capsule-verifier ballot on this operation's P5 host build
	// (signed here, platform-control) and the guest's signed reviewer
	// ballot. Without the host build service it refuses every request.
	consensus, err := svc.ConsensusActuatorRuntime(modeCAS, hostBuilds)
	if err != nil {
		log.Fatalf("corpusd consensus poster: %v", err)
	}
	if err := handler.ConfigureConsensusActuator(consensus); err != nil {
		log.Fatalf("corpusd consensus poster routes: %v", err)
	}
	// CHOIR_KEY_ESCROW_SEAL unset is today's load-or-generate; permit and
	// require read a sealed key under the KEK and never generate (FP9).
	escrowPrivateKey, escrowKeyState, escrowSealMode, err := keyescrow.LoadConfiguredPrivateKey(cfg.KeyEscrowKeyPath, os.Getenv)
	if err != nil {
		log.Fatalf("corpusd key escrow key: %v", err)
	}
	if escrowSealMode != keyescrow.SealOff {
		log.Printf("corpusd key escrow key: %s (seal mode %s)", escrowKeyState, escrowSealMode)
	}
	if err := handler.ConfigureKeyEscrow(escrowPrivateKey, cfg.KeyEscrowOperatorsRaw); err != nil {
		log.Fatalf("corpusd key escrow routes: %v", err)
	}
	s := server.NewServer("corpusd", cfg.Port)
	platform.RegisterRoutes(s, handler)

	// Object graph API: allows VMs to project and query
	// object graph data stored in the platform PostgreSQL databases (corpusd).
	ogStore := platform.NewObjectGraphStore(store)
	ogService := objectgraph.NewService(objectgraph.Config{
		Durable: ogStore,
	})
	ogHandler := platform.NewObjectGraphHandler(ogService, ogStore)
	if cfg.VmctlURL != "" {
		if err := ogHandler.ConfigureGuestBinding(vmctl.NewClient(cfg.VmctlURL)); err != nil {
			log.Fatalf("corpusd object graph guest binding: %v", err)
		}
	}
	ogHandler.ConfigureCaptureBodies(platform.CaptureBodiesRoot(cfg.ArtifactsRoot))
	ogHandler.ConfigureCaptureIndex(store)
	platform.RegisterObjectGraphRoutes(s, ogHandler)

	s.Start()
}
