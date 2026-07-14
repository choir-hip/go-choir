package textureowner

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/modelpolicy"
	"github.com/yusefmosiah/go-choir/internal/provider"
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
func (h *Handler) HandleTexturePromptEval(w http.ResponseWriter, r *http.Request) {
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

	title := provider.InitialTextureTitle(text, strings.TrimSpace(req.Title))
	metadata := map[string]any{
		"agent_profile":                     agentprofile.Conductor,
		"agent_role":                        agentprofile.Conductor,
		modelpolicy.MetadataPolicyOverlayID: overlayID,
		"input_source":                      "prompt_bar",
		"requested_app":                     agentprofile.Texture,
		"seed_prompt":                       text,
		"initial_document_title":            title,
		"submission_surface":                "prompt_bar",
		"request_source":                    "texture_prompt_eval",
		"eval_kind":                         texturePromptEvalKind,
	}
	if ownerEmail := authenticatedUserEmail(r); ownerEmail != "" {
		metadata["owner_email"] = ownerEmail
	}
	if desktopID := requestDesktopID(r); desktopID != "" {
		metadata["desktop_id"] = desktopID
	}

	// Resolve the overlay against the Texture role up front so an unknown,
	// expired, or invalid arm fails the request instead of silently falling back.
	resolveMeta := clonePromptEvalMetadata(metadata)
	resolveMeta["agent_profile"] = agentprofile.Texture
	resolveMeta["agent_role"] = agentprofile.Texture
	resolved := h.ModelPolicy.EnrichMetadata(r.Context(), ownerID, agentprofile.Texture, resolveMeta)
	if errText := promptEvalMetadataString(resolved, modelpolicy.MetadataPolicyError); errText != "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "model policy overlay did not resolve: " + errText})
		return
	}
	if promptEvalMetadataString(resolved, modelpolicy.MetadataProvider) == "" || promptEvalMetadataString(resolved, modelpolicy.MetadataModel) == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "model policy overlay did not resolve provider and model"})
		return
	}

	rec, err := h.Core.CompletePromptBarDecision(r.Context(), text, ownerID, metadata, agentcore.PromptBarDecisionSpec{
		Action: "open_app",
		App:    agentprofile.Texture,
		Title:  title,
	})
	if err != nil {
		log.Printf("runtime api: start texture prompt eval: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to start texture prompt eval"})
		return
	}
	handoff, err := h.EnsureTextureHandoff(r.Context(), rec, HandoffRequest{
		Kind:          HandoffKindUserPrompt,
		CallerProfile: agentprofile.Conductor,
		Objective:     text,
		Title:         title,
	})
	if err != nil {
		log.Printf("runtime api: materialize texture prompt eval route: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to prepare texture prompt eval"})
		return
	}

	writeAPIJSON(w, http.StatusAccepted, texturePromptEvalStartResponse{
		SubmissionID:         rec.RunID,
		DocID:                handoff.DocID,
		ModelPolicyOverlayID: overlayID,
		Provider:             promptEvalMetadataString(resolved, modelpolicy.MetadataProvider),
		Model:                promptEvalMetadataString(resolved, modelpolicy.MetadataModel),
		ReasoningEffort:      promptEvalMetadataString(resolved, modelpolicy.MetadataReasoningEffort),
		StatusURL:            "/api/prompt-bar/submissions/" + rec.RunID,
		CreatedAt:            rec.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
	})
}

func clonePromptEvalMetadata(metadata map[string]any) map[string]any {
	cloned := make(map[string]any, len(metadata))
	for key, value := range metadata {
		cloned[key] = value
	}
	return cloned
}

func promptEvalMetadataString(metadata map[string]any, key string) string {
	value, _ := metadata[key].(string)
	return strings.TrimSpace(value)
}
