package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/auth"
)

func TestRunPlanUserNumbersDryRun(t *testing.T) {
	dbPath, storageRoot := setupMaildctlStore(t)
	authPath := filepath.Join(t.TempDir(), "auth.db")
	as, err := auth.OpenStore(authPath)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	add := func(id, email string, at time.Time, cred bool) {
		if _, err := as.DB().Exec(`INSERT INTO users (id, email, created_at) VALUES (?, ?, ?)`, id, email, at); err != nil {
			t.Fatal(err)
		}
		if cred {
			if _, err := as.DB().Exec(`INSERT INTO credentials (id, user_id, public_key, attestation_type, transport, aaguid, created_at)
				VALUES (?, ?, x'00', 'none', '[]', x'00', ?)`, "c-"+id, id, at); err != nil {
				t.Fatal(err)
			}
		}
	}
	add("owner-000", "owner@gmail.com", base, true)
	add("acct-later", "l@gmail.com", base.Add(2*time.Hour), true)
	add("acct-early", "e@gmail.com", base.Add(time.Hour), true)
	add("acct-test", "t@example.com", base.Add(time.Hour), true)
	add("acct-nocred", "n@gmail.com", base.Add(time.Hour), false)
	_ = as.Close()

	t.Setenv("MAILD_ROOT_OWNER_ID", "owner-000")
	var stdout, stderr bytes.Buffer
	code := run([]string{"plan-user-numbers", "--db", dbPath, "--storage-root", storageRoot, "--auth-db", authPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	t.Logf("dry run output:\n%s", stdout.String())
	out := stdout.String()
	if strings.Contains(out, "@") {
		t.Fatalf("plan printed an email address: %s", out)
	}
	for _, want := range []string{"acct-early -\\u003e 001 (new)", "acct-later -\\u003e 002 (new)"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %s in %s", want, out)
		}
	}
	for _, bad := range []string{"owner-000", "acct-test", "acct-nocred"} {
		if strings.Contains(out, bad) {
			t.Fatalf("plan includes %s: %s", bad, out)
		}
	}
}
