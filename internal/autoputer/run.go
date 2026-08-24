package autoputer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/yusefmosiah/go-choir/internal/actorruntime"
	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/apihandler"
	"github.com/yusefmosiah/go-choir/internal/browsercontrol"
	"github.com/yusefmosiah/go-choir/internal/coagentowner"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/content"
	"github.com/yusefmosiah/go-choir/internal/desktopstate"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/gatewayruntime"
	"github.com/yusefmosiah/go-choir/internal/health"
	"github.com/yusefmosiah/go-choir/internal/mediastate"
	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/receiptsigner"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/textureowner"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/trace"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/updater"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
	"github.com/yusefmosiah/go-choir/internal/zot"
)

// replayHealthGate serves the guest /health surface while the computer event
// tape replay is running. During replay it reports 503 ReplayInProgress with the
// applied and durably committed sequence so the host wait-for-ready probe can
// distinguish liveness (progress) from readiness (head+witness matched). Product
// readiness (HTTP 200) is served only after the replay completes and the guest
// reaches its canonical head (B5/B6/B9).
type replayHealthGate struct {
	mu       sync.Mutex
	pending  bool
	appender *computerevent.ComputerEventAppender
	base     http.HandlerFunc
}

func (g *replayHealthGate) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	g.mu.Lock()
	pending := g.pending
	app := g.appender
	base := g.base
	g.mu.Unlock()
	if pending {
		var snap computerevent.ReplaySnapshot
		if app != nil {
			snap = app.ReplaySnapshot()
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":             "replaying",
			"sequence":           snap.Sequence,
			"committed_sequence": snap.CommittedSequence,
		})
		return
	}
	if base == nil {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
		return
	}
	base(w, r)
}

func (g *replayHealthGate) setPending(pending bool) {
	g.mu.Lock()
	g.pending = pending
	g.mu.Unlock()
}

// RunZotSession runs the autoputer binary's zot-session process mode.
func RunZotSession(stdin io.Reader, stdout, stderr io.Writer) int {
	return zot.RunSession(zot.SessionConfig{
		SessionID: os.Getenv("ZOT_SESSION_ID"),
		RootDir:   os.Getenv("ZOT_ROOT_DIR"),
		UserID:    os.Getenv("ZOT_USER_ID"),
	}, stdin, stdout, stderr)
}

