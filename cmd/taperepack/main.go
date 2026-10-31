// Command taperepack copies legacy tape files into the per-computer segment
// store and proves, file by file, which have a verified segment copy
// (station FP10, internal/taperepack). Only delete removes a legacy file; it
// never moves or edits one.
//
//	taperepack census --root <artifacts> --work-dir <dir> [--computer <id>[,<id>]] [--rate N]
//	taperepack copy   --root <artifacts> --work-dir <dir> [--computer <id>[,<id>]] [--apply] [--fresh] [--rate N]
//	taperepack delete --root <artifacts> --work-dir <dir> --census-receipt <digest> [--computer <id>[,<id>]] [--keep-digests <file> [--keep-digests-empty-ok]] [--apply] [--max-age 24h] [--pause 250ms] [--rate N]
//	taperepack compact --root <artifacts> [--computer <id>[,<id>] | --all-computers] [--apply] [--rate-mb 16] [--pause 1s] [--verify-records=true] [--compress=true] [--min-avg-extent 65536] [--max-segments N] [--compsize <path>]
//
// compact rewrites each sealed segment once as a fresh sequential file with
// identical bytes (rule F of docs/design/small-object-storage-2026-10-31.md),
// freeing the btrfs copy-on-write bookends its fsynced appends left. It is a
// dry run unless --apply is given, and --apply needs --computer or
// --all-computers. Every copy is verified before its rename. --compsize names
// the compsize binary, run as "compsize -b <file>" before and after each
// segment, for the disk bytes reclaimed (st_blocks does not show them on
// btrfs). It needs no --work-dir and prints its report as one JSON line.
//
// census reads everything and writes nothing under the artifacts root. copy
// is a dry run unless --apply is given, and --apply needs --computer: a
// computer is never copied without being named, and never the owner's or the
// paper's without the owner. --work-dir (outside the artifacts root) holds the
// spill files, the state of an interrupted copy, and the report, stored as
// census-<digest>.json or copy-<digest>.json. Each command prints the report
// as one JSON line (its "receipt" is that digest) and progress on stderr.
// delete is the one command that removes legacy files (owner ruling
// 2026-10-22): it needs the digest of a recent, healthy census of this root
// from --work-dir, verifies everything again, and unlinks only files whose
// fresh verdict is covered by a verified segment copy. It is a dry run unless
// --apply is given; its exit status is 1 on any refusal or error and 3 when
// its own fresh verification found a damaged store (verify_healthy false).
// --apply needs --keep-digests: a file of 64-hex digests, one per line, that
// the platform's database keeps (the tape sweep's tapeExtraRefQueries, which
// this host binary cannot read). A listed digest is never unlinked and is
// counted as kept_by_list; a malformed line, or a file with no digest unless
// --keep-digests-empty-ok, is an error.
// census exits 3 unless every file it is responsible for is covered by a
// verified segment copy; copy exits 3 if the store is not healthy.
//
// Memory is bounded by the bucket (1/256 of the files) and 64 MiB of pending
// appends, not by the number of files. Run it at idle priority on the host:
//
//	ionice -c3 nice -n 19 taperepack census --root … --work-dir …
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/taperepack"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

type listFlag []string

