package autoputer

import "github.com/yusefmosiah/go-choir/internal/deskworker"

// RunDeskSessionWorker is the host desk-cell session-worker entrypoint the
// daemon re-executes into with `autoputer desk-session`; the body lives in
// internal/deskworker so the dedicated cmd/desk-worker binary shares it.
func RunDeskSessionWorker() int { return deskworker.Run() }
