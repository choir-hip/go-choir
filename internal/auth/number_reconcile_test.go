package auth

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestIsEphemeralEmailDomain(t *testing.T) {
	for _, e := range []string{"a@example.com", "a@EXAMPLE.org", "a@example.net", "a@x.example", "a@foo.test", "a@bar.invalid", "a@localhost", "a@mail.example.com", "nodomain", "a@"} {
		if !IsEphemeralEmailDomain(e) {
			t.Errorf("%q should be ephemeral", e)
		}
	}
	for _, e := range []string{"a@gmail.com", "a@choir.news", "a@notexample.com", "a@contest.io"} {
		if IsEphemeralEmailDomain(e) {
			t.Errorf("%q should be real", e)
		}
	}
}

func addAccount(t *testing.T, s *Store, id, email string, created time.Time, withCredential bool) {
	t.Helper()
	if _, err := s.db.Exec(`INSERT INTO users (id, email, created_at) VALUES (?, ?, ?)`, id, email, created); err != nil {
		t.Fatal(err)
	}
	if withCredential {
		if _, err := s.db.Exec(`INSERT INTO credentials (id, user_id, public_key, attestation_type, transport, aaguid, created_at)
			VALUES (?, ?, x'00', 'none', '[]', x'00', ?)`, "cred-"+id, id, created); err != nil {
			t.Fatal(err)
		}
	}
}

func TestListNumberCandidatesOldestFirstRealOnly(t *testing.T) {
	s := TestStore(t)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	addAccount(t, s, "third", "c@gmail.com", base.Add(3*time.Hour), true)
	addAccount(t, s, "first", "a@gmail.com", base.Add(1*time.Hour), true)
	addAccount(t, s, "unfinished", "u@gmail.com", base.Add(90*time.Minute), false)
	addAccount(t, s, "testacct", "t@example.com", base.Add(2*time.Hour), true)
	addAccount(t, s, "second", "b@choir.news", base.Add(150*time.Minute), true)
	got, err := s.ListNumberCandidates(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(got) != "[first second third]" {
		t.Fatalf("candidates = %v", got)
	}
}

type fakeAssigner struct {
	mu      sync.Mutex
	batches [][]string
	fail    bool
}

func (f *fakeAssigner) AssignNumbers(_ context.Context, ids []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail {
		return errors.New("maild unreachable")
	}
	f.batches = append(f.batches, append([]string(nil), ids...))
	return nil
}

func TestReconcileRetriesAfterMaildOutageWithoutReordering(t *testing.T) {
	s := TestStore(t)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	addAccount(t, s, "a", "a@gmail.com", base, true)
	addAccount(t, s, "b", "b@gmail.com", base.Add(time.Hour), true)
	f := &fakeAssigner{fail: true}
	h := &Handler{store: s, numberAssigner: f, numberNudge: make(chan struct{}, 1)}
	if err := h.ReconcileNumbers(context.Background()); err == nil {
		t.Fatal("expected the outage to surface")
	}
	f.fail = false
	addAccount(t, s, "c", "c@gmail.com", base.Add(2*time.Hour), true)
	if err := h.ReconcileNumbers(context.Background()); err != nil {
		t.Fatal(err)
	}
	// A later pass sends only what is new.
	addAccount(t, s, "d", "d@gmail.com", base.Add(3*time.Hour), true)
	if err := h.ReconcileNumbers(context.Background()); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(f.batches) != "[[a b c] [d]]" {
		t.Fatalf("batches = %v", f.batches)
	}
}

func TestNudgeNeverBlocks(t *testing.T) {
	h := &Handler{numberNudge: make(chan struct{}, 1)}
	for i := 0; i < 5; i++ {
		h.nudgeNumberReconcile()
	}
}

func TestListNumberCandidatesReadOnlyOnFile(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/auth.db"
	s, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	addAccount(t, s, "x", "x@gmail.com", time.Now(), true)
	_ = s.Close()
	got, err := ListNumberCandidatesReadOnly(context.Background(), path)
	if err != nil || fmt.Sprint(got) != "[x]" {
		t.Fatalf("got %v err=%v", got, err)
	}
}
