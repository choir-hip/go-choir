package agentcore

import (
	"fmt"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// The rule: nothing is delivered, incorporated or consumed unless the desk was
// shown it whole. Ways it could fail, written first: the wake turn names more
// ids than the cell's frame shows, so a Management report consumes updates it
// never read; a run whose frame was cut still delivers every pending report
// (the Texture consume-at-commit default); the next window repeats the first
// instead of moving past what was disposed; a run that was never cut is
// restricted and stops consuming what it always consumed.

func pendingReports(n, size int) []types.CoagentSourcePacket {
	content := strings.Repeat("c", size)
	out := make([]types.CoagentSourcePacket, n)
	for i := range out {
		out[i] = types.CoagentSourcePacket{
			UpdateID: fmt.Sprintf("u%03d", i), MessageSeq: int64(i + 1), Content: content,
			Direction: types.LifecyclePacketDirectionProducerReport, TargetAgentID: "texture:doc-1",
			Disposition: types.UpdatePending, AgentID: "research:doc-1",
		}
	}
	return out
}

func TestUpdateWindowIsAnOldestFirstPrefixThatMovesAsUpdatesAreDisposed(t *testing.T) {
	all := pendingReports(40, 100<<10) // 4 MB of reports, over the 4 MiB frame
	first, remaining := windowUpdatePackets(all)
	if len(first) == 0 || len(first) >= len(all) || remaining != len(all)-len(first) {
		t.Fatalf("window = %d of %d, remaining %d", len(first), len(all), remaining)
	}
	if first[0].UpdateID != "u000" || first[len(first)-1].UpdateID != fmt.Sprintf("u%03d", len(first)-1) {
		t.Fatalf("the window is not the oldest prefix: %s..%s", first[0].UpdateID, first[len(first)-1].UpdateID)
	}
	size := 0
	for _, u := range first {
		size += yaegikernel.RecordSize(yaegikernel.InboxRecord{Body: u.Content})
	}
	if size > yaegikernel.FrameUpdatesBudget+len(first)*1024 {
		t.Fatalf("window is %d bytes, over the %d budget", size, yaegikernel.FrameUpdatesBudget)
	}
	// The first window is disposed: the next one starts where it ended.
	next, _ := windowUpdatePackets(all[len(first):])
	if next[0].UpdateID != fmt.Sprintf("u%03d", len(first)) {
		t.Fatalf("the next window starts at %s, want u%03d", next[0].UpdateID, len(first))
	}
	// One update larger than the budget is shown alone and whole, never cut.
	huge := pendingReports(2, 600<<10)
	one, rest := windowUpdatePackets(huge)
	if len(one) != 1 || rest != 1 || len(one[0].Content) != 600<<10 {
		t.Fatalf("an oversize update must be shown alone and whole: %d shown, %d left, %d bytes", len(one), rest, len(one[0].Content))
	}
}

// (b) Management: the wake turn names only ids the cell's frame shows, so the
// ids a report consumes (the ids the wake named) are all ids the desk read;
// the rest stay pending and are named once the window has moved.
func TestWakeNamesOnlyUpdatesTheCellFrameShows(t *testing.T) {
	rec := &types.RunRecord{RunID: "run-wake", AgentID: "research:doc-1"}
	all := pendingReports(40, 100<<10)
	for i := range all {
		all[i].TargetAgentID = "research:doc-1"
	}
	shown, _ := windowUpdatePackets(all)
	windowed := wakeWindowIDs(rec, all)
	if len(windowed) != len(shown) {
		t.Fatalf("wake window %d ids, frame window %d", len(windowed), len(shown))
	}
	for _, u := range shown {
		if !windowed[u.UpdateID] {
			t.Fatalf("the frame shows %s and the wake would not name it", u.UpdateID)
		}
	}
	// The wake of a desk whose first window an earlier wake already named names
	// nothing new until that window is consumed: u010.. are past the frame.
	seen := map[string]bool{}
	for id := range windowed {
		seen[id] = true
	}
	var fresh []types.CoagentSourcePacket
	for _, u := range all {
		if !seen[u.UpdateID] && windowed[u.UpdateID] {
			fresh = append(fresh, u)
		}
	}
	if len(fresh) != 0 {
		t.Fatalf("a second wake named %d updates past the frame's window", len(fresh))
	}
	// After the report consumed the window, the wake names the next one, and
	// again only ids its frame shows.
	rest := all[len(shown):]
	nextShown, _ := windowUpdatePackets(rest)
	nextWindowed := wakeWindowIDs(rec, rest)
	if len(nextWindowed) != len(nextShown) || !nextWindowed[rest[0].UpdateID] || windowed[rest[0].UpdateID] {
		t.Fatalf("the next wake window is wrong: %d ids", len(nextWindowed))
	}
	// The wake turn itself names exactly the ids it is given.
	msgs, ids, err := buildCoagentUpdateUserMessages(shown, coagentPacketDeliveryMid, rec.AgentID, nil, nil)
	if err != nil || len(msgs) != 1 || len(ids) != len(shown) {
		t.Fatalf("wake turn: %d messages, %d ids, %v", len(msgs), len(ids), err)
	}
}

func TestUpdatesShownRestrictionOnlyAppliesToACutRun(t *testing.T) {
	rt, _ := testRuntime(t)
	// A run whose frames carried every pending update consumes as before.
	rt.noteUpdatesShown("run-whole", []string{"a", "b"}, false)
	if _, restricted := rt.UpdatesShownRestriction("run-whole"); restricted {
		t.Fatal("a run that was never cut must not be restricted")
	}
	// A cut run delivers only what it was shown, across its cells.
	rt.noteUpdatesShown("run-cut", []string{"a", "b"}, true)
	rt.noteUpdatesShown("run-cut", []string{"b", "c"}, false)
	shown, restricted := rt.UpdatesShownRestriction("run-cut")
	if !restricted || len(shown) != 3 || !shown["a"] || !shown["c"] || shown["d"] {
		t.Fatalf("shown = %v restricted = %v", shown, restricted)
	}
}
