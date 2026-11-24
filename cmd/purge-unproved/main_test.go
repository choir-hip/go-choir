package main

import (
	"bytes"
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/auth"
	"github.com/yusefmosiah/go-choir/internal/dbx/dbxtest"
	"github.com/yusefmosiah/go-choir/internal/maild"
	"github.com/yusefmosiah/go-choir/internal/platform"
)

func TestMain(m *testing.M) { os.Exit(dbxtest.Main(m)) }

// The whole command, through its flags, against the real schemas of every service: a dry run, then the real run on
// a receipt path that does not exist yet, then the same again (a rerun) on the receipt the first run left.
func TestTheCommandDryRunApplyAndRerun(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	authPath, mailPath, mailRoot := filepath.Join(dir, "auth.db"), filepath.Join(dir, "mail.db"), filepath.Join(dir, "mailroot")

	as, err := auth.OpenStore(authPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range []struct{ id, email string }{{"proved", "proved@example.com"}, {"goes", "goes@example.com"}} {
		if _, err := as.CreateUser(u.id, u.email); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := as.DB().Exec(`UPDATE users SET email_verified_at = NULL WHERE id = 'goes'`); err != nil {
		t.Fatal(err)
	}
	if err := as.Close(); err != nil {
		t.Fatal(err)
	}

	ms, err := maild.OpenStore(mailPath, mailRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := ms.EnsureSchema(&maild.Config{DBPath: mailPath, StorageRoot: mailRoot, PrimaryDomain: "choir.news", RootOwnerID: "root"}); err != nil {
		t.Fatal(err)
	}
	if err := ms.Close(); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"proved", "goes"} {
		if err := os.MkdirAll(filepath.Join(mailRoot, "users", id), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	cdsn, gdsn := dbxtest.PostgresDSN(t), dbxtest.PostgresDSN(t)
	ps, err := platform.OpenStore(ctx, cdsn, gdsn)
	if err != nil {
		t.Fatal(err)
	}
	defer ps.Close()

	vm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ownerships":[{"user_id":"somebody-else"}],"count":1}`))
	}))
	defer vm.Close()

	receipt := filepath.Join(dir, "receipts", "purge.jsonl")
	if err := os.MkdirAll(filepath.Dir(receipt), 0o755); err != nil {
		t.Fatal(err)
	}
	base := []string{"--auth-db", authPath, "--maild-db", mailPath, "--maild-root", mailRoot + "/",
		"--pg-control-dsn", cdsn, "--pg-graph-dsn", gdsn, "--vmctl-url", vm.URL}
	do := func(extra ...string) (int, string, string) {
		var out, errb bytes.Buffer
		code := run(append(append([]string{}, base...), extra...), &out, &errb)
		return code, out.String(), errb.String()
	}
	digestOf := func(out string) string {
		m := regexp.MustCompile(`order does not matter\): ([0-9a-f]+)`).FindStringSubmatch(out)
		if m == nil {
			t.Fatalf("no digest in:\n%s", out)
		}
		return m[1]
	}
	users := func() int {
		db, err := sql.Open("sqlite", authPath)
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	code, out, errs := do()
	if code != 0 || !strings.Contains(out, "accounts to purge: 1") || users() != 2 {
		t.Fatalf("dry run: %d\n%s\n%s", code, out, errs)
	}
	digest := digestOf(out)

	// the first real run: a receipt path that does not exist yet (the opening of it must not break the read of the last run)
	code, out, errs = do("--apply", "--digest", digest, "--receipt", receipt)
	if code != 0 || users() != 1 {
		t.Fatalf("apply: %d, users %d\n%s\n%s", code, users(), out, errs)
	}
	if _, err := os.Stat(filepath.Join(mailRoot, "users", "goes")); err == nil {
		t.Fatal("the mailbox directory stayed")
	}
	b, _ := os.ReadFile(receipt)
	for _, want := range []string{`"stage":"intent"`, `"stage":"before-commit:auth"`, `"stage":"after-commit:auth"`, `"stage":"done"`} {
		if !strings.Contains(string(b), want) {
			t.Fatalf("the receipt lacks %s:\n%s", want, b)
		}
	}

	// a rerun: nothing left, the receipt stays readable and is closed again, with no warning
	code, out, errs = do()
	if code != 0 || !strings.Contains(out, "accounts to purge: 0") {
		t.Fatalf("rerun dry run: %d\n%s\n%s", code, out, errs)
	}
	code, out, errs = do("--apply", "--digest", digestOf(out), "--receipt", receipt)
	if code != 0 || strings.Contains(errs, "WARNING") {
		t.Fatalf("rerun apply: %d\n%s\n%s", code, out, errs)
	}
	lines := strings.Split(strings.TrimSpace(func() string { b, _ := os.ReadFile(receipt); return string(b) }()), "\n")
	if last := lines[len(lines)-1]; !strings.Contains(last, `"stage":"done"`) {
		t.Fatalf("the rerun left the receipt open: %s", last)
	}

	// refusals before anything is touched: the same database twice, and a receipt that cannot be opened
	if code, _, _ := run2(base, "--pg-graph-dsn", cdsn); code == 0 {
		t.Fatal("the same database as control and graph was accepted")
	}
	if code, _, errs := do("--apply", "--digest", "x", "--receipt", filepath.Join(dir, "no-dir", "r.jsonl")); code == 0 || !strings.Contains(errs, "nothing was changed") {
		t.Fatalf("an unusable receipt: %d %s", code, errs)
	}
}

// run2 is run with an argument overridden after the base ones (a later flag wins).
func run2(base []string, extra ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := run(append(append([]string{}, base...), extra...), &out, &errb)
	return code, out.String(), errb.String()
}

func TestHelpExitsZero(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"--help"}, &out, &errb); code != 0 {
		t.Fatalf("--help exited %d", code)
	}
}
