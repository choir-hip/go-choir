package gateway

// Account deletion removes a destroyed computer's gateway identity, and only
// for a host caller that carries a deletion record's request time at least 72
// hours old.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func forgetRequest(computerID string, requested time.Time, host bool) *http.Request {
	body := `{"computer_id":"` + computerID + `","deletion_requested_at":"` + requested.UTC().Format(time.RFC3339) + `"}`
	if requested.IsZero() {
		body = `{"computer_id":"` + computerID + `"}`
	}
	req := httptest.NewRequest(http.MethodPost, "/provider/v1/credentials/forget", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if host {
		req.Header.Set("X-Internal-Caller", "true")
		req.Host = "localhost:8084"
		req.RemoteAddr = "127.0.0.1:12345"
	} else {
		req.Host = "10.0.0.1:8084"
		req.RemoteAddr = "10.0.0.1:45678"
	}
	return req
}

func TestForgetCredentialNeedsHostAndAnOldRecord(t *testing.T) {
	h, reg := setupHandlerNoProvider(t)
	res, err := reg.IssueCredential("computer-doomed")
	if err != nil {
		t.Fatal(err)
	}
	other, _ := reg.IssueCredential("computer-other")
	now := time.Now()
	cases := []struct {
		name string
		req  *http.Request
		want int
	}{
		{"not the host", forgetRequest("computer-doomed", now.Add(-100*time.Hour), false), http.StatusForbidden},
		{"no time", forgetRequest("computer-doomed", time.Time{}, true), http.StatusForbidden},
		{"too young", forgetRequest("computer-doomed", now.Add(-71*time.Hour), true), http.StatusForbidden},
		{"future", forgetRequest("computer-doomed", now.Add(time.Hour), true), http.StatusForbidden},
	}
	for _, c := range cases {
		w := httptest.NewRecorder()
		h.HandleForgetCredential(w, c.req)
		if w.Code != c.want {
			t.Fatalf("%s: %d %s", c.name, w.Code, w.Body)
		}
		if _, err := reg.ValidateCredential(res.RawToken); err != nil {
			t.Fatalf("%s removed the identity: %v", c.name, err)
		}
	}
	w := httptest.NewRecorder()
	h.HandleForgetCredential(w, forgetRequest("computer-doomed", now.Add(-73*time.Hour), true))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"existed":true`) {
		t.Fatalf("forget: %d %s", w.Code, w.Body)
	}
	if _, err := reg.ValidateCredential(res.RawToken); err == nil {
		t.Fatal("the identity still validates")
	}
	if _, err := reg.ValidateCredential(other.RawToken); err != nil {
		t.Fatalf("another computer's identity was removed: %v", err)
	}
	w = httptest.NewRecorder()
	h.HandleForgetCredential(w, forgetRequest("computer-doomed", now.Add(-73*time.Hour), true))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"existed":false`) {
		t.Fatalf("second forget: %d %s", w.Code, w.Body)
	}
}
