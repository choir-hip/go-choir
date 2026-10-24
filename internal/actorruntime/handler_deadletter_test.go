package actorruntime

import (
	"bytes"
	"log"
	"strings"
	"sync"
	"testing"
)

// lockedLog is a log destination the test reads while the runtime's own
// background goroutines (started by the adapter env) may still be logging.
type lockedLog struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *lockedLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *lockedLog) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

// A poisoned wake lands in the delivery-poison sink as delivery_failed. The
// sink must dead-letter it once with a log line naming what died, never
// error (an erroring sink handler retries and loops), and never call it an
// "unknown update kind" (paper-silent-since-10-56-after-host-reboot-2026-10-15.md,
// failure mode 9).
func TestHandlerDeadLettersDeliveryFailedOnceAndNamesTheSourcePoll(t *testing.T) {
	env := newAdapterTestEnv(t)
	handler := newActorHandler(env.adapter.Runtime, nil)

	var buf lockedLog
	prev := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(prev) })

	for _, kind := range []string{"delivery_failed", "delivery_invalid"} {
		u := actorUpdate("owner-dl", kind, "delivery-poison", "update-lost-1")
		u.FromAgentID = scopedActorMailboxID("owner-dl", "autoputer-test", "source:rss:news")
		mem := []byte(`{"phase":"idle"}`)
		out, err := handler.HandleUpdate(env.ctx, "delivery-poison", u, mem)
		if err != nil {
			t.Fatalf("%s: HandleUpdate error = %v; the sink must consume it (an error retries forever)", kind, err)
		}
		if string(out) != string(mem) {
			t.Fatalf("%s: memory changed: %q", kind, out)
		}
	}
	logged := buf.String()
	if strings.Contains(logged, "unknown update kind") {
		t.Fatalf("dead letter logged as an unknown kind:\n%s", logged)
	}
	for _, want := range []string{"dead letter", "delivery_failed", "delivery_invalid", "update-lost-1", "source:rss:news", "sweep"} {
		if !strings.Contains(logged, want) {
			t.Fatalf("log lacks %q:\n%s", want, logged)
		}
	}
}

func TestDeadLetterDescriptionForANonSourceWake(t *testing.T) {
	u := actorUpdate("owner-dl", "delivery_failed", "delivery-poison", "update-9")
	u.FromAgentID = scopedActorMailboxID("owner-dl", "autoputer-test", "agent-1")
	got := describeDeadLetter(u)
	if !strings.Contains(got, "update-9") || !strings.Contains(got, "agent-1") || strings.Contains(got, "sweep") {
		t.Fatalf("description = %q", got)
	}
	// An unparseable sender is still described, never dropped silently.
	u.FromAgentID = "raw-mailbox"
	if got := describeDeadLetter(u); !strings.Contains(got, "raw-mailbox") {
		t.Fatalf("description = %q", got)
	}
}
