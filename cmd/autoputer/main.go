package main

import (
	"log"
	"os"

	"github.com/yusefmosiah/go-choir/internal/autoputer"
	"github.com/yusefmosiah/go-choir/internal/capfloor"
	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/doctool"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "desk-session" {
		// Host desk-cell session worker (R3b): serve framed yaegi eval cells on
		// the inherited session socket for a non-capsule desk activation. The
		// worker is the killable subprocess boundary for model-authored Go —
		// the daemon never runs a desk cell in-process.
		os.Exit(autoputer.RunDeskSessionWorker())
	}
	if len(os.Args) > 1 && os.Args[1] == capsule.SeccompSelfTestArg {
		// The isolation self-test's seccomp leg (?leg=seccomp): a throwaway
		// child that loads one capsule filter and probes clone3 and a new
		// user namespace under it. Only the daemon's self-test starts it.
		os.Exit(capsule.RunSeccompSelfTestChild(os.Args[2:]))
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
	// The daemon's capability floor (SDC SEC-15): before anything starts,
	// drop every capability outside the inventory from the bounding set
	// (enforce re-executes once). Enforce refuses to start rather than run
	// above the floor.
	if err := capfloor.Apply(capfloor.ModeFromEnv(os.Getenv)); err != nil {
		log.Printf("autoputer: %v", err)
		os.Exit(1)
	}
	autoputer.Run()
}