// Run starts the autoputer service.
func Run() {
	cfg := LoadConfig()

	s := server.NewServer("autoputer", cfg.Port)

	// Initialize the placeholder shell handlers.
	h := NewHandler(cfg.ComputerID)
	RegisterRoutes(s, h)

	filesRoot := provideriface.ResolveFilesRoot(os.Getenv("AUTOPUTER_FILES_ROOT"))

	// Initialize the singleton Super Console PTY handler. The PTY process is
	// zot, not an interactive shell.
	superConsoleHandler := NewSuperConsoleHandler(filesRoot)
	RegisterSuperConsoleRoutes(s, superConsoleHandler)

	// Initialize the runtime engine with persisted state.
	rtRuntimeCfg := provideriface.LoadConfig()
	rtCfg := buildRuntimeConfig(cfg, rtRuntimeCfg, filesRoot)

	// Ensure the store directory exists.
	if err := os.MkdirAll(storeDir(rtCfg.StorePath), 0o755); err != nil {
		log.Fatalf("autoputer: create store directory: %v", err)
	}

	log.Printf("autoputer: startup phase=dolt-maintenance status=starting")
	if err := store.MaybeRunDoltGC(filepath.Dir(rtCfg.StorePath), rtCfg.StorePath); err != nil {
		log.Printf("autoputer: dolt gc maintenance: %v", err)
	}
	log.Printf("autoputer: startup phase=dolt-maintenance status=complete")

	log.Printf("autoputer: startup phase=runtime-store-open status=starting")
	db, err := store.Open(rtCfg.StorePath)
	if err != nil {
		log.Fatalf("autoputer: open runtime store: %v", err)
	}
	log.Printf("autoputer: startup phase=runtime-store-open status=complete")
	defer func() {
		_ = db.Close()
	}()

	bus := events.NewEventBus()
	browserHandler := browsercontrol.NewHandler(rtRuntimeCfg, db, bus)
	defer browserHandler.Close()
	desktopHandler := desktopstate.NewHandler(db, bus)
	contentService := content.NewService(db, bus)
	mediaHandler := mediastate.NewHandler(db, bus)

	// Resolve the runtime provider. VM guests route through the host-side
	// gateway so provider credentials and upstream adapter code stay out of
	// the guest image. A missing gateway falls back to the stub provider for
	// local diagnostics only.
	var rtProvider provideriface.Provider
	gatewayURL := os.Getenv("RUNTIME_GATEWAY_URL")
	if gatewayURL == "" {
		// Fallback: also check PROXY_VMCTL_URL which signals VM mode.
		gatewayURL = os.Getenv("PROXY_VMCTL_URL")
	}

	if gatewayURL != "" {
		gatewayToken := os.Getenv("RUNTIME_GATEWAY_TOKEN")
		if strings.TrimSpace(gatewayToken) == "" {
			log.Printf("autoputer: gateway provider configured without RUNTIME_GATEWAY_TOKEN; LLM calls will fail until the VM receives a autoputer credential")
		}
		bridge := gatewayruntime.New(gatewayURL, gatewayToken)
		bridge.SetRuntimeLLMConfig(rtCfg.LLMProvider, rtCfg.LLMModel, rtCfg.LLMReasoningEffort)
		rtProvider = bridge
		log.Printf("autoputer: using gateway provider (url=%s provider=%s model=%s reasoning=%s)",
			gatewayURL, rtCfg.LLMProvider, rtCfg.LLMModel, rtCfg.LLMReasoningEffort)
	} else {
		rtProvider = provider.NewStubProvider(rtCfg.ProviderTimeout)
		log.Printf("autoputer: using stub provider (no gateway configured)")
	}

	// Compose product owners into the runtime core explicitly; adapter options
	// remain limited to actor-lifecycle concerns.
	coreOpts := []agentcore.RuntimeOption{
		agentcore.WithDesktopStateOwner(desktopHandler),
		agentcore.WithContentService(contentService),
	}
	capsuleExecutor, capsuleConfigured, err := configuredCapsuleExecutor()
	if err != nil {
		log.Fatalf("autoputer: configure production capsule executor: %v", err)
	}
	if capsuleConfigured {
		coreOpts = append(coreOpts, agentcore.WithCapsuleExecutor(capsuleExecutor))
		log.Printf("autoputer: production networkless assignment capsule executor enabled")
	}
	var (
		replayAppender        *computerevent.ComputerEventAppender
		replayClient          *computerevent.HTTPClient
		replayCredentials     *selfdev.GuestCredentials
		replayComputerID      string
		replayBootstrapCtx    context.Context
		replayBootstrapCancel context.CancelFunc
	)
	if credentialPath := strings.TrimSpace(os.Getenv("CHOIR_COMPUTER_CREDENTIAL_FILE")); credentialPath != "" {
		computerID := strings.TrimSpace(os.Getenv("CHOIR_COMPUTER_ID"))
		realizationID := strings.TrimSpace(os.Getenv("CHOIR_REALIZATION_ID"))
		platformURL := strings.TrimSpace(os.Getenv("CHOIR_PLATFORM_URL"))
		restartHandoffPath := strings.TrimSpace(os.Getenv("CHOIR_RESTART_CREDENTIAL_HANDOFF"))
		recoveryHandoffPath := strings.TrimSpace(os.Getenv("CHOIR_REVOCATION_CREDENTIAL_HANDOFF"))
		bootstrapCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		var credentials *selfdev.GuestCredentials
		var restoredPath string
		var err error
		if _, recoveryErr := os.Stat(recoveryHandoffPath); recoveryErr == nil {
			restoredPath = recoveryHandoffPath
			credentials, err = selfdev.RestoreGuestCredentials(restoredPath, platformURL, computerID, realizationID)
		} else if !os.IsNotExist(recoveryErr) {
			err = recoveryErr
		} else if _, restartErr := os.Stat(restartHandoffPath); restartErr == nil {
			restoredPath = restartHandoffPath
			credentials, err = selfdev.RestoreGuestCredentials(restoredPath, platformURL, computerID, realizationID)
		} else if !os.IsNotExist(restartErr) {
			err = restartErr
		} else {
			var encodedEnvelope string
			encodedEnvelope, err = readComputerCredentialEnvelope(credentialPath)
			if err == nil {
				credentials, err = selfdev.ExchangeGuestCredential(bootstrapCtx, platformURL, encodedEnvelope, computerID, realizationID)
			}
		}
		if err == nil {
			err = credentials.ConfigureRecoveryHandoff(bootstrapCtx, recoveryHandoffPath)
		}
		if err == nil {
			err = credentials.CompleteCredentialExchange(bootstrapCtx)
		}
		if err == nil {
			if _, statErr := os.Stat(credentialPath); statErr == nil {
				err = eraseComputerCredentialEnvelope(credentialPath)
			} else if !os.IsNotExist(statErr) {
				err = statErr
			}
		}
		if err == nil && restoredPath != "" && restoredPath != recoveryHandoffPath {
			err = os.Remove(restoredPath)
		}
		if err == nil {
			_, err = credentials.RecoverPostRevocationCapability(bootstrapCtx)
		}
		if err != nil {
			cancel()
			log.Fatalf("autoputer: acquire or recover computer event credential: %v", err)
		}
		eventClient, err := computerevent.NewGuestHTTPClient(platformURL, credentials.Capability)
		if err != nil {
			cancel()
			log.Fatalf("autoputer: configure computer event client: %v", err)
		}
		canonicalHead, err := eventClient.Head(bootstrapCtx, computerID)
		if err != nil {
			cancel()
			log.Fatalf("autoputer: resolve canonical event head before keyring: %v", err)
		}
		privacyKeyPath := strings.TrimSpace(os.Getenv("CHOIR_PRIVACY_KEY_FILE"))
		privateCipher, err := computerevent.LoadGuestPrivateArtifactCipher(privacyKeyPath, computerID, canonicalHead == nil)
		if err != nil {
			cancel()
			log.Fatalf("autoputer: configure guest-owned private artifact cipher: %v", err)
		}
		appender, err := computerevent.NewComputerEventAppender(
			computerID, eventClient, db, eventClient,
			computerevent.EventHeadReceiptVerifier{Keys: credentials.KeyResolver()},
		)
		if err == nil {
			appender.SetPayloadResolver(eventClient, privateCipher)
			err = db.BindProjectionTape(computerID, appender)
		}
		if err == nil {
			// Defer the tape replay to a dedicated phase that runs AFTER the HTTP
			// surface is up, so the host wait-for-ready probe sees the replay
			// progress instead of connection refusal (B5/B6/B7/B9).
			replayAppender = appender
			replayClient = eventClient
			replayCredentials = credentials
			replayComputerID = computerID
			replayBootstrapCtx = bootstrapCtx
			replayBootstrapCancel = cancel
		} else {
			cancel()
			log.Fatalf("autoputer: acquire computer event authority: %v", err)
		}
		coreOpts = append(coreOpts, agentcore.WithComputerEventAppender(appender), agentcore.WithPrivateArtifactCipher(privateCipher))
		if credentials != nil {
			credentials.StartBackgroundRenewal(context.Background())
			coreOpts = append(coreOpts, guestControlOptions(credentials)...)
			log.Printf("autoputer: owner-recovery and self-development mode credentials wired; proposal still requires signed propose_only")
		}
		log.Printf("autoputer: computer event authority reconstructed")
	}
	if opt, ok, err := selfDevelopmentUpdaterOption(); err != nil {
		log.Fatalf("autoputer: configure self-development updater: %v", err)
	} else if ok {
		coreOpts = append(coreOpts, opt)
		log.Printf("autoputer: self-development updater root wired")
	}
	if opt, ok, err := selfDevelopmentRouteOption(); err != nil {
		log.Fatalf("autoputer: configure self-development route: %v", err)
	} else if ok {
		coreOpts = append(coreOpts, opt)
		log.Printf("autoputer: self-development computer-version route wired")
	}
	if opt, ok, err := selfDevelopmentVerifierOption(); err != nil {
		log.Fatalf("autoputer: configure self-development verifier: %v", err)
	} else if ok {
		coreOpts = append(coreOpts, opt)
		log.Printf("autoputer: self-development verifier authority wired; mode remains off")
	}
	var rtOpts []actorruntime.RuntimeOption

	// Mount the Dolt-backed trace observability store when enabled. The store
	// wraps the same Dolt *sql.DB that owns runtime/Texture state, so no extra
	// connection is opened. A schema-application or append failure degrades
	// gracefully: the runtime logs and continues without persistence (existing
	// event recording and bus publishing are unchanged).
	if rtCfg.TracePersistenceEnabled {
		traceStore, err := trace.NewDoltStore(db.DB())
		if err != nil {
			log.Printf("autoputer: trace persistence disabled (dolt store init failed, continuing without): %v", err)
		} else {
			rtOpts = append(rtOpts, actorruntime.WithTraceStore(traceStore))
			log.Printf("autoputer: trace persistence enabled (dolt-backed observability store mounted)")
		}
	}

	rt := actorruntime.New(rtCfg, db, bus, rtProvider, coreOpts, rtOpts...)

	// Initialize the file browser handler with autoputer files root. File
	// mutations publish owner-scoped product events after the filesystem write
	// succeeds so other devices can refresh Files without manual reload UI.
	fileHandler := NewFilesHandlerWithObserver(filesRoot, func(r *http.Request, event FileChangeEvent) {
		ownerID := strings.TrimSpace(r.Header.Get("X-Authenticated-User"))
		if ownerID == "" {
			return
		}
		_, err := rt.Runtime.EmitProductEvent(r.Context(), ownerID, desktopIDFromRequest(r), types.EventFileChanged, map[string]any{
			"operation":        event.Operation,
			"path":             event.Path,
			"parent_path":      event.ParentPath,
			"name":             event.Name,
			"entry_type":       event.EntryType,
			"size":             event.Size,
			"source_device_id": strings.TrimSpace(r.Header.Get("X-Choir-Device")),
		})
		if err != nil {
			log.Printf("autoputer: file change event failed: %v", err)
		}
	})
	RegisterFileRoutes(s, fileHandler)

	// Default-on: install the full per-profile tool registry. Set
	// RUNTIME_DISABLE_TOOLS=1 to opt out (for stub-only tests where no tools
	// should run). RUNTIME_ENABLE_TOOLS is still honored for back-compat but
	// is no longer required.
	toolsEnabled := os.Getenv("RUNTIME_DISABLE_TOOLS") == ""
	if toolsEnabled {
		toolCWD := os.Getenv("RUNTIME_TOOL_CWD")
		if strings.TrimSpace(toolCWD) == "" {
			toolCWD = filesRoot
		}
		if err := rt.Runtime.InstallDefaultAgentTools(toolCWD); err != nil {
			log.Fatalf("autoputer: install default agent tools: %v", err)
		}
	} else {
		log.Printf("autoputer: tool profiles DISABLED via RUNTIME_DISABLE_TOOLS (stub-only mode)")
	}

	textureHandler := textureowner.NewHandler(rt.Runtime)
	if err := rt.BindTextureOwner(textureHandler); err != nil {
		log.Fatalf("autoputer: bind Texture lifecycle owner: %v", err)
	}
	if toolsEnabled {
		if err := textureowner.RegisterTools(rt.Runtime.ToolRegistryForProfile(agentprofile.Texture), textureHandler); err != nil {
			log.Fatalf("autoputer: register Texture tools: %v", err)
		}
		for _, profile := range []string{
			agentprofile.Conductor,
			agentprofile.Super,
			agentprofile.CoSuper,
			agentprofile.Researcher,
			agentprofile.Texture,
			agentprofile.Processor,
			agentprofile.Reconciler,
		} {
			if err := coagentowner.RegisterSpawnTool(rt.Runtime.ToolRegistryForProfile(profile), rt.Runtime, textureHandler, agentprofile.PolicyFor(profile)); err != nil {
				log.Fatalf("autoputer: register coagent spawn tool for %s: %v", profile, err)
			}
		}
	}

	// Register canonical API routes (overrides default /health).
	runtimeHandler := agentcore.NewAPIHandler(rt.Runtime)
	apiHandler := apihandler.NewHandler(rt.Runtime.Store())
	apihandler.RegisterRoutes(s, runtimeHandler, textureHandler, apiHandler, browserHandler, desktopHandler, contentService, mediaHandler, rtRuntimeCfg.EnableTestAPIs)
	if toolsEnabled {
		superRegistry := rt.Runtime.ToolRegistryForProfile(agentprofile.Super)
		if err := apihandler.RegisterProductAPIRequestTool(s, superRegistry); err != nil {
			log.Fatalf("autoputer: register product API tool: %v", err)
		}
		log.Printf("autoputer: tool profiles enabled (conductor=%d super=%d researcher=%d texture=%d)",
			sizeOfRegistry(rt.Runtime.ToolRegistryForProfile(agentprofile.Conductor)),
			superRegistry.Size(),
			sizeOfRegistry(rt.Runtime.ToolRegistryForProfile(agentprofile.Researcher)),
			sizeOfRegistry(rt.Runtime.ToolRegistryForProfile(agentprofile.Texture)),
		)
	}

	// Readiness endpoint: probes Qdrant and Ollama, the two external
	// dependencies of the semantic-dedup path. Both degrade gracefully via
	// circuit breakers, so an unhealthy dependency reports "degraded" (200)
	// rather than "unhealthy" (503) — the autoputer can still serve requests
	// with content-hash dedup only. The result is cached for 5s to keep the
	// endpoint lightweight.
	readyAgg := health.NewAggregator("autoputer", 5*time.Second,
		health.HTTPChecker{NameStr: "qdrant", URL: strings.TrimRight(rtCfg.QdrantURL, "/") + "/healthz", Timeout: 2 * time.Second},
		health.HTTPChecker{NameStr: "ollama", URL: strings.TrimRight(rtCfg.OllamaURL, "/") + "/api/tags", Timeout: 2 * time.Second},
	)
	s.HandleFunc("/health/ready", health.ReadinessHandler("autoputer", readyAgg))
	RegisterComputerSurface(s, NewComputerSurfaceFromEnv())

	// Capture boot/reconcile log into a bounded ring before Start so guest
	// observability is servable through the product API, not only shell access.
	rt.Runtime.CaptureBootLog(512)

	// Start the runtime engine and supervisor. When a computer event authority is
	// present, the server starts FIRST with the replay-aware health gate so the
	// long tape replay is observable as 503 ReplayInProgress; the runtime starts
	// only after the replay reaches the canonical head (B5/B6/B9).
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	log.Printf("autoputer: orchestration topology (super=1, researchers=%d)", rtCfg.ResearcherCount)
	gate := &replayHealthGate{base: s.HealthHandler()}
	if replayAppender != nil {
		gate.mu.Lock()
		gate.pending = true
		gate.appender = replayAppender
		gate.mu.Unlock()
		s.SetHealthHandler(gate.ServeHTTP)
		go runReplayPhase(gate, replayAppender, replayClient, replayCredentials, replayComputerID, replayBootstrapCtx, replayBootstrapCancel, func() error {
			startPeriodicDoltGC(rtCfg.StorePath)
			return rt.Start(ctx)
		})
	} else {
		startPeriodicDoltGC(rtCfg.StorePath)
		if err := rt.Start(ctx); err != nil {
			log.Fatalf("autoputer: runtime startup refused: %v", err)
		}
	}

	s.Start()
}


