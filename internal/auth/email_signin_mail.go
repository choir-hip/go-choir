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

	"github.com/yusefmosiah/go-choir/internal/hostauth"
)

// maildSignInPath mirrors maild.AuthSignInCodePath (auth does not import
// maild; the two services talk only over loopback HTTP).
const maildSignInPath = "/internal/maild/auth/signin-code"

const signInMailTimeout = 30 * time.Second

// revokeAllRotatedGrace empties the rotation grace cache (the email switch
// went off: no cached session may be handed back).
func (h *Handler) revokeAllRotatedGrace() {
	h.rotationsMu.Lock()
	defer h.rotationsMu.Unlock()
	for k := range h.recentRotations {
		delete(h.recentRotations, k)
	}
}

// SignInMail is one sign-in (or verify-address) message. Link is empty for
// verify_address. None of it is ever logged.
type SignInMail struct {
	To             string
	Purpose        string
	Code           string
	Link           string
	Device         string
	Place          string
	ExpiresMinutes int
}

// SignInMailer delivers the fixed-template code mail.
type SignInMailer interface {
	SendSignInCode(ctx context.Context, m SignInMail) error
}

type maildSignInMailer struct {
	endpoint string
	client   *http.Client
}

// NewMaildSignInMailer posts to maild's host-internal sign-in route.
func NewMaildSignInMailer(maildURL string, client *http.Client) SignInMailer {
	if client == nil {
		client = &http.Client{Timeout: signInMailTimeout}
	}
	return &maildSignInMailer{
		endpoint: strings.TrimRight(strings.TrimSpace(maildURL), "/") + maildSignInPath,
		client:   client,
	}
}

func (m *maildSignInMailer) SendSignInCode(ctx context.Context, mail SignInMail) error {
	body, err := json.Marshal(map[string]any{
		"to_address":      mail.To,
		"purpose":         mail.Purpose,
		"code":            mail.Code,
		"link":            mail.Link,
		"device":          mail.Device,
		"place":           mail.Place,
		"expires_minutes": mail.ExpiresMinutes,
	})
	if err != nil {
		return fmt.Errorf("marshal sign-in mail: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build sign-in mail request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	hostauth.MarkInternal(req)
	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("post sign-in mail: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("maild sign-in mail status %d", resp.StatusCode)
	}
	return nil
}

// sendSignInMail delivers off the request path (as recovery does), so a known
// and an unknown address answer in the same time. A failure is logged by
// attempt hash only, leaves the row unsent, and never changes the response.
func (h *Handler) sendSignInMail(r *http.Request, a *EmailAttempt, to, code, link string) {
	mailer := h.signInMailer
	attempt := shortHash(a.ID)
	if mailer == nil {
		log.Printf("[auth] operation=email_signin_mail attempt=%s result=skipped reason=mailer_not_configured sent=false", attempt)
		return
	}
	mail := SignInMail{
		To:             to,
		Purpose:        a.Purpose,
		Code:           code,
		Link:           link,
		Device:         a.UserAgentSummary,
		Place:          summarizePlace(clientIP(r)),
		ExpiresMinutes: int(EmailAttemptTTL / time.Minute),
	}
	id := a.ID
	dispatch := h.signInMailDispatch
	if dispatch == nil {
		dispatch = func(f func()) { go f() }
	}
	dispatch(func() {
		ctx, cancel := context.WithTimeout(context.Background(), signInMailTimeout)
		defer cancel()
		if err := mailer.SendSignInCode(ctx, mail); err != nil {
			log.Printf("[auth] operation=email_signin_mail attempt=%s result=error sent=false error=%q", attempt, err)
			return
		}
		if err := h.store.MarkEmailAttemptSent(ctx, id); err != nil {
			log.Printf("[auth] operation=email_signin_mail attempt=%s result=warning step=mark_sent error=%q", attempt, err)
		}
		log.Printf("[auth] operation=email_signin_mail attempt=%s result=sent", attempt)
	})
}
