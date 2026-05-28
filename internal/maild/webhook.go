package maild

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/server"
)

const providerResend = "resend"
const webhookTimestampTolerance = 5 * time.Minute

// Handler serves maild HTTP routes.
type Handler struct {
	cfg    *Config
	store  *Store
	resend resendClient
}

type webhookResponse struct {
	Status  string `json:"status"`
	EventID string `json:"event_id,omitempty"`
}

type resendWebhookEvent struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Data struct {
		EmailID string `json:"email_id"`
	} `json:"data"`
}

// NewHandler creates a maild HTTP handler.
func NewHandler(cfg *Config, store *Store) *Handler {
	return &Handler{cfg: cfg, store: store, resend: newResendClient(cfg, http.DefaultClient)}
}

// RegisterRoutes registers maild routes.
func RegisterRoutes(s *server.Server, h *Handler) {
	s.SetHealthHandler(h.HandleHealth)
	s.HandleFunc("/api/email/resend/webhook", h.HandleResendWebhook)
	s.HandleFunc("/api/email/aliases", h.HandleAliases)
	s.HandleFunc("/api/email/drafts", h.HandleDrafts)
	s.HandleFunc("/api/email/drafts/", h.HandleDrafts)
	s.HandleFunc("/api/email/messages", h.HandleMessages)
	s.HandleFunc("/api/email/messages/", h.HandleMessages)
	s.HandleFunc("/api/notifications/completion-email", h.HandleCompletionEmail)
	s.HandleFunc("/api/notifications/email-risk-alert", h.HandleRiskAlert)
}

// HandleResendWebhook verifies and stores Resend webhook metadata.
func (h *Handler) HandleResendWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if strings.TrimSpace(h.cfg.WebhookSecret) == "" {
		writeJSON(w, http.StatusServiceUnavailable, webhookResponse{Status: "webhook_secret_not_configured"})
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, h.cfg.WebhookMaxBytes+1))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, webhookResponse{Status: "read_error"})
		return
	}
	if int64(len(body)) > h.cfg.WebhookMaxBytes {
		writeJSON(w, http.StatusRequestEntityTooLarge, webhookResponse{Status: "payload_too_large"})
		return
	}
	if err := verifyWebhook(body, r.Header, h.cfg.WebhookSecret); err != nil {
		writeJSON(w, http.StatusBadRequest, webhookResponse{Status: "invalid_signature"})
		return
	}

	var event resendWebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		writeJSON(w, http.StatusBadRequest, webhookResponse{Status: "invalid_json"})
		return
	}
	providerEventID := strings.TrimSpace(event.ID)
	if providerEventID == "" {
		providerEventID = strings.TrimSpace(r.Header.Get("svix-id"))
	}
	if providerEventID == "" {
		writeJSON(w, http.StatusBadRequest, webhookResponse{Status: "missing_event_id"})
		return
	}
	eventType := strings.TrimSpace(event.Type)
	if eventType == "" {
		writeJSON(w, http.StatusBadRequest, webhookResponse{Status: "missing_event_type"})
		return
	}

	created, err := h.store.RecordWebhookEvent(r.Context(), WebhookEvent{
		ID:                eventRowID(providerEventID),
		Provider:          providerResend,
		ProviderEventID:   providerEventID,
		ProviderMessageID: strings.TrimSpace(event.Data.EmailID),
		EventType:         eventType,
		RawPayload:        string(body),
		ReceivedAt:        time.Now().UTC(),
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, webhookResponse{Status: "store_error"})
		return
	}
	if !created {
		if shouldIngestEmail(eventType, event.Data.EmailID) {
			if retried, err := h.retryMissingReceivedEmail(r.Context(), providerEventID, strings.TrimSpace(event.Data.EmailID)); err != nil {
				log.Printf("maild: retry duplicate ingest event=%s email=%s: %v", providerEventID, event.Data.EmailID, err)
				if shouldRetryIngest(err) {
					writeJSON(w, http.StatusServiceUnavailable, webhookResponse{Status: "duplicate_ingest_retry_requested", EventID: providerEventID})
					return
				}
				writeJSON(w, http.StatusAccepted, webhookResponse{Status: "duplicate_ingest_failed", EventID: providerEventID})
				return
			} else if retried {
				writeJSON(w, http.StatusAccepted, webhookResponse{Status: "duplicate_ingested", EventID: providerEventID})
				return
			}
		}
		writeJSON(w, http.StatusOK, webhookResponse{Status: "duplicate", EventID: providerEventID})
		return
	}
	status := "accepted"
	if shouldIngestEmail(eventType, event.Data.EmailID) {
		if err := h.ingestReceivedEmail(r.Context(), providerEventID, strings.TrimSpace(event.Data.EmailID)); err != nil {
			log.Printf("maild: ingest received email event=%s email=%s: %v", providerEventID, event.Data.EmailID, err)
			if shouldRetryIngest(err) {
				writeJSON(w, http.StatusServiceUnavailable, webhookResponse{Status: "ingest_retry_requested", EventID: providerEventID})
				return
			}
			status = "accepted_ingest_failed"
		}
	}
	writeJSON(w, http.StatusAccepted, webhookResponse{Status: status, EventID: providerEventID})
}

