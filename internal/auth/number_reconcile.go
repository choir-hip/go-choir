package auth

// Numbered-address reconcile (owner, 2026-10-31: every new account gets an
// email address; existing real accounts are backfilled, oldest first).
//
// Auth owns the account list, so auth drives: a loop (at boot, on a nudge after
// registration, and on a timer) sends the real accounts, oldest first, to
// maild's host-internal assign-numbers route. Registration never waits on it
// and never fails because of it; if maild is down the next tick retries. maild
// makes each assignment idempotent, so the loop can repeat forever.

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
)

const (
	maildAssignNumbersPath = "/internal/maild/auth/assign-numbers"
	numberReconcileBatch   = 200
	numberReconcileTimeout = 30 * time.Second
	// NumberReconcileInterval is the retry cadence when nothing nudges.
	NumberReconcileInterval = 5 * time.Minute
)

// IsEphemeralEmailDomain reports whether an address is on a test or ephemeral
// domain that must never consume a user number: example.com/.org/.net, any
// *.example, *.test, *.invalid, and localhost. An unparseable address counts
// as ephemeral (nothing real to number).
func IsEphemeralEmailDomain(email string) bool {
	email = strings.ToLower(strings.TrimSpace(email))
	at := strings.LastIndex(email, "@")
	if at < 0 || at == len(email)-1 {
		return true
	}
	d := strings.TrimSuffix(email[at+1:], ".")
	switch d {
	case "example.com", "example.org", "example.net", "localhost", "example", "test", "invalid":
		return true
	}
	for _, suffix := range []string{".example", ".test", ".invalid", ".localhost", ".example.com", ".example.org", ".example.net"} {
		if strings.HasSuffix(d, suffix) {
			return true
		}
	}
	return false
}

const numberCandidatesQuery = `SELECT u.id, u.email FROM users u
	WHERE EXISTS (SELECT 1 FROM credentials c WHERE c.user_id = u.id)
	ORDER BY u.created_at ASC, u.id ASC`

func scanNumberCandidates(rows *sql.Rows) ([]string, error) {
	return scanCandidates(rows, false)
}

// scanCandidates reads (id, email) rows, keeping the real-domain accounts or,
// with ephemeral set, the test and ephemeral-domain ones.
func scanCandidates(rows *sql.Rows, ephemeral bool) ([]string, error) {
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id, email string
		if err := rows.Scan(&id, &email); err != nil {
			return nil, err
		}
		if IsEphemeralEmailDomain(email) != ephemeral {
			continue
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ListNumberCandidates returns the ids of accounts that should hold a number:
// a credential exists (registration finished) and the sign-in address is not
// on a test or ephemeral domain, oldest account first.
func (s *Store) ListNumberCandidates(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, numberCandidatesQuery)
	if err != nil {
		return nil, err
	}
	return scanNumberCandidates(rows)
}

// ListTestNumberCandidates returns the accounts on test or ephemeral sign-in
// domains that finished registration, oldest first. They get numbers from the
// test range (owner, 2026-11-04: every account has a numbered address, test
// accounts included); nothing bound them before.
func (s *Store) ListTestNumberCandidates(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, numberCandidatesQuery)
	if err != nil {
		return nil, err
	}
	return scanCandidates(rows, true)
}

// ListNumberCandidatesReadOnly does the same against an auth database file
// opened read-only, without running any migration (for the maildctl dry run).
func ListNumberCandidatesReadOnly(ctx context.Context, dbPath string) ([]string, error) {
	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.QueryContext(ctx, numberCandidatesQuery)
	if err != nil {
		return nil, err
	}
	return scanNumberCandidates(rows)
}

// NumberAssigner sends an ordered batch of account ids to maild.
type NumberAssigner interface {
	AssignNumbers(ctx context.Context, ownerIDs []string) error
	// AssignTestNumbers numbers test and ephemeral-domain accounts from the
	// test range.
	AssignTestNumbers(ctx context.Context, ownerIDs []string) error
}

