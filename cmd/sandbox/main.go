package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/gatewayruntime"
	"github.com/yusefmosiah/go-choir/internal/runtime"
	"github.com/yusefmosiah/go-choir/internal/sandbox"
	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/zot"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "zot-session" {
		os.Exit(zot.RunSession(zot.SessionConfig{
			SessionID: os.Getenv("ZOT_SESSION_ID"),
			RootDir:   os.Getenv("ZOT_ROOT_DIR"),
			UserID:    os.Getenv("ZOT_USER_ID"),
		}, os.Stdin, os.Stdout, os.Stderr))
	}

	cfg := sandbox.LoadConfig()

	s := server.NewServer("sandbox", cfg.Port)

	// Initialize the placeholder shell handlers.
	h := sandbox.NewHandler(cfg.SandboxID)
	sandbox.RegisterRoutes(s, h)

	filesRoot := sandbox.ResolveFilesRoot(os.Getenv("SANDBOX_FILES_ROOT"))

	// Initialize the singleton Super Console PTY handler. The PTY process is
	// zot, not an interactive shell.
	superConsoleHandler := sandbox.NewSuperConsoleHandler(filesRoot)
	sandbox.RegisterSuperConsoleRoutes(s, superConsoleHandler)

	// Initialize the runtime engine with persisted state.
	rtRuntimeCfg := runtime.LoadConfig()
	rtCfg := buildRuntimeConfig(cfg, rtRuntimeCfg, filesRoot)

	// Ensure the store directory exists.
	if err := os.MkdirAll(storeDir(rtCfg.StorePath), 0o755); err != nil {
		log.Fatalf("sandbox: create store directory: %v", err)
	}

	db, err := store.Open(rtCfg.StorePath)
	if err != nil {
		log.Fatalf("sandbox: open runtime store: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	bus := events.NewEventBus()

	// Resolve the runtime provider. VM guests route through the host-side
	// gateway so provider credentials and upstream adapter code stay out of
	// the guest image. A missing gateway falls back to the stub provider for
	// local diagnostics only.
	var rtProvider runtime.Provider
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
		rtProvider = runtime.NewStubProvider(rtCfg.ProviderTimeout)
		log.Printf("sandbox: using stub provider (no gateway configured)")
	}

	// Build runtime options based on configuration.
	var rtOpts []runtime.RuntimeOption

	rt := runtime.New(rtCfg, db, bus, rtProvider, rtOpts...)

	// Initialize the file browser handler with sandbox files root. File
	// mutations publish owner-scoped product events after the filesystem write
	// succeeds so other devices can refresh Files without manual reload UI.
	fileHandler := sandbox.NewFilesHandlerWithObserver(filesRoot, func(r *http.Request, event sandbox.FileChangeEvent) {
		ownerID := strings.TrimSpace(r.Header.Get("X-Authenticated-User"))
		if ownerID == "" {
			return
		}
		_, err := rt.EmitProductEvent(r.Context(), ownerID, desktopIDFromRequest(r), types.EventFileChanged, map[string]any{
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
	sandbox.RegisterFileRoutes(s, fileHandler)

	// Default-on: install the full per-profile tool registry. Set
	// RUNTIME_DISABLE_TOOLS=1 to opt out (for stub-only tests where no tools
	// should run). RUNTIME_ENABLE_TOOLS is still honored for back-compat but
	// is no longer required.
	if os.Getenv("RUNTIME_DISABLE_TOOLS") == "" {
		toolCWD := os.Getenv("RUNTIME_TOOL_CWD")
		if strings.TrimSpace(toolCWD) == "" {
			toolCWD = filesRoot
		}
		if err := rt.InstallDefaultAgentTools(toolCWD); err != nil {
			log.Fatalf("sandbox: install default agent tools: %v", err)
		}
		superTools := 0
		if registry := rt.ToolRegistryForProfile(runtime.AgentProfileSuper); registry != nil {
			superTools = registry.Size()
		}
		log.Printf("sandbox: tool profiles enabled (conductor=%d super=%d researcher=%d vtext=%d)",
			sizeOfRegistry(rt.ToolRegistryForProfile(runtime.AgentProfileConductor)),
			superTools,
			sizeOfRegistry(rt.ToolRegistryForProfile(runtime.AgentProfileResearcher)),
			sizeOfRegistry(rt.ToolRegistryForProfile(runtime.AgentProfileVText)),
		)
	} else {
		log.Printf("sandbox: tool profiles DISABLED via RUNTIME_DISABLE_TOOLS (stub-only mode)")
	}

	// Register runtime API routes (overrides default /health).
	apiHandler := runtime.NewAPIHandler(rt)
	runtime.RegisterRoutes(s, apiHandler)

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

func buildRuntimeConfig(cfg sandbox.Config, rtRuntimeCfg runtime.Config, filesRoot string) runtime.Config {
	rtCfg := runtime.Config{
		SandboxID:                       cfg.SandboxID,
		StorePath:                       cfg.StorePath,
		PromptRoot:                      rtRuntimeCfg.PromptRoot,
		SkillsRoot:                      rtRuntimeCfg.SkillsRoot,
		ProviderTimeout:                 rtRuntimeCfg.ProviderTimeout,
		SupervisionInterval:             rtRuntimeCfg.SupervisionInterval,
		ResearcherCount:                 rtRuntimeCfg.ResearcherCount,
		VTextWakeDebounce:               rtRuntimeCfg.VTextWakeDebounce,
		VmctlURL:                        rtRuntimeCfg.VmctlURL,
		MaildURL:                        rtRuntimeCfg.MaildURL,
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
	}
	if rtCfg.StorePath == "" {
		rtCfg.StorePath = runtime.DefaultStorePath
	}
	if strings.TrimSpace(rtCfg.ModelPolicyPath) == "" {
		rtCfg.ModelPolicyPath = runtime.DefaultModelPolicyPath(filesRoot)
	}
	return rtCfg
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

func sizeOfRegistry(registry *runtime.ToolRegistry) int {
	if registry == nil {
		return 0
	}
	return registry.Size()
}
