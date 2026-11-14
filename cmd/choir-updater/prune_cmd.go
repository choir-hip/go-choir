package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/yusefmosiah/go-choir/internal/updater"
)

// pruneMain is `choir-updater prune --root DIR [--budget DURATION]`: the
// boot-time prune of the updater root (internal/updater/prune.go). It needs no
// signer, socket, network, identity or runtime, and runs before the daemon and
// the runtime start (nix/autoputer-vm.nix, go-choir-updater-prune). It exits 0
// whenever the root was examined, even when it skipped or its budget ran out
// (the report line says which): a prune never fails a boot. A usage error, or a
// root that is not a directory, exits 2.
func pruneMain(args []string, logOut io.Writer) int {
	flags := flag.NewFlagSet("prune", flag.ContinueOnError)
	flags.SetOutput(logOut)
	root := flags.String("root", "", "updater root directory to prune")
	budget := flags.Duration("budget", updater.DefaultPruneBudget, "time to spend deleting; the next boot continues")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 || *root == "" || *budget <= 0 {
		fmt.Fprintln(logOut, "usage: choir-updater prune --root DIR [--budget DURATION]")
		return 2
	}
	cleaned := filepath.Clean(*root)
	if info, err := os.Stat(cleaned); err != nil || !info.IsDir() {
		fmt.Fprintf(logOut, "choir-updater: prune: root %s is not a directory\n", cleaned)
		return 2
	}
	logger := log.New(logOut, "", 0)
	report := updater.Prune(updater.PruneOptions{Root: cleaned, Budget: *budget})
	logger.Print(report.Line())
	return 0
}

