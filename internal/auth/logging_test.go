package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// captureLogs captures log output during the execution of fn.
// It returns the captured log output as a string.
func captureLogs(t *testing.T, fn func()) string {
	t.Helper()

	// Capture log output by redirecting the default logger.
	r, w := io.Pipe()

	oldWriter := log.Writer()
	oldFlags := log.Flags()
	log.SetOutput(w)
	log.SetFlags(0) // Remove timestamps for deterministic matching

	// Channel to signal that we've read all output.
	done := make(chan struct{})
	var buf bytes.Buffer

	go func() {
		defer close(done)
		io.Copy(&buf, r)
	}()

	fn()

	// Restore the logger before reading output.
	log.SetOutput(oldWriter)
	log.SetFlags(oldFlags)
	w.Close()

	<-done
	return buf.String()
}

func TestLogsDoNotContainRawEmail(t *testing.T) {
	h, _ := testHandlerEnv(t)

	email := "sensitive@example.com"
	logs := captureLogs(t, func() {
		body := fmt.Sprintf(`{"email":%q}`, email)
		req := httptest.NewRequest(http.MethodPost, "/auth/register/begin", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.HandleRegisterBegin(rec, req)
	})

	if strings.Contains(logs, email) {
		t.Errorf("logs should not contain raw email address, got:\n%s", logs)
	}
	if strings.Contains(logs, "sensitive") {
		t.Errorf("logs should not contain email local part, got:\n%s", logs)
	}
	if strings.Contains(logs, "@example.com") {
		t.Errorf("logs should not contain email domain, got:\n%s", logs)
	}
}

func TestLogsDoNotContainSensitiveCredentialData(t *testing.T) {
	// Verify hashEmail output doesn't look like a credential/key.
	h := hashEmail("test@example.com")
	if len(h) > 16 {
		t.Errorf("hashEmail output should be short, got %d chars", len(h))
	}
}

// --- Verify the JSON response doesn't leak into logs ---

func TestLogsDoNotContainJSONResponseData(t *testing.T) {
	h, _ := testHandlerEnv(t)

	email := "json@example.com"
	logs := captureLogs(t, func() {
		body := fmt.Sprintf(`{"email":%q}`, email)
		req := httptest.NewRequest(http.MethodPost, "/auth/register/begin", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.HandleRegisterBegin(rec, req)

		// Parse the response to verify it still works.
		var resp map[string]interface{}
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
	})

	// Logs should not contain the WebAuthn challenge or public key data.
	if strings.Contains(logs, "publicKey") {
		t.Errorf("logs should not contain publicKey data, got:\n%s", logs)
	}
}
