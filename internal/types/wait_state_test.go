package types

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// Failure modes written before the code:
//  1. a host-memory wait renders without need, have or holders, so the model
//     cannot weigh Stop against waiting;
//  2. a holder's age or size is lost or misrendered;
//  3. a policy wait reads like contention (class missing or wrong);
//  4. a nil wait panics where a caller renders it unconditionally;
//  5. the tally double-counts an episode whose end is observed twice, or
//     loses one that never saw an end;
//  6. a JSON round trip drops fields the frame and records carry.

func TestWaitStateSentenceHostMemory(t *testing.T) {
	since := time.Date(2026, 11, 4, 1, 0, 0, 0, time.UTC)
	now := since.Add(4 * time.Minute)
	w := &WaitState{
		Kind: WaitKindHostMemory, Class: WaitClassContention, Subject: "job j-1", Resource: "host memory",
		NeedBytes: 1536 << 20, HaveBytes: 600 << 20, HasHave: true,
		Holders: []WaitHolder{
			{Who: "job j-0", What: "go test ./internal/capsule/...", Bytes: 3072 << 20, Since: since.Add(-4 * time.Minute)},
			{Who: "other computers", What: "builds and starts", Bytes: 1536 << 20},
		},
		Since: since, Until: since.Add(30 * time.Minute), CheckEverySec: 10,
	}
	got := w.SentenceAt(now)
	for _, want := range []string{
		"waiting for host memory",
		"needs 1536 MiB",
		"600 MiB is free beyond the floor",
		"held by job j-0 (go test ./internal/capsule/..., 3072 MiB, 8m)",
		"other computers (builds and starts, 1536 MiB)",
		"checking every 10s",
		"waited 4m",
		"gives up in 26m",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("sentence lacks %q:\n%s", want, got)
		}
	}
}

func TestWaitStateSentencePolicyNamesPolicy(t *testing.T) {
	now := time.Date(2026, 11, 4, 1, 0, 0, 0, time.UTC)
	w := &WaitState{Kind: WaitKindPlatformUpdate, Class: WaitClassPolicy, Subject: "assignment a-1",
		Resource: "the platform update to abc123", Since: now.Add(-2 * time.Minute), Until: now.Add(18 * time.Minute)}
	got := w.SentenceAt(now)
	if !strings.Contains(got, "waiting for the platform update to abc123") || !strings.Contains(got, "(policy)") {
		t.Fatalf("policy wait sentence: %s", got)
	}
	if strings.Contains(got, "free beyond the floor") {
		t.Fatalf("a non-memory wait renders memory figures: %s", got)
	}
}

func TestWaitStateNilIsSafe(t *testing.T) {
	var w *WaitState
	if w.SentenceAt(time.Now()) != "" || w.Active() {
		t.Fatal("nil wait must render empty and be inactive")
	}
	if got := w.TextureLine(time.Now()); got != "" {
		t.Fatalf("nil texture line: %q", got)
	}
}

func TestWaitStateTextureLine(t *testing.T) {
	now := time.Date(2026, 11, 4, 1, 0, 0, 0, time.UTC)
	w := &WaitState{Kind: WaitKindHostMemory, Class: WaitClassContention, Resource: "host memory",
		Holders: []WaitHolder{{Who: "other computers", What: "builds and starts", Bytes: 3 << 30}},
		Since:   now.Add(-4 * time.Minute)}
	if got, want := w.TextureLine(now), "Waiting 4 min for host memory, held by other computers (builds and starts)"; got != want {
		t.Fatalf("texture line = %q, want %q", got, want)
	}
}

func TestWaitTallyEpisodes(t *testing.T) {
	t0 := time.Date(2026, 11, 4, 1, 0, 0, 0, time.UTC)
	var tally WaitTally
	tally.Add(WaitKindHostMemory, 9*time.Minute)
	tally.Add(WaitKindPlatformUpdate, 3*time.Minute)
	tally.Add(WaitKindHostMemory, 30*time.Second)
	if got := tally.Seconds[string(WaitKindHostMemory)]; got != 570 {
		t.Fatalf("host memory seconds = %d, want 570", got)
	}
	if got := tally.Total(); got != 750*time.Second {
		t.Fatalf("total = %s", got)
	}
	s := tally.Summary()
	if !strings.HasPrefix(s, "waited 13 min: host memory 10 min, platform update 3 min") {
		t.Fatalf("summary = %q", s)
	}
	_ = t0
	var empty WaitTally
	if empty.Summary() != "" {
		t.Fatal("empty tally must render empty")
	}
	empty.Add(WaitKindHostMemory, -time.Second)
	if empty.Total() != 0 {
		t.Fatal("a negative duration must not count")
	}
}

func TestWaitStateJSONRoundTrip(t *testing.T) {
	since := time.Date(2026, 11, 4, 1, 0, 0, 0, time.UTC)
	w := WaitState{Kind: WaitKindHostMemory, Class: WaitClassContention, Subject: "job j-1", Resource: "host memory",
		NeedBytes: 1, HaveBytes: 0, HasHave: true, Holders: []WaitHolder{{Who: "a", What: "b", Bytes: 2, Since: since}},
		Since: since, Until: since.Add(time.Minute), CheckEverySec: 5, Detail: "d"}
	w.Stamp(since.Add(time.Minute))
	b, err := json.Marshal(w)
	if err != nil {
		t.Fatal(err)
	}
	var back WaitState
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.Kind != w.Kind || back.HaveBytes != 0 || !back.HasHave || len(back.Holders) != 1 || !back.Until.Equal(w.Until) || back.Detail != "d" {
		t.Fatalf("round trip lost fields: %s", b)
	}
	if back.Sentence == "" || !strings.Contains(back.Sentence, "waiting for host memory") {
		t.Fatalf("a stamped wait carries its sentence for readers that do not render: %s", b)
	}
}
