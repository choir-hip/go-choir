package hostauth

// The issuer credential is a second, narrower host credential. Only vmctl
// and corpusd load it, and corpusd's credential-issuance route requires it
// in addition to the internal credential.
//
// Credential issuance is narrower than the rest of the internal surface, so
// it gets a narrower credential: a unit that holds only the shared internal
// credential should not be able to start it (blast-radius design, panel
// finding F1; public summary
// docs/problems/host-compromise-blast-radius-2026-10-20.md).
//
// It rolls out like the internal credential (CHOIR_ISSUER_AUTH_MODE: off,
// observe, enforce) so a caller that lacks it is found in the log before
// anything is refused: a refused issuance is a computer that cannot boot.

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"
	"sync"
)

const (
	// IssuerHeader carries the issuer credential.
	IssuerHeader = "X-Choir-Issuer-Token"
	// IssuerCredentialName is the systemd credential name (LoadCredential=).
	IssuerCredentialName = "choir-issuer-token"
	// EnvIssuerMode selects off, observe or enforce.
	EnvIssuerMode = "CHOIR_ISSUER_AUTH_MODE"
	// EnvIssuerTokenFile overrides the credential path (tests, CI shells).
	EnvIssuerTokenFile = "CHOIR_ISSUER_TOKEN_FILE"
	// issuerPort is corpusd, the only service that checks the credential.
	issuerPort = "8086"
)

var (
	issuerOnce sync.Once
	issuerMu   sync.RWMutex
	issuerCfg  *config
)

func currentIssuer() *config {
	issuerOnce.Do(func() {
		c := loadConfig(EnvIssuerMode, EnvIssuerTokenFile, IssuerCredentialName)
		c.ports = map[string]bool{issuerPort: true}
		issuerMu.Lock()
		if issuerCfg == nil {
			issuerCfg = c
		}
		issuerMu.Unlock()
	})
	issuerMu.RLock()
	defer issuerMu.RUnlock()
	return issuerCfg
}

// SetIssuerForTesting installs an issuer mode and token for a test and
// returns the restore function. An empty token means none loaded.
func SetIssuerForTesting(mode Mode, token string) (restore func()) {
	currentIssuer()
	c := &config{mode: mode, source: "test", ports: map[string]bool{issuerPort: true}}
	if token != "" {
		c.setToken(token)
	}
	issuerMu.Lock()
	prev := issuerCfg
	issuerCfg = c
	issuerMu.Unlock()
	resetLogLimiter()
	return func() {
		issuerMu.Lock()
		issuerCfg = prev
		issuerMu.Unlock()
		resetLogLimiter()
	}
}

// IssuerStatus describes the issuer configuration without the secret.
func IssuerStatus() string { return currentIssuer().Status() }

type issuerCtxKey struct{}

// issuerVerdict checks the issuer header and removes it from the request,
// so nothing downstream sees it. Middleware calls it for every request.
func issuerVerdict(r *http.Request) verdict {
	values := r.Header.Values(IssuerHeader)
	v := verdictMissing
	switch {
	case len(values) > 1:
		v = verdictAmbiguous
	case len(values) == 1:
		v = verdictWrong
		if c := currentIssuer(); c.hasToken {
			presented := sha256.Sum256([]byte(strings.TrimSpace(values[0])))
			if subtle.ConstantTimeCompare(presented[:], c.digest[:]) == 1 {
				v = verdictValid
			}
		}
	}
	if len(values) > 0 {
		r.Header.Del(IssuerHeader)
	}
	return v
}

func withIssuerVerdict(ctx context.Context, v verdict) context.Context {
	return context.WithValue(ctx, issuerCtxKey{}, v)
}

// AuthorizeIssuer is the issuer half of the issuance gate. Call it after
// the internal gate has passed. It returns false only in enforce mode.
func AuthorizeIssuer(r *http.Request) bool {
	if r == nil {
		return false
	}
	c := currentIssuer()
	if c.mode == ModeOff {
		return true
	}
	v, _ := r.Context().Value(issuerCtxKey{}).(verdict)
	if v == verdictValid && c.hasToken {
		return true
	}
	reason := "issuer: " + v.reason()
	if !c.hasToken {
		reason = "issuer: no credential loaded on this service"
		if c.loadErr != nil {
			reason += " (" + c.loadErr.Error() + ")"
		}
	}
	route := r.Pattern
	if route == "" && r.URL != nil {
		route = r.URL.Path
	}
	if c.mode == ModeObserve {
		logLimited("observe: would refuse", r, route, reason)
		return true
	}
	logLimited("enforce: refused", r, route, reason)
	return false
}

// MarkIssuer attaches the issuer credential to an outbound request, only
// when it goes to corpusd on loopback, so the secret never reaches a guest
// (through a 127.0.0.1 DNAT port) or the internet. MarkInternal is still
// required for the internal half.
func MarkIssuer(req *http.Request) {
	if req == nil {
		return
	}
	req.Header.Del(IssuerHeader)
	if req.URL == nil || !loopbackHost(req.URL.Hostname()) {
		return
	}
	if c := currentIssuer(); c.hasToken && c.ports[req.URL.Port()] {
		req.Header.Set(IssuerHeader, c.raw)
	}
}