// startPeriodicDoltGC starts the milestone-based Dolt garbage collector. It is
// deferred until AFTER the tape replay completes so GC churn never looks like a
// replay stall and never races the recovery working set (B11).
func startPeriodicDoltGC(storePath string) {
	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		store.StartPeriodicDoltGC(ctx, filepath.Dir(storePath), storePath, 5*time.Minute)
	}()
}

// runReplayPhase replays the computer event tape (30m resume quanta per B7),
// reconciles fenced pending lifecycle receipts (B9), then starts the runtime.
// A quantum expiry is a controlled end-of-session: the reconstruct loop has
// already flushed a durable checkpoint, so exiting lets systemd
// (Restart=on-failure) restart the guest and the next boot resumes from the
// committed head. Never CAS during replay (B8); the appender is read-only over
// the canonical tape.
func runReplayPhase(gate *replayHealthGate, appender *computerevent.ComputerEventAppender, client *computerevent.HTTPClient, credentials *selfdev.GuestCredentials, computerID string, bootstrapCtx context.Context, cancel context.CancelFunc, afterReplay func() error) {
	defer cancel()
	// B14 host-drive boundary: when RUNTIME_RECOVERY_REPLAY_ONLY is set the
	// reconstruct is a one-shot, deterministic projection materialization on
	// the host against the retained disk. It MUST NOT start the runtime,
	// reconcile lifecycle receipts, or append any semantic event; the process
	// exits after the target head is reached (or a quantum boundary) with the
	// workspace durably checkpointed. The guest boot afterwards sees
	// local==platform and takes over verification + route authority.
	replayOnly := strings.TrimSpace(os.Getenv("RUNTIME_RECOVERY_REPLAY_ONLY")) == "1"
	appender.SetReplayMode(true)
	err := appender.Reconstruct(bootstrapCtx, client)
	appender.SetReplayMode(false)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			// 30m resume quantum complete; the final durable checkpoint was flushed
			// inside the reconstruct loop before this error returned.
			log.Printf("autoputer: replay quantum complete; resuming on next boot (progress seq=%d)", appender.ReplaySnapshot().Sequence)
			os.Exit(1)
		}
		log.Fatalf("autoputer: reconstruct computer event authority: %v", err)
	}
	// Pre-genesis guard (PF-3 fix): a computer whose canonical chain is empty
	// (no genesis_imported ever CAS'd) must not serve run dispatch — every
	// semantic write would fail at Reduce(nil, ...) with "invalid genesis" and
	// run-dispatch would re-mint into an endless failed-run churn. Refuse
	// runtime start with a clear pre-genesis state; bootstrap-chain (product
	// path, owner authority) is the required repair. Recovery replay-only
	// drives exit before this point.
	if !replayOnly {
		if snap := appender.ReplaySnapshot(); snap.Sequence == 0 && snap.CommittedSequence == 0 {
			log.Fatalf("autoputer: computer is pre-genesis (no canonical genesis on the tape); refusing runtime start — bootstrap-chain required")
		}
	}
	if replayOnly {
		snap := appender.ReplaySnapshot()
		log.Printf("autoputer: recovery replay-only drive complete (seq=%d committed=%d); exiting without runtime start or reconciliation", snap.Sequence, snap.CommittedSequence)
		os.Exit(0)
	}
	gate.setPending(false)
	if err := reconcilePendingLifecycleReceipts(appender, credentials, computerID, bootstrapCtx); err != nil {
		log.Fatalf("autoputer: reconcile pending lifecycle receipts: %v", err)
	}
	if afterReplay != nil {
		if err := afterReplay(); err != nil {
			log.Fatalf("autoputer: runtime startup refused: %v", err)
		}
	}
	log.Printf("autoputer: computer event authority reconstructed (replay complete)")
}

