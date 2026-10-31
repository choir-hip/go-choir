package maild

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestAssignUserNumberSequentialIdempotentRootExcluded(t *testing.T) {
	s, cfg := newTestStore(t)
	ctx := context.Background()
	// The root already holds 000 and never gets a second number.
	a, err := s.AssignUserNumber(ctx, cfg.PrimaryDomain, cfg.RootOwnerID, t0)
	if err != nil || a.Created || a.Number != 0 {
		t.Fatalf("root = %+v err=%v, want existing 000", a, err)
	}
	for i, want := range []int64{1, 2, 3} {
		a, err := s.AssignUserNumber(ctx, cfg.PrimaryDomain, fmt.Sprintf("acct-%d", i), t0)
		if err != nil || !a.Created || a.Number != want {
			t.Fatalf("acct-%d = %+v err=%v, want new %d", i, a, err, want)
		}
	}
	// Idempotent: acct-1 keeps 2, nothing is burned.
	a, err = s.AssignUserNumber(ctx, cfg.PrimaryDomain, "acct-1", t0)
	if err != nil || a.Created || a.Number != 2 {
		t.Fatalf("repeat = %+v err=%v", a, err)
	}
	b, err := s.AssignUserNumber(ctx, cfg.PrimaryDomain, "acct-9", t0)
	if err != nil || b.Number != 4 {
		t.Fatalf("next after repeat = %+v err=%v, want 4 (no gap)", b, err)
	}
	// The alias is on the public quarantining policy and routes to the owner.
	alias, err := s.ResolveAlias(ctx, cfg.PrimaryDomain, "001")
	if err != nil || alias.TargetID != "acct-0" || alias.ReceivePolicyID != DefaultPublicPolicyID {
		t.Fatalf("alias 001 = %+v err=%v", alias, err)
	}
}

func TestAssignUserNumberConcurrentNoDoubleNoGap(t *testing.T) {
	s, cfg := newTestStore(t)
	ctx := context.Background()
	const owners, dup = 12, 4
	var wg sync.WaitGroup
	var mu sync.Mutex
	got := map[string]int64{}
	for o := 0; o < owners; o++ {
		for d := 0; d < dup; d++ {
			wg.Add(1)
			go func(o int) {
				defer wg.Done()
				id := fmt.Sprintf("acct-%d", o)
				var a UserNumberAssignment
				var err error
				for try := 0; try < 50; try++ {
					if a, err = s.AssignUserNumber(ctx, cfg.PrimaryDomain, id, t0); err == nil {
						break
					}
				}
				if err != nil {
					t.Errorf("assign %s: %v", id, err)
					return
				}
				mu.Lock()
				defer mu.Unlock()
				if prev, ok := got[id]; ok && prev != a.Number {
					t.Errorf("%s got %d then %d", id, prev, a.Number)
				}
				got[id] = a.Number
			}(o)
		}
	}
	wg.Wait()
	seen := map[int64]bool{}
	for _, n := range got {
		if seen[n] || n < 1 || n > owners {
			t.Fatalf("number %d duplicated or outside 1..%d: %v", n, owners, got)
		}
		seen[n] = true
	}
	var rows int
	if err := s.routingDB.QueryRow(`SELECT count(*) FROM email_aliases WHERE canonical_number > 0 AND canonical_number < 10000000`).Scan(&rows); err != nil || rows != owners {
		t.Fatalf("alias rows = %d err=%v, want %d", rows, err, owners)
	}
}

func TestAssignUserNumberSkipsTestRange(t *testing.T) {
	s, cfg := newTestStore(t)
	seqSet(t, s, "user", 9_999_999)
	ctx := context.Background()
	a, _ := s.AssignUserNumber(ctx, cfg.PrimaryDomain, "last-low", t0)
	b, err := s.AssignUserNumber(ctx, cfg.PrimaryDomain, "first-high", t0)
	if err != nil || a.Number != 9_999_999 || b.Number != UserNumberBlockTwoStart {
		t.Fatalf("a=%+v b=%+v err=%v, want 9999999 then 110000000", a, b, err)
	}
}

func TestAssignUserNumberRefusedBeforeRootSeeded(t *testing.T) {
	s, _ := newTestStore(t)
	if _, err := s.routingDB.Exec(`DELETE FROM email_aliases WHERE canonical_number = 0`); err != nil {
		t.Fatal(err)
	}
	_, err := s.AssignUserNumber(context.Background(), "choir.news", "acct", t0)
	if !errors.Is(err, ErrNumberSchemaNotReady) {
		t.Fatalf("err = %v, want ErrNumberSchemaNotReady", err)
	}
	var next int64
	_ = s.routingDB.QueryRow(`SELECT next FROM email_number_sequence WHERE range_name='user'`).Scan(&next)
	if next != UserNumberStart {
		t.Fatalf("sequence moved to %d on a refused assignment", next)
	}
}

