package store

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"testing"
)

// docs/problems/research-more-slow-and-hidden-2026-10-21.md, E6: the
// paper's engine was held 14.9 s in a measured window by 276 JSON scans of
// every capture label for one capture's $.capture_id (CaptureLabels, and
// LatestCaptureLabel before every append). Every label is keyed
// captureLabelKey(computer, capture, version), with versions 1..N and no
// gaps (AppendCaptureLabel), since labels began (ba297cbd3). Failure modes
// of reading the versions by key instead of the scan:
//  1. a version a writer appended is missed (a gap, or another key);
//  2. a label of another owner, computer or capture is returned;
//  3. the order is not oldest first;
//  4. a capture with no labels errors instead of reading none;
//  5. the read still scans capture labels.

func TestCaptureLabelsReadByKeyMatchTheScan(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	owners := []string{storyOwner, "other-owner"}
	computers := []string{storyComputer, "another-computer"}
	captures := []string{"cap-1", "cap-2", "cap-ü", "cap-3"}
	rng := rand.New(rand.NewSource(21))
	for i := 0; i < 40; i++ {
		label := CaptureLabel{
			OwnerID: owners[rng.Intn(len(owners))], ComputerID: computers[rng.Intn(len(computers))],
			CaptureID: captures[rng.Intn(len(captures)-1)], Title: fmt.Sprintf("label %d", i), Event: "new",
		}
		if _, err := s.AppendCaptureLabel(ctx, label); err != nil {
			t.Fatal(err)
		}
	}
	scan := func(owner, computer, capture string) string {
		objs, err := s.ogListAllByMetadata(ctx, ogKindCaptureLabel, "capture_id", capture)
		if err != nil {
			t.Fatal(err)
		}
		var versions []int
		for _, obj := range objs {
			var l CaptureLabel
			if err := ogDecode(obj, &l); err != nil {
				t.Fatal(err)
			}
			if obj.OwnerID == owner && l.ComputerID == computer {
				versions = append(versions, l.Version)
			}
		}
		sort.Ints(versions)
		return fmt.Sprint(versions)
	}
	found := 0
	for _, owner := range owners {
		for _, computer := range computers {
			for _, capture := range captures {
				want := scan(owner, computer, capture)
				before := captureLabelScanCalls(s)
				labels, err := s.CaptureLabels(ctx, owner, computer, capture)
				if err != nil {
					t.Fatalf("CaptureLabels(%s,%s,%s): %v", owner, computer, capture, err)
				}
				if n := captureLabelScanCalls(s) - before; n != 0 {
					t.Fatalf("CaptureLabels made %d capture-label scans; want reads by key", n)
				}
				versions := make([]int, len(labels))
				for i, l := range labels {
					if l.OwnerID != owner || l.ComputerID != computer || l.CaptureID != capture {
						t.Fatalf("label of %s/%s/%s returned for %s/%s/%s", l.OwnerID, l.ComputerID, l.CaptureID, owner, computer, capture)
					}
					versions[i] = l.Version
				}
				if got := fmt.Sprint(versions); got != want {
					t.Fatalf("%s/%s/%s versions %s, the scan reads %s", owner, computer, capture, got, want)
				}
				found += len(labels)
				latest, ok, err := s.LatestCaptureLabel(ctx, owner, computer, capture)
				if err != nil || ok != (len(labels) > 0) || (ok && latest.Version != labels[len(labels)-1].Version) {
					t.Fatalf("LatestCaptureLabel = %+v %v %v", latest, ok, err)
				}
			}
		}
	}
	if found != 40 {
		t.Fatalf("read %d labels in all, appended 40", found)
	}
}

// captureLabelScanCalls counts metadata scans of capture labels.
func captureLabelScanCalls(s *Store) uint64 {
	var n uint64
	for op, st := range s.EngineMutexStats() {
		if strings.HasPrefix(op, "(*DoltStore).ListObjectsByMetadata") && strings.Contains(op, "choir.capture_label ") {
			n += st.Calls
		}
	}
	return n
}
