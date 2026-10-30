package main

// The checkpoint verdict pass after the projection bases, written before the
// code. Ways it could fail:
//
//  1. a pending checkpoint stays pending forever (nobody rebuilds it);
//  2. a Dolt-era witness (version 1 or 2) is settled as no_base instead of
//     naming its version, so the listing never says why it cannot be judged;
//  3. one checkpoint's store error stops the rest;
//  4. a checkpoint is settled verified or not_derivable with no rebuild to
//     back it.

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

type fakeVerdictStore struct {
	pending  []platform.CheckpointVerdictRecord
	versions map[string]int // by digest
	failRead map[string]bool
	resolved map[string]selfdevprotocol.CheckpointVerdict
}

func (f *fakeVerdictStore) PendingCheckpointVerdicts(context.Context) ([]platform.CheckpointVerdictRecord, error) {
	var out []platform.CheckpointVerdictRecord
	for _, r := range f.pending {
		if _, done := f.resolved[r.CheckpointDigest]; !done {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeVerdictStore) ResolveCheckpointVerdict(_ context.Context, _, digest string, v selfdevprotocol.CheckpointVerdict) (bool, error) {
	f.resolved[digest] = v
	return true, nil
}

func (f *fakeVerdictStore) StoredCheckpoint(_ context.Context, _, digest string) (selfdevprotocol.Checkpoint, error) {
	if f.failRead[digest] {
		return selfdevprotocol.Checkpoint{}, errors.New("store unavailable")
	}
	var c selfdevprotocol.Checkpoint
	c.Request.VMLocalContentWitness.WitnessVersion = f.versions[digest]
	return c, nil
}

func TestReconcileSettlesEveryPendingCheckpointUnverified(t *testing.T) {
	store := &fakeVerdictStore{
		versions: map[string]int{"v3": 3, "v2": 2, "v1": 0, "broken": 3},
		failRead: map[string]bool{"broken": true},
		resolved: map[string]selfdevprotocol.CheckpointVerdict{},
	}
	for _, digest := range []string{"broken", "v3", "v2", "v1"} {
		store.pending = append(store.pending, platform.CheckpointVerdictRecord{ComputerID: "c", CheckpointDigest: digest})
	}
	err := reconcileCheckpointVerdicts(context.Background(), store)
	if err == nil || !strings.Contains(err.Error(), "broken") {
		t.Fatalf("a failing checkpoint's error was lost: %v", err)
	}
	for _, digest := range []string{"v3", "v2", "v1"} {
		v, ok := store.resolved[digest]
		if !ok {
			t.Fatalf("checkpoint %s stayed pending", digest)
		}
		if v.Status != selfdevprotocol.CheckpointVerdictUnverified {
			t.Fatalf("checkpoint %s settled %q without a rebuild", digest, v.Status)
		}
	}
	if got := store.resolved["v3"].Reason; got != selfdevprotocol.VerdictReasonNoBase {
		t.Fatalf("v3 reason = %q, want no_base", got)
	}
	for _, digest := range []string{"v2", "v1"} {
		v := store.resolved[digest]
		if v.Reason != selfdevprotocol.VerdictReasonWitnessVersion || !strings.Contains(v.Detail, "not judgeable") {
			t.Fatalf("%s verdict = %+v, want witness_version with a reason", digest, v)
		}
	}
}
