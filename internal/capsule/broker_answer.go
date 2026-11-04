package capsule

import (
	"errors"
	"time"
)

// A broker read's own bound (docs/problems/engineering-cell-never-returns-2026-11-03.md).
// A plain choir.Poll cell read the broker socket for 46 minutes because the
// read deadline was the caller's (the activation's). The broker bounds a
// cell itself (timeout_ms, else 60 s) and answers a job verb at once, so a
// read longer than that plus a slack is a broker that did not answer.

// ErrBrokerNoAnswer marks a broker RPC whose answer did not come within its
// bound.
var ErrBrokerNoAnswer = errors.New("the capsule broker did not answer")

// ErrBrokerExited marks a broker RPC to a capsule whose broker process
// exited before the capsule's destroy: it fails at once instead of waiting
// in the host-held listener's backlog for its read bound.
var ErrBrokerExited = errors.New("the capsule broker exited")

// brokerGoEvalDefault is the broker's own cell bound when timeout_ms is
// unset (cmd/capsule-broker handleGoEvalSession).
const brokerGoEvalDefault = 60 * time.Second

// brokerAnswerSlack is how long past the broker's own cell bound the host
// reads before it calls the broker silent. A test may shorten it.
var brokerAnswerSlack = 15 * time.Second

// brokerJobAnswerBound bounds a job verb's read (job_start, job_status,
// job_stop). A test may shorten it.
var brokerJobAnswerBound = 60 * time.Second

// BrokerGoEvalAnswerBound is how long the host reads a go_eval answer with
// this timeout_ms: the broker's own bound plus the slack.
func BrokerGoEvalAnswerBound(timeoutMS int) time.Duration {
	bound := brokerGoEvalDefault
	if timeoutMS > 0 {
		bound = time.Duration(timeoutMS) * time.Millisecond
	}
	return bound + brokerAnswerSlack
}
