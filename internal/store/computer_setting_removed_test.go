package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

// A computer may hold a stored row for a setting that was later removed
// (docs/problems/research-economy-switches-removed-2026-10-26.md: b@c has
// research.run_memory_elision and research.search_caps stored true; d@e and
// the paper stored false). The row stays on the tape. Listing the computer's
// settings must not fail on it and must not show it; putting a removed key
// is refused like any unknown key.
func TestStoredRowForARemovedSettingIsInvisibleAndHarmless(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	const owner, computer = "owner-removed", "computer-removed"
	removed := []string{"research.run_memory_elision", "research.search_caps"}
	for _, key := range removed {
		// The raw row, as an older build wrote it.
		setting := ComputerSetting{OwnerID: owner, ComputerID: computer, Key: key, Value: true, UpdatedAt: time.Now().UTC()}
		metadata := map[string]any{"computer_id": computer, "key": key, "value": true}
		if _, err := s.ogPut(ctx, ogKindComputerSetting, owner, computerSettingKey(computer, key), setting, metadata, setting.UpdatedAt); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.PutComputerSetting(ctx, ComputerSetting{OwnerID: owner, ComputerID: computer, Key: SettingIntakeLabels, Value: true}); err != nil {
		t.Fatal(err)
	}
	listed, err := s.ListComputerSettings(ctx, owner, computer)
	if err != nil {
		t.Fatalf("listing with stored rows for removed keys: %v", err)
	}
	if len(listed) != len(KnownComputerSettings) {
		t.Fatalf("listed %d settings, want the %d known", len(listed), len(KnownComputerSettings))
	}
	for _, got := range listed {
		for _, key := range removed {
			if got.Key == key {
				t.Fatalf("removed key %s is listed", key)
			}
		}
		if got.Key == SettingIntakeLabels && !got.Value {
			t.Fatal("a known stored setting lost its value")
		}
	}
	for _, key := range removed {
		if _, known := KnownComputerSettings[key]; known {
			t.Fatalf("%s is still known", key)
		}
		if err := s.PutComputerSetting(ctx, ComputerSetting{OwnerID: owner, ComputerID: computer, Key: key, Value: false}); err == nil {
			t.Fatalf("PutComputerSetting stored the removed key %s", key)
		}
		if _, err := s.GetComputerSetting(ctx, owner, "another-computer", key); !errors.Is(err, ErrNotFound) {
			t.Fatalf("Get of a removed key on another computer = %v, want ErrNotFound", err)
		}
	}
}