func TestAssignUserNumberFailureBurnsNothing(t *testing.T) {
	s, cfg := newTestStore(t)
	ctx := context.Background()
	// A bad policy reference makes the insert fail after the allocation.
	if _, err := s.routingDB.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.routingDB.Exec(`CREATE TRIGGER fail_user_alias BEFORE INSERT ON email_aliases
		WHEN NEW.target_id = 'bad' BEGIN SELECT RAISE(ABORT, 'boom'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AssignUserNumber(ctx, cfg.PrimaryDomain, "bad", t0); err == nil {
		t.Fatal("expected failure")
	}
	a, err := s.AssignUserNumber(ctx, cfg.PrimaryDomain, "good", t0)
	if err != nil || a.Number != 1 {
		t.Fatalf("after failure = %+v err=%v, want 1 (no gap)", a, err)
	}
}

func TestAssignUserNumberDisabledKeepsItsNumber(t *testing.T) {
	s, cfg := newTestStore(t)
	ctx := context.Background()
	a, _ := s.AssignUserNumber(ctx, cfg.PrimaryDomain, "acct", t0)
	if _, err := s.routingDB.Exec(`UPDATE email_aliases SET disabled_at = ? WHERE canonical_number = ?`, "2026-10-31T00:00:00Z", a.Number); err != nil {
		t.Fatal(err)
	}
	b, err := s.AssignUserNumber(ctx, cfg.PrimaryDomain, "acct", t0)
	if err != nil || b.Created || b.Number != a.Number {
		t.Fatalf("b=%+v err=%v, want the disabled number kept", b, err)
	}
}

func TestPlanUserNumbersMatchesAssignment(t *testing.T) {
	s, cfg := newTestStore(t)
	ctx := context.Background()
	if _, err := s.AssignUserNumber(ctx, cfg.PrimaryDomain, "old", t0); err != nil {
		t.Fatal(err)
	}
	order := []string{"old", cfg.RootOwnerID, "mid", "new", "mid"}
	plan, err := s.PlanUserNumbers(ctx, cfg.RootOwnerID, order)
	if err != nil {
		t.Fatal(err)
	}
	want := []PlannedUserNumber{{"old", 1, true}, {"mid", 2, false}, {"new", 3, false}}
	if fmt.Sprint(plan) != fmt.Sprint(want) {
		t.Fatalf("plan = %v, want %v", plan, want)
	}
	// Planning wrote nothing; assigning in plan order yields exactly the plan.
	var n int
	_ = s.routingDB.QueryRow(`SELECT count(*) FROM email_aliases WHERE target_id IN ('mid','new')`).Scan(&n)
	if n != 0 {
		t.Fatalf("plan wrote %d rows", n)
	}
	for _, p := range plan {
		a, err := s.AssignUserNumber(ctx, cfg.PrimaryDomain, p.OwnerID, t0)
		if err != nil || a.Number != p.Number {
			t.Fatalf("assign %s = %+v err=%v, planned %d", p.OwnerID, a, err, p.Number)
		}
	}
}

func assignRequest(body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, AuthAssignNumbersPath, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Caller", "true")
	return req
}

func TestHandleAuthAssignNumbers(t *testing.T) {
	s, cfg := newTestStore(t)
	h := NewHandler(cfg, s)
	w := httptest.NewRecorder()
	h.HandleAuthAssignNumbers(w, assignRequest(`{"owner_ids":["`+cfg.RootOwnerID+`","a","b"]}`))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"assigned":2`) {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var root sql.NullInt64
	_ = s.routingDB.QueryRow(`SELECT count(*) FROM email_aliases WHERE target_id = ?`, cfg.RootOwnerID).Scan(&root)
	if root.Int64 != 1 {
		t.Fatalf("root has %d aliases, want 1", root.Int64)
	}
	// A request that carries a user session is refused.
	req := assignRequest(`{"owner_ids":["c"]}`)
	req.Header.Set("X-Authenticated-User", "someone")
	w = httptest.NewRecorder()
	h.HandleAuthAssignNumbers(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("forwarded user request status = %d, want 403", w.Code)
	}
	// Missing internal marker.
	req = assignRequest(`{"owner_ids":["c"]}`)
	req.Header.Del("X-Internal-Caller")
	w = httptest.NewRecorder()
	h.HandleAuthAssignNumbers(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("unmarked status = %d, want 403", w.Code)
	}
}
