package agentcore

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// A desk's product API call is labelled by what its answer carries
// (2026-11-08): the paper's publishes asked because a Texture start through
// POST /api/prompt-bar labelled the run prompt-bar, and the story list read
// labelled it texture.
func TestProductAPICallsAreLabelledByWhatTheyServe(t *testing.T) {
	for _, c := range []struct {
		method, path string
		want         []string
	}{
		{"POST", "/api/prompt-bar", nil},
		{"POST", "/api/autopaper/sources/schedules", nil},
		{"GET", "/api/texture/documents", []string{types.OwnerPrivateLabel("story")}},
		{"GET", "/api/texture/documents?limit=20", []string{types.OwnerPrivateLabel("story")}},
		{"GET", "/api/autopaper/events", []string{types.OwnerPrivateLabel("capture"), types.LabelWorldUntrusted}},
		{"GET", "/api/autopaper/sources/schedules", []string{types.OwnerPrivateLabel("autopaper")}},
		{"GET", "/api/prompt-bar", []string{types.OwnerPrivateLabel("prompt-bar")}},
		{"GET", "/api/computers/x", []string{types.OwnerPrivateLabel("computers")}},
		{"POST", "/api/model-policy/verify", []string{types.OwnerPrivateLabel("model-policy")}},
		{"POST", "/api/run-acceptances/synthesize", []string{types.OwnerPrivateLabel("run-acceptances")}},
		{"", "/api/run-acceptances", []string{types.OwnerPrivateLabel("run-acceptances")}},
	} {
		got := productAPIReadLabels(c.method, c.path)
		if len(got.Strings()) != len(c.want) {
			t.Errorf("%s %s = %v, want %v", c.method, c.path, got.Strings(), c.want)
			continue
		}
		for _, w := range c.want {
			if !got.Has(w) {
				t.Errorf("%s %s = %v, want %v", c.method, c.path, got.Strings(), c.want)
			}
		}
	}
}
