package store

import (
	"context"
	"testing"
)

// The store.compact_force setting makes a small, unfragmented store take a
// copy: the proof ring (d@e) is under the layout trigger's 1 GiB, so without
// it the copy and swap would first run on the paper.
func TestCompactCopyForced(t *testing.T) {
	s, _ := compactTestStore(t, 2, 3000, 50)
	if done, err := s.prepareCompactCopy(context.Background(), false); done || err != nil {
		t.Fatalf("unforced = %v, %v; want a quiet skip", done, err)
	}
	done, err := s.prepareCompactCopy(context.Background(), true)
	if err != nil || !done {
		t.Fatalf("forced = %v, %v; want a staged copy", done, err)
	}
	if !exists(s.statePath+compactStagedSuffix) || !exists(s.statePath+compactSidecarSuffix) {
		t.Fatal("forced run staged no copy")
	}
	// A fresh staged copy is kept, forced or not: force picks the trigger,
	// not the cadence.
	if done, err := s.prepareCompactCopy(context.Background(), true); done || err != nil {
		t.Fatalf("second forced run = %v, %v; want the fresh copy kept", done, err)
	}
}

func TestCompactForceSettingIsKnownAndOff(t *testing.T) {
	on, known := KnownComputerSettings[SettingCompactForce]
	if !known || on {
		t.Fatalf("%s known=%v default=%v; want known and off", SettingCompactForce, known, on)
	}
}
