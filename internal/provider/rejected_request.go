package provider

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
)

var rejectedReasonSecrets = regexp.MustCompile(`(?i)(sk-[a-z0-9_-]{8,}|bearer\s+\S+|(api[_-]?key|key|token|secret)\s*[=:]\s*\S+|\?[^\s"]*)`)

// logRejectedRequest drains a non-2xx body and, for a request the provider
// rejected (4xx), logs the provider's stated reason to the gateway journal:
// redacted, truncated, never returned to the caller. Without it a rejected
// request leaves no cause anywhere
// (problems/inference-breaker-trips-on-client-errors-2026-10-10.md).
func logRejectedRequest(providerName string, resp *http.Response) {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<10))
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 400 || resp.StatusCode >= 500 {
		return
	}
	log.Printf("provider: %s rejected request status=%d reason=%q", providerName, resp.StatusCode, rejectedReason(body))
}

func rejectedReason(body []byte) string {
	var payload struct {
		Error   json.RawMessage `json:"error"`
		Message string          `json:"message"`
		Type    string          `json:"type"`
	}
	reason := ""
	if json.Unmarshal(body, &payload) == nil {
		var nested struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    any    `json:"code"`
		}
		var flat string
		switch {
		case json.Unmarshal(payload.Error, &nested) == nil && nested.Message != "":
			reason = strings.TrimSpace(nested.Type + " " + nested.Message)
		case json.Unmarshal(payload.Error, &flat) == nil && flat != "":
			reason = flat
		case payload.Message != "":
			reason = strings.TrimSpace(payload.Type + " " + payload.Message)
		}
	}
	if reason == "" {
		reason = "unparsed body"
	}
	reason = rejectedReasonSecrets.ReplaceAllString(reason, "[redacted]")
	if len(reason) > 240 {
		reason = reason[:240] + "…"
	}
	return reason
}
