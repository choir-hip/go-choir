package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/auth"
	"github.com/yusefmosiah/go-choir/internal/purgeguard"
)

// Round 3, S5: a purge that stops on a tape refusal (or anything else) stays
// purging and was left with no operator path. `auth deletion-ops` lists the
// open deletions by job (never by person) and clears named steps of one so the
// runner does them again on its next pass. It never finalizes and never
// deletes a record: a purge forced past a refusal is an owner act.
func opsFixture(t *testing.T) (dbPath, job, userID string) {
	t.Helper()
	dbPath = filepath.Join(t.TempDir(), "auth.db")
	store, err := auth.OpenStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	u, err := store.CreateUser("ops-user", "ops-user@example.com")
	if err != nil {
		t.Fatal(err)
	}
	long := time.Now().UTC().Add(-purgeguard.GracePeriod - time.Hour)
	d, _, err := store.RequestAccountDeletion(u.ID, long)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.BeginPurge(u.ID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	for _, step := range []string{"mail", "data", "computers", "gateway"} {
		if err := store.RecordDeletionStep(u.ID, step, auth.DeletionStep{DoneAt: time.Now().UTC()}); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.EndAttempt(u.ID, "tape refused: a computer is in a quiet window", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	return dbPath, d.JobID, u.ID
}

func TestDeletionOpsListsByJobAndNeverNamesThePerson(t *testing.T) {
	dbPath, job, userID := opsFixture(t)
	var out bytes.Buffer
	if code := runDeletionOps([]string{"list", "-db", dbPath}, &out); code != 0 {
		t.Fatalf("list: %d %s", code, out.String())
	}
	s := out.String()
	for _, want := range []string{job, "purging", "tape refused", "mail", "gateway"} {
		if !strings.Contains(s, want) {
			t.Fatalf("the list lacks %q: %s", want, s)
		}
	}
	for _, bad := range []string{userID, "ops-user", "example.com"} {
		if strings.Contains(s, bad) {
			t.Fatalf("the list names the person (%q): %s", bad, s)
		}
	}
}

func TestDeletionOpsRerunClearsNamedStepsAndNothingElse(t *testing.T) {
	dbPath, job, userID := opsFixture(t)
	var out bytes.Buffer
	if code := runDeletionOps([]string{"rerun", "-db", dbPath, "-job", job, "-steps", "tape,gateway"}, &out); code != 0 {
		t.Fatalf("rerun: %d %s", code, out.String())
	}
	store, err := auth.OpenStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	d, _ := store.AccountDeletionFor(userID)
	if d == nil || d.State != auth.DeletionPurging {
		t.Fatalf("the record after a rerun: %+v", d)
	}
	if _, ok := d.Steps["gateway"]; ok {
		t.Fatal("gateway was not cleared")
	}
	if _, ok := d.Steps["mail"]; !ok {
		t.Fatal("a step that was not named was cleared")
	}
	// Unknown step names, an unknown job and a missing job are refused and change nothing.
	for _, args := range [][]string{
		{"rerun", "-db", dbPath, "-job", job, "-steps", "finalize"},
		{"rerun", "-db", dbPath, "-job", "del-nope", "-steps", "tape"},
		{"rerun", "-db", dbPath, "-steps", "tape"},
	} {
		out.Reset()
		if code := runDeletionOps(args, &out); code == 0 {
			t.Fatalf("%v was accepted: %s", args, out.String())
		}
	}
	d, _ = store.AccountDeletionFor(userID)
	if _, ok := d.Steps["mail"]; !ok || len(d.Steps) != 3 {
		t.Fatalf("a refused rerun changed the record: %v", d.Steps)
	}
}
