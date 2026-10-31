package maild

import (
	"fmt"
	"html"
	"log"
	"net/http"
	"net/url"
	"strings"
	"unicode"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
)

// AuthSignInCodePath is the host-internal route the auth service calls to
// deliver an email sign-in code and link (docs/design/email-sign-in-2026-10-31.md).
// It copies the recovery route's gate verbatim (loopback host, X-Internal-Caller,
// hostauth.Authorize, refusal of X-Authenticated-User) and adds a fourth
// requirement: hostauth must be enforcing, because loopback alone is not proof
// across the guest DNAT and Authorize passes in off and observe. A guest can
// never send Choir-branded sign-in mail.
const AuthSignInCodePath = "/internal/maild/auth/signin-code"

const (
	authSignInTokenPrefix = "choir_st_"
	authSignInLinkPath    = "/signin/email"
	authSignInLinkMax     = 512
	authSignInFieldMax    = 80

	authSignInPurposeSignIn        = "sign_in"
	authSignInPurposeVerifyAddress = "verify_address"
)

type authSignInRequest struct {
	ToAddress      string `json:"to_address"`
	Purpose        string `json:"purpose"`
	Code           string `json:"code"`
	Link           string `json:"link"`
	Device         string `json:"device"`
	Place          string `json:"place"`
	ExpiresMinutes int    `json:"expires_minutes"`
}

// HandleAuthSignInCode sends one fixed-template sign-in (or verify-address)
// email. The caller supplies the recipient, the code, the link and the
// requesting device and place; maild validates every field and renders the
// message itself.
func (h *Handler) HandleAuthSignInCode(w http.ResponseWriter, r *http.Request) {
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
	if !hostauth.Authorize(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "internal credential required"})
		return
	}
	if strings.TrimSpace(r.Header.Get("X-Authenticated-User")) != "" {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "not available to user requests"})
		return
	}
	// Sign-in mail needs the credential actually enforced, not merely
	// tolerated: off and observe let any loopback-looking caller through.
	if !hostauth.Enforcing() {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "internal credential enforcement required"})
		return
	}
	var in authSignInRequest
	if err := h.decodeJSON(r, &in); err != nil {
		writeDecodeError(w, err)
		return
	}
	to := normalizedTrustedEmail(in.ToAddress)
	if to == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "valid to_address is required"})
		return
	}
	if in.Purpose != authSignInPurposeSignIn && in.Purpose != authSignInPurposeVerifyAddress {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid purpose"})
		return
	}
	if !isEightDigitCode(in.Code) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid code"})
		return
	}
	link := ""
	if in.Purpose == authSignInPurposeSignIn {
		var err error
		link, err = h.validateAuthSignInLink(in.Link)
		if err != nil {
			// Never echo the link: it carries a live token.
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid sign-in link"})
			return
		}
	} else if strings.TrimSpace(in.Link) != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid sign-in link"})
		return
	}
	device, ok1 := cleanSignInField(in.Device)
	place, ok2 := cleanSignInField(in.Place)
	if !ok1 || !ok2 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid device or place"})
		return
	}
	if in.ExpiresMinutes < 1 || in.ExpiresMinutes > 60 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "expires_minutes must be between 1 and 60"})
		return
	}
	toHash := authMailAddressHash(to)
	sent, err := h.resend.sendEmail(r.Context(), h.buildAuthSignInMail(to, in.Purpose, in.Code, link, device, place, in.ExpiresMinutes))
	if err != nil {
		log.Printf("maild: operation=auth_signin_mail to_hash=%s result=error error=%q", toHash, err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to send sign-in email"})
		return
	}
	log.Printf("maild: operation=auth_signin_mail to_hash=%s result=sent provider_message_id=%s", toHash, sent.ID)
	writeJSON(w, http.StatusAccepted, authRecoveryLinkResponse{Status: "sent", ProviderMessageID: sent.ID})
}

