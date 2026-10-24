package vmctl

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// The boot credential an offline maintenance hook outlives
// (docs/problems/offline-maintenance-outlives-the-boot-credential-2026-10-24.md).
// Failure modes, vmctl side:
//  1. a hook longer than the envelope's life boots a stale envelope;
//  2. a plain refresh issues more than it does today;
//  3. a second re-mint for one realization issues a second envelope
//     instead of replaying the first.

// clockedCorpusd mirrors corpusd's issuance rules on a test clock: a key's
// first request records its issue time, a later one replays from it, and a
// replay at or past expiry is refused (mintComputerCredentialEnvelope).
type clockedCorpusd struct {
	mu       sync.Mutex
	now      time.Time
	ttl      time.Duration
	issued   map[string]time.Time
	requests []map[string]string
}

func newClockedCorpusd(t *testing.T) (*clockedCorpusd, string) {
	t.Helper()
	c := &clockedCorpusd{now: time.Date(2026, 10, 24, 4, 0, 0, 0, time.UTC), ttl: 10 * time.Minute, issued: map[string]time.Time{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/computers/credentials/issue" {
			http.Error(w, "unexpected", http.StatusBadRequest)
			return
		}
		var request map[string]string
		_ = json.NewDecoder(r.Body).Decode(&request)
		key := request["idempotency_key"]
		c.mu.Lock()
		c.requests = append(c.requests, request)
		issuedAt, ok := c.issued[key]
		if !ok {
			issuedAt = c.now
			c.issued[key] = issuedAt
		}
		expiresAt := issuedAt.Add(c.ttl)
		live := c.now.Before(expiresAt)
		c.mu.Unlock()
		if !live {
			http.Error(w, "credential issuance refused", http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"envelope": map[string]any{
			"computer_id": request["computer_id"], "realization_id": request["realization_id"],
			"issuance_idempotency_key": key,
			"issued_at":                issuedAt.Format(time.RFC3339Nano),
			"expires_at":               expiresAt.Format(time.RFC3339Nano),
		}})
	}))
	t.Cleanup(srv.Close)
	return c, srv.URL
}

func (c *clockedCorpusd) advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.mu.Unlock()
}

func (c *clockedCorpusd) clock() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clockedCorpusd) requestsSince(n int) []map[string]string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]map[string]string(nil), c.requests[n:]...)
}

func (c *clockedCorpusd) requestCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.requests)
}

type decodedEnvelope struct {
	RealizationID string `json:"realization_id"`
	Key           string `json:"issuance_idempotency_key"`
	IssuedAt      string `json:"issued_at"`
	ExpiresAt     string `json:"expires_at"`
}

func decodeBootEnvelope(t *testing.T, encoded string) decodedEnvelope {
	t.Helper()
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("boot envelope %q is not base64url: %v", encoded, err)
	}
	var env decodedEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("boot envelope: %v", err)
	}
	return env
}

func newMaintenanceCredentialRegistry(t *testing.T, user string) (*OwnershipRegistry, *mockVMManager, *clockedCorpusd) {
	t.Helper()
	corpusd, url := newClockedCorpusd(t)
	mgr := &mockVMManager{maintenanceImage: "/state/vm/data.img"}
	reg := NewOwnershipRegistry("")
	reg.SetCorpusdURL(url)
	reg.SetVMManager(mgr)
	if _, err := reg.ResolveOrAssign(user); err != nil {
		t.Fatal(err)
	}
	return reg, mgr, corpusd
}

