package runtime

import (
	"encoding/json"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/provider"
	"log"
	"net/http"
	"strings"
)

// texturePromptEvalKind marks conductor runs created by the overlay-pinned
// Texture prompt eval so model sweeps can be distinguished from organic
// prompt-bar traffic.
const texturePromptEvalKind = "texture_prompt"

type texturePromptEvalStartRequest struct {
	Text                 string `json:"text"`
	ModelPolicyOverlayID string `json:"model_policy_overlay_id"`
	Title                string `json:"title,omitempty"`
}

type texturePromptEvalStartResponse struct {
	SubmissionID         string `json:"submission_id"`
	DocID                string `json:"doc_id,omitempty"`
	ModelPolicyOverlayID string `json:"model_policy_overlay_id"`
	Provider             string `json:"provider,omitempty"`
	Model                string `json:"model,omitempty"`
	ReasoningEffort      string `json:"reasoning_effort,omitempty"`
	StatusURL            string `json:"status_url"`
	CreatedAt            string `json:"created_at"`
}

// HandleTexturePromptEval handles POST /api/evals/texture-prompt. It drives the
// same conductor -> Texture route as /api/prompt-bar but pins a model-policy
// overlay onto the trajectory so the prompt-bar Texture flow can be swept across
// model arms. The overlay covers the Texture run (via durable revision metadata)
// and any researcher it spawns (via coagent overlay inheritance). Verification
// reuses the public submission/trace/texture product endpoints.
func (h *APIHandler) HandleTexturePromptEval(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}

	var req texturePromptEvalStartRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid texture prompt eval request"})
		return
	}
	text := strings.TrimSpace(req.Text)
	if text == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "text is required"})
		return
	}
	overlayID := strings.TrimSpace(req.ModelPolicyOverlayID)
	if overlayID == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "model_policy_overlay_id is required"})
		return
	}

	metadata := map[string]any{
		runMetadataAgentProfile:       agentprofile.Conductor,
		runMetadataAgentRole:          agentprofile.Conductor,
		runMetadataLLMPolicyOverlayID: overlayID,
		"input_source":                "prompt_bar",
		"requested_app":               agentprofile.Texture,
		"seed_prompt":                 text,
		"initial_document_title":      provider.InitialTextureTitle(text, strings.TrimSpace(req.Title)),
		"submission_surface":          "prompt_bar",
		"request_source":              "texture_prompt_eval",
		"eval_kind":                   texturePromptEvalKind,
	}
	if ownerEmail := authenticatedUserEmail(r); ownerEmail != "" {
		metadata[runMetadataOwnerEmail] = ownerEmail
	}
	if desktopID := requestDesktopID(r); desktopID != "" {
		metadata[runMetadataDesktopID] = desktopID
	}

	// Resolve the overlay against the Texture role up front so an unknown,
	// expired, or invalid arm fails the request instead of silently falling back.
	resolveMeta := cloneMetadata(metadata)
	resolveMeta[runMetadataAgentProfile] = agentprofile.Texture
	resolveMeta[runMetadataAgentRole] = agentprofile.Texture
	resolved := h.rt.ensureResolvedLLMMetadata(r.Context(), ownerID, resolveMeta)
	if errText := metadataStringValue(resolved, runMetadataLLMPolicyError); errText != "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "model policy overlay did not resolve: " + errText})
		return
	}
	if metadataStringValue(resolved, runMetadataLLMProvider) == "" || metadataStringValue(resolved, runMetadataLLMModel) == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "model policy overlay did not resolve provider and model"})
		return
	}

	decision := conductorDecision{
		Action: "open_app",
		App:    agentprofile.Texture,
		Title:  provider.InitialTextureTitle(text, strings.TrimSpace(req.Title)),
	}
	rec, err := h.rt.completePromptBarDecisionRun(r.Context(), text, ownerID, metadata, decision)
	if err != nil {
		log.Printf("runtime api: start texture prompt eval: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to start texture prompt eval"})
		return
	}
	routeDecision, routeErr := h.rt.ensureConductorTextureRoute(r.Context(), rec, text, "")
	if routeErr != nil {
		log.Printf("runtime api: materialize texture prompt eval route: %v", routeErr)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to prepare texture prompt eval"})
		return
	}

	writeAPIJSON(w, http.StatusAccepted, texturePromptEvalStartResponse{
		SubmissionID:         rec.RunID,
		DocID:                routeDecision.DocID,
		ModelPolicyOverlayID: overlayID,
		Provider:             metadataStringValue(resolved, runMetadataLLMProvider),
		Model:                metadataStringValue(resolved, runMetadataLLMModel),
		ReasoningEffort:      metadataStringValue(resolved, runMetadataLLMReasoningEffort),
		StatusURL:            "/api/prompt-bar/submissions/" + rec.RunID,
		CreatedAt:            rec.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
	})
}