// reconcilePendingLifecycleReceipts appends lifecycle-observed events for any
// pending platform lifecycle receipts (start/stop/restart/credential events).
// It runs ONLY after the replay reached the canonical head so the finish is an
// intact witness chain (B9) and never re-enters the replay path.
func reconcilePendingLifecycleReceipts(appender *computerevent.ComputerEventAppender, credentials *selfdev.GuestCredentials, computerID string, ctx context.Context) error {
	if appender == nil || credentials == nil {
		return nil
	}
	for _, lifecycleReceipt := range credentials.PendingLifecycleReceipts() {
		payload, payloadErr := lifecycleReceipt.CanonicalBytes()
		computerField, _ := lifecycleReceipt.KindFields["computer_id"].(string)
		actionField, _ := lifecycleReceipt.KindFields["action"].(string)
		if payloadErr != nil || lifecycleReceipt.ReceiptKind != "LifecycleReceipt" || lifecycleReceipt.Verify(credentials.KeyResolver()) != nil ||
			computerField != computerID || (actionField != "start" && actionField != "stop" && actionField != "restart" && actionField != "credential_envelope_consumed") {
			return fmt.Errorf("autoputer: pending lifecycle receipt binding refused")
		}
		eventID, eventErr := computerevent.NewEventID()
		if eventErr != nil {
			return eventErr
		}
		eventKind := computerevent.EventLifecycleObserved
		authorityRef := "platform-control:lifecycle"
		if actionField == "credential_envelope_consumed" {
			eventKind = computerevent.EventKeyRevoked
			authorityRef = "platform-control:credential-revocation"
		}
		event := computerevent.Event{
			SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: computerID,
			EventKind: eventKind, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
			IdempotencyKey: "lifecycle-observed:" + lifecycleReceipt.ReceiptID,
			ActorProfile:   agentprofile.Super, AuthorityRef: authorityRef,
			PrivacyClass: "public", ReducerVersion: computerevent.ReducerVersionV1,
		}
		if _, _, appendErr := appender.AppendNewPayload(ctx, event, computerevent.TransitionInput{}, payload, "application/vnd.choir.lifecycle-receipt+json", "public"); appendErr != nil {
			return appendErr
		}
		if actionField == "credential_envelope_consumed" {
			if transitionErr := credentials.CompletePostRevocation(lifecycleReceipt.ReceiptID); transitionErr != nil {
				return transitionErr
			}
		} else if acknowledgeErr := credentials.AcknowledgePendingLifecycleReceipt(lifecycleReceipt.ReceiptID); acknowledgeErr != nil {
			return acknowledgeErr
		}
	}
	return nil
}

