package platform

// What an embedder outcome may do to durable state: the one place that says.
//
// Every caller that embeds (the worker's story and capture writers, the
// import's sample, the inbox) reaches the embedder through embedInputs, which
// maps each outcome to exactly one of three effects and hands the caller
// results in those terms, never an error type to inspect:
//
//	refused   the server names this input as the problem (embedder.InputRefusedError: its
//	          two "too large" answers). The only effect that may write anything permanent,
//	          and only about that one input.
//	down      the server cannot answer now (health fails, or any status error that does not
//	          name the input: a 500, 503, 429, 404). Nothing durable is written; the caller
//	          stops, the error wraps errEmbedderDown, and the same work is found again next pass.
//	skip      this one call failed in transport or decoding while the server is healthy. The
//	          input is left out of this pass and nothing is written.
//
// (The import of the Mac's first-pass file, which applied the same rule to a file's
// fate, is built on lane/vec-inbox and not part of this landing.)

import (
	"errors"

	"github.com/yusefmosiah/go-choir/internal/embedder"
)

type embedEffect int

const (
	effectRefused embedEffect = iota
	effectDown
	effectSkip
)

// classifyEmbedOutcome maps the error of one embedder call (health already
// known to be up) to its effect.
func classifyEmbedOutcome(err error) embedEffect {
	var ir *embedder.InputRefusedError
	var se *embedder.StatusError
	switch {
	case errors.As(err, &ir):
		return effectRefused
	case errors.As(err, &se):
		return effectDown
	}
	return effectSkip
}
