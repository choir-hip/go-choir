package maild

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
)

// AuthRecoveryLinkPath is the host-internal route the auth service calls to
// deliver an account-recovery magic link. It lives under /internal/, which
// Caddy refuses at the public edge and the proxy never forwards, and it
// accepts only loopback host-service callers: maild also listens on the guest
// tap, and a guest computer must never be able to send Choir-branded sign-in
// mail (docs/problems/account-recovery-token-discarded-2026-10-14.md, failure
// modes 9 and 10).
const AuthRecoveryLinkPath = "/internal/maild/auth/recovery-link"

// authRecoveryTokenPrefix mirrors auth.RecoveryTokenPrefix. maild checks the
// link shape only; it never stores or logs the token.
const authRecoveryTokenPrefix = "choir_rt_"

const authRecoveryLinkMaxBytes = 512

type authRecoveryLinkRequest struct {
	ToAddress      string `json:"to_address"`
	Link           string `json:"link"`
	ExpiresMinutes int    `json:"expires_minutes"`
}

type authRecoveryLinkResponse struct {
	Status            string `json:"status"`
	ProviderMessageID string `json:"provider_message_id,omitempty"`
}

// HandleAuthRecoveryLink sends one fixed-template recovery email. The caller
// supplies only the recipient and the link; maild validates that the link
// points at Choir's own /recover page and renders the message itself.
func (h *Handler) HandleAuthRecoveryLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if !isMaildLoopbackHost(maildRemoteAddrHost(r)) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "host service caller required"})
		return
	}
	if !strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Internal-Caller")), "true") {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "internal caller required"})
		return
	}
	// Loopback is not a credential on its own: the caller must also hold the
	// host-internal token (docs/problems/loopback-is-a-credential-2026-10-14.md).
	// Checked after the transport, so a guest call never reaches this path.
	if !hostauth.Authorize(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "internal credential required"})
		return
	}
	// The proxy marks everything it forwards with X-Authenticated-User. Auth
	// mail is never a signed-in user's action, so a forwarded request is
	// refused even though it arrives from loopback.
	if strings.TrimSpace(r.Header.Get("X-Authenticated-User")) != "" {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "not available to user requests"})
		return
	}
	var in authRecoveryLinkRequest
	if err := h.decodeJSON(r, &in); err != nil {
		writeDecodeError(w, err)
		return
	}
	to := normalizedTrustedEmail(in.ToAddress)
	if to == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "valid to_address is required"})
		return
	}
	link, err := h.validateAuthRecoveryLink(in.Link)
	if err != nil {
		// Never echo the link: it carries a live token.
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid recovery link"})
		return
	}
	if in.ExpiresMinutes < 1 || in.ExpiresMinutes > 60 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "expires_minutes must be between 1 and 60"})
		return
	}
	toHash := authMailAddressHash(to)
	sent, err := h.resend.sendEmail(r.Context(), h.buildAuthRecoveryMail(to, link, in.ExpiresMinutes))
	if err != nil {
		log.Printf("maild: operation=auth_recovery_mail to_hash=%s result=error error=%q", toHash, err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to send recovery email"})
		return
	}
	log.Printf("maild: operation=auth_recovery_mail to_hash=%s result=sent provider_message_id=%s", toHash, sent.ID)
	writeJSON(w, http.StatusAccepted, authRecoveryLinkResponse{Status: "sent", ProviderMessageID: sent.ID})
}

// validateAuthRecoveryLink accepts only <origin>/recover#token=choir_rt_…
// where the origin is maild's primary domain over https, or localhost for
// development. The link origin comes from auth config, never from a request
// header, so anything else means a misconfiguration or misuse.
func (h *Handler) validateAuthRecoveryLink(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > authRecoveryLinkMaxBytes || strings.ContainsAny(raw, "\r\n\t ") {
		return "", fmt.Errorf("malformed link")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse link: %w", err)
	}
	if u.User != nil || u.RawQuery != "" || u.Opaque != "" {
		return "", fmt.Errorf("link must not carry userinfo or query")
	}
	host := strings.ToLower(u.Hostname())
	local := host == "localhost" || host == "127.0.0.1"
	primary := ""
	if h != nil && h.cfg != nil {
		primary = strings.ToLower(strings.TrimSpace(h.cfg.PrimaryDomain))
	}
	switch {
	case u.Scheme == "https" && primary != "" && host == primary && u.Port() == "":
	case local && (u.Scheme == "http" || u.Scheme == "https"):
	default:
		return "", fmt.Errorf("link origin not allowed")
	}
	if u.Path != "/recover" {
		return "", fmt.Errorf("link path not allowed")
	}
	token, ok := strings.CutPrefix(u.Fragment, "token="+authRecoveryTokenPrefix)
	if !ok || len(token) < 16 || !isBase64URLText(token) {
		return "", fmt.Errorf("link fragment not allowed")
	}
	return u.String(), nil
}

func isBase64URLText(s string) bool {
	for _, c := range s {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_':
		default:
			return false
		}
	}
	return true
}

func (h *Handler) buildAuthRecoveryMail(to, link string, expiresMinutes int) resendSendRequest {
	domain := DefaultPrimaryDomain
	if h != nil && h.cfg != nil && strings.TrimSpace(h.cfg.PrimaryDomain) != "" {
		domain = strings.ToLower(strings.TrimSpace(h.cfg.PrimaryDomain))
	}
	text := fmt.Sprintf("Someone asked to recover the Choir account for this email address.\n\n"+
		"Open this link to add a new passkey and sign in:\n%s\n\n"+
		"The link works once and expires in %d minutes.\n\n"+
		"If you did not ask for this, ignore this email. Your account has not changed.", link, expiresMinutes)
	escaped := html.EscapeString(link)
	htmlBody := `<!doctype html><html><body style="margin:0;padding:0;background:#ffffff;">` +
		`<main style="max-width:560px;margin:0 auto;padding:24px;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;font-size:16px;line-height:1.55;color:#111827;">` +
		`<p style="margin:0 0 16px;">Someone asked to recover the Choir account for this email address.</p>` +
		`<p style="margin:0 0 20px;"><a href="` + escaped + `" style="display:inline-block;padding:12px 18px;border-radius:12px;background:#111827;color:#ffffff;font-weight:700;text-decoration:none;">Add a new passkey</a></p>` +
		`<p style="margin:0 0 16px;">The link works once and expires in ` + fmt.Sprint(expiresMinutes) + ` minutes. If the button does not work, open this address:<br><span style="word-break:break-all;color:#374151;">` + escaped + `</span></p>` +
		`<p style="margin:0 0 16px;color:#4b5563;">If you did not ask for this, ignore this email. Your account has not changed.</p>` +
		choirAutomatedHTMLSignature() +
		`</main></body></html>`
	return resendSendRequest{
		From:    "Choir <updates@" + domain + ">",
		To:      []string{to},
		Subject: "Recover your Choir account",
		Text:    appendChoirAutomatedTextSignature(text),
		HTML:    htmlBody,
		Headers: map[string]any{
			"X-Choir-Maild": "v0-auth-recovery-link",
		},
	}
}

func authMailAddressHash(address string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(address))))
	return hex.EncodeToString(sum[:])[:8]
}
