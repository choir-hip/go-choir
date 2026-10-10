package agentcore

import (
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Trace review F13: every desk's system prompt names its exact REPL surface
// (generated from the desk's choir exports) and how to get more (choir.Help).
func TestEveryDeskPromptCarriesItsSurface(t *testing.T) {
	cases := []struct {
		profile string
		meta    map[string]any
		want    []string
		absent  []string
	}{
		{agentprofile.Texture, nil, []string{"choir.ReadDoc()", "choir.ApplyTexture(edit any)"}, []string{"choir.Exec("}},
		{agentprofile.Management, nil, []string{"choir.Cast(", "choir.ProductAPI("}, []string{"choir.ApplyTexture("}},
		{agentprofile.Research, nil, []string{"choir.WebSearch(", "choir.FetchURL("}, []string{"choir.Exec(", "choir.WriteFile("}},
		{agentprofile.Engineering, map[string]any{runMetadataEngineeringSlot: "verifier"}, []string{"choir.Exec(", "choir.Verify(", "choir.InspectBundle()"}, nil},
	}
	for _, tc := range cases {
		block := deskSurfacePrompt(tc.profile, &types.RunRecord{Metadata: tc.meta})
		if !strings.Contains(block, "choir.Help()") {
			t.Errorf("%s surface does not say how to get more: %q", tc.profile, block)
		}
		for _, want := range tc.want {
			if !strings.Contains(block, want) {
				t.Errorf("%s surface missing %q", tc.profile, want)
			}
		}
		for _, absent := range tc.absent {
			if strings.Contains(block, absent) {
				t.Errorf("%s surface lists %q, which it cannot call", tc.profile, absent)
			}
		}
	}
}
