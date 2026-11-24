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
//	demo-account reset --id-file PATH [--auth-db PATH] [--vmctl URL]
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
		fmt.Fprintln(stderr, "usage: demo-account credential|reset ...")
		return 2
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	fs.SetOutput(stderr)
	user := fs.String("user", "", "the reviewer's user name (credential)")
	out := fs.String("out", "", "the credential file to write (credential)")
	idFile := fs.String("id-file", "", "the host file holding the demo account id (reset)")
	authDB := fs.String("auth-db", "/var/lib/go-choir/auth/auth.db", "auth database (reset)")
	vmctlURL := fs.String("vmctl", "http://127.0.0.1:8083", "vmctl base URL (reset)")
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
	case "reset":
		if *idFile == "" {
			fmt.Fprintln(stderr, "reset needs --id-file")
			return 2
		}
		raw, err := os.ReadFile(*idFile)
		id := strings.TrimSpace(strings.SplitN(string(raw), "\n", 2)[0])
		if err != nil || id == "" {
			fmt.Fprintln(stderr, "reset: cannot read the account id file")
			return 1
		}
		store, err := auth.OpenStore(*authDB)
		if err != nil {
			fmt.Fprintln(stderr, "reset: open auth db:", err)
			return 1
		}
		defer func() { _ = store.Close() }()
		keys, sessions, err := auth.ResetDemoAccess(context.Background(), store, id)
		if err != nil {
			fmt.Fprintln(stderr, "reset:", err)
			return 1
		}
		fmt.Fprintf(stdout, "revoked %d keys and %d sessions\n", keys, sessions)
		status, err := removeComputer(client, *vmctlURL, id)
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

func removeComputer(client *http.Client, base, userID string) (string, error) {
	body, _ := json.Marshal(map[string]any{"user_id": userID, "desktop_id": vmctl.PrimaryDesktopID})
	req, err := http.NewRequest(http.MethodPost, vmctl.RemoveEndpoint(strings.TrimRight(base, "/")), bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(vmctl.LifecycleCallerHeader, "demo-account-reset")
	hostauth.MarkInternal(req)
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
