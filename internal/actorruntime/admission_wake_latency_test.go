package actorruntime

import (
	"bytes"
	"context"
	"log"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/actor"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// FP4 round 3, P1 next probe 1
// (docs/problems/fp4-nudge-waits-behind-management-activation-2026-10-22.md):
// every admission wake logs how long it waited from due to handled, with
// its mailbox and content, so drill step 4 can read where a nudge waited.
// Ways it can fail: no line; waited measured from creation for a scheduled
// retry (the backoff would read as latency); no mailbox, so a wait behind
// management cannot be told from one on the admission mailbox.
func TestAdmissionWakeLogsItsDispatchLatency(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "admission.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	adapter := New(provideriface.Config{
		ComputerID: "autoputer-test", StorePath: dbPath, PromptRoot: filepath.Join(dir, "prompts"),
		ProviderTimeout: time.Second,
	}, s, events.NewEventBus(), provider.NewStubProvider(0), nil, WithKernelMode())
	t.Cleanup(adapter.cleanupLog)

	var buf bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(previous) })

	now := time.Now().UTC()
	mailbox := scopedActorMailboxID("owner-latency", "autoputer-test", "management:owner-latency")
	u := actor.Update{
		UpdateID: "spawn-retry", ToAgentID: mailbox, Kind: "delegated_assignment_spawn_deadline",
		Content:   `{"assignment_id":"assignment-absent","attempt":1,"retry":2}`,
		CreatedAt: now.Add(-time.Minute), NotBefore: now.Add(-3 * time.Second),
	}
	handler := newActorHandler(adapter.Runtime, nil)
	handler.bootAt = now.Add(-time.Hour) // the wake is this boot's work
	if _, err := handler.HandleUpdate(ctx, mailbox, u, nil); err != nil {
		t.Fatal(err)
	}
	line := ""
	for _, l := range strings.Split(buf.String(), "\n") {
		if strings.Contains(l, "engineering admission wake latency") {
			line = l
		}
	}
	if line == "" {
		t.Fatalf("no admission wake latency line; log:\n%s", buf.String())
	}
	m := regexp.MustCompile(`waited=(\S+)`).FindStringSubmatch(line)
	if m == nil {
		t.Fatalf("latency line has no waited: %s", line)
	}
	waited, err := time.ParseDuration(m[1])
	if err != nil || waited < 2500*time.Millisecond || waited > 10*time.Second {
		t.Fatalf("waited %q is not measured from not_before (3 s ago; want 2.5 s to 10 s): %s", m[1], line)
	}
	for _, want := range []string{"mailbox=management:owner-latency", "assignment-absent"} {
		if !strings.Contains(line, want) {
			t.Fatalf("latency line lacks %q: %s", want, line)
		}
	}
}
