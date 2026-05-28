package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/mail"
	"strings"
	"time"
)

var (
	featureAdoptionWatchPollInterval = 5 * time.Second
	featureAdoptionWatchTimeout      = 20 * time.Minute
)

type featureAdoptionWatchRequest struct {
	AdoptionID string `json:"adoption_id"`
	ToEmail    string `json:"to_email"`
	Title      string `json:"title"`
	FeatureID  string `json:"feature_id,omitempty"`
	Link       string `json:"link,omitempty"`
}

type featureAdoptionWatchResponse struct {
	Status     string `json:"status"`
	AdoptionID string `json:"adoption_id"`
}

type featureAdoptionWatchRecord struct {
	AdoptionID string `json:"adoption_id"`
	PackageID  string `json:"package_id"`
	AppID      string `json:"app_id"`
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
}

type featureCompletionEmailResponse struct {
	Status            string `json:"status"`
	ProviderMessageID string `json:"provider_message_id,omitempty"`
}

func (h *Handler) HandleNotificationAPI(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api/notifications/watch-adoption-completion" {
		h.HandleFeatureAdoptionCompletionWatch(w, r)
		return
	}
	h.forwardMaildAuthenticated(w, r)
}

func (h *Handler) HandleFeatureAdoptionCompletionWatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	authResult, err := h.validateAccessJWT(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}
	var in featureAdoptionWatchRequest
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid adoption watch request"})
		return
	}
	in.AdoptionID = strings.TrimSpace(in.AdoptionID)
	if in.AdoptionID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "adoption_id is required"})
		return
	}
	in.ToEmail = strings.TrimSpace(in.ToEmail)
	if _, err := mail.ParseAddress(in.ToEmail); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "valid signup email is required"})
		return
	}
	in.Title = conciseNotificationField(in.Title, "Feature import", 120)
	in.FeatureID = conciseNotificationField(in.FeatureID, "", 120)
	in.Link = conciseNotificationLink(in.Link)

	sandboxURL, err := h.resolveSandboxURL(r.Context(), authResult.UserID, requestDesktopID(r))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to resolve user sandbox"})
		return
	}
	go h.watchFeatureAdoptionCompletion(authResult.UserID, sandboxURL, in)
	writeJSON(w, http.StatusAccepted, featureAdoptionWatchResponse{Status: "watching", AdoptionID: in.AdoptionID})
}

func conciseNotificationField(value, fallback string, max int) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if value == "" {
		value = fallback
	}
	if max > 0 && len(value) > max {
		value = strings.TrimSpace(value[:max])
	}
	return value
}

func conciseNotificationLink(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") {
		return "/?app=features"
	}
	if len(value) > 240 {
		return "/?app=features"
	}
	return value
}

func featureAdoptionTerminalStatus(status string) bool {
	switch status {
	case "verified", "owner_approved", "adopted", "rolled_back", "blocked":
		return true
	default:
		return false
	}
}

func (h *Handler) watchFeatureAdoptionCompletion(userID, sandboxURL string, in featureAdoptionWatchRequest) {
	ctx, cancel := context.WithTimeout(context.Background(), featureAdoptionWatchTimeout)
	defer cancel()
	ticker := time.NewTicker(featureAdoptionWatchPollInterval)
	defer ticker.Stop()
	for {
		rec, err := h.fetchFeatureAdoptionForWatch(ctx, sandboxURL, userID, in.AdoptionID)
		if err != nil {
			if ctx.Err() != nil {
				log.Printf("proxy: feature adoption watch timeout adoption=%s user=%s: %v", in.AdoptionID, userID, err)
				return
			}
			log.Printf("proxy: feature adoption watch poll adoption=%s user=%s: %v", in.AdoptionID, userID, err)
		} else if featureAdoptionTerminalStatus(rec.Status) {
			providerMessageID, err := h.sendFeatureAdoptionCompletionEmail(ctx, userID, in, rec)
			if err != nil {
				log.Printf("proxy: feature adoption completion email adoption=%s user=%s status=%s: %v", in.AdoptionID, userID, rec.Status, err)
			} else {
				log.Printf("proxy: feature adoption completion email sent adoption=%s user=%s status=%s provider_message_id=%s", in.AdoptionID, userID, rec.Status, providerMessageID)
			}
			return
		}
		select {
		case <-ctx.Done():
			log.Printf("proxy: feature adoption watch timeout adoption=%s user=%s", in.AdoptionID, userID)
			return
		case <-ticker.C:
		}
	}
}

func (h *Handler) fetchFeatureAdoptionForWatch(ctx context.Context, sandboxURL, userID, adoptionID string) (featureAdoptionWatchRecord, error) {
	target, err := joinBasePath(sandboxURL, "/api/adoptions/"+adoptionID)
	if err != nil {
		return featureAdoptionWatchRecord{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return featureAdoptionWatchRecord{}, err
	}
	req.Header.Set("X-Authenticated-User", userID)
	resp, err := h.sandboxHTTP.Do(req)
	if err != nil {
		return featureAdoptionWatchRecord{}, fmt.Errorf("call sandbox: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return featureAdoptionWatchRecord{}, fmt.Errorf("sandbox status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var rec featureAdoptionWatchRecord
	if err := json.NewDecoder(resp.Body).Decode(&rec); err != nil {
		return featureAdoptionWatchRecord{}, fmt.Errorf("decode sandbox adoption: %w", err)
	}
	if rec.AdoptionID == "" {
		rec.AdoptionID = adoptionID
	}
	return rec, nil
}

func (h *Handler) sendFeatureAdoptionCompletionEmail(ctx context.Context, userID string, in featureAdoptionWatchRequest, rec featureAdoptionWatchRecord) (string, error) {
	target, err := joinBasePath(h.cfg.MaildURL, "/api/notifications/completion-email")
	if err != nil {
		return "", err
	}
	title := in.Title
	if title == "" {
		title = conciseNotificationField(rec.AppID, "Feature import", 120)
	}
	payload, err := json.Marshal(map[string]string{
		"to_email":   in.ToEmail,
		"title":      title,
		"status":     rec.Status,
		"feature_id": firstNonEmptyString(in.FeatureID, rec.PackageID),
		"link":       in.Link,
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Authenticated-User", userID)
	req.Header.Set("X-Internal-Caller", "true")
	resp, err := h.maild.Do(req)
	if err != nil {
		return "", fmt.Errorf("call maild: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("maild status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var sent featureCompletionEmailResponse
	if err := json.NewDecoder(resp.Body).Decode(&sent); err != nil {
		return "", fmt.Errorf("decode maild completion response: %w", err)
	}
	return sent.ProviderMessageID, nil
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