// storeDir extracts the directory portion of a file path.
func storeDir(path string) string {
	if path == "" {
		return "/tmp/go-choir-m3"
	}
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i]
		}
	}
	return "."
}

func buildRuntimeConfig(cfg Config, rtRuntimeCfg provideriface.Config, filesRoot string) provideriface.Config {
	rtCfg := provideriface.Config{
		ComputerID:                      cfg.ComputerID,
		StorePath:                       cfg.StorePath,
		PromptRoot:                      rtRuntimeCfg.PromptRoot,
		SkillsRoot:                      rtRuntimeCfg.SkillsRoot,
		ProviderTimeout:                 rtRuntimeCfg.ProviderTimeout,
		SupervisionInterval:             rtRuntimeCfg.SupervisionInterval,
		ResearcherCount:                 rtRuntimeCfg.ResearcherCount,
		TextureWakeDebounce:             rtRuntimeCfg.TextureWakeDebounce,
		TextureActorParkIdle:            rtRuntimeCfg.TextureActorParkIdle,
		VmctlURL:                        rtRuntimeCfg.VmctlURL,
		MaildURL:                        rtRuntimeCfg.MaildURL,
		WirePublishURL:                  rtRuntimeCfg.WirePublishURL,
		CorpusdURL:                      rtRuntimeCfg.CorpusdURL,
		QdrantURL:                       rtRuntimeCfg.QdrantURL,
		OllamaURL:                       rtRuntimeCfg.OllamaURL,
		OllamaEmbeddingModel:            rtRuntimeCfg.OllamaEmbeddingModel,
		QdrantDedupThreshold:            rtRuntimeCfg.QdrantDedupThreshold,
		LLMProvider:                     rtRuntimeCfg.LLMProvider,
		LLMModel:                        rtRuntimeCfg.LLMModel,
		LLMReasoningEffort:              rtRuntimeCfg.LLMReasoningEffort,
		ModelPolicyPath:                 rtRuntimeCfg.ModelPolicyPath,
		ObscuraPath:                     rtRuntimeCfg.ObscuraPath,
		ObscuraCDPScreenshots:           rtRuntimeCfg.ObscuraCDPScreenshots,
		EnableTestAPIs:                  rtRuntimeCfg.EnableTestAPIs,
		RunMemoryContextThresholdTokens: rtRuntimeCfg.RunMemoryContextThresholdTokens,
		RunMemoryKeepRecentTokens:       rtRuntimeCfg.RunMemoryKeepRecentTokens,
		TracePersistenceEnabled:         rtRuntimeCfg.TracePersistenceEnabled,
	}
	if rtCfg.StorePath == "" {
		rtCfg.StorePath = provideriface.DefaultStorePath
	}
	if strings.TrimSpace(rtCfg.ModelPolicyPath) == "" {
		rtCfg.ModelPolicyPath = provideriface.DefaultModelPolicyPath(filesRoot)
	}
	return rtCfg
}

