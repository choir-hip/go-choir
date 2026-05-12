package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/yusefmosiah/go-choir/internal/shipper"
)

type checkFlags []string

func (f *checkFlags) String() string {
	return fmt.Sprint([]string(*f))
}

func (f *checkFlags) Set(value string) error {
	*f = append(*f, value)
	return nil
}

func main() {
	log.SetFlags(0)
	if len(os.Args) < 2 || os.Args[1] != "import" {
		fmt.Fprintln(os.Stderr, "usage: shipper import --repo PATH --manifest manifest.json --patchset changes.patch --branch agent/<run-id>/<slug> [--check CMD] [--report report.json] [--push]")
		os.Exit(2)
	}

	var checks checkFlags
	fs := flag.NewFlagSet("import", flag.ExitOnError)
	opts := shipper.Options{}
	fs.StringVar(&opts.RepoPath, "repo", ".", "clean repository checkout to import into")
	fs.StringVar(&opts.ManifestPath, "manifest", "", "verification manifest JSON from the worker VM")
	fs.StringVar(&opts.PatchsetPath, "patchset", "", "patch file or directory of .patch/.diff files")
	fs.StringVar(&opts.Branch, "branch", "", "target branch, must match agent/<run-id>/<slug>")
	fs.StringVar(&opts.Remote, "remote", "origin", "git remote used when --push is set")
	fs.StringVar(&opts.ReportPath, "report", "", "optional JSON report path")
	fs.StringVar(&opts.CommitMessage, "message", "", "optional commit subject")
	fs.BoolVar(&opts.Push, "push", false, "push the imported branch to the configured remote")
	fs.Var(&checks, "check", "verification command to run after commit; may be repeated")
	if err := fs.Parse(os.Args[2:]); err != nil {
		log.Fatal(err)
	}
	opts.Checks = checks

	report, err := shipper.ImportPatchset(context.Background(), opts)
	if err != nil {
		log.Fatal(err)
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(data))
}
