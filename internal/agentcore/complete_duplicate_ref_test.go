package agentcore

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// FP4 round 4, P1
// (docs/problems/frozen-candidate-lost-when-complete-cites-a-duplicate-execution-ref-2026-10-22.md).
// A Complete that cites one execution ref twice made one recorded command
// per ref; after the durable freeze the granted binder collapses duplicates,
// the count check failed, and a frozen capsule left the model no cell to
// retry in. Ways the fix can fail, written first: a repeated ref still
// yields two commands; dedupe reorders refs (the proposition digest is
// ordered); dedupe drops a distinct ref; an empty or blank ref survives; the
// count check still wraps a nil error ("%!w(<nil>)") and names no cause.

func TestCompleteExecutionRefsCollapseRepeats(t *testing.T) {
	got := completeExecutionRefs([]string{" capsule-exec:sha256:a ", "capsule-exec:sha256:b", "", "capsule-exec:sha256:a", "  ", "capsule-go-eval:sha256:c", "capsule-exec:sha256:b"})
	want := []string{"capsule-exec:sha256:a", "capsule-exec:sha256:b", "capsule-go-eval:sha256:c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("completeExecutionRefs = %q, want %q (first occurrence order, repeats and blanks dropped)", got, want)
	}
}

func TestExecutionEvidenceCountErrorNamesTheCause(t *testing.T) {
	err := executionEvidenceCountError("assignment command evidence unavailable after durable freeze", 5, 4, nil)
	if err == nil {
		t.Fatal("a count mismatch with no resolver error returned no error")
	}
	if strings.Contains(err.Error(), "%!w") || !strings.Contains(err.Error(), "5") || !strings.Contains(err.Error(), "4") {
		t.Fatalf("count mismatch error does not name the counts: %v", err)
	}
	cause := errors.New("capsule execution evidence requires frozen capsule")
	wrapped := executionEvidenceCountError("assignment command evidence unavailable after durable freeze", 5, 0, cause)
	if !errors.Is(wrapped, cause) {
		t.Fatalf("a resolver error is not wrapped: %v", wrapped)
	}
}