func readComputerCredentialEnvelope(path string) (string, error) {
	return readComputerCredentialEnvelopeOwned(path, 0)
}

func readComputerCredentialEnvelopeOwned(path string, expectedUID uint32) (string, error) {
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("credential path must be absolute")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != expectedUID || !info.Mode().IsRegular() || info.Mode().Perm() != 0o400 || info.Size() <= 0 || info.Size() > 128<<10 {
		return "", fmt.Errorf("credential file must be root-owned mode-0400 regular input")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	encoded := strings.TrimSpace(string(raw))
	for index := range raw {
		raw[index] = 0
	}
	if encoded == "" {
		return "", fmt.Errorf("credential file is empty")
	}
	return encoded, nil
}

func eraseComputerCredentialEnvelope(path string) error {
	return eraseComputerCredentialEnvelopeOwned(path, 0)
}

func eraseComputerCredentialEnvelopeOwned(path string, expectedUID uint32) error {
	if _, err := readComputerCredentialEnvelopeOwned(path, expectedUID); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("erase consumed credential file: %w", err)
	}
	return nil
}

func desktopIDFromRequest(r *http.Request) string {
	if r == nil {
		return types.PrimaryDesktopID
	}
	if desktopID := strings.TrimSpace(r.URL.Query().Get("desktop_id")); desktopID != "" {
		return desktopID
	}
	if desktopID := strings.TrimSpace(r.Header.Get("X-Choir-Desktop")); desktopID != "" {
		return desktopID
	}
	return types.PrimaryDesktopID
}

