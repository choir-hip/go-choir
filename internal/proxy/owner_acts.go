package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/auth"
)

// OwnerActsScope marks an API key the owner minted, from a signed-in session,
// to perform owner acts headlessly. Auth refuses to grant it to a key created
// by another key, and `admin` does not imply it
// (docs/problems/auth-boundary-gaps-from-surface-audit-2026-10-14.md F4).
const OwnerActsScope = auth.OwnerActsScope

// authorizeOwnerAct admits an owner act (arming self-development, approving a
// candidate, rollback, restore, rematerialize, bootstrap-chain) only for the
// owner's browser session or an API key that carries OwnerActsScope. The
// route's own scope is checked separately; this is the additional gate that
// keeps an agent's key, including an `admin` key, from performing the act the
// owner is supposed to supervise.
func authorizeOwnerAct(w http.ResponseWriter, authResult *AuthResult, act string) bool {
	if authResult != nil {
		switch authResult.AuthMethod {
		case "cookie":
			return true
		case "api_key":
			if hasAPIKeyScope(authResult.Scopes, OwnerActsScope) {
				return true
			}
		}
	}
	writeJSON(w, http.StatusForbidden, errorResponse{
		Error: "owner act: " + act + " needs the owner's signed-in session or an API key the owner minted with the " + OwnerActsScope + " scope; admin does not grant it",
	})
	return false
}

// maxDecisionBodyBytes matches the decision route's upstream body allowance.
const maxDecisionBodyBytes = 1 << 20

// selfDevelopmentDecisionIsReject reports whether a decision request body is
// a rejection, the one decision an agent may take without the owner: it
// withdraws a candidate and frees a pinned computer. The body is buffered and
// restored for forwarding. Anything the proxy cannot read as a rejection is
// treated as an approval, so it falls under the owner-act gate.
func selfDevelopmentDecisionIsReject(r *http.Request) (bool, error) {
	if r == nil || r.Body == nil {
		return false, nil
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxDecisionBodyBytes+1))
	_ = r.Body.Close()
	r.Body = io.NopCloser(bytes.NewReader(body))
	if err != nil {
		return false, err
	}
	if len(body) > maxDecisionBodyBytes {
		return false, nil
	}
	var decision struct {
		Decision string `json:"decision"`
	}
	if json.Unmarshal(body, &decision) != nil {
		return false, nil
	}
	return strings.TrimSpace(decision.Decision) == "reject", nil
}
