// Command taperepack copies legacy tape pins into the segment store and
// proves the copies (station FP9, L2-04; internal/taperepack). Copy only:
// it never deletes, moves or edits a legacy file.
//
//	taperepack plan   --root <artifacts> --computer <id>[,<id>] --out <manifest> [--count-unreceipted]
//	taperepack copy   --root <artifacts> --manifest <manifest> --computer <id>[,<id>] [--batch 64]
//	taperepack census --root <segment root> [--legacy-root <artifacts>] --manifest <manifest> [--computer <id>]
//
// Each prints its counts as one JSON line. census exits 3 unless every
// manifest record reads from a segment with the legacy bytes. Never name the
// paper's or the owner's computer without the owner.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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
		fmt.Fprintln(stderr, "usage: taperepack plan|copy|census [flags]")
		return 2
	}
	sub := args[0]
	fs := flag.NewFlagSet("taperepack "+sub, flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", "", "artifacts root (census: the root holding tape-seg)")
	var computers listFlag
	fs.Var(&computers, "computer", "computer id; repeat or comma-separate")
	out := fs.String("out", "", "plan: manifest to write (outside the artifacts root)")
	manifest := fs.String("manifest", "", "copy, census: manifest to read")
	legacyRoot := fs.String("legacy-root", "", "census: root holding the legacy files (default --root)")
	batch := fs.Int("batch", taperepack.DefaultBatch, "copy: pins per append")
	countUnreceipted := fs.Bool("count-unreceipted", false, "plan: count legacy artifacts no receipt names")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if *root == "" {
		fmt.Fprintln(stderr, "taperepack: --root is required")
		return 2
	}
	emit := func(v any) {
		b, _ := json.Marshal(v)
		fmt.Fprintln(stdout, string(b))
	}
	switch sub {
	case "plan":
		if *out == "" {
			fmt.Fprintln(stderr, "taperepack plan: --out is required")
			return 2
		}
		if inside, err := under(*out, *root); err != nil || inside {
			fmt.Fprintln(stderr, "taperepack plan: --out must be outside the artifacts root")
			return 2
		}
		res, err := taperepack.Plan(taperepack.PlanOptions{Root: *root, Computers: computers, CountUnreceipted: *countUnreceipted})
		if err != nil {
			fmt.Fprintf(stderr, "taperepack plan: %v\n", err)
			return 1
		}
		if err := writeAtomic(*out, res.Entries); err != nil {
			fmt.Fprintf(stderr, "taperepack plan: %v\n", err)
			return 1
		}
		emit(res)
		return 0
	case "copy", "census":
		if *manifest == "" {
			fmt.Fprintf(stderr, "taperepack %s: --manifest is required\n", sub)
			return 2
		}
		f, err := os.Open(*manifest)
		if err != nil {
			fmt.Fprintf(stderr, "taperepack %s: %v\n", sub, err)
			return 1
		}
		entries, err := taperepack.ReadManifest(f)
		f.Close()
		if err != nil {
			fmt.Fprintf(stderr, "taperepack %s: %v\n", sub, err)
			return 1
		}
		if sub == "copy" {
			counts, err := taperepack.Copy(taperepack.CopyOptions{Root: *root, Entries: entries, Computers: computers, Batch: *batch})
			emit(counts)
			if err != nil {
				fmt.Fprintf(stderr, "taperepack copy: %v\n", err)
				return 1
			}
			return 0
		}
		counts, err := taperepack.Census(taperepack.CensusOptions{SegmentRoot: *root, LegacyRoot: *legacyRoot, Entries: entries, Computers: computers})
		if err != nil {
			fmt.Fprintf(stderr, "taperepack census: %v\n", err)
			return 1
		}
		emit(counts)
		if !counts.OK() {
			return 3
		}
		return 0
	}
	fmt.Fprintf(stderr, "taperepack: unknown subcommand %q\n", sub)
	return 2
}

// under reports whether path (which need not exist) lies inside root.
func under(path, root string) (bool, error) {
	r, err := filepath.EvalSymlinks(root)
	if err != nil {
		return false, err
	}
	r, err = filepath.Abs(r)
	if err != nil {
		return false, err
	}
	dir, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return false, err
	}
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	rel, err := filepath.Rel(r, filepath.Join(dir, filepath.Base(path)))
	if err != nil {
		return false, err
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))), nil
}

func writeAtomic(path string, entries []taperepack.Entry) (err error) {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer func() {
		if err != nil {
			_ = os.Remove(name)
		}
	}()
	err = tmp.Chmod(0o600)
	if err == nil {
		err = taperepack.WriteManifest(tmp, entries)
	}
	if err == nil {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, path)
}