func sizeOfRegistry(registry *toolregistry.ToolRegistry) int {
	if registry == nil {
		return 0
	}
	return registry.Size()
}

const defaultSelfDevelopmentUpdaterSocket = "/run/choir/updater.sock"

// guestControlOptions mounts owner-recovery publication and signed mode reads
// on the same guest credential. Mode remains platform-controlled; this does
// not set mode, arm the outbox, or start the materializer.
func guestControlOptions(credentials *selfdev.GuestCredentials) []agentcore.RuntimeOption {
	if credentials == nil {
		return nil
	}
	return []agentcore.RuntimeOption{
		agentcore.WithOwnerRecoveryControl(credentials),
		agentcore.WithSelfDevelopmentControl(credentials),
	}
}

// selfDevelopmentUpdaterOption wires the boot and checkpoint serving-join
// repair to the same CHOIR_UPDATER_ROOT that ComputerSurface serves. A missing
// current/frontend remains fail-closed until the trusted immutable baseline is
// imported through the permissioned updater contract.
func selfDevelopmentUpdaterOption() (agentcore.RuntimeOption, bool, error) {
	root := strings.TrimSpace(os.Getenv("CHOIR_UPDATER_ROOT"))
	if root == "" {
		return nil, false, nil
	}
	socket := strings.TrimSpace(os.Getenv("CHOIR_UPDATER_SOCKET"))
	if socket == "" {
		socket = defaultSelfDevelopmentUpdaterSocket
	}
	client, err := updater.NewClient(socket)
	if err != nil {
		return nil, false, err
	}
	return agentcore.WithSelfDevelopmentUpdater(
		client,
		root,
		strings.TrimSpace(os.Getenv("CHOIR_COMPUTER_ID")),
		strings.TrimSpace(os.Getenv("CHOIR_REALIZATION_ID")),
	), true, nil
}

