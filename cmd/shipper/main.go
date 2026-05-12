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
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: shipper <export|import> ...")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "export":
		runExport(os.Args[2:])
	case "import":
		runImport(os.Args[2:])
	default:
		fmt.Fprintln(os.Stderr, "usage: shipper <export|import> ...")
		os.Exit(2)
	}
}

func runImport(args []string) {
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
	if err := fs.Parse(args); err != nil {
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

func runExport(args []string) {
	var checks checkFlags
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	opts := shipper.ExportOptions{}
	fs.StringVar(&opts.RepoPath, "repo", ".", "worker repository checkout to export from")
	fs.StringVar(&opts.OutputDir, "out", "", "output directory for manifest.json, changes.patch, and export-report.json")
	fs.StringVar(&opts.BaseSHA, "base", "", "base SHA the worker branch started from")
	fs.StringVar(&opts.RunID, "run-id", "", "Choir run id")
	fs.StringVar(&opts.TraceID, "trace-id", "", "Trace trajectory id")
	fs.StringVar(&opts.VMID, "vm-id", "", "worker VM id")
	fs.StringVar(&opts.SnapshotID, "snapshot-id", "", "optional VM snapshot/base id")
	fs.StringVar(&opts.Summary, "summary", "", "summary used in the generated manifest")
	fs.Var(&checks, "check", "verification command to run before export; may be repeated")
	if err := fs.Parse(args); err != nil {
		log.Fatal(err)
	}
	opts.Checks = checks

	report, err := shipper.ExportPatchset(context.Background(), opts)
	if err != nil {
		log.Fatal(err)
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(data))
}
