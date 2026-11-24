// demo-account is the host command for the reviewer account
// (docs/ship/demo-account.md, internal/auth/demo_account.go). Run it on the
// host as root, by the lead; nothing calls it automatically.
//
//	demo-account credential --user NAME --out PATH
//	    Makes a long random password, writes the credential file (user name,
//	    then an argon2id hash) to PATH with mode 0600 through a rename, and
//	    prints the password ONCE on stdout. Run it again to rotate: the auth
//	    service reads the file on every attempt, so the old password stops
//	    working at once. Removing the file turns the reviewer sign-in off.
//
//	demo-account off --id-file PATH [--credential PATH] [--auth-db PATH] [--already-off]
//	    --credential defaults to the file the auth service reads and refuses any other path.
//	    The owner's one step to turn the account off: removes the credential
//	    file (no new sign-in) first, then revokes every key and session the
//	    account holds (no phone stays in). It keeps the id file, so the
//	    account stays protected, and leaves the computer alone (reset removes
//	    it). Idempotent.
//
//	demo-account reset --id-file PATH [--auth-db PATH] [--vmctl URL] [--token-file PATH]
//	    Returns the account to empty before a review: revokes every key and
//	    every session the account holds (each reviewer phone is signed out),
//	    then asks vmctl to remove the account's computer (it is stopped, its
//	    ownership and snapshot are dropped). The next reviewer sign-in makes
//	    a new empty computer on first use. It does not purge the old computer's
//	    disk image or tape: the retention prune takes an orphaned state
//	    directory after its minimum age, and the account-deletion work purges
//	    the rest. It does not unpublish anything the account published.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/auth"
	"github.com/yusefmosiah/go-choir/internal/hostauth"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// defaultTokenFile is Node B's host-internal credential (nix/node-b.nix internalTokenFile),
// root-only. vmctl enforces it, and this command is run from a root shell that has no
// credential directory, so it reads the file itself.
// tokenPorts are the only ports the host token may be sent to; offSettle is how long off waits
// before it proves the revocation a second time.
var tokenPorts = map[string]bool{"8083": true}
var offSettle = 2 * time.Second

// credentialPath is the file the auth service reads (nix/node-b.nix AUTH_DEMO_CREDENTIAL_FILE).
var credentialPath = "/var/lib/go-choir/auth-signing/demo/demo-credential"

// staleCredentialPath is where it lived before the move into demo/.
var staleCredentialPath = "/var/lib/go-choir/auth-signing/demo-credential"

const defaultTokenFile = "/var/lib/go-choir/secrets/internal-service-token"

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, http.DefaultClient)) }

const passwordAlphabet = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// newPassword is 32 characters of a 56-letter alphabet without look-alikes:
// about 186 bits.
func newPassword() (string, error) {
	out := make([]byte, 32)
	for i := range out {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(passwordAlphabet))))
		if err != nil {
			return "", err
		}
		out[i] = passwordAlphabet[n.Int64()]
	}
	return string(out), nil
}

