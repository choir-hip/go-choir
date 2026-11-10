package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Management's story page is a window of time, not a count (D2). Ways it can
// fail: W1 the window page is shorter than the page it replaces (a quiet
// hour); W2 a busy window still shows only the newest 50; W3 the window is
// unbounded (more than storiesMaxLimit); W4 a verb read that asks for a
// limit changes.
func TestStoriesWindowPageHoldsEveryStoryOfTheWindow(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	owner, computer := "owner-stories-window", rt.cfg.ComputerID
	if err := s.PutComputerSetting(ctx, store.ComputerSetting{OwnerID: owner, ComputerID: computer, Key: store.SettingIntakeLabels, Value: true}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 57; i++ {
		openTestStory(t, s, owner, computer, fmt.Sprintf("Story number %02d", i))
		time.Sleep(time.Millisecond)
	}
	exec := toolregistry.ExecutionContext{OwnerID: owner, ComputerID: computer, Role: agentprofile.Management, Profile: agentprofile.Management}
	page := func(payload json.RawMessage, window time.Duration) yaegikernel.StoriesPage {
		t.Helper()
		raw, err := rt.storiesEgressWithin(ctx, exec.Role, exec, payload, window)
		if err != nil {
			t.Fatal(err)
		}
		var p yaegikernel.StoriesPage
		if err := json.Unmarshal(raw, &p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	if got := page(nil, storiesWindow); len(got.Stories) != 57 || got.Next != "" {
		t.Fatalf("W2: window page = %d stories, next %q; want all 57 of the last 8 hours", len(got.Stories), got.Next)
	}
	if got := page(nil, time.Nanosecond); len(got.Stories) != 50 || got.Next == "" {
		t.Fatalf("W1: an empty window page = %d stories, next %q; want the newest 50 as before", len(got.Stories), got.Next)
	}
	if got := page(nil, 0); len(got.Stories) != 50 {
		t.Fatalf("W4: the unwindowed page = %d; want 50", len(got.Stories))
	}
	limited, _ := json.Marshal(yaegikernel.StoriesPayload{Limit: 10})
	if got := page(limited, 0); len(got.Stories) != 10 {
		t.Fatalf("W4: a limit-10 read = %d stories", len(got.Stories))
	}
}
