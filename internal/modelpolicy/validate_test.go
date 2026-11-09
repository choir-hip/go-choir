package modelpolicy

// ValidateText is the write-time gate on System/model-policy.toml. Ways it
// could fail:
//   - it refuses the platform's own default file (every first write fails);
//   - it accepts what the loader would reject (so a saved file falls back
//     silently), or refuses what the loader accepts (a working edit blocked);
//   - an error does not name the line or field the writer must fix.

import (
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
)

func TestValidateTextAcceptsThePlatformDefault(t *testing.T) {
	if err := ValidateText(defaultPolicyText(provideriface.Config{})); err != nil {
		t.Fatalf("the platform default is refused: %v", err)
	}
}

func TestValidateTextNamesTheFieldToFix(t *testing.T) {
	cases := map[string]struct{ text, want string }{
		"syntax":      {"[defaults]\nmodel = \"pool:default\"\noops\n", "line 3"},
		"role":        {"[defaults]\nmodel = \"pool:default\"\n[roles.zzz]\nmodel = \"pool:default\"\n", "zzz"},
		"pool":        {"[defaults]\nmodel = \"pool:gone\"\n", "gone"},
		"key":         {"[defaults]\nmodel = \"pool:default\"\nmodle = \"x\"\n", "line 3"},
		"effort":      {"[defaults]\nmodel = \"pool:default\"\nreasoning = \"warp\"\n", "warp"},
		"model":       {"[defaults]\nprovider = \"chatgpt\"\nmodel = \"no-such-model\"\n", "no-such-model"},
		"no defaults": {"[roles.texture]\nmodel = \"pool:default\"\nmodel = \"\"\n", "texture"},
	}
	for name, tc := range cases {
		err := ValidateText(tc.text)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: err = %v, want it to name %q", name, err, tc.want)
		}
	}
}

func TestValidateTextAcceptsAPinnedCataloguedModel(t *testing.T) {
	text := "[defaults]\nprovider = \"chatgpt\"\nmodel = \"gpt-6-luna\"\nreasoning = \"high\"\n"
	if err := ValidateText(text); err != nil {
		t.Fatal(err)
	}
}
