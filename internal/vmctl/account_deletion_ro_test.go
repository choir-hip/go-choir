package vmctl

// vmctl reads auth.db under systemd ReadOnlyPaths (nix/node-b.nix) while the
// auth service holds it open in WAL mode (seat 1 and seat 2, round 2, blocking
// 1). A unit test in a writable temp dir proves nothing about that. These
// tests run the auth service's side in another process, then make the files
// and the directory read-only (what the mount does to an unprivileged reader)
// and read the record the way the destroy route and the retention sweeper do.
//
// Failure list:
//  W1 the read fails or answers "no record" while the auth service holds the
//     database open (the production case): every destroy would be refused
//     after mail and data were already destroyed;
//  W2 with nobody holding the database open (no -wal or -shm to attach to) the
//     read answers "no record" instead of failing: a destroyer of user data
//     would then see an account as not deleting. It must fail, never answer
//     empty.

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/auth"
	"github.com/yusefmosiah/go-choir/internal/purgeguard"
)

const roHelperEnv = "VMCTL_RO_HELPER_DB"

// TestHelperHoldAuthDB is the auth service for the tests below: it opens the
// store (WAL), writes a deletion record, says "ready" and holds the database
// open until its stdin closes. It is not a test when run normally.
func TestHelperHoldAuthDB(t *testing.T) {
	path := os.Getenv(roHelperEnv)
	if path == "" {
		t.Skip("helper process only")
	}
	store, err := auth.OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	u, err := store.CreateUser("ro-held", "ro-held@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.RequestAccountDeletion(u.ID, time.Now().UTC().Add(-100*time.Hour)); err != nil {
		t.Fatal(err)
	}
	os.Stdout.WriteString("ready\n")
	buf := make([]byte, 1)
	_, _ = os.Stdin.Read(buf)
	_ = store.Close()
}

func startAuthHolder(t *testing.T, path string) (stop func()) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperHoldAuthDB$")
	cmd.Env = append(os.Environ(), roHelperEnv+"="+path)
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	line := make(chan string, 1)
	go func() {
		r := bufio.NewReader(out)
		for {
			s, err := r.ReadString('\n')
			if err != nil {
				line <- ""
				return
			}
			if s == "ready\n" {
				line <- s
				return
			}
		}
	}()
	select {
	case s := <-line:
		if s == "" {
			t.Fatal("the auth holder did not start")
		}
	case <-time.After(60 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("the auth holder never said ready")
	}
	return func() { _ = in.Close(); _ = cmd.Wait() }
}

// makeReadOnly is what ReadOnlyPaths does to a reader that is not root: nothing
// under the directory can be created or opened for writing.
func makeReadOnly(t *testing.T, dir string) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root ignores file modes; the test needs an unprivileged reader")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if err := os.Chmod(filepath.Join(dir, e.Name()), 0o444); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
}

func TestTheRecordIsReadOnAReadOnlyMountWhileTheAuthServiceHoldsTheWALOpen(t *testing.T) { // W1
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.db")
	stop := startAuthHolder(t, path)
	defer stop()
	makeReadOnly(t, dir)

	db, err := purgeguard.OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	users, err := purgeguard.PendingUsers(db)
	if err != nil {
		t.Fatalf("the read failed on a read-only mount with the auth service up: %v", err)
	}
	if !users["ro-held"] {
		t.Fatalf("the record is not seen (answered %v): a destroyer would think the account is not deleting", users)
	}
	if err := purgeguard.Eligible(db, "ro-held", time.Now().UTC()); err != nil {
		t.Fatalf("destroy's own read of the record: %v", err)
	}
	// And through the registry's route, the way vmctl does it.
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetAccountPurgeAuthDB(path)
	if err := reg.accountPurgeEligibleFor("ro-held", time.Now().UTC()); err != nil {
		t.Fatalf("registry read: %v", err)
	}
	// The sweeper's read, with the same file.
	cfg := withDeletionRecords(RetentionPruneConfig{AuthDBPath: path})
	if cfg.deletionUnreadable || !cfg.accountDeleting("ro-held") {
		t.Fatalf("the sweeper cannot see the record: %+v", cfg)
	}
}

func TestAReadOnlyMountWithNobodyHoldingTheDatabaseFailsAndNeverAnswersEmpty(t *testing.T) { // W2
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.db")
	stop := startAuthHolder(t, path)
	stop() // the auth service is gone: a clean close removes the -wal and -shm
	makeReadOnly(t, dir)

	db, err := purgeguard.OpenReadOnly(path)
	if err != nil {
		return // failing is the safe answer
	}
	defer db.Close()
	users, err := purgeguard.PendingUsers(db)
	if err != nil {
		return // failing is the safe answer
	}
	if !users["ro-held"] {
		t.Fatalf("an unreadable-or-empty answer is not allowed: got %v with no error; the record exists", users)
	}
}
