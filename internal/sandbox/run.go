package sandbox

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/yusefmosiah/go-choir/internal/actorruntime"
	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/apihandler"
	"github.com/yusefmosiah/go-choir/internal/browsercontrol"
	"github.com/yusefmosiah/go-choir/internal/capsule"
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

// RunZotSession runs the sandbox binary's zot-session process mode.
func RunZotSession(stdin io.Reader, stdout, stderr io.Writer) int {
	return zot.RunSession(zot.SessionConfig{
		SessionID: os.Getenv("ZOT_SESSION_ID"),
		RootDir:   os.Getenv("ZOT_ROOT_DIR"),
		UserID:    os.Getenv("ZOT_USER_ID"),
	}, stdin, stdout, stderr)
}

// Run starts the sandbox service.
func Run() {
	cfg := LoadConfig()

	s := server.NewServer("sandbox", cfg.Port)

	// Initialize the placeholder shell handlers.
	h := NewHandler(cfg.SandboxID)
	RegisterRoutes(s, h)

	filesRoot := provideriface.ResolveFilesRoot(os.Getenv("SANDBOX_FILES_ROOT"))
	log.Printf("sandbox: startup phase=source-workspace-bootstrap status=starting")
	if _, err := BootstrapSourceWorkspace(filesRoot, SourceWorkspaceOptions{
		ComputerID: cfg.SandboxID,
	}); err != nil {
		log.Fatalf("sandbox: bootstrap source workspace: %v", err)
	}
	log.Printf("sandbox: startup phase=source-workspace-bootstrap status=complete")

	// Initialize the singleton Super Console PTY handler. The PTY process is
	// zot, not an interactive shell.
	superConsoleHandler := NewSuperConsoleHandler(filesRoot)
	RegisterSuperConsoleRoutes(s, superConsoleHandler)

	// Initialize the runtime engine with persisted state.
	rtRuntimeCfg := provideriface.LoadConfig()
	rtCfg := buildRuntimeConfig(cfg, rtRuntimeCfg, filesRoot)

	// Ensure the store directory exists.
	if err := os.MkdirAll(storeDir(rtCfg.StorePath), 0o755); err != nil {
		log.Fatalf("sandbox: create store directory: %v", err)
	}

	log.Printf("sandbox: startup phase=dolt-maintenance status=starting")
	if err := store.MaybeRunDoltGC(filepath.Dir(rtCfg.StorePath), rtCfg.StorePath); err != nil {
		log.Printf("sandbox: dolt gc maintenance: %v", err)
	}
	log.Printf("sandbox: startup phase=dolt-maintenance status=complete")

	log.Printf("sandbox: startup phase=runtime-store-open status=starting")
	db, err := store.Open(rtCfg.StorePath)
	if err != nil {
		log.Fatalf("sandbox: open runtime store: %v", err)
	}
	log.Printf("sandbox: startup phase=runtime-store-open status=complete")
	defer func() {
		_ = db.Close()
	}()

	// Start periodic DoltDB garbage collection to reclaim disk space between
	// sandbox restarts (milestone-based, idempotent).
	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		store.StartPeriodicDoltGC(ctx, filepath.Dir(rtCfg.StorePath), rtCfg.StorePath, 5*time.Minute)
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
			log.Printf("sandbox: gateway provider configured without RUNTIME_GATEWAY_TOKEN; LLM calls will fail until the VM receives a sandbox credential")
		}
		bridge := gatewayruntime.New(gatewayURL, gatewayToken)
		bridge.SetRuntimeLLMConfig(rtCfg.LLMProvider, rtCfg.LLMModel, rtCfg.LLMReasoningEffort)
		rtProvider = bridge
		log.Printf("sandbox: using gateway provider (url=%s provider=%s model=%s reasoning=%s)",
			gatewayURL, rtCfg.LLMProvider, rtCfg.LLMModel, rtCfg.LLMReasoningEffort)
	} else {
		rtProvider = provider.NewStubProvider(rtCfg.ProviderTimeout)
		log.Printf("sandbox: using stub provider (no gateway configured)")
	}

	// Compose product owners into the runtime core explicitly; adapter options
	// remain limited to actor-lifecycle concerns.
	coreOpts := []agentcore.RuntimeOption{
		agentcore.WithDesktopStateOwner(desktopHandler),
		agentcore.WithContentService(contentService),
	}
	if updaterRoot := strings.TrimSpace(os.Getenv("CHOIR_UPDATER_ROOT")); updaterRoot != "" {
		updaterClient, err := updater.NewClient("/run/choir/updater.sock")
		if err != nil {
			log.Fatalf("sandbox: configure self-development updater: %v", err)
		}
		coreOpts = append(coreOpts, agentcore.WithSelfDevelopmentUpdater(updaterClient, updaterRoot, os.Getenv("CHOIR_COMPUTER_ID"), os.Getenv("CHOIR_REALIZATION_ID")))
	}
	if brokerPath := strings.TrimSpace(os.Getenv("CHOIR_CAPSULE_BROKER_PATH")); brokerPath != "" {
		stateDir := strings.TrimSpace(os.Getenv("CHOIR_CAPSULE_STATE_DIR"))
		lowerRoot := strings.TrimSpace(os.Getenv("CHOIR_CAPSULE_LOWER_ROOT"))
		sourceRoot := strings.TrimSpace(os.Getenv("CHOIR_CAPSULE_SOURCE_ROOT"))
		executor := capsule.NewExecutorWithSource(stateDir, lowerRoot, sourceRoot, brokerPath, 6*1024*1024*1024)
		coreOpts = append(coreOpts, agentcore.WithCapsuleExecutor(executor))
		log.Printf("sandbox: guest-local capsule authority configured")
	}
	if credentialPath := strings.TrimSpace(os.Getenv("CHOIR_COMPUTER_CREDENTIAL_FILE")); credentialPath != "" {
		encodedEnvelope, err := consumeComputerCredentialEnvelope(credentialPath)
		if err != nil {
			log.Fatalf("sandbox: consume computer event credential file: %v", err)
		}
		computerID := strings.TrimSpace(os.Getenv("CHOIR_COMPUTER_ID"))
		realizationID := strings.TrimSpace(os.Getenv("CHOIR_REALIZATION_ID"))
		platformURL := strings.TrimSpace(os.Getenv("CHOIR_PLATFORM_URL"))
		bootstrapCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		credentials, err := selfdev.ExchangeGuestCredential(bootstrapCtx, platformURL, encodedEnvelope, computerID, realizationID)
		if err != nil {
			cancel()
			log.Fatalf("sandbox: exchange computer event credential: %v", err)
		}
		eventClient, err := computerevent.NewGuestHTTPClient(platformURL, credentials.Capability)
		if err != nil {
			cancel()
			log.Fatalf("sandbox: configure computer event client: %v", err)
		}
		privacyRoot := strings.TrimSpace(os.Getenv("CHOIR_PRIVATE_ARTIFACT_KEYRING"))
		keyring, err := computerevent.NewFilePrivacyKeyring(privacyRoot)
		if err != nil {
			cancel()
			log.Fatalf("sandbox: configure private artifact keyring: %v", err)
		}
		privateCipher, err := computerevent.NewPrivateArtifactCipher(keyring)
		if err != nil {
			cancel()
			log.Fatalf("sandbox: configure private artifact cipher: %v", err)
		}
		appender, err := computerevent.NewComputerEventAppender(
			computerID, eventClient, db, eventClient,
			computerevent.EventHeadReceiptVerifier{Keys: credentials.KeyResolver()},
		)
		if err == nil {
			err = appender.Reconstruct(bootstrapCtx, eventClient)
		}
		cancel()
		if err != nil {
			log.Fatalf("sandbox: reconstruct computer event authority: %v", err)
		}
		coreOpts = append(coreOpts, agentcore.WithComputerEventAppender(appender), agentcore.WithPrivateArtifactCipher(privateCipher), agentcore.WithSelfDevelopmentControl(credentials))
		if vmctlURL := strings.TrimSpace(os.Getenv("RUNTIME_VMCTL_URL")); vmctlURL != "" {
			coreOpts = append(coreOpts, agentcore.WithSelfDevelopmentRoute(vmctl.NewClient(vmctlURL), os.Getenv("CHOIR_OWNER_ID"), os.Getenv("CHOIR_DESKTOP_ID")))
		}
		log.Printf("sandbox: computer event authority reconstructed")
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
			log.Printf("sandbox: trace persistence disabled (dolt store init failed, continuing without): %v", err)
		} else {
			rtOpts = append(rtOpts, actorruntime.WithTraceStore(traceStore))
			log.Printf("sandbox: trace persistence enabled (dolt-backed observability store mounted)")
		}
	}

	rt := actorruntime.New(rtCfg, db, bus, rtProvider, coreOpts, rtOpts...)

	// Initialize the file browser handler with sandbox files root. File
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
			log.Printf("sandbox: file change event failed: %v", err)
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
			log.Fatalf("sandbox: install default agent tools: %v", err)
		}
	} else {
		log.Printf("sandbox: tool profiles DISABLED via RUNTIME_DISABLE_TOOLS (stub-only mode)")
	}

	textureHandler := textureowner.NewHandler(rt.Runtime)
	if err := rt.BindTextureOwner(textureHandler); err != nil {
		log.Fatalf("sandbox: bind Texture lifecycle owner: %v", err)
	}
	if toolsEnabled {
		if err := textureowner.RegisterTools(rt.Runtime.ToolRegistryForProfile(agentprofile.Texture), textureHandler); err != nil {
			log.Fatalf("sandbox: register Texture tools: %v", err)
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
				log.Fatalf("sandbox: register coagent spawn tool for %s: %v", profile, err)
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
			log.Fatalf("sandbox: register product API tool: %v", err)
		}
		log.Printf("sandbox: tool profiles enabled (conductor=%d super=%d researcher=%d texture=%d)",
			sizeOfRegistry(rt.Runtime.ToolRegistryForProfile(agentprofile.Conductor)),
			superRegistry.Size(),
			sizeOfRegistry(rt.Runtime.ToolRegistryForProfile(agentprofile.Researcher)),
			sizeOfRegistry(rt.Runtime.ToolRegistryForProfile(agentprofile.Texture)),
		)
	}

	// Readiness endpoint: probes Qdrant and Ollama, the two external
	// dependencies of the semantic-dedup path. Both degrade gracefully via
	// circuit breakers, so an unhealthy dependency reports "degraded" (200)
	// rather than "unhealthy" (503) — the sandbox can still serve requests
	// with content-hash dedup only. The result is cached for 5s to keep the
	// endpoint lightweight.
	readyAgg := health.NewAggregator("sandbox", 5*time.Second,
		health.HTTPChecker{NameStr: "qdrant", URL: strings.TrimRight(rtCfg.QdrantURL, "/") + "/healthz", Timeout: 2 * time.Second},
		health.HTTPChecker{NameStr: "ollama", URL: strings.TrimRight(rtCfg.OllamaURL, "/") + "/api/tags", Timeout: 2 * time.Second},
	)
	s.HandleFunc("/health/ready", health.ReadinessHandler("sandbox", readyAgg))

	// Start the runtime engine and supervisor.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	log.Printf("sandbox: orchestration topology (super=1, researchers=%d)", rtCfg.ResearcherCount)
	rt.Start(ctx)

	s.Start()
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
		SandboxID:                       cfg.SandboxID,
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
		PromotionSourceRepo:             rtRuntimeCfg.PromotionSourceRepo,
		SourceLedgerRepo:                rtRuntimeCfg.SourceLedgerRepo,
		PromotionWorkspaceRoot:          rtRuntimeCfg.PromotionWorkspaceRoot,
		AppPromotionRuntimeBuildCommand: rtRuntimeCfg.AppPromotionRuntimeBuildCommand,
		AppPromotionRuntimeArtifactPath: rtRuntimeCfg.AppPromotionRuntimeArtifactPath,
		AppPromotionUIBuildCommand:      rtRuntimeCfg.AppPromotionUIBuildCommand,
		AppPromotionUIArtifactPath:      rtRuntimeCfg.AppPromotionUIArtifactPath,
		AppPromotionBuildTimeout:        rtRuntimeCfg.AppPromotionBuildTimeout,
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

func consumeComputerCredentialEnvelope(path string) (string, error) {
	return consumeComputerCredentialEnvelopeOwned(path, 0)
}

func consumeComputerCredentialEnvelopeOwned(path string, expectedUID uint32) (string, error) {
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
	if err := os.Remove(path); err != nil {
		return "", fmt.Errorf("erase consumed credential file: %w", err)
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
