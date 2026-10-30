package store

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// Corpus search reads Jev's labels as topic hints for captures
// (docs/archive/problems/prompt-bar-research-slow-unsynthesized-mangled-2026-10-14.md,
// symptom 3). Ways the label listing can fail, written first:
//   - it returns an older version of a relabelled capture, so the hint is
//     the label management already corrected;
//   - it returns another computer's or another owner's labels;
//   - it lists every label the computer ever wrote, unbounded.
func TestRecentCaptureLabelsKeepEachCapturesNewestVersion(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	base := time.Date(2026, 10, 14, 4, 0, 0, 0, time.UTC)
	put := func(owner, computer, capture, topic string, at time.Time) {
		t.Helper()
		if _, err := s.AppendCaptureLabel(ctx, CaptureLabel{OwnerID: owner, ComputerID: computer, CaptureID: capture,
			Event: CaptureLabelEventNew, TopicHeadline: topic, Outlet: "Harbor Daily", Script: "Latin", LabelledAt: at}); err != nil {
			t.Fatal(err)
		}
	}
	put("owner", "computer", "choir.web_capture:a", "first reading", base)
	put("owner", "computer", "choir.web_capture:a", "corrected reading", base.Add(time.Minute))
	put("owner", "computer", "choir.web_capture:b", "port strike", base.Add(2*time.Minute))
	put("owner", "computer-other", "choir.web_capture:c", "elsewhere", base.Add(3*time.Minute))
	put("owner-other", "computer", "choir.web_capture:d", "not mine", base.Add(4*time.Minute))

	got, err := s.RecentCaptureLabels(ctx, "owner", "computer", 100)
	if err != nil {
		t.Fatalf("RecentCaptureLabels: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("labels = %+v; want captures a and b alone", got)
	}
	if a := got["choir.web_capture:a"]; a.Version != 2 || a.TopicHeadline != "corrected reading" {
		t.Fatalf("capture a = %+v; want its newest version", a)
	}
	if b := got["choir.web_capture:b"]; b.Outlet != "Harbor Daily" || b.Script != "Latin" {
		t.Fatalf("capture b = %+v", b)
	}

	for i := 0; i < 5; i++ {
		put("owner", "computer", fmt.Sprintf("choir.web_capture:n%d", i), "more", base.Add(time.Duration(10+i)*time.Minute))
	}
	bounded, err := s.RecentCaptureLabels(ctx, "owner", "computer", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(bounded) > 3 {
		t.Fatalf("bounded listing returned %d labels; want at most 3", len(bounded))
	}
	if _, ok := bounded["choir.web_capture:n4"]; !ok {
		t.Fatalf("bounded listing %+v dropped the newest label", bounded)
	}
}
