// Command choir-bundle exports and verifies a computer bundle: a sealed
// snapshot root, its chunks and the tape from the snapshot's head to the
// current head (internal/bundle).
//
//	choir-bundle export --computer <id> --out <dir> [--from-seq N] [--root <digest>] [--owner <id>] [--artifacts-root <dir>] [--dsn <dsn>]
//	choir-bundle verify <dir> [--trust <file>]... [--no-pinned-trust]
//	choir-bundle import <dir> --artifacts-root <dir> --dsn <dsn> [--trust <file>]... [--no-pinned-trust] [--bootstrap --graph-dsn <dsn>]
//	choir-bundle trust-key [--signing-key <path>] [--label <name>]
//	choir-bundle follow --computer <id> --header-file <file> [--backend <url>] [--store <dir>] [--once] [--max-bundles N] [--trust <file>]...
//	choir-bundle follow --computer <id> --verify [--store <dir>] [--trust <file>]...
//
// verify and import authenticate the bundle: every receipt must be signed by
// a platform-control key the importing host trusts. The trust set is the
// repository-pinned staging key (unless --no-pinned-trust) plus each --trust
// file; it never comes from the bundle. trust-key prints the trust file of
// this host's platform signing key (public part only), for copying to the
// host that will import this host's bundles
// (docs/design/bundle-authenticity-2026-10-31.md).
//
// export reads only the platform's control database and its sealed host
// files, never the guest, and works with the computer up or down. Files
// already in --out are not rewritten, so exporting again to the same
// directory is incremental. --artifacts-root defaults to
// $CORPUSD_ARTIFACTS_ROOT or the host default; --dsn to $CHOIR_PG_CONTROL_DSN
// or the host default (the password comes from the credential store, never
// the DSN). import writes a verified bundle into a platform store; it has no
// defaults, so it never writes a host's store unless that store is named. It
// refuses (exit 3) when the bundle does not chain to the target's head for the
// computer: a fork is a new computer, never a merge. --bootstrap first applies
// the platform schema to the target's control and graph databases. follow
// keeps a read-only local copy of the owner's computer by pulling bundles
// from the server's owner-authenticated tape route (follow.go, the Mac app's
// syncer as a command). Each
// command prints one JSON line; the exit status is 1 on error.
package main

import (
	"context"
	"crypto/ed25519"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/bundle"
	"github.com/yusefmosiah/go-choir/internal/dbcred"
	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/platform"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: choir-bundle export|verify|import|follow [flags]")
		return 2
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "export":
		return runExport(rest, stdout, stderr)
	case "verify":
		return runVerify(rest, stdout, stderr)
	case "import":
		return runImport(rest, stdout, stderr)
	case "trust-key":
		return runTrustKey(rest, stdout, stderr)
	case "follow":
		return runFollow(rest, stdout, stderr)
	}
	fmt.Fprintf(stderr, "choir-bundle: unknown command %q\n", sub)
	return 2
}

func emit(stdout io.Writer, v any) {
	raw, _ := json.Marshal(v)
	fmt.Fprintln(stdout, string(raw))
}

func fail(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "choir-bundle: %v\n", err)
	return 1
}

func openDB(dsn string) (*sql.DB, error) {
	applied, err := dbcred.ApplyPGDSN(dsn)
	if err != nil {
		return nil, err
	}
	db, err := dbx.OpenPostgres(applied, dbx.PGOptions{MaxOpenConns: 4, MaxIdleConns: 2})
	if err != nil {
		return nil, err
	}
	return db, nil
}

func runExport(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir-bundle export", flag.ContinueOnError)
	fs.SetOutput(stderr)
	computer := fs.String("computer", "", "computer id")
	out := fs.String("out", "", "bundle directory")
	fromSeq := fs.Int64("from-seq", -1, "start the range at this sequence instead of the snapshot head")
	root := fs.String("root", "", "snapshot root to export (default: the newest)")
	since := fs.String("since", "", "previous bundle directory: export only the events after its head, with no snapshot (a delta)")
	owner := fs.String("owner", "", "owner id to record in the manifest")
	artifacts := fs.String("artifacts-root", envOr("CORPUSD_ARTIFACTS_ROOT", platform.DefaultArtifactsRoot), "platform artifacts root")
	dsn := fs.String("dsn", envOr(platform.EnvPGControlDSN, platform.DefaultPGControlDSN), "platform control database DSN")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *computer == "" || *out == "" {
		fmt.Fprintln(stderr, "choir-bundle export: --computer and --out are required")
		return 2
	}
	db, err := openDB(*dsn)
	if err != nil {
		return fail(stderr, err)
	}
	defer db.Close()
	host, _ := os.Hostname()
	opts := bundle.ExportOptions{
		ComputerID: *computer, Out: *out, FromSeq: *fromSeq, Root: *root, OwnerID: *owner,
		Producer: bundle.Producer{Host: host, Tool: "choir-bundle"},
	}
	var m bundle.Manifest
	if *since != "" {
		m, err = bundle.ExportDelta(context.Background(), db, *artifacts, *since, opts)
	} else {
		m, err = bundle.Export(context.Background(), db, *artifacts, opts)
	}
	if err != nil {
		return fail(stderr, err)
	}
	emit(stdout, summary(m))
	return 0
}

