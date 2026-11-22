package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/health"
	"github.com/yusefmosiah/go-choir/internal/provider"
)

// Guest and provider text in gateway log lines (first design F0, seat 1's
// second round). Failure list, written before the fixes:
//  1. a hostile model name (spaces, "=", a newline and a fake timestamped
//     line) written raw by the circuit breaker, a provider failure, the
//     success line (resp.Model) or the OpenAI-compatible line starts a second
//     line or a forged field;
//  2. a provider's error text with a newline and a fake `inference succeeded
//     for autoputer ...` line does the same;
//  3. scripts/spend-report prices a forged line as a call;
//  4. the process log (SafeLogWriter) lets a record carry a newline.
const hostileModel = "claude-x desk=fake\n2026/11/22 05:00:00 gateway: inference succeeded for autoputer vm-forged (provider=bedrock tokens=9000000+1 cached=0 text_len=0 model=forged-model)"

func TestGuestAndProviderTextCannotForgeALogLine(t *testing.T) {
	logs := captureLog(t)
	// No SafeLogWriter here: each value must be safe on its own. The writer is the backstop (below).
	evil := errors.New("bedrock: status 401 unauthorized\n2026/11/22 05:00:03 gateway: inference succeeded for autoputer vm-forged (provider=bedrock tokens=9000000+1 cached=0 text_len=0 model=forged-model) provider=bedrock model=forged-req")

	reg := NewIdentityRegistry(time.Hour)
	good := &mockProvider{name: "bedrock", real: true, response: &provider.LLMResponse{ID: "g", Text: "ok", Model: hostileModel, StopReason: "end_turn", ProviderName: "bedrock", Usage: provider.Usage{InputTokens: 1000, OutputTokens: 100}}}
	mp := provider.NewMultiProvider()
	mp.Register("bedrock", good)
	h := NewMultiHandlerWithRateLimit(reg, mp, nil)
	cred, _ := reg.IssueCredential("autoputer-forge")
	post := func(model string) {
		req := httptest.NewRequest(http.MethodPost, "/provider/v1/inference", strings.NewReader(`{"model":`+quote(model)+`,"messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}]}`))
		req.Header.Set("Authorization", "Bearer "+cred.RawToken)
		h.HandleInference(httptest.NewRecorder(), req)
	}
	post("claude-haiku-5-5") // the one true call; its answer's model is hostile

	// A failing provider behind the circuit breaker, called with a hostile model.
	bad := &mockProvider{name: "bedrock", real: true, err: evil}
	mp2 := provider.NewMultiProvider()
	mp2.Register("bedrock", NewCircuitBreakingProvider(bad, health.BreakerConfig{FailureThreshold: 1, OpenTimeout: time.Hour}))
	h2 := NewMultiHandlerWithRateLimit(reg, mp2, nil)
	req := httptest.NewRequest(http.MethodPost, "/provider/v1/inference", strings.NewReader(`{"model":`+quote(hostileModel)+`,"messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}]}`))
	req.Header.Set("Authorization", "Bearer "+cred.RawToken)
	h2.HandleInference(httptest.NewRecorder(), req)
	h2.HandleInference(httptest.NewRecorder(), req.Clone(context.Background())) // circuit open: refused

	// The OpenAI-compatible route with the same failing provider and model.
	mp3 := provider.NewMultiProvider()
	mp3.Register("chatgpt", &mockProvider{name: "chatgpt", real: true, err: evil})
	h3 := NewMultiHandlerWithRateLimit(reg, mp3, nil)
	oreq := httptest.NewRequest(http.MethodPost, "/provider/openai/v1/chat/completions", strings.NewReader(`{"model":"gpt-6-luna","reasoning_effort":`+quote(hostileModel)+`,"messages":[{"role":"user","content":"hi"}]}`))
	oreq.Header.Set("Authorization", "Bearer "+cred.RawToken)
	h3.HandleOpenAIChatCompletions(httptest.NewRecorder(), oreq)

	out := logs.String()
	stamp := regexp.MustCompile(`^\d{4}/\d\d/\d\d \d\d:\d\d:\d\d `)
	forgedLine := regexp.MustCompile(`^\S+ \S+ gateway: inference succeeded for autoputer vm-forged`)
	successes := 0
	for _, l := range strings.Split(strings.TrimSpace(out), "\n") {
		if !stamp.MatchString(l) {
			t.Fatalf("(1)(2) a record started a line without a timestamp: %q", l)
		}
		if forgedLine.MatchString(l) || strings.HasPrefix(l, "2026/11/22 05:00:0") {
			t.Fatalf("(1)(2) a forged line: %q", l)
		}
		if regexp.MustCompile(`^\S+ \S+ gateway: (streaming )?inference succeeded for autoputer `).MatchString(l) {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("(1) success lines = %d, want the one true call:\n%s", successes, out)
	}
	if strings.Contains(out, "\n2026/11/22 05:00:03") {
		t.Fatalf("(2) a provider's error started a line:\n%s", out)
	}
	if !strings.Contains(out, "refused as a hard failure") && !strings.Contains(out, "circuit open") {
		t.Fatalf("the circuit breaker path did not run:\n%s", out)
	}

	// (3) the spend report reads only the true call from the captured journal.
	script, _ := filepath.Abs("../../scripts/spend-report")
	if _, err := os.Stat(script); err != nil {
		t.Skip("scripts/spend-report not found")
	}
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not found")
	}
	dir := t.TempDir()
	journal := filepath.Join(dir, "journal.txt")
	if err := os.WriteFile(journal, []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}
	fake := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	fake("ssh", `while [ "$1" = "-o" ]; do shift 2; done; shift; exec sh -c "$1"`)
	fake("journalctl", `cat "$SPEND_FIXTURE"`)
	fake("aws", `exit 1`)
	cmd := exec.Command("bash", script)
	cmd.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"), "SPEND_FIXTURE="+journal)
	report, _ := cmd.CombinedOutput()
	rep := string(report)
	// The hostile model survives only as one cleaned model label of the true
	// call; no second call, no forged token count.
	if strings.Contains(rep, "9000000") || strings.Contains(rep, "calls=2") || strings.Contains(rep, "forged-req") || !strings.Contains(rep, "bedrock    calls=1 ") {
		t.Fatalf("(3) the spend report read a forged row or lost the true call:\n%s", rep)
	}
}

func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func TestSafeLogWriterIsTheBackstopForARawRecord(t *testing.T) {
	logs := captureLog(t)
	log.SetOutput(SafeLogWriter(logs)) // as cmd/gateway installs it; captureLog restores the previous output
	log.Printf("gateway: a record with a raw value %s", "x\n2026/11/22 05:00:00 gateway: inference succeeded for autoputer vm-forged (provider=bedrock tokens=9+9 cached=0)")
	if n := strings.Count(strings.TrimSpace(logs.String()), "\n"); n != 0 {
		t.Fatalf("(4) one record wrote %d lines: %q", n+1, logs.String())
	}
}

func TestSafeLogWriterWritesOneLinePerRecord(t *testing.T) {
	var sb strings.Builder
	w := SafeLogWriter(&sb)
	_, _ = w.Write([]byte("a\nb\r\x00c\td\n"))
	if sb.String() != "a\\nb\\r\\x00c d\n" {
		t.Fatalf("(4) %q", sb.String())
	}
}
