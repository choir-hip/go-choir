// Command desk-worker is the dedicated desk-cell session worker: the same
// worker `autoputer desk-session` runs, without the daemon's Dolt engine,
// gRPC and HTTP server linked in. The daemon prefers it when it sits next to
// the autoputer binary (agentcore deskWorkerBinary).
package main

import (
	"os"

	"github.com/yusefmosiah/go-choir/internal/deskworker"
)

func main() { os.Exit(deskworker.Run()) }