// selfDevelopmentRouteOption mounts signed computer-version route reads for
// kernel-capability probes. Mode is not set.
func selfDevelopmentRouteOption() (agentcore.RuntimeOption, bool, error) {
	baseURL := strings.TrimSpace(os.Getenv("RUNTIME_VMCTL_URL"))
	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv("PROXY_VMCTL_URL"))
	}
	ownerID := strings.TrimSpace(os.Getenv("CHOIR_OWNER_ID"))
	if baseURL == "" || ownerID == "" {
		return nil, false, nil
	}
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		return nil, false, fmt.Errorf("self-development route: absolute http(s) vmctl URL is required")
	}
	desktopID := strings.TrimSpace(os.Getenv("CHOIR_DESKTOP_ID"))
	if desktopID == "" {
		desktopID = "primary"
	}
	return agentcore.WithSelfDevelopmentRoute(vmctl.NewClient(baseURL), ownerID, desktopID), true, nil
}

// selfDevelopmentVerifierOption mounts verifier-control certificate signing.
// The materializer still no-ops unless updater, verifier, control, and route
// are all present and an authorized operation exists. Mode is not set.
func selfDevelopmentVerifierOption() (agentcore.RuntimeOption, bool, error) {
	socket := strings.TrimSpace(os.Getenv("CHOIR_VERIFIER_AUTHORITY_SOCKET"))
	if socket == "" {
		return nil, false, nil
	}
	client, err := receiptsigner.NewClient(socket, receiptsigner.ModeVerifier)
	if err != nil {
		return nil, false, fmt.Errorf("self-development verifier: %w", err)
	}
	return agentcore.WithSelfDevelopmentVerifier(client), true, nil
}