func isEightDigitCode(s string) bool {
	if len(s) != 8 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// cleanSignInField accepts a short single-line printable string.
func cleanSignInField(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if len(s) > authSignInFieldMax {
		return "", false
	}
	for _, c := range s {
		if !unicode.IsPrint(c) {
			return "", false
		}
	}
	return s, true
}

// validateAuthSignInLink accepts only <origin>/signin/email#token=choir_st_...
// where the origin is maild's primary domain over https (or localhost for
// development), with no query and no userinfo, and the token only in the
// fragment.
func (h *Handler) validateAuthSignInLink(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > authSignInLinkMax || strings.ContainsAny(raw, "\r\n\t ") {
		return "", fmt.Errorf("malformed link")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse link: %w", err)
	}
	if u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Opaque != "" {
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
	if u.Path != authSignInLinkPath || u.RawPath != "" {
		return "", fmt.Errorf("link path not allowed")
	}
	token, ok := strings.CutPrefix(u.Fragment, "token="+authSignInTokenPrefix)
	if !ok || len(token) < 16 || !isBase64URLText(token) {
		return "", fmt.Errorf("link fragment not allowed")
	}
	return u.String(), nil
}

func (h *Handler) buildAuthSignInMail(to, purpose, code, link, device, place string, expiresMinutes int) resendSendRequest {
	domain := DefaultPrimaryDomain
	if h != nil && h.cfg != nil && strings.TrimSpace(h.cfg.PrimaryDomain) != "" {
		domain = strings.ToLower(strings.TrimSpace(h.cfg.PrimaryDomain))
	}
	subject := "Your Choir sign-in code"
	intro := "Someone asked to sign in to the Choir account for this email address."
	if purpose == authSignInPurposeVerifyAddress {
		subject = "Verify your email address for Choir"
		intro = "Someone signed in to Choir and asked to verify this email address."
	}
	origin := fmt.Sprintf("Requested from %s, on %s.", device, place)
	if device == "" || place == "" {
		origin = "Requested from a device we could not describe."
	}
	text := fmt.Sprintf("%s\n\nYour code: %s\n\n", intro, code)
	if link != "" {
		text += fmt.Sprintf("Or open this link on the same device:\n%s\n\n", link)
	}
	text += fmt.Sprintf("The code works once and expires in %d minutes.\n%s\n\n"+
		"Never give this code to anyone. Choir will never ask you for it.\n\n"+
		"If you did not ask for this, ignore this email. Your account has not changed.", expiresMinutes, origin)

	htmlBody := `<!doctype html><html><body style="margin:0;padding:0;background:#ffffff;">` +
		`<main style="max-width:560px;margin:0 auto;padding:24px;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;font-size:16px;line-height:1.55;color:#111827;">` +
		`<p style="margin:0 0 16px;">` + html.EscapeString(intro) + `</p>` +
		`<p style="margin:0 0 20px;font-size:32px;letter-spacing:6px;font-weight:700;">` + html.EscapeString(code) + `</p>`
	if link != "" {
		escaped := html.EscapeString(link)
		htmlBody += `<p style="margin:0 0 20px;"><a href="` + escaped + `" style="display:inline-block;padding:12px 18px;border-radius:12px;background:#111827;color:#ffffff;font-weight:700;text-decoration:none;">Sign in</a></p>` +
			`<p style="margin:0 0 16px;">If the button does not work, open this address on the same device:<br><span style="word-break:break-all;color:#374151;">` + escaped + `</span></p>`
	}
	htmlBody += `<p style="margin:0 0 16px;">The code works once and expires in ` + fmt.Sprint(expiresMinutes) + ` minutes. ` + html.EscapeString(origin) + `</p>` +
		`<p style="margin:0 0 16px;font-weight:700;">Never give this code to anyone. Choir will never ask you for it.</p>` +
		`<p style="margin:0 0 16px;color:#4b5563;">If you did not ask for this, ignore this email. Your account has not changed.</p>` +
		choirAutomatedHTMLSignature() +
		`</main></body></html>`
	return resendSendRequest{
		From:    "Choir <updates@" + domain + ">",
		To:      []string{to},
		Subject: subject,
		Text:    appendChoirAutomatedTextSignature(text),
		HTML:    htmlBody,
		Headers: map[string]any{
			"X-Choir-Maild": "v0-auth-signin-code",
		},
	}
}