type maildNumberAssigner struct {
	endpoint string
	client   *http.Client
}

// NewMaildNumberAssigner posts to maild's host-internal assign-numbers route.
func NewMaildNumberAssigner(maildURL string, client *http.Client) NumberAssigner {
	if client == nil {
		client = &http.Client{Timeout: numberReconcileTimeout}
	}
	return &maildNumberAssigner{endpoint: strings.TrimRight(strings.TrimSpace(maildURL), "/") + maildAssignNumbersPath, client: client}
}

func (m *maildNumberAssigner) AssignNumbers(ctx context.Context, ownerIDs []string) error {
	return m.post(ctx, map[string]any{"owner_ids": ownerIDs})
}

func (m *maildNumberAssigner) AssignTestNumbers(ctx context.Context, ownerIDs []string) error {
	return m.post(ctx, map[string]any{"test_owner_ids": ownerIDs})
}

func (m *maildNumberAssigner) post(ctx context.Context, payload map[string]any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	hostauth.MarkInternal(req)
	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("post assign numbers: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("maild assign numbers status %d", resp.StatusCode)
	}
	return nil
}

// ReconcileNumbers sends every candidate not yet confirmed, oldest first, in
// order-preserving batches. It stops at the first failed batch so a later
// account never gets a lower number than an earlier one.
func (h *Handler) ReconcileNumbers(ctx context.Context) error {
	if h.numberAssigner == nil {
		return nil
	}
	ids, err := h.store.ListNumberCandidates(ctx)
	if err != nil {
		return err
	}
	h.numbersMu.Lock()
	if h.numbersDone == nil {
		h.numbersDone = map[string]struct{}{}
	}
	var todo []string
	for _, id := range ids {
		if _, ok := h.numbersDone[id]; !ok {
			todo = append(todo, id)
		}
	}
	h.numbersMu.Unlock()
	for len(todo) > 0 {
		n := min(numberReconcileBatch, len(todo))
		if err := h.numberAssigner.AssignNumbers(ctx, todo[:n]); err != nil {
			return err
		}
		h.numbersMu.Lock()
		for _, id := range todo[:n] {
			h.numbersDone[id] = struct{}{}
		}
		h.numbersMu.Unlock()
		todo = todo[n:]
	}
	// Test and ephemeral-domain accounts, oldest first, from the test range.
	testIDs, err := h.store.ListTestNumberCandidates(ctx)
	if err != nil {
		return err
	}
	h.numbersMu.Lock()
	var testTodo []string
	for _, id := range testIDs {
		if _, ok := h.numbersDone[id]; !ok {
			testTodo = append(testTodo, id)
		}
	}
	h.numbersMu.Unlock()
	for len(testTodo) > 0 {
		n := min(numberReconcileBatch, len(testTodo))
		if err := h.numberAssigner.AssignTestNumbers(ctx, testTodo[:n]); err != nil {
			return err
		}
		h.numbersMu.Lock()
		for _, id := range testTodo[:n] {
			h.numbersDone[id] = struct{}{}
		}
		h.numbersMu.Unlock()
		testTodo = testTodo[n:]
	}
	return nil
}

// nudgeNumberReconcile asks the loop for a pass now. It never blocks.
func (h *Handler) nudgeNumberReconcile() {
	select {
	case h.numberNudge <- struct{}{}:
	default:
	}
}

// StartNumberReconciler runs a pass at boot, on every nudge, and every
// interval until ctx ends. A failed pass is logged and retried at the next
// tick; it is standing work, not a resumed run.
func (h *Handler) StartNumberReconciler(ctx context.Context, interval time.Duration) {
	if h.numberAssigner == nil {
		return
	}
	if interval <= 0 {
		interval = NumberReconcileInterval
	}
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			if err := h.ReconcileNumbers(ctx); err != nil {
				log.Printf("[auth] operation=number_reconcile result=error error=%q", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			case <-h.numberNudge:
			}
		}
	}()
}
