package maild

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)

func seqSet(t *testing.T, s *Store, name string, next int64) {
	t.Helper()
	if _, err := s.routingDB.Exec(`UPDATE email_number_sequence SET next = ? WHERE range_name = ?`, next, name); err != nil {
		t.Fatal(err)
	}
}

func allocUser(t *testing.T, s *Store) int64 {
	t.Helper()
	tx, err := s.routingDB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	n, err := allocateNumber(context.Background(), tx, numberRangeUser)
	if err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestConcurrentTestReservationsAreUnique(t *testing.T) {
	s, _ := newTestStore(t)
	const n = 40
	var wg sync.WaitGroup
	var mu sync.Mutex
	seen := map[string]bool{}
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := s.ReserveTestNumber(context.Background(), "choir.news", t0, NumberReservationTTL)
			if err != nil {
				t.Errorf("reserve: %v", err)
				return
			}
			mu.Lock()
			defer mu.Unlock()
			if seen[r.Address] {
				t.Errorf("duplicate reservation %s", r.Address)
			}
			seen[r.Address] = true
		}()
	}
	wg.Wait()
	if len(seen) != n {
		t.Fatalf("got %d distinct, want %d", len(seen), n)
	}
	for a := range seen {
		var num int64
		_, _ = fmt.Sscanf(a, "%d@", &num)
		if num < TestNumberStart || num >= TestNumberEnd {
			t.Fatalf("test number %d out of range", num)
		}
	}
}

func TestReservedUnboundAddressDoesNotRoute(t *testing.T) {
	s, _ := newTestStore(t)
	r, err := s.ReserveTestNumber(context.Background(), "choir.news", t0, NumberReservationTTL)
	if err != nil {
		t.Fatal(err)
	}
	if r.LocalPart != "10000000" || r.Address != "10000000@choir.news" {
		t.Fatalf("first test number = %+v", r)
	}
	if _, err := s.ResolveAlias(context.Background(), "choir.news", r.LocalPart); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("unbound reserved alias resolved: %v", err)
	}
}

func TestBindRoutesToOwnerWithPublicPolicy(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	r, _ := s.ReserveTestNumber(ctx, "choir.news", t0, NumberReservationTTL)
	a, err := s.BindNumber(ctx, "choir.news", r.LocalPart, "acct-a", t0.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if a.TargetID != "acct-a" || a.ReceivePolicyID != DefaultPublicPolicyID {
		t.Fatalf("bound alias = %+v", a)
	}
	got, err := s.ResolveAlias(ctx, "choir.news", r.LocalPart)
	if err != nil || got.TargetID != "acct-a" {
		t.Fatalf("resolve = %+v %v", got, err)
	}
	// double bind by the same owner is idempotent
	if _, err := s.BindNumber(ctx, "choir.news", r.LocalPart, "acct-a", t0.Add(2*time.Hour)); err != nil {
		t.Fatalf("double bind: %v", err)
	}
	// a second account is refused
	if _, err := s.BindNumber(ctx, "choir.news", r.LocalPart, "acct-b", t0.Add(2*time.Hour)); !errors.Is(err, ErrNumberBound) {
		t.Fatalf("second account bind = %v, want ErrNumberBound", err)
	}
	// one numbered alias per owner
	r2, _ := s.ReserveTestNumber(ctx, "choir.news", t0, NumberReservationTTL)
	if _, err := s.BindNumber(ctx, "choir.news", r2.LocalPart, "acct-a", t0); err == nil {
		t.Fatal("owner got a second numbered alias")
	}
}

func TestReservationExpires(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	r, _ := s.ReserveTestNumber(ctx, "choir.news", t0, NumberReservationTTL)
	late := t0.Add(NumberReservationTTL + time.Second)
	if _, err := s.BindNumber(ctx, "choir.news", r.LocalPart, "acct-a", late); !errors.Is(err, ErrNumberExpired) {
		t.Fatalf("late bind = %v, want ErrNumberExpired", err)
	}
	// an expired number is burned, never reissued
	r2, _ := s.ReserveTestNumber(ctx, "choir.news", late, NumberReservationTTL)
	if r2.LocalPart == r.LocalPart {
		t.Fatal("expired number reissued")
	}
	if _, err := s.BindNumber(ctx, "choir.news", r.LocalPart, "acct-a", t0); !errors.Is(err, ErrNumberExpired) {
		t.Fatalf("bind after sweep = %v, want ErrNumberExpired", err)
	}
	if _, err := s.BindNumber(ctx, "choir.news", "10999999", "acct-a", t0); !errors.Is(err, ErrNumberNotReserved) {
		t.Fatalf("bind unreserved = %v", err)
	}
}

func TestNonCanonicalFormsRouteNowhere(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	for i, lp := range []string{"0", "00", "0000", "01", "0001", "000000000000000", "00+x"} {
		if _, err := s.routingDB.Exec(`INSERT INTO email_aliases (id, domain, local_part, target_type, target_id, visibility, receive_policy_id, created_at)
			VALUES (?, 'choir.news', ?, 'user', ?, 'public', ?, 'now')`, fmt.Sprintf("a%d", i), lp, fmt.Sprintf("x%d", i), DefaultPublicPolicyID); err != nil {
			t.Fatal(err)
		}
		if _, err := s.ResolveAlias(ctx, "choir.news", lp); !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("%q routed: %v", lp, err)
		}
	}
	if a, err := s.ResolveAlias(ctx, "choir.news", "000"); err != nil || a.CanonicalNumber != 0 {
		t.Fatalf("000 = %+v %v", a, err)
	}
	if n, ok := canonicalNumberFromLocalPart("000"); !ok || n != 0 {
		t.Fatal("000 must be canonical 0")
	}
	for _, lp := range []string{"abc", "0", "01", "000+x", "1234567890123"} {
		if _, ok := canonicalNumberFromLocalPart(lp); ok {
			t.Fatalf("%q must have no canonical number", lp)
		}
	}
	if n, ok := canonicalNumberFromLocalPart("1000"); !ok || n != 1000 {
		t.Fatal("1000 canonical")
	}
}