func shouldIngestEmail(eventType, providerMessageID string) bool {
	return strings.TrimSpace(eventType) == "email.received" && strings.TrimSpace(providerMessageID) != ""
}

func (h *Handler) retryMissingReceivedEmail(ctx context.Context, providerEventID, providerMessageID string) (bool, error) {
	exists, err := h.store.HasProviderMessage(ctx, providerResend, providerMessageID)
	if err != nil {
		return false, err
	}
	if exists {
		return false, nil
	}
	if err := h.ingestReceivedEmail(ctx, providerEventID, providerMessageID); err != nil {
		return false, err
	}
	return true, nil
}

func shouldRetryIngest(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, errReceivePolicyRejected) || errors.Is(err, sql.ErrNoRows) {
		return false
	}
	return true
}

func (h *Handler) ingestReceivedEmail(ctx context.Context, providerEventID, providerMessageID string) error {
	email, err := h.resend.retrieveReceivedEmail(ctx, providerMessageID)
	if err != nil {
		return err
	}
	if token, ok := approvalReplyToken(email.To, h.cfg.PrimaryDomain); ok {
		return h.processApprovalReply(ctx, providerEventID, email, token)
	}
	alias, recipient, err := h.resolveReceivedAlias(ctx, email.To)
	if err != nil {
		return err
	}
	policyResult, err := h.enforceReceivePolicy(ctx, email, alias)
	if err != nil {
		return err
	}
	return h.store.StoreInboundMessage(ctx, providerEventID, email, alias, recipient, policyResult)
}

func (h *Handler) resolveReceivedAlias(ctx context.Context, recipients []string) (EmailAlias, string, error) {
	for _, recipient := range recipients {
		localPart, domain, ok := splitEmailAddress(recipient)
		if !ok {
			continue
		}
		alias, err := h.store.ResolveAlias(ctx, domain, localPart)
		if err == nil {
			return alias, recipient, nil
		}
		if err != sql.ErrNoRows {
			return EmailAlias{}, "", err
		}
	}
	return EmailAlias{}, "", sql.ErrNoRows
}

var errReceivePolicyRejected = errors.New("receive policy rejected inbound email")

type receivePolicyResult struct {
	PolicyID              string
	PolicyName            string
	TrustedSender         bool
	SenderAuthority       string
	PromptBarEquivalent   bool
	WorkflowHandoffStatus string
}