func TestMaintenanceLongerThanEnvelopeBootsUnexpiredEnvelope(t *testing.T) {
	reg, mgr, corpusd := newMaintenanceCredentialRegistry(t, "owner-long-compact")
	before := corpusd.requestCount()
	// The 2026-10-24 compaction: 11 min 27 s, past the 10-minute life.
	if _, err := reg.RefreshVMForDesktopWithMaintenance("owner-long-compact", PrimaryDesktopID, func(string) error {
		corpusd.advance(11*time.Minute + 27*time.Second)
		return nil
	}); err != nil {
		t.Fatalf("refresh with maintenance: %v", err)
	}
	if len(mgr.refreshCfgs) != 1 || len(mgr.bootedEnvelopes) != 1 {
		t.Fatalf("refreshes=%d booted=%d, want one each", len(mgr.refreshCfgs), len(mgr.bootedEnvelopes))
	}
	cfg := mgr.refreshCfgs[0]
	env := decodeBootEnvelope(t, mgr.bootedEnvelopes[0])
	expiresAt, err := time.Parse(time.RFC3339Nano, env.ExpiresAt)
	if err != nil {
		t.Fatal(err)
	}
	if !corpusd.clock().Before(expiresAt) {
		t.Fatalf("the boot envelope expired at %s, before the boot at %s (key %s)", env.ExpiresAt, corpusd.clock().Format(time.RFC3339Nano), env.Key)
	}
	if env.RealizationID != cfg.RealizationID {
		t.Fatalf("re-minted envelope binds %s, want the reserved realization %s", env.RealizationID, cfg.RealizationID)
	}
	want := fmt.Sprintf("guest-credential-maintained:%s:%d", cfg.RealizationID, cfg.Epoch)
	if env.Key != want {
		t.Fatalf("boot envelope key = %q, want %q (the trace must name the re-mint)", env.Key, want)
	}
	requests := corpusd.requestsSince(before)
	if len(requests) != 2 || requests[0]["idempotency_key"] != fmt.Sprintf("guest-credential:%s:%d", cfg.RealizationID, cfg.Epoch) || requests[1]["idempotency_key"] != want {
		t.Fatalf("issuance requests = %v, want the boot key before the kill and the maintained key after the hook", requests)
	}
}

func TestRefreshWithoutMaintenanceIssuesOnceAsToday(t *testing.T) {
	reg, mgr, corpusd := newMaintenanceCredentialRegistry(t, "owner-plain-refresh")
	before := corpusd.requestCount()
	if _, err := reg.RefreshVMForDesktop("owner-plain-refresh", PrimaryDesktopID); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if len(mgr.refreshCfgs) != 1 {
		t.Fatalf("refreshes = %d", len(mgr.refreshCfgs))
	}
	cfg := mgr.refreshCfgs[0]
	if cfg.PostMaintenanceCredential != nil {
		t.Fatal("a plain refresh carried a re-mint")
	}
	requests := corpusd.requestsSince(before)
	if len(requests) != 1 || requests[0]["idempotency_key"] != fmt.Sprintf("guest-credential:%s:%d", cfg.RealizationID, cfg.Epoch) {
		t.Fatalf("issuance requests = %v, want one under the boot key", requests)
	}
	if len(mgr.bootedEnvelopes) != 1 || mgr.bootedEnvelopes[0] != cfg.ComputerCredentialEnvelope {
		t.Fatal("a plain refresh booted an envelope other than the one minted with its config")
	}
}

func TestMaintenanceReMintReplaysForSameRealization(t *testing.T) {
	reg, mgr, corpusd := newMaintenanceCredentialRegistry(t, "owner-remint-replay")
	if _, err := reg.RefreshVMForDesktopWithMaintenance("owner-remint-replay", PrimaryDesktopID, func(string) error {
		corpusd.advance(11 * time.Minute)
		return nil
	}); err != nil {
		t.Fatalf("refresh with maintenance: %v", err)
	}
	remint := mgr.refreshCfgs[0].PostMaintenanceCredential
	if remint == nil {
		t.Fatal("a maintenance refresh carried no re-mint")
	}
	before := corpusd.requestCount()
	corpusd.advance(time.Minute)
	again, err := remint()
	if err != nil {
		t.Fatalf("retried re-mint: %v", err)
	}
	if again != mgr.bootedEnvelopes[0] {
		t.Fatalf("a retried boot of the same realization got a new envelope:\n first %s\n again %s", mgr.bootedEnvelopes[0], again)
	}
	requests := corpusd.requestsSince(before)
	if len(requests) != 1 || requests[0]["idempotency_key"] != decodeBootEnvelope(t, again).Key {
		t.Fatalf("retry requests = %v, want one under the same maintained key", requests)
	}
}
