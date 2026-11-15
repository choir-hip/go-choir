package agentcore

import (
	"strings"
)

// Failure of one Jev call must not lose a job, and handling it must not depend
// on reading the vendor's reason: the gateway answers every upstream failure,
// a vendor refusal for size included, with 502 "Jev judgment provider
// unavailable" (internal/gateway/jev.go), so the caller never sees why.
//
// The rule, for a call that fails (not a cancelled context):
//   - A rate refusal (429, or a 529 overloaded) backs off and is asked again,
//     up to jevRateTries more times, whole. It is never halved: halving a
//     rate-limited call only makes more calls.
//   - A refusal known to be for size (a hint in the text, secondary: the
//     gateway does not relay it today) halves the call at once.
//   - Any other failure is asked once more after a backoff; if it fails again
//     and the call carries more than one unit, it is halved at a unit
//     boundary, to jevHalvingDepth levels. A half that fails halves again
//     without a retry of its own (it follows two failures of its parent).
//
// Worst case on a persistent failure, then: the call twice, its two halves,
// their four quarters, eight calls, and every capture of a persistent failure
// unlabelled; a failure that a smaller request avoids loses nothing.
const (
	jevHalvingDepth = 2
	jevRateTries    = 2
)

type jevAction int

const (
	jevFail jevAction = iota
	jevRetry
	jevHalve
)

// jevNextStep is what to do after err on a call that has been tried retries
// more times than its first, sits depth halvings below the original and can
// (halvable) be split. forcedBy names the reason a halving is logged with.
func jevNextStep(err error, retries, depth int, halvable bool) (act jevAction, forcedBy string) {
	if err == nil {
		return jevFail, ""
	}
	canHalve := halvable && depth < jevHalvingDepth
	switch {
	case jevRateRefused(err):
		if retries < jevRateTries {
			return jevRetry, ""
		}
		return jevFail, ""
	case jevSizeRefused(err) && canHalve:
		return jevHalve, "vendor_refused"
	case retries < 1:
		return jevRetry, ""
	case canHalve:
		return jevHalve, "call_failed"
	}
	return jevFail, ""
}

// jevRateRefused reports a provider's real rate refusal: a 429 or an
// overloaded 529. Backing off after it is allowed.
func jevRateRefused(err error) bool {
	msg := strings.ToLower(err.Error())
	for _, s := range []string{"429", "rate limit", "rate_limit", "too many requests", "529", "overloaded"} {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}

// jevSizeRefused is a secondary hint that a request was refused for its size:
// the vendor's max_tokens_exceeded and the common phrasings. The gateway does
// not relay the vendor's reason today, so nothing relies on this firing.
func jevSizeRefused(err error) bool {
	msg := strings.ToLower(err.Error())
	for _, s := range []string{"max_tokens_exceeded", "context length", "context window", "maximum context", "request entity too large", "payload too large", "too many tokens", "token limit"} {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}