func run(args []string, stdout, stderr io.Writer, client *http.Client) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: demo-account credential|off|reset ...")
		return 2
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	fs.SetOutput(stderr)
	user := fs.String("user", "", "the reviewer's user name (credential)")
	out := fs.String("out", "", "the credential file to write (credential)")
	idFile := fs.String("id-file", "", "the host file holding the demo account id (reset)")
	authDB := fs.String("auth-db", "/var/lib/go-choir/auth/auth.db", "auth database (reset)")
	vmctlURL := fs.String("vmctl", "http://127.0.0.1:8083", "vmctl base URL (reset)")
	credential := fs.String("credential", credentialPath, "off: the credential file the auth service reads")
	alreadyOff := fs.Bool("already-off", false, "off: the credential file is already gone; do not fail for its absence")
	tokenFile := fs.String("token-file", defaultTokenFile, "the host-internal service token vmctl enforces (reset); a root shell has none loaded")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	switch args[0] {
	case "credential":
		if *user == "" || *out == "" {
			fmt.Fprintln(stderr, "credential needs --user and --out")
			return 2
		}
		pw, err := newPassword()
		if err == nil {
			var line string
			if line, err = auth.FormatDemoCredential(*user, pw); err == nil {
				err = writeAtomic(*out, []byte(line))
			}
		}
		if err != nil {
			fmt.Fprintln(stderr, "credential:", err)
			return 1
		}
		fmt.Fprintln(stdout, pw)
		return 0
	case "off":
		id, err := readID(*idFile)
		if err != nil {
			fmt.Fprintln(stderr, "off needs --id-file (readable)")
			return 2
		}
		// Nothing is touched until the database and the account are found: a typo in
		// --auth-db or a swapped flag must not look like success.
		if err := checkAccount(*authDB, id); err != nil {
			fmt.Fprintln(stderr, "off:", err, "(nothing was removed)")
			return 1
		}
		// The file is the one the auth service is configured to read (nix sets
		// AUTH_DEMO_CREDENTIAL_FILE to the same path): any other path, a stale copy
		// included, would leave the real one live.
		if filepath.Clean(*credential) != filepath.Clean(credentialPath) {
			fmt.Fprintf(stderr, "off: --credential must be the file the auth service reads, %s (nothing was removed)\n", credentialPath)
			return 1
		}
		raw, rerr := os.ReadFile(*credential)
		switch {
		case rerr == nil && !auth.ValidDemoCredential(raw):
			fmt.Fprintln(stderr, "off: the file at the credential path is not a demo credential (nothing was removed):", *credential)
			return 1
		case rerr != nil && !(*alreadyOff && os.IsNotExist(rerr)):
			fmt.Fprintln(stderr, "off: cannot read the credential file (pass --already-off if it is already removed):", rerr)
			return 1
		}
		// Sign-in first: nothing new can come in while the keys are revoked.
		if err := os.Remove(*credential); err != nil && !os.IsNotExist(err) {
			fmt.Fprintln(stderr, "off: remove credential:", err)
			return 1
		}
		// Whatever --already-off says, the configured path must be absent now.
		if _, err := os.Stat(*credential); err == nil {
			fmt.Fprintln(stderr, "off: the credential file is still there after the remove")
			return 1
		}
		// A copy at the path used before the move is not read by the service, but it must not
		// sit there to be put back by mistake.
		if staleCredentialPath != "" && filepath.Clean(staleCredentialPath) != filepath.Clean(credentialPath) {
			if err := os.Remove(staleCredentialPath); err == nil {
				fmt.Fprintln(stdout, "removed a stale credential file at the old path", staleCredentialPath)
			}
		}
		fmt.Fprintln(stdout, "credential file removed: the reviewer sign-in answers not available")
		keys, sessions, err := revokeAll(*authDB, id)
		if err != nil {
			fmt.Fprintln(stderr, "off:", err)
			return 1
		}
		// A login that passed the credential check before it was removed may still be
		// inside its password check; it takes its own key back, and this pass proves it.
		time.Sleep(offSettle)
		if _, _, err := revokeAll(*authDB, id); err != nil {
			fmt.Fprintln(stderr, "off: second pass:", err)
			return 1
		}
		fmt.Fprintf(stdout, "revoked %d keys and %d sessions; pending grants deleted; keys and sessions proved gone twice\n", keys, sessions)
		return 0
	case "reset":
		if *idFile == "" {
			fmt.Fprintln(stderr, "reset needs --id-file")
			return 2
		}
		id, err := readID(*idFile)
		if err != nil {
			fmt.Fprintln(stderr, "reset: cannot read the account id file")
			return 1
		}
		if err := checkAccount(*authDB, id); err != nil {
			fmt.Fprintln(stderr, "reset:", err)
			return 1
		}
		keys, sessions, err := revokeAll(*authDB, id)
		if err != nil {
			fmt.Fprintln(stderr, "reset:", err)
			return 1
		}
		fmt.Fprintf(stdout, "revoked %d keys and %d sessions\n", keys, sessions)
		status, err := removeComputer(client, *vmctlURL, id, *tokenFile)
		if err != nil {
			fmt.Fprintln(stderr, "reset: remove computer:", err)
			return 1
		}
		fmt.Fprintf(stdout, "computer: %s\n", status)
		return 0
	}
	fmt.Fprintln(stderr, "unknown command", args[0])
	return 2
}

