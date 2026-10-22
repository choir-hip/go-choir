package main

import (
	"os"

	"github.com/yusefmosiah/go-choir/internal/autoputer"
	"github.com/yusefmosiah/go-choir/internal/capsule"
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
	autoputer.Run()
}