func (h *Handler) enforceReceivePolicy(ctx context.Context, email resendReceivedEmail, alias EmailAlias) (receivePolicyResult, error) {
	policy, err := h.store.GetReceivePolicy(ctx, alias.ReceivePolicyID)
	if err != nil {
		return receivePolicyResult{}, fmt.Errorf("load receive policy: %w", err)
	}
	if policy.RequireSecretAlias && (alias.Visibility == "public" || !strings.Contains(alias.LocalPart, "+")) {
		return receivePolicyResult{}, fmt.Errorf("%w: secret alias required", errReceivePolicyRejected)
	}

	senderAddress, _ := parseSender(email.From, email.Headers["from"])
	whitelisted := false
	senderAuthority := "public"
	if policy.RequireSenderWhitelist {
		if !hasPassingAuthenticationResultEvidence(email.Headers) {
			return receivePolicyResult{}, fmt.Errorf("%w: passing authentication results required", errReceivePolicyRejected)
		}
		whitelisted, err = h.store.IsSenderWhitelisted(ctx, alias.TargetID, alias.ID, senderAddress)
		if err != nil {
			return receivePolicyResult{}, err
		}
		if !whitelisted {
			return receivePolicyResult{}, fmt.Errorf("%w: sender whitelist required", errReceivePolicyRejected)
		}
		senderAuthority = "verified_sender_policy"
	}
	if !policy.AllowPublicInbound && !whitelisted {
		return receivePolicyResult{}, fmt.Errorf("%w: public inbound disabled", errReceivePolicyRejected)
	}
	if len(email.Attachments) > 0 && !policy.AllowAttachments && !policy.QuarantineByDefault {
		return receivePolicyResult{}, fmt.Errorf("%w: attachments disabled", errReceivePolicyRejected)
	}
	result := receivePolicyResult{
		PolicyID:        policy.ID,
		PolicyName:      policy.Name,
		TrustedSender:   whitelisted,
		SenderAuthority: senderAuthority,
	}
	if whitelisted && policy.AllowAutoAgentRead {
		result.PromptBarEquivalent = true
		result.WorkflowHandoffStatus = "pending_email_appagent_intent"
	}
	return result, nil
}

func hasPassingAuthenticationResultEvidence(headers map[string]string) bool {
	for key, value := range headers {
		normalized := strings.ToLower(strings.TrimSpace(key))
		if normalized != "authentication-results" && normalized != "arc-authentication-results" {
			continue
		}
		value = strings.ToLower(strings.TrimSpace(value))
		if strings.Contains(value, "dmarc=pass") || strings.Contains(value, "dkim=pass") || strings.Contains(value, "spf=pass") {
			return true
		}
	}
	return false
}

func verifyWebhook(payload []byte, headers http.Header, secret string) error {
	msgID := strings.TrimSpace(headers.Get("svix-id"))
	timestampValue := strings.TrimSpace(headers.Get("svix-timestamp"))
	signatureValue := strings.TrimSpace(headers.Get("svix-signature"))
	if msgID == "" || timestampValue == "" || signatureValue == "" {
		return fmt.Errorf("missing svix headers")
	}
	unixSeconds, err := strconv.ParseInt(timestampValue, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid svix timestamp: %w", err)
	}
	timestamp := time.Unix(unixSeconds, 0)
	if age := time.Since(timestamp); age > webhookTimestampTolerance || age < -webhookTimestampTolerance {
		return fmt.Errorf("svix timestamp outside tolerance")
	}

	key, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(strings.TrimSpace(secret), "whsec_"))
	if err != nil {
		return fmt.Errorf("decode svix secret: %w", err)
	}
	if len(key) == 0 {
		return fmt.Errorf("decode svix secret: empty signing key")
	}
	expected := signSvixPayload(key, msgID, unixSeconds, payload)
	for _, signature := range strings.Fields(signatureValue) {
		signature = strings.TrimSpace(strings.TrimPrefix(signature, "v1,"))
		if signature == "" {
			continue
		}
		if hmac.Equal([]byte(signature), []byte(expected)) {
			return nil
		}
	}
	return fmt.Errorf("signature mismatch")
}

func signSvixPayload(key []byte, msgID string, unixSeconds int64, payload []byte) string {
	mac := hmac.New(sha256.New, key)
	_, _ = fmt.Fprintf(mac, "%s.%d.%s", msgID, unixSeconds, payload)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func eventRowID(providerEventID string) string {
	sum := sha256.Sum256([]byte(providerResend + ":" + providerEventID))
	return "resend-webhook-" + hex.EncodeToString(sum[:16])
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