func isLoopback(host string) bool {
	return host == "127.0.0.1" || host == "localhost" || host == "::1" || host == "[::1]"
}

func readID(path string) (string, error) {
	raw, err := os.ReadFile(path)
	id := strings.TrimSpace(strings.SplitN(string(raw), "\n", 2)[0])
	if err != nil || id == "" {
		return "", errors.New("no account id")
	}
	return id, nil
}

// checkAccount proves the database exists and holds the account, without creating
// anything (auth.OpenStore would make and migrate a new database at a typo path).
func checkAccount(authDB, id string) error {
	if info, err := os.Stat(authDB); err != nil || info.IsDir() {
		return fmt.Errorf("no auth database at %s", authDB)
	}
	store, err := auth.OpenStore(authDB)
	if err != nil {
		return fmt.Errorf("open auth db: %w", err)
	}
	defer func() { _ = store.Close() }()
	if u, err := store.GetUserByID(id); err != nil || u == nil {
		return errors.New("the auth database has no account with the id in the id file")
	}
	return nil
}

// revokeAll revokes the account's access and then proves it: no key left unrevoked
// and no session left. It fails otherwise.
func revokeAll(authDB, id string) (int, int, error) {
	if err := checkAccount(authDB, id); err != nil {
		return 0, 0, err
	}
	store, err := auth.OpenStore(authDB)
	if err != nil {
		return 0, 0, fmt.Errorf("open auth db: %w", err)
	}
	defer func() { _ = store.Close() }()
	keys, sessions, err := auth.ResetDemoAccess(context.Background(), store, id)
	if err != nil {
		return keys, sessions, err
	}
	list, err := store.ListAPIKeys(context.Background(), id)
	if err != nil {
		return keys, sessions, fmt.Errorf("could not prove the keys are revoked: %w", err)
	}
	for _, k := range list {
		if k.RevokedAt == nil {
			return keys, sessions, errors.New("a key of the account is still live after the revoke")
		}
	}
	rs, err := store.ListRefreshSessionsByUserID(id)
	if err != nil || len(rs) != 0 {
		return keys, sessions, errors.New("could not prove the sessions are deleted")
	}
	return keys, sessions, nil
}

func writeAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".demo-credential-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if err := tmp.Chmod(0o600); err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func removeComputer(client *http.Client, base, userID, tokenFile string) (string, error) {
	body, _ := json.Marshal(map[string]any{"user_id": userID, "desktop_id": vmctl.PrimaryDesktopID})
	req, err := http.NewRequest(http.MethodPost, vmctl.RemoveEndpoint(strings.TrimRight(base, "/")), bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(vmctl.LifecycleCallerHeader, "demo-account-reset")
	hostauth.MarkInternal(req)
	// vmctl enforces the host credential; MarkInternal sends it only from a process that
	// loaded it at start, which a root shell did not. Read the file here, and send it only
	// to a loopback host on vmctl's own port (never a guest's DNAT port).
	if isLoopback(req.URL.Hostname()) && tokenPorts[req.URL.Port()] {
		raw, err := os.ReadFile(tokenFile)
		t := strings.TrimSpace(string(raw))
		if err != nil || t == "" {
			return "", fmt.Errorf("cannot read the host-internal token at %s (pass --token-file)", tokenFile)
		}
		req.Header.Set(hostauth.Header, t)
	} else {
		req.Header.Del(hostauth.Header)
	}
	if client == nil {
		client = http.DefaultClient
	}
	// A redirect is never followed: Go would copy the custom header to the new host.
	c := *client
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	client = &c
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	var reply struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	_ = json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&reply)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("vmctl answered %d: %s", resp.StatusCode, reply.Error)
	}
	return reply.Status, nil
}
