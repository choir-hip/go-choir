package main

import (
	"os"

	"github.com/yusefmosiah/go-choir/internal/projectionbase"
)

// publishesParts reports whether checkpointd publishes this computer's
// bases in the parts form (S4 of
// docs/problems/paper-never-checkpointed-2026-10-27.md), by the
// CHECKPOINTD_BASE_PARTS_ONLY selector, read at each publish. checkpointd
// does not know a computer's warmness class, so only computer ids and "*"
// select here; a class: token selects nothing until a class source exists.
func publishesParts(computerID string) bool {
	return projectionbase.SelectsComputer(projectionbase.ParseComputerSelector(os.Getenv(projectionbase.PartsSelectorEnv)), computerID, "")
}
