package computerversion

import (
	"context"
	"errors"
	"testing"
	"time"
)

// W3: the merge of spilled runs checks its context.
func TestRowSorterSumStopsWhenItsContextEnds(t *testing.T) {
	s := newRowSorter(t.TempDir(), 64)
	defer s.Close()
	for i := 0; i < 20000; i++ {
		if err := s.Add(string(rune('a'+i%26)) + time.Duration(i).String()); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Sum(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Sum on an ended context = %v, want context.Canceled", err)
	}
}
