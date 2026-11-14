package main

import (
	"context"
	"github.com/yusefmosiah/go-choir/internal/apppackage"
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
	"time"
)

// storeOpenTimeout bounds each attempt to open and bootstrap the platform
// store (20 attempts below).
const storeOpenTimeout = 60 * time.Second

func main() {
	// Host side: a release is admitted by the shipped isolation; the installing
	// guest checks its own (apppackage.CurrentPlatform).
	apppackage.SetPlatformSource(apppackage.ShippedPlatform)
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
	// The published-piece search index is derived from Store B; build it
	// beside the listener (a search before it is whole says capped).
	go svc.WarmPublishedPieceIndex(context.Background())
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
	// PLATFORM_TAPE_SWEEP_MODE=active; on demand at
	// /internal/platform/artifact-gc/tape and, for destroyed computers, daily.
	// The destroyed proof needs vmctl.
	svc.SetTapeSweepPolicy(platform.TapeSweepPolicy{Mode: cfg.TapeSweepMode, ArchiveDir: cfg.TapeSweepArchiveDir})
	// Tape segments (stage A2): off unless PLATFORM_TAPE_SEGMENTS=write. The
	// read path is segment first, legacy second whatever this says.
	svc.SetTapeSegmentsMode(cfg.TapeSegments)
	// Seal-time compaction of tape segments (rule F.2): each sealed segment is
	// rewritten once as a fresh file, so btrfs frees the bookends of its
	// fsynced appends. Background, low priority, never blocks an append.
	svc.EnableTapeSealCompaction()
	log.Printf("corpusd: tape segments: %s", svc.TapeSegmentsMode())
	// Append-receipt compaction (docs/design/append-receipt-trim-2026-10-31.md):
	// the named computers' receipt bodies are stored in the lossless compact
	// form, at the append and by a paced backfill; dry-run only counts and
	// expand writes them back to full JSON (the rollback). A malformed value
	// compacts nothing and says so; corpusd still starts.
	receiptPolicy, err := platform.ParseReceiptCompactComputers(os.Getenv(platform.EnvReceiptCompactComputers))
	if err != nil {
		log.Printf("corpusd: receipt compaction disabled: %v", err)
		receiptPolicy = platform.ReceiptCompactionPolicy{}
	}
	store.SetReceiptCompaction(receiptPolicy)
	log.Printf("corpusd: receipt compaction: %s", receiptPolicy)
	handler := platform.NewHandler(svc)
	if cfg.VmctlURL != "" {
		svc.SetComputerRegistry(platform.NewVmctlComputerRegistry(vmctl.NewClient(cfg.VmctlURL)))
		if err := handler.ConfigureGuestBinding(vmctl.NewClient(cfg.VmctlURL)); err != nil {
			log.Fatalf("corpusd guest binding: %v", err)
		}
	}
	// The destroyed-computer tape sweep on its own, daily, beside the hourly
	// artifact collector (station S7). No setting: it follows
	// PLATFORM_TAPE_SWEEP_MODE as the route does, never acknowledges
	// unrecorded fork lineage and never passes no_archive.
	tapeSweepCtx, cancelTapeSweep := context.WithCancel(context.Background())
	tapeSweepSched := platform.NewTapeSweepScheduler(svc)
	tapeSweepSched.Start(tapeSweepCtx)
	defer cancelTapeSweep()
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
	// A package published by decision shows its maker by the numbered Choir
	// address maild owns, asked as the proxy asks it (host-local maild).
	maildURL := os.Getenv("CORPUSD_MAILD_URL")
	if maildURL == "" {
		maildURL = "http://127.0.0.1:8087"
	}
	handler.ConfigurePublisherAddresses(maildURL, nil)
	log.Printf("corpusd: package base store: %q", os.Getenv("CORPUSD_HOST_BUILD_SOURCE_DIR"))
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
	var cleanRebaseHost platform.CleanRebaseHost
	if buildCfg, enabled := hostbuild.ConfigFromEnv(os.Getenv); enabled {
		if buildSvc, err := handler.ConfigureHostBuild(buildCfg); err != nil {
			log.Printf("corpusd host build: disabled, configuration failed: %v", err)
		} else {
			buildSvc.Start(context.Background())
			hostBuilds = buildSvc
			cleanRebaseHost = buildSvc
			log.Printf("corpusd host build: enabled (source %s, installable %s)", buildCfg.SourceDir, buildCfg.Installable)
		}
	}
	// P4 decision poster (docs/archive/problems/p4-standing-consensus-autonomy-2026-10-15.md):
	// under an owner's standing arm it mints a ConsensusDecisionReceipt from
	// the host's capsule-verifier ballot on this operation's P5 host build
	// (signed here, platform-control) and the guest's signed reviewer
	// ballot. Without the host build service it refuses every request.
	// Clean rebase admission (owner 2026-11-04): an effect_accepted under
	// platform-clean-rebase: is checked against the tape and the host build;
	// without the host build service every clean rebase is refused.
	ownerActs.ConfigureCleanRebase(svc.ReadTapeObject, cleanRebaseHost)
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
	handler.ConfigurePlatformOperators(cfg.PlatformOperatorsRaw)
	// The platform operator role (docs/design/platform-operator-account-2026-11-06.md):
	// account ids supplied at deploy; a malformed list grants nobody.
	if count, err := handler.ConfigurePlatformOperatorUsers(cfg.PlatformOperatorUsersRaw); err != nil {
		log.Printf("corpusd: platform operator accounts: %v", err)
	} else {
		log.Printf("corpusd: platform operator accounts: %d", count)
	}
	s := server.NewServer("corpusd", cfg.Port)
	platform.RegisterRoutes(s, handler)
	// A scheduled tape sweep is stopped at a batch boundary and waited for
	// up to the shutdown deadline, before the store closes (its tombstone
	// counts are written per batch, so a wait that runs out leaves them
	// truthful: an under-count, the rest re-listed by the next dry run).
	s.AddShutdownHook(func(ctx context.Context) {
		cancelTapeSweep()
		if err := tapeSweepSched.Wait(ctx); err != nil {
			log.Printf("corpusd: tape sweep schedule still running at the shutdown deadline")
		}
	})

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

	// The receipt backfill walk and the append timing line start as the
	// listener does: the walk waits its boot delay and then only for free space
	// (receipt_walk.go), and the timing line is what an operator watches while
	// the receipt table's indexes are swapped by hand.
	platform.NewReceiptCompactor(store).Start(context.Background())
	store.StartAppendTimingLog(context.Background())

	s.Start()
}
