package apihandler

import (
	"github.com/yusefmosiah/go-choir/internal/browsercontrol"
	"github.com/yusefmosiah/go-choir/internal/content"
	"github.com/yusefmosiah/go-choir/internal/desktopstate"
	"github.com/yusefmosiah/go-choir/internal/mediastate"
	"github.com/yusefmosiah/go-choir/internal/runtime"
	"github.com/yusefmosiah/go-choir/internal/server"
)

// RegisterRoutes registers the canonical sandbox API route table.
// The health handler overrides the default server health handler to report
// runtime readiness.
func RegisterRoutes(s *server.Server, h *runtime.APIHandler, api *Handler, browser *browsercontrol.Handler, desktop *desktopstate.Handler, contentOwner *content.Service, mediaOwner *mediastate.Handler, enableTestAPIs bool) {
	s.SetHealthHandler(h.HandleHealth)
	s.HandleFunc("/api/prompt-bar", h.HandlePromptBar)
	s.HandleFunc("/api/prompt-bar/submissions/", h.HandlePromptBarSubmission)
	s.HandleFunc("/api/agent/loops", h.HandleRunList)
	s.HandleFunc("/api/agent/cancel", h.HandleCancel)
	s.HandleFunc("/api/model-policy/", h.HandleModelPolicyRouter)
	s.HandleFunc("/api/costs", api.HandleCosts)
	s.HandleFunc("/api/podcast/subscriptions/refresh", h.HandlePodcastSubscriptionsRefresh)
	s.HandleFunc("/api/podcast/subscriptions", h.HandlePodcastSubscriptions)
	s.HandleFunc("/api/podcast/search", h.HandlePodcastSearch)
	s.HandleFunc("/api/content/items", contentOwner.HandleContentItemsRoot)
	s.HandleFunc("/api/content/", contentOwner.HandleContentRouter)
	s.HandleFunc("/api/ws", h.HandleLiveWS)
	s.HandleFunc("/api/browser/capabilities", browser.HandleBrowserCapabilities)
	s.HandleFunc("/api/browser/sessions", browser.HandleBrowserSessionsRoot)
	s.HandleFunc("/api/browser/sessions/", browser.HandleBrowserSessionRouter)
	s.HandleFunc("/api/desktop/state", desktop.HandleDesktopState)
	s.HandleFunc("/api/media/progress", mediaOwner.HandleMediaProgress)
	s.HandleFunc("/api/media/recents", mediaOwner.HandleMediaRecents)
	s.HandleFunc("/api/preferences/theme", mediaOwner.HandleThemePreference)
	s.HandleFunc("/api/computers/", h.HandleComputersRouter)
	s.HandleFunc("/api/app-change-packages", h.HandleAppChangePackagesRoot)
	s.HandleFunc("/api/app-change-packages/", h.HandleAppChangePackageDetail)
	s.HandleFunc("/api/adoptions", h.HandleAppAdoptionsRoot)
	s.HandleFunc("/api/adoptions/", h.HandleAppAdoptionDetail)
	s.HandleFunc("/api/candidate-package-intakes/", h.HandleCandidatePackageReviewSurfaceReadOnly)
	s.HandleFunc("/api/trajectories", h.HandleTrajectoriesRoot)
	s.HandleFunc("/api/trajectories/", h.HandleTrajectoryDetail)
	s.HandleFunc("/api/run-acceptances", h.HandleRunAcceptancesRoot)
	s.HandleFunc("/api/run-acceptances/synthesize", h.HandleRunAcceptanceSynthesize)
	s.HandleFunc("/api/run-acceptances/", h.HandleRunAcceptanceDetail)
	s.HandleFunc("/api/evals/texture-prompt", h.HandleTexturePromptEval)
	s.HandleFunc("/internal/runtime/app-change-packages", h.HandleInternalAppChangePackagesRoot)
	s.HandleFunc("/internal/runtime/app-change-packages/", h.HandleInternalAppChangePackageDetail)
	s.HandleFunc("/internal/runtime/channel-casts", h.HandleInternalChannelCast)
	s.HandleFunc("/internal/runtime/refresh", h.HandleInternalRuntimeRefresh)
	s.HandleFunc("/internal/runtime/runs", h.HandleInternalRunSubmission)
	s.HandleFunc("/internal/runtime/runs/", h.HandleInternalRuntimeRunRouter)
	s.HandleFunc("/internal/texture/documents/", h.HandleInternalTextureDocument)
	s.HandleFunc("/internal/texture/revisions/", h.HandleInternalTextureRevision)
	s.HandleFunc("/internal/texture/proposals", h.HandleInternalTextureProposalDelivery)
	if enableTestAPIs {
		s.HandleFunc("/api/prompts", h.HandlePromptList)
		s.HandleFunc("/api/prompts/", h.HandlePromptRole)
		s.HandleFunc("/api/test/texture/worker-update", h.HandleTestTextureWorkerUpdate)
	}

	// Texture document/revision/history/diff/blame APIs.
	// All routes are dispatched from a single prefix handler that inspects
	// the URL path and method to route to the correct handler. This avoids
	// ambiguity with Go's ServeMux prefix matching.
	s.HandleFunc("/api/texture/documents", h.HandleTextureDocumentsRoot)
	s.HandleFunc("/api/texture/", h.HandleTextureRouter)
}
