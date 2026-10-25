package store

import (
	"encoding/json"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// P7 (trace review round 4): the evidence embed is not owner input; an
// owner's edit, and any other agent-principal revision, still is.
func TestEvidenceEmbedRevisionIsNotOwnerInput(t *testing.T) {
	embed := json.RawMessage(`{"texture_evidence_embed":{"operation_id":"selfdev-f3076820","digests":["aa"]},"texture_embedded_assets":["aa"],"input_origin":"user_prompt"}`)
	for name, tc := range map[string]struct {
		rev  types.Revision
		want bool
	}{
		"the evidence embed":               {types.Revision{AuthorKind: types.AuthorAppAgent, AuthorLabel: "texture", ParentRevisionID: "r1", Metadata: embed}, false},
		"an owner edit":                    {types.Revision{AuthorKind: types.AuthorUser, ParentRevisionID: "r2", Metadata: embed}, true},
		"another agent-principal revision": {types.Revision{AuthorKind: types.AuthorAppAgent, AuthorLabel: "management", ParentRevisionID: "r1", Metadata: json.RawMessage(`{"input_origin":"user_prompt"}`)}, true},
		"the desk's own write":             {types.Revision{AuthorKind: types.AuthorAppAgent, ParentRevisionID: "r1", Metadata: json.RawMessage(`{"source":"texture_cell"}`)}, false},
	} {
		if got := textureRevisionIsOwnerInput(tc.rev); got != tc.want {
			t.Errorf("%s: owner input = %v, want %v", name, got, tc.want)
		}
	}
}
