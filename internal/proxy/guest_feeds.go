package proxy

// A computer's feeds from corpusd (autopaper contracts 2 and 3): the op-eds
// on its stories, the notices for its owner, and the paper's verdict post.
// corpusd sees the proxy as the trusted internal caller, so the proxy is the
// one that scopes each request to the owner vmctl binds to the calling guest:
// whatever owner the guest names is overwritten.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	opedFeedPathProxy     = "/internal/platform/opeds"
	noticeFeedPathProxy   = "/internal/platform/notices"
	opedVerdictsPathProxy = "/internal/platform/oped-verdicts"
)

func isGuestFeedPath(path string) bool {
	return path == opedFeedPathProxy || path == noticeFeedPathProxy || path == opedVerdictsPathProxy
}

// scopeGuestFeedRequest returns the query and body to relay to corpusd: the
// guest's own, with owner_id set to the bound owner.
func (h *Handler) scopeGuestFeedRequest(r *http.Request) (string, io.Reader, error) {
	ownership, err := h.boundProxyGuest(r)
	if err != nil || strings.TrimSpace(ownership.UserID) == "" {
		return "", nil, fmt.Errorf("guest caller is not bound to an owner")
	}
	owner := strings.TrimSpace(ownership.UserID)
	if r.Method == http.MethodGet {
		q := r.URL.Query()
		q.Set("owner_id", owner)
		return url.Values(q).Encode(), r.Body, nil
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<16))
	if err != nil {
		return "", nil, fmt.Errorf("read verdict request: %w", err)
	}
	var body map[string]any
	if json.Unmarshal(raw, &body) != nil || body == nil {
		return "", nil, fmt.Errorf("verdict request must be a JSON object")
	}
	body["owner_id"] = owner
	scoped, err := json.Marshal(body)
	if err != nil {
		return "", nil, err
	}
	return r.URL.RawQuery, bytes.NewReader(scoped), nil
}
