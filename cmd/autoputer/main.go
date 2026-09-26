package main

import (
	"os"

	"github.com/yusefmosiah/go-choir/internal/autoputer"
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
	autoputer.Run()
}
