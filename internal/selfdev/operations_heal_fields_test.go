package selfdev

import "testing"

// The heal compares rows field by field through operationFields. A rendering
// that is not injective hides a real divergence from refused_diverged and lets
// the row be selected: %v printed ["a b","c"] and ["a","b c"] both as
// [a b c]. Each field is compared as its JSON.
func TestOperationFieldsDistinguishVerifierRefsThatPrintTheSameWithPercentV(t *testing.T) {
	left := Operation{OperationID: "op", VerifierRefs: []string{"a b", "c"}}
	right := Operation{OperationID: "op", VerifierRefs: []string{"a", "b c"}}
	l, r := operationFields(left), operationFields(right)
	if l["verifier_refs"] == r["verifier_refs"] {
		t.Fatalf("verifier_refs render the same: %q", l["verifier_refs"])
	}
	if l["operation_id"] != "op" {
		t.Fatalf("a string field is not rendered as itself: %q", l["operation_id"])
	}
	// Equal rows render equal, including a nil route generation.
	same := operationFields(Operation{OperationID: "op", VerifierRefs: []string{"a b", "c"}})
	for name, value := range l {
		if same[name] != value {
			t.Fatalf("equal rows differ in %s: %q vs %q", name, value, same[name])
		}
	}
	generation := uint64(7)
	if operationFields(Operation{RouteGeneration: &generation})["route_generation"] == operationFields(Operation{})["route_generation"] {
		t.Fatal("a set route generation renders like a nil one")
	}
}
