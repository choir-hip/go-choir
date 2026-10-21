package mirrorgrant

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// Failure modes pinned here (station FP10 F12, F14): a computer without a
// readable, well-formed, granted record gets mirrors; an ecosystem nobody
// granted, or one the guest does not know, reaches the capsule.

func write(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if body != "" {
		if err := os.WriteFile(filepath.Join(dir, GrantFile), []byte(body), 0o400); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestReadGrantIsOffUnlessGranted(t *testing.T) {
	for name, dir := range map[string]string{
		"no directory": "",
		"no file":      write(t, ""),
		"malformed":    write(t, "{"),
		"withheld":     write(t, `{"computer_id":"c","status":"withheld","ecosystems":["npm"]}`),
		"no ecosystem": write(t, `{"computer_id":"c","status":"granted","ecosystems":[]}`),
		"unknown only": write(t, `{"computer_id":"c","status":"granted","ecosystems":["maven"]}`),
	} {
		if st := ReadGrant(dir); st.Granted || len(st.Ecosystems) != 0 || st.MirrorsOnly || st.Reason == "" {
			t.Errorf("%s: %+v, want off with a reason", name, st)
		}
	}
}

func TestReadGrantKeepsKnownEcosystemsOnly(t *testing.T) {
	st := ReadGrant(write(t, `{"computer_id":"c","status":"granted","basis":"computer","ecosystems":["pypi","maven","npm","npm","git"],"mirrors_only":true}`))
	if !st.Granted || !st.MirrorsOnly || !reflect.DeepEqual(st.Ecosystems, []string{"git", "npm", "pypi"}) {
		t.Fatalf("status = %+v", st)
	}
}

func TestParseEcosystems(t *testing.T) {
	if got := ParseEcosystems(" npm, crates pypi,,git,maven "); !reflect.DeepEqual(got, []string{"crates", "git", "npm", "pypi"}) {
		t.Fatalf("ParseEcosystems = %v", got)
	}
	if got := ParseEcosystems(""); len(got) != 0 {
		t.Fatalf("empty = %v", got)
	}
}
