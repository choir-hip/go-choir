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
// A file's fate follows from the same rule (importFate): it is refused, terminally,
// only for a fault of the file itself; anything else is "cannot be done now" and
// the file waits where it is.

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

// errVectorImportCannotCheck means a file could not be checked enough now (too
// few sample rows could be embedded): not a fault of the file.
var errVectorImportCannotCheck = errors.New("vector import: the file cannot be checked now")

// errVectorImportShortDraw marks a shortfall found in a draw that covered only
// part of the file (more rows than vectorImportCandidates): the next draw is
// different, so it is no proof about the rows not drawn.
var errVectorImportShortDraw = errors.New("the draw covered only part of the file")

type importFateKind int

const (
	fateDone importFateKind = iota
	// fateRefuse is terminal: a fault of the file itself (ErrVectorImportRefused:
	// format, model, dimension, bounds, a text this host does not cut the same,
	// a sampled vector that disagrees with this host's embedding of a row it could embed).
	fateRefuse
	// fateRetry leaves the file where it is: the server cannot answer, the file cannot be
	// checked now, or something other than the file failed. Unstarted work, not resumed work.
	fateRetry
)

func importFate(err error) importFateKind {
	switch {
	case err == nil:
		return fateDone
	case errors.Is(err, ErrVectorImportRefused):
		return fateRefuse
	}
	return fateRetry
}
