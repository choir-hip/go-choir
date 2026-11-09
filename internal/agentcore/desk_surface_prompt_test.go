package agentcore

import (
	"regexp"
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
		{agentprofile.Texture, nil, []string{"choir.ReadDoc()", "choir.Write(edits string, why string, pSupported ...float64)", "choir.Done(why string)", "choir.Wait(why string)", "choir.Reject(record string, why string)", "choir.Cast("}, []string{"choir.Exec(", "choir.ApplyTexture(", "choir.Judge(", "choir.System1(", "choir.RelateStories(", "choir.Relate("}},
		{agentprofile.Management, nil, []string{"choir.Cast(", "choir.ProductAPI(", "choir.ReadCapture(", "choir.Verify("}, []string{"choir.Write(", "choir.Judge(", "choir.System1(", "choir.RelateStories(", "choir.Relate("}},
		{agentprofile.Research, nil, []string{"choir.SearchStart(", "choir.Searches.Get(", "choir.SearchWait(", "choir.Read(", "choir.ReadCapture(", "choir.SearchWireCorpus(", "choir.System1(", "choir.RelateStories(", "choir.Relate("}, []string{"choir.Exec(", "choir.WriteFile(", "choir.WebSearch(", "choir.Judge("}},
		{agentprofile.Engineering, map[string]any{runMetadataEngineeringSlot: "verifier"}, []string{"choir.Exec(", "choir.InspectBundle()"}, []string{"choir.Verify(", "choir.System1(", "choir.RelateStories(", "choir.Relate("}},
	}
	for _, tc := range cases {
		block := deskSurfacePrompt(tc.profile, &types.RunRecord{Metadata: tc.meta})
		if !strings.Contains(block, "choir.Help(") {
			t.Errorf("%s surface does not say how to get more: %q", tc.profile, block)
		}
		for _, want := range tc.want {
			if !blockNamesVerb(block, want) {
				t.Errorf("%s surface missing %q", tc.profile, want)
			}
		}
		for _, absent := range tc.absent {
			if blockNamesVerb(block, absent) {
				t.Errorf("%s surface lists %q, which it cannot call", tc.profile, absent)
			}
		}
	}
}

// blockNamesVerb reports whether the prompt block names the verb in a card
// (choir.Name(...)) or in its cold-verb line. want is "choir.Name(" or
// "choir.Name()"-style text from the table above.
func blockNamesVerb(block, want string) bool {
	if strings.Contains(block, want) {
		return true
	}
	m := regexp.MustCompile(`choir\.(\w+)`).FindStringSubmatch(want)
	if m == nil {
		return false
	}
	for _, line := range strings.Split(block, "\n") {
		if rest, ok := strings.CutPrefix(line, "Also available, by name: "); ok {
			names, _, _ := strings.Cut(rest, ". Call choir.Help")
			for _, n := range strings.Split(names, ", ") {
				if n == m[1] {
					return true
				}
			}
		}
	}
	return false
}
