package objectgraph

import (
	"log"
	"time"
)

// BootPhase logs the start of a boot-path step and returns the function that
// logs its end and duration (or its failure). Every store step between the
// process start and the listener is wrapped in one, so a boot that hangs
// leaves a "status=starting" line with no end: that line names the step. The
// 2026-10-21 outage (docs/archive/problems/stored-column-add-blocks-paper-boot-2026-10-21.md)
// had no such line for its ALTER, and took 50 minutes to read.
//
//	done := objectgraph.BootPhase("store: open", "runtime-schema/apply-ddl")
//	err := apply()
//	done(err)
func BootPhase(prefix, name string) func(err error) {
	start := time.Now()
	log.Printf("%s phase=%s status=starting", prefix, name)
	return func(err error) {
		took := time.Since(start).Round(time.Millisecond)
		if err != nil {
			log.Printf("%s phase=%s status=failed duration=%s err=%v", prefix, name, took, err)
			return
		}
		log.Printf("%s phase=%s status=complete duration=%s", prefix, name, took)
	}
}
