package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

// Email sign-in from the native side (docs/design/email-sign-in-2026-10-31.md,
// "Mac app"). The renderer posts the address and then the code to these two
// local routes; the app calls the backend's /auth/email/begin and
// /auth/email/verify with its own cookie jar, so the attempt cookie that binds
// the code to this client and the session it earns both land in the jar the
// canonical issuer wrote them to, and nowhere else. The renderer never sees a
// cookie, a token or a Set-Cookie header: it gets the attempt id and, on
// success, {authenticated: true}. No choir:// handoff is involved.

const (
	desktopEmailBeginPath  = "/desktop-auth/email/begin"
	desktopEmailVerifyPath = "/desktop-auth/email/verify"
)

func (s *desktopSession) handleEmailBegin(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email string `json:"email"`
	}
	if !readDesktopAuthJSON(w, r, &input) {
		return
	}
	input.Email = strings.TrimSpace(input.Email)
	if input.Email == "" || len(input.Email) > 320 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid email sign-in request"})
		return
	}
	status, payload, err := s.postEmailAuth(r.Context(), "/auth/email/begin", map[string]string{"email": input.Email})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "email sign-in failed"})
		return
	}
	if status != http.StatusOK {
		writeJSON(w, mappedEmailStatus(status), map[string]string{"error": "email sign-in failed"})
		return
	}
	var out struct {
		AttemptID string `json:"attempt_id"`
	}
	if err := json.Unmarshal(payload, &out); err != nil || out.AttemptID == "" {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "email sign-in failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"attempt_id": out.AttemptID})
}

func (s *desktopSession) handleEmailVerify(w http.ResponseWriter, r *http.Request) {
	var input struct {
		AttemptID string `json:"attempt_id"`
		Code      string `json:"code"`
	}
	if !readDesktopAuthJSON(w, r, &input) {
		return
	}
	if input.AttemptID == "" || len(input.AttemptID) > 64 || input.Code == "" || len(input.Code) > 16 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid email sign-in request"})
		return
	}
	status, payload, err := s.postEmailAuth(r.Context(), "/auth/email/verify", map[string]string{
		"attempt_id": input.AttemptID,
		"code":       input.Code,
	})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "email sign-in failed"})
		return
	}
	if status != http.StatusOK {
		writeJSON(w, mappedEmailStatus(status), map[string]string{"error": "email sign-in failed"})
		return
	}
	var out struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(payload, &out); err != nil || !out.OK {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "email sign-in failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"authenticated": true})
}

// mappedEmailStatus passes the statuses the form acts on and folds the rest.
func mappedEmailStatus(status int) int {
	switch status {
	case http.StatusBadRequest, http.StatusForbidden, http.StatusTooManyRequests:
		return status
	}
	return http.StatusBadGateway
}

func readDesktopAuthJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, desktopAuthRequestLimit)
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(dst); err != nil {
		status := http.StatusBadRequest
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			status = http.StatusRequestEntityTooLarge
		}
		writeJSON(w, status, map[string]string{"error": "invalid email sign-in request"})
		return false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid email sign-in request"})
		return false
	}
	return true
}

// postEmailAuth posts JSON to a backend auth route with the app's own cookie
// jar and returns the status and a bounded body.
func (s *desktopSession) postEmailAuth(ctx context.Context, path string, body map[string]string) (int, []byte, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return 0, nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint(path).String(), bytes.NewReader(raw))
	if err != nil {
		return 0, nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "Choir-Desktop/"+appVersion)
	response, err := s.client.Do(request)
	if err != nil {
		return 0, nil, err
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, desktopAuthResponseLimit+1))
	if err != nil || len(payload) > desktopAuthResponseLimit {
		return 0, nil, errors.New("invalid email sign-in response")
	}
	return response.StatusCode, payload, nil
}