// trustFlags are the trust set flags of verify and import.
type trustFlags struct {
	files    []string
	noPinned bool
}

func (t *trustFlags) register(fs *flag.FlagSet) {
	fs.Func("trust", "trust file of platform-control keys (repeatable)", func(v string) error {
		t.files = append(t.files, v)
		return nil
	})
	fs.BoolVar(&t.noPinned, "no-pinned-trust", false, "do not trust the repository-pinned staging platform key")
}

func (t *trustFlags) load() (*bundle.TrustSet, error) {
	set, err := bundle.NewTrustSet()
	if err != nil {
		return nil, err
	}
	if !t.noPinned {
		if err := set.Add(bundle.PinnedTrust()); err != nil {
			return nil, err
		}
	}
	for _, f := range t.files {
		if err := set.LoadTrustFile(f); err != nil {
			return nil, err
		}
	}
	if set.Len() == 0 {
		return nil, fmt.Errorf("no trusted keys: pass --trust <file>")
	}
	return set, nil
}

// splitDir takes the leading bundle directory argument, if any.
func splitDir(args []string) (string, []string) {
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		return args[0], args[1:]
	}
	return "", args
}

func runVerify(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir-bundle verify", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var tf trustFlags
	tf.register(fs)
	dir, args := splitDir(args)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if dir == "" && fs.NArg() == 1 {
		dir = fs.Arg(0)
	}
	if dir == "" {
		fmt.Fprintln(stderr, "usage: choir-bundle verify <dir> [--trust <file>]... [--no-pinned-trust]")
		return 2
	}
	trust, err := tf.load()
	if err != nil {
		return fail(stderr, err)
	}
	v, err := bundle.VerifyBundle(dir, trust)
	if err != nil {
		return fail(stderr, err)
	}
	out := summary(v.Manifest)
	out["verified"] = true
	out["signatures"] = v.Signatures
	emit(stdout, out)
	return 0
}

func runTrustKey(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir-bundle trust-key", flag.ContinueOnError)
	fs.SetOutput(stderr)
	keyPath := fs.String("signing-key", envOr("PLATFORM_SIGNING_KEY_PATH", filepath.Join(envOr("CORPUSD_ARTIFACTS_ROOT", platform.DefaultArtifactsRoot), "signing-key")), "platform signing key file (raw ed25519 private key; read, never created)")
	label := fs.String("label", "", "label for the key (default: this host's name)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	raw, err := os.ReadFile(*keyPath)
	if err != nil {
		return fail(stderr, err)
	}
	if len(raw) != ed25519.PrivateKeySize {
		return fail(stderr, fmt.Errorf("%s is %d bytes, not an ed25519 private key", *keyPath, len(raw)))
	}
	if *label == "" {
		*label, _ = os.Hostname()
	}
	entry := bundle.TrustEntry(ed25519.PrivateKey(raw).Public().(ed25519.PublicKey), *label)
	out, _ := json.MarshalIndent(bundle.TrustFile{Keys: []bundle.TrustedKey{entry}}, "", "  ")
	fmt.Fprintln(stdout, string(out))
	return 0
}

func runImport(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir-bundle import", flag.ContinueOnError)
	fs.SetOutput(stderr)
	artifacts := fs.String("artifacts-root", "", "target platform artifacts root (required)")
	dsn := fs.String("dsn", "", "target platform control database DSN (required)")
	bootstrap := fs.Bool("bootstrap", false, "apply the platform schema to the target first")
	graphDSN := fs.String("graph-dsn", "", "target graph database DSN (with --bootstrap)")
	var tf trustFlags
	tf.register(fs)
	dir, args := splitDir(args)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if dir == "" && fs.NArg() == 1 {
		dir = fs.Arg(0)
	}
	if dir == "" || *artifacts == "" || *dsn == "" || (*bootstrap && *graphDSN == "") {
		fmt.Fprintln(stderr, "usage: choir-bundle import <dir> --artifacts-root <dir> --dsn <dsn> [--trust <file>]... [--no-pinned-trust] [--bootstrap --graph-dsn <dsn>]")
		return 2
	}
	trust, err := tf.load()
	if err != nil {
		return fail(stderr, err)
	}
	ctx := context.Background()
	if *bootstrap {
		store, err := platform.OpenStore(ctx, *dsn, *graphDSN)
		if err != nil {
			return fail(stderr, err)
		}
		_ = store.Close()
	}
	db, err := openDB(*dsn)
	if err != nil {
		return fail(stderr, err)
	}
	defer db.Close()
	res, err := bundle.Import(ctx, dir, db, *artifacts, trust)
	if err != nil {
		code := fail(stderr, err)
		if errors.Is(err, bundle.ErrRefused) {
			code = 3
		}
		return code
	}
	emit(stdout, res)
	return 0
}

func summary(m bundle.Manifest) map[string]any {
	out := map[string]any{
		"computer_id": m.ComputerID, "start_seq": m.Tape.StartSeq, "head_seq": m.Tape.HeadSeq,
		"head_digest": m.Tape.HeadDigest, "counts": m.Counts, "bytes": m.Bytes,
	}
	if m.Delta {
		out["delta"] = true
	}
	if m.SnapshotRoot != nil {
		out["snapshot_root"] = m.SnapshotRoot.Root
		out["snapshot_head_seq"] = m.SnapshotRoot.HeadSequence
	}
	return out
}
