package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// RecoveryLinkPath is the frontend page a recovery link opens. The token
// travels in the URL fragment, which browsers never send to a server, so it
// stays out of access logs and Referer headers.
const RecoveryLinkPath = "/recover"

// maildRecoveryLinkPath mirrors maild.AuthRecoveryLinkPath (auth does not
// import maild; the two services talk only over loopback HTTP).
const maildRecoveryLinkPath = "/internal/maild/auth/recovery-link"

// recoveryMailTimeout bounds one delivery attempt. Delivery is off the
// request path, and a crash drops it: the person asks for a new link.
const recoveryMailTimeout = 30 * time.Second

// RecoveryMailer delivers an account-recovery link to an account's stored
// email address. Implementations must never log the link: it carries a live
// single-use token.
type RecoveryMailer interface {
	SendRecoveryLink(ctx context.Context, toAddress, link string, expiresIn time.Duration) error
}

type maildRecoveryMailer struct {
	endpoint string
	client   *http.Client
}

// NewMaildRecoveryMailer returns a RecoveryMailer that posts to maild's
// host-internal recovery-link route.
func NewMaildRecoveryMailer(maildURL string, client *http.Client) RecoveryMailer {
	if client == nil {
		client = &http.Client{Timeout: recoveryMailTimeout}
	}
	return &maildRecoveryMailer{
		endpoint: strings.TrimRight(strings.TrimSpace(maildURL), "/") + maildRecoveryLinkPath,
		client:   client,
	}
}

func (m *maildRecoveryMailer) SendRecoveryLink(ctx context.Context, toAddress, link string, expiresIn time.Duration) error {
	body, err := json.Marshal(map[string]any{
		"to_address":      toAddress,
		"link":            link,
		"expires_minutes": int(expiresIn / time.Minute),
	})
	if err != nil {
		return fmt.Errorf("marshal recovery mail: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build recovery mail request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Caller", "true")
	resp, err := m.client.Do(req)
	if err != nil {
		// The *url.Error names the endpoint, never the body.
		return fmt.Errorf("post recovery mail: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("maild recovery mail status %d", resp.StatusCode)
	}
	return nil
}

// recoveryLink builds the link from configuration, never from the request
// (no Host or Origin header can redirect it).
func (h *Handler) recoveryLink(token string) string {
	origin := ""
	if h.config != nil && len(h.config.RPOrigins) > 0 {
		origin = strings.TrimRight(strings.TrimSpace(h.config.RPOrigins[0]), "/")
	}
	return origin + RecoveryLinkPath + "#token=" + token
}

// sendRecoveryMail delivers the link for a real account off the request path,
// so known and unknown emails take the same time to answer. Failures are
// logged without the token and never change the HTTP response.
func (h *Handler) sendRecoveryMail(toAddress, emailHash, userID, token string) {
	mailer := h.recoveryMailer
	if mailer == nil {
		log.Printf("[auth] operation=recovery_mail email_hash=%s user_id=%s result=skipped reason=mailer_not_configured", emailHash, userID)
		return
	}
	link := h.recoveryLink(token)
	dispatch := h.recoveryMailDispatch
	if dispatch == nil {
		dispatch = func(f func()) { go f() }
	}
	dispatch(func() {
		ctx, cancel := context.WithTimeout(context.Background(), recoveryMailTimeout)
		defer cancel()
		if err := mailer.SendRecoveryLink(ctx, toAddress, link, RecoveryTokenTTL); err != nil {
			log.Printf("[auth] operation=recovery_mail email_hash=%s user_id=%s result=error error=%q", emailHash, userID, err)
			return
		}
		log.Printf("[auth] operation=recovery_mail email_hash=%s user_id=%s result=sent", emailHash, userID)
	})
}