func TestPlusAliasOwnership(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	cfg := func(owner, lp string) TrustedWorkflowAliasConfig {
		return TrustedWorkflowAliasConfig{OwnerID: owner, Domain: "choir.news", LocalPart: lp, SenderAddress: "a@example.com"}
	}
	if _, err := s.ConfigureTrustedWorkflowAlias(ctx, cfg("user-root", "000+tag")); err != nil {
		t.Fatalf("owner of 000: %v", err)
	}
	if _, err := s.ConfigureTrustedWorkflowAlias(ctx, cfg("someone-else", "000+tag2")); err == nil {
		t.Fatal("non-owner created 000+tag2")
	}
	if _, err := s.ConfigureTrustedWorkflowAlias(ctx, cfg("someone-else", "001+tag")); err == nil {
		t.Fatal("plus-alias under an unassigned number")
	}
	if _, err := s.ConfigureTrustedWorkflowAlias(ctx, cfg("user-root", "01+tag")); err == nil {
		t.Fatal("non-canonical numeric base accepted")
	}
	r, _ := s.ReserveTestNumber(ctx, "choir.news", t0, NumberReservationTTL)
	if _, err := s.ConfigureTrustedWorkflowAlias(ctx, cfg("acct-a", r.LocalPart+"+x")); err == nil {
		t.Fatal("plus-alias under a reserved, unbound number")
	}
	if _, err := s.BindNumber(ctx, "choir.news", r.LocalPart, "acct-a", t0); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ConfigureTrustedWorkflowAlias(ctx, cfg("acct-a", r.LocalPart+"+x")); err != nil {
		t.Fatalf("bound owner: %v", err)
	}
	if _, err := s.ConfigureTrustedWorkflowAlias(ctx, cfg("acct-b", r.LocalPart+"+y")); err == nil {
		t.Fatal("other account created a plus-alias under acct-a's number")
	}
	if _, err := s.ConfigureTrustedWorkflowAlias(ctx, cfg("acct-b", "named+y")); err != nil {
		t.Fatalf("non-numeric namespace: %v", err)
	}
	// plus-aliases never carry the numbered-address index
	var n sql.NullInt64
	if err := s.routingDB.QueryRow(`SELECT canonical_number FROM email_aliases WHERE local_part = '000+tag'`).Scan(&n); err != nil || n.Valid {
		t.Fatalf("plus alias canonical_number = %v %v", n, err)
	}
}

