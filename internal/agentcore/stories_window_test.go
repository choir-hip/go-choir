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

// The verb's page is a count of stories, newest first; the task's opening
// holds the window instead (management_opening_page_test.go).
func TestStoriesVerbPageIsACount(t *testing.T) {
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
	page := func(payload json.RawMessage) yaegikernel.StoriesPage {
		t.Helper()
		raw, err := rt.storiesEgress(ctx, exec.Role, exec, payload)
		if err != nil {
			t.Fatal(err)
		}
		var p yaegikernel.StoriesPage
		if err := json.Unmarshal(raw, &p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	if got := page(nil); len(got.Stories) != 50 || got.Next == "" {
		t.Fatalf("W4: the default page = %d stories, next %q; want 50 and a cursor", len(got.Stories), got.Next)
	}
	limited, _ := json.Marshal(yaegikernel.StoriesPayload{Limit: 10})
	if got := page(limited); len(got.Stories) != 10 {
		t.Fatalf("W4: a limit-10 read = %d stories", len(got.Stories))
	}
}
