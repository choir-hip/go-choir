package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"path/filepath"

	"github.com/yusefmosiah/go-choir/internal/updater"
)

// pruneMain is `choir-updater prune --root DIR [--budget DURATION]`: the
// boot-time prune of the updater root (internal/updater/prune.go). It needs no
// signer, socket, network, identity or runtime, and runs before the daemon and
// the runtime start (nix/autoputer-vm.nix, go-choir-updater-prune). It exits 0
// whenever the root was examined, even when it skipped or its budget ran out
// (the report line says which): a prune never fails a boot. Only a usage error exits 2; a root that is
// missing or not a directory is a skip.
func pruneMain(args []string, logOut io.Writer) int {
	flags := flag.NewFlagSet("prune", flag.ContinueOnError)
	flags.SetOutput(logOut)
	root := flags.String("root", "", "updater root directory to prune")
	runDir := flags.String("run-dir", "/run/choir", "directory holding the updater daemon's socket and started marker; the prune refuses when either exists")
	budget := flags.Duration("budget", updater.DefaultPruneBudget, "time to spend deleting; the next boot continues")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 || *root == "" || *budget <= 0 {
		fmt.Fprintln(logOut, "usage: choir-updater prune --root DIR [--budget DURATION]")
		return 2
	}
	cleaned := filepath.Clean(*root)
	logger := log.New(logOut, "", 0)
	report := updater.Prune(updater.PruneOptions{Root: cleaned, Budget: *budget, RunDir: *runDir})
	logger.Print(report.Line())
	return 0
}