func TestUserSequenceJumpsOverTestAndGapBlock(t *testing.T) {
	s, _ := newTestStore(t)
	if got := allocUser(t, s); got != 1 {
		t.Fatalf("first user number = %d, want 1", got)
	}
	seqSet(t, s, "user", UserNumberBlockOneLast)
	if got := allocUser(t, s); got != UserNumberBlockOneLast {
		t.Fatalf("got %d", got)
	}
	if got := allocUser(t, s); got != UserNumberBlockTwoStart {
		t.Fatalf("after 9,999,999 got %d, want 110,000,000", got)
	}
	if got := allocUser(t, s); got != UserNumberBlockTwoStart+1 {
		t.Fatalf("counting on: %d", got)
	}
}

func TestNoUserNumberInReservedBlock(t *testing.T) {
	s, _ := newTestStore(t)
	for _, start := range []int64{UserNumberBlockOneLast, TestNumberStart, TestNumberStart + 5, TestNumberEnd, TestNumberEnd + 1, UserNumberBlockTwoStart - 1} {
		seqSet(t, s, "user", start)
		for i := 0; i < 3; i++ {
			n := allocUser(t, s)
			if n >= TestNumberStart && n < UserNumberBlockTwoStart {
				t.Fatalf("user number %d in [10M,110M) (seq started at %d)", n, start)
			}
		}
	}
	for i := 0; i < 5; i++ {
		r, err := s.ReserveTestNumber(context.Background(), "choir.news", t0, NumberReservationTTL)
		if err != nil {
			t.Fatal(err)
		}
		var n int64
		_, _ = fmt.Sscanf(r.LocalPart, "%d", &n)
		if n < TestNumberStart || n >= TestNumberEnd {
			t.Fatalf("test number %d outside its range", n)
		}
	}
}

func TestTestRangeRefusesAtCeiling(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	seqSet(t, s, "test", TestNumberEnd-1)
	r, err := s.ReserveTestNumber(ctx, "choir.news", t0, NumberReservationTTL)
	if err != nil || r.LocalPart != "99999999" {
		t.Fatalf("last test number = %+v %v", r, err)
	}
	if _, err := s.ReserveTestNumber(ctx, "choir.news", t0, NumberReservationTTL); !errors.Is(err, ErrTestRangeExhausted) {
		t.Fatalf("at 100,000,000 got %v, want ErrTestRangeExhausted", err)
	}
	var next int64
	_ = s.routingDB.QueryRow(`SELECT next FROM email_number_sequence WHERE range_name='test'`).Scan(&next)
	if next != TestNumberEnd {
		t.Fatalf("sequence moved past the ceiling: %d", next)
	}
	var spilled int
	_ = s.routingDB.QueryRow(`SELECT count(*) FROM email_aliases WHERE local_part='100000000'`).Scan(&spilled)
	if spilled != 0 {
		t.Fatal("spilled alias created")
	}
}

func TestSchemaRepairsAmbiguousCanonicalNumber(t *testing.T) {
	s, cfg := newTestStore(t)
	// simulate the legacy state: a plus alias and a non-numeric alias carrying 0
	if _, err := s.routingDB.Exec(`DROP INDEX IF EXISTS idx_email_aliases_canonical_number`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.routingDB.Exec(`DROP INDEX IF EXISTS idx_email_aliases_numbered_target`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.routingDB.Exec(`INSERT INTO email_aliases (id, domain, local_part, canonical_number, target_type, target_id, visibility, receive_policy_id, created_at)
		VALUES ('l1','choir.news','000+uitest',0,'user','u1','unlisted',?, 'n'), ('l2','choir.news','named',0,'user','u2','unlisted',?, 'n')`, DefaultPublicPolicyID, DefaultPublicPolicyID); err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureSchema(cfg); err != nil {
		t.Fatalf("re-ensure: %v", err)
	}
	var c int
	_ = s.routingDB.QueryRow(`SELECT count(*) FROM email_aliases WHERE canonical_number = 0`).Scan(&c)
	if c != 1 {
		t.Fatalf("%d aliases still carry canonical_number 0, want only 000", c)
	}
	if _, err := s.routingDB.Exec(`INSERT INTO email_aliases (id, domain, local_part, canonical_number, target_type, target_id, visibility, receive_policy_id, created_at)
		VALUES ('dup','choir.news','x',0,'user','u9','public',?, 'n')`, DefaultPublicPolicyID); err == nil {
		t.Fatal("unique index on canonical_number missing")
	}
}