func (l *listFlag) String() string { return strings.Join(*l, ",") }
func (l *listFlag) Set(v string) error {
	for _, part := range strings.Split(v, ",") {
		if part = strings.TrimSpace(part); part != "" {
			*l = append(*l, part)
		}
	}
	return nil
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: taperepack census|copy|delete|compact [flags]")
		return 2
	}
	sub := args[0]
	if sub == "compact" {
		return runCompact(args[1:], stdout, stderr)
	}
	fs := flag.NewFlagSet("taperepack "+sub, flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", "", "artifacts root (holds sha256/ and tape-seg/)")
	workDir := fs.String("work-dir", "", "work directory outside the artifacts root")
	var computers listFlag
	fs.Var(&computers, "computer", "computer id; repeat or comma-separate (copy --apply requires it)")
	rate := fs.Int("rate", 0, "legacy files read per second (0 = unlimited)")
	apply := fs.Bool("apply", false, "copy: really write (default is a dry run)")
	fresh := fs.Bool("fresh", false, "copy --apply: discard the saved state of an interrupted run")
	quiet := fs.Bool("quiet", false, "no progress lines")
	receipt := fs.String("census-receipt", "", "delete: digest of the census report in --work-dir")
	maxAge := fs.Duration("max-age", 24*time.Hour, "delete: oldest census accepted")
	keepFile := fs.String("keep-digests", "", "delete: file of digests (one 64-hex per line) the platform database keeps; never unlinked; required with --apply")
	keepEmptyOK := fs.Bool("keep-digests-empty-ok", false, "delete: allow --keep-digests to name no digest")
	pause := fs.Duration("pause", 250*time.Millisecond, "delete: pause between batches of 256 unlinks")
	if sub != "census" && sub != "copy" && sub != "delete" {
		fmt.Fprintf(stderr, "taperepack: unknown subcommand %q\n", sub)
		return 2
	}
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if *root == "" || *workDir == "" {
		fmt.Fprintln(stderr, "taperepack: --root and --work-dir are required")
		return 2
	}
	if sub == "census" && (*apply || *fresh) {
		fmt.Fprintln(stderr, "taperepack census: --apply and --fresh belong to copy")
		return 2
	}
	if sub != "delete" && (*receipt != "") {
		fmt.Fprintf(stderr, "taperepack %s: --census-receipt belongs to delete\n", sub)
		return 2
	}
	if sub != "delete" && (*keepFile != "" || *keepEmptyOK) {
		fmt.Fprintf(stderr, "taperepack %s: --keep-digests belongs to delete\n", sub)
		return 2
	}
	if sub == "delete" {
		if *keepEmptyOK && *keepFile == "" {
			fmt.Fprintln(stderr, "taperepack delete: --keep-digests-empty-ok needs --keep-digests <file>")
			return 2
		}
		if *fresh {
			fmt.Fprintln(stderr, "taperepack delete: --fresh belongs to copy")
			return 2
		}
		dopts := taperepack.DeleteOptions{Root: *root, WorkDir: *workDir, CensusReceipt: *receipt, Computers: computers, Apply: *apply, MaxAge: *maxAge, Pause: *pause, Rate: *rate}
		if *keepFile != "" {
			keep, err := taperepack.LoadKeepDigests(*keepFile, *keepEmptyOK)
			if err != nil {
				fmt.Fprintf(stderr, "taperepack delete: %v\n", err)
				return 1
			}
			dopts.Keep = keep
		}
		if !*quiet {
			dopts.Progress = func(s string) { fmt.Fprintln(stderr, s) }
		}
		rep, err := taperepack.Delete(dopts)
		if rep != nil {
			b, _ := json.Marshal(rep)
			fmt.Fprintln(stdout, string(b))
		}
		if err != nil {
			fmt.Fprintf(stderr, "taperepack delete: %v\n", err)
			return 1
		}
		if !rep.VerifyHealthy {
			return 3
		}
		return 0
	}
	opts := taperepack.Options{Root: *root, WorkDir: *workDir, Computers: computers, Copy: sub == "copy", Apply: *apply, Fresh: *fresh, Rate: *rate}
	if !*quiet {
		opts.Progress = func(s string) { fmt.Fprintln(stderr, s) }
	}
	rep, err := taperepack.Run(opts)
	if err != nil {
		fmt.Fprintf(stderr, "taperepack %s: %v\n", sub, err)
		return 1
	}
	b, _ := json.Marshal(rep)
	fmt.Fprintln(stdout, string(b))
	switch {
	case sub == "census" && !rep.OK():
		return 3
	case sub == "copy" && !rep.Healthy():
		return 3
	}
	return 0
}

func runCompact(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("taperepack compact", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", "", "artifacts root (holds tape-seg/)")
	var computers listFlag
	fs.Var(&computers, "computer", "computer id; repeat or comma-separate")
	all := fs.Bool("all-computers", false, "with --apply: every computer under tape-seg")
	apply := fs.Bool("apply", false, "really rewrite (default is a dry run)")
	rateMB := fs.Int64("rate-mb", 16, "copy rate per segment, MiB/s")
	pause := fs.Duration("pause", time.Second, "rest between compacted segments")
	verify := fs.Bool("verify-records", true, "read back every indexed record of each copy before its rename")
	compress := fs.Bool("compress", true, "ask btrfs to compress the copies (btrfs.compression property)")
	minExtent := fs.Int64("min-avg-extent", 0, "skip segments whose extents average at least this many bytes (0 = 64 KiB, -1 = never skip)")
	maxSegs := fs.Int("max-segments", 0, "stop after this many compactions (0 = no limit)")
	compsize := fs.String("compsize", "", "path of the compsize binary, for disk bytes before/after")
	quiet := fs.Bool("quiet", false, "no progress lines")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *root == "" {
		fmt.Fprintln(stderr, "taperepack compact: --root is required")
		return 2
	}
	if *rateMB <= 0 {
		fmt.Fprintln(stderr, "taperepack compact: --rate-mb must be positive")
		return 2
	}
	opts := taperepack.CompactOptions{Root: *root, Computers: computers, AllComputers: *all, Apply: *apply, BytesPerSecond: *rateMB << 20, Pause: *pause, VerifyRecords: *verify, Compress: *compress, MinAvgExtentBytes: *minExtent, MaxSegments: *maxSegs}
	if *compsize != "" {
		bin := *compsize
		opts.Measure = func(path string) (int64, error) { return compsizeDisk(bin, path) }
	}
	if !*quiet {
		opts.Progress = func(s string) { fmt.Fprintln(stderr, s) }
	}
	rep, err := taperepack.Compact(opts)
	if rep != nil {
		rep.Results = nil // one line per segment went to stderr
		b, _ := json.Marshal(rep)
		fmt.Fprintln(stdout, string(b))
	}
	if err != nil {
		fmt.Fprintf(stderr, "taperepack compact: %v\n", err)
		return 1
	}
	if rep.Errors > 0 {
		return 3
	}
	return 0
}

// compsizeDisk runs "compsize -b <path>" and returns the Disk Usage column of
// its TOTAL line.
func compsizeDisk(bin, path string) (int64, error) {
	out, err := exec.Command(bin, "-b", path).Output()
	if err != nil {
		return 0, fmt.Errorf("compsize %s: %w", path, err)
	}
	return parseCompsizeTotal(string(out))
}

func parseCompsizeTotal(out string) (int64, error) {
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) >= 3 && f[0] == "TOTAL" {
			return strconv.ParseInt(f[2], 10, 64)
		}
	}
	return 0, fmt.Errorf("compsize: no TOTAL line in %q", out)
}
