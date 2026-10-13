package types

import (
	"encoding/json"
	"testing"
)

// Failure modes pinned here: a renamed or omitempty field silently changes
// the wire shape every citation is stored in; a round trip loses a field.
func TestRefWireShape(t *testing.T) {
	ref := Ref{Doc: "capture:abc", Version: "v3", CID: "0123456789abcdef.1", Hash: "sha256:ff", Quote: "the source's words"}
	data, err := json.Marshal(ref)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"doc":"capture:abc","version":"v3","cid":"0123456789abcdef.1","hash":"sha256:ff","quote":"the source's words"}`
	if string(data) != want {
		t.Fatalf("json = %s, want %s", data, want)
	}
	var back Ref
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if back != ref {
		t.Fatalf("round trip = %+v, want %+v", back, ref)
	}
	empty, _ := json.Marshal(Ref{})
	if string(empty) != `{"doc":"","version":"","cid":"","hash":"","quote":""}` {
		t.Fatalf("empty ref json = %s", empty)
	}
}
