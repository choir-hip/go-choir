package main

import (
	"os"

	"github.com/yusefmosiah/go-choir/internal/autoputer"
	"github.com/yusefmosiah/go-choir/internal/doctool"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "zot-session" {
		os.Exit(autoputer.RunZotSession(os.Stdin, os.Stdout, os.Stderr))
	}
	if len(os.Args) > 1 && os.Args[1] == "desk-session" {
		// Host desk-cell session worker (R3b): serve framed yaegi eval cells on
		// the inherited session socket for a non-capsule desk activation. The
		// worker is the killable subprocess boundary for model-authored Go —
		// the daemon never runs a desk cell in-process.
		os.Exit(autoputer.RunDeskSessionWorker())
	}
	if len(os.Args) > 1 && os.Args[1] == doctool.HelperArg {
		// Document-tool sandbox helper (SDC SEC-15): started by the daemon
		// as the sandbox user; confines itself and execs the tool.
		os.Exit(doctool.RunHelper(os.Args[2:]))
	}
	if len(os.Args) > 1 && os.Args[1] == doctool.ProbeArg {
		// The isolation self-test's probe tool, run inside that sandbox.
		os.Exit(doctool.RunProbe(os.Args[2:]))
	}
	autoputer.Run()
}
