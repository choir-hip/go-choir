package runtime

import "net/http"

func (h *APIHandler) HandleTextureSourceGapRepair(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	writeAPIJSON(w, http.StatusGone, apiError{Error: "legacy Texture source repair endpoint is retired; use structured Texture source_ref operations"})
}

func (h *APIHandler) HandleTextureSourceArtifactAttachment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	writeAPIJSON(w, http.StatusGone, apiError{Error: "legacy Texture source attachment endpoint is retired; attach sources through structured Texture source entities"})
}
