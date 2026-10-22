package eventtablegrant

// Ways the host grant can fail, written before the code:
//   - a missing, unreadable or malformed grant, or one whose status is not
//     "granted", reads as granted (it must fail closed: off);
//   - a grant written for another computer (a disk attached to the wrong
//     VM) turns this computer on;
//   - an empty credential directory or computer id reads as granted;
//   - a reason quotes the file's contents.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeGrant(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, GrantFile), []byte(content), 0o400); err != nil {
		t.Fatal(err)
	}
}

func TestReadGrantFailsClosed(t *testing.T) {
	const self = "computer-self"
	if st := ReadGrant("", self); st.Granted {
		t.Fatal("no credential directory reads as granted")
	}
	dir := t.TempDir()
	if st := ReadGrant(dir, self); st.Granted || !strings.Contains(st.Reason, "not granted") {
		t.Fatalf("no grant file = %+v", st)
	}
	cases := map[string]string{
		"malformed":        `{"computer_id":`,
		"withheld":         `{"computer_id":"computer-self","status":"withheld"}`,
		"another computer": `{"computer_id":"computer-other","status":"granted","basis":"computer"}`,
		"no computer":      `{"status":"granted","basis":"computer"}`,
		"secret-looking":   `SECRET-CONTENT`,
	}
	for name, content := range cases {
		writeGrant(t, dir, content)
		st := ReadGrant(dir, self)
		if st.Granted {
			t.Fatalf("%s grant reads as granted: %+v", name, st)
		}
		if strings.Contains(st.Reason, "SECRET") || strings.Contains(st.Reason, "computer-other") {
			t.Fatalf("%s: the reason quotes the file: %q", name, st.Reason)
		}
		if err := os.Remove(filepath.Join(dir, GrantFile)); err != nil {
			t.Fatal(err)
		}
	}
	writeGrant(t, dir, `{"computer_id":"computer-self","status":"granted","basis":"computer","granted_at":"2026-10-22T12:00:00Z"}`)
	if st := ReadGrant(dir, self); !st.Granted || st.ComputerID != self {
		t.Fatalf("a well-formed grant for this computer = %+v", st)
	}
	if st := ReadGrant(dir, ""); st.Granted {
		t.Fatal("a grant reads as granted for an unknown computer id")
	}
	if st := ReadGrant(dir, "computer-other"); st.Granted {
		t.Fatal("this computer's grant turns another on")
	}
}
