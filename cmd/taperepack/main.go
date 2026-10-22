// Command taperepack copies legacy tape files into the per-computer segment
// store and proves, file by file, which have a verified segment copy
// (station FP10, internal/taperepack). It never deletes, moves or edits a
// legacy file.
//
//	taperepack census --root <artifacts> --work-dir <dir> [--computer <id>[,<id>]] [--rate N]
//	taperepack copy   --root <artifacts> --work-dir <dir> [--computer <id>[,<id>]] [--apply] [--fresh] [--rate N]
//	taperepack delete --root <artifacts> --work-dir <dir> --census-receipt <digest> [--computer <id>[,<id>]] [--apply] [--max-age 24h] [--pause 250ms] [--rate N]
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
// --apply is given; its exit status is 1 on any refusal or error. census exits 3 unless every file it is responsible for is covered by a
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
		fmt.Fprintln(stderr, "usage: taperepack census|copy|delete [flags]")
		return 2
	}
	sub := args[0]
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
	if sub == "delete" {
		if *fresh {
			fmt.Fprintln(stderr, "taperepack delete: --fresh belongs to copy")
			return 2
		}
		dopts := taperepack.DeleteOptions{Root: *root, WorkDir: *workDir, CensusReceipt: *receipt, Computers: computers, Apply: *apply, MaxAge: *maxAge, Pause: *pause, Rate: *rate}
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
