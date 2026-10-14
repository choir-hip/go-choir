package vmctl

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

// The test tier (docs/reports/resource-efficiency-memory-assessment-2026-10-13.md,
// R2, lever 2). Ways this could fail, written before the code: a running
// tracer computer, protected by recent activity, outlives a real user's
// idle computer under pressure; a paid or platform account whose email
// looks ephemeral is demoted; a busy test run is killed mid-flight; an
// unreadable auth DB empties the set or crashes the sweep.

func TestTestClassRanksBelowEveryRealTier(t *testing.T) {
	cfg := tierConfig()
	cfg.TestUserIDs = map[string]bool{"qa-user": true, "paid-user": true}
	if got := warmnessClassForOwnership(&VMOwnership{UserID: "qa-user", ComputerID: "computer-qa"}, cfg); got != WarmnessClassTest {
		t.Fatalf("qa class = %s, want test", got)
	}
	if got := warmnessClassForOwnership(&VMOwnership{UserID: "paid-user", ComputerID: "computer-p"}, cfg); got != WarmnessClassPremiumAlwaysOn {
		t.Fatalf("a paid account listed as test = %s, want premium", got)
	}
	if got := warmnessClassForOwnership(&VMOwnership{UserID: "qa-user", ComputerID: paperComputerID}, cfg); got != WarmnessClassCriticalProtected {
		t.Fatalf("a platform computer of a test-looking owner = %s", got)
	}
	if warmnessPriority(WarmnessClassTest) >= warmnessPriority(WarmnessClassPrimary) {
		t.Fatal("test must rank below the free tier")
	}
}

func TestPressureReclaimsARecentTestComputerBeforeAnIdleRealOne(t *testing.T) {
	sample := ampleMemory()
	sample.MemoryAvailableBytes = 512
	sample.MemoryAvailablePercent = 2
	reg := signalRegistry(t, sample) // idle-user: real, idle 2 h
	cfg := tierConfig()
	cfg.TestUserIDs = map[string]bool{"qa-user": true}
	reg.SetWarmnessPolicyConfig(cfg)
	if _, err := reg.ResolveOrAssign("qa-user"); err != nil {
		t.Fatal(err)
	}
	reg.mu.Lock()
	reg.ownerships[ownershipKey("qa-user", PrimaryDesktopID)].LastActiveAt = time.Now().Add(-time.Minute)
	reg.mu.Unlock()

	got := reg.pressureReclaimActionCandidates()
	if len(got) == 0 || got[0].own.UserID != "qa-user" {
		t.Fatalf("first reclaim candidate = %+v, want the test computer", got)
	}

	reg.setGuestBusyProbeForTest(func(own *VMOwnership) bool { return own.UserID == "qa-user" })
	for _, c := range reg.pressureReclaimActionCandidates() {
		if c.own.UserID == "qa-user" {
			t.Fatal("a busy test computer was offered for reclaim")
		}
	}
}

func TestRefreshTestAccountsReadsTheAuthDBAndSurvivesItsLoss(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE users (id TEXT PRIMARY KEY, email TEXT UNIQUE NOT NULL, created_at DATETIME NOT NULL);
		INSERT INTO users VALUES ('qa-1','tracer-1@example.com','2026-10-13'),('real-1','someone@mosiah.org','2026-10-13'),('diagnostic-7','d@mosiah.org','2026-10-13');`); err != nil {
		t.Fatal(err)
	}
	db.Close()

	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetRetentionPruneConfig(RetentionPruneConfig{AuthDBPath: path, EphemeralEmailDomains: []string{"example.com"}, EphemeralUserIDPrefixes: []string{"diagnostic-"}})
	reg.SetWarmnessPolicyConfig(tierConfig())
	reg.refreshTestAccounts()
	cfg := reg.warmnessPolicySnapshot()
	if !cfg.TestUserIDs["qa-1"] || !cfg.TestUserIDs["diagnostic-7"] || cfg.TestUserIDs["real-1"] {
		t.Fatalf("test accounts = %v", cfg.TestUserIDs)
	}
	if !cfg.PlatformComputerIDs[paperComputerID] {
		t.Fatal("refresh dropped the platform tier")
	}

	reg.SetRetentionPruneConfig(RetentionPruneConfig{AuthDBPath: filepath.Join(t.TempDir(), "missing", "auth.db"), EphemeralEmailDomains: []string{"example.com"}})
	reg.refreshTestAccounts()
	if !reg.warmnessPolicySnapshot().TestUserIDs["qa-1"] {
		t.Fatal("an unreadable auth DB emptied the test set")
	}
}
