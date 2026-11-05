package runtimeprompts

// Prompt drift fails CI (MA1.3 review): every choir.<Verb>(...) call example
// in a prompt file for a request verb must carry the request-fields argument
// the parser now requires, and no prompt teaches the deleted Note.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// requestArity is the argument count of each request verb with its request
// fields (choir.go: Cast, Ask, Escalate, Feed, Decline).
var requestArity = map[string]int{"Cast": 4, "Ask": 3, "Escalate": 3, "Feed": 4, "Decline": 3}

var callStart = regexp.MustCompile(`choir\.(Cast|Ask|Escalate|Feed|Decline)\(`)

// topLevelArgs splits the text after an opening paren into its top-level
// arguments, honouring quotes and nesting. ok is false when it never closes.
func topLevelArgs(s string) (args []string, ok bool) {
	depth, start := 0, 0
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if c == '\\' {
				i++
			} else if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '"', '`':
			quote = c
		case '(', '{', '[':
			depth++
		case '}', ']':
			depth--
		case ')':
			if depth == 0 {
				if a := strings.TrimSpace(s[start:i]); a != "" || len(args) > 0 {
					args = append(args, a)
				}
				return args, true
			}
			depth--
		case ',':
			if depth == 0 {
				args = append(args, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	return nil, false
}

func TestPromptRequestExamplesCarryTheRequestFields(t *testing.T) {
	var files []string
	for _, dir := range []string{"overlays", "../promptstore/defaults", "../textureprompts"} {
		ents, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read %s: %v", dir, err)
		}
		for _, e := range ents {
			if !e.IsDir() && !strings.HasSuffix(e.Name(), "_test.go") {
				files = append(files, filepath.Join(dir, e.Name()))
			}
		}
	}
	checked := 0
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		text := string(b)
		for _, m := range callStart.FindAllStringSubmatchIndex(text, -1) {
			verb := text[m[2]:m[3]]
			args, ok := topLevelArgs(text[m[1]:])
			if !ok {
				continue // prose that never closes the call
			}
			checked++
			if len(args) != requestArity[verb] {
				t.Errorf("%s: choir.%s(%s) has %d arguments, want %d (the last is the request fields)", f, verb, strings.Join(args, ", "), len(args), requestArity[verb])
			}
		}
		if strings.Contains(text, "rlm:note") {
			t.Errorf("%s teaches the deleted rlm:note receipt", f)
		}
		for _, bad := range []string{"a Note", "Note to", "Ask and Note", "Cast, Ask or Note", "that Note"} {
			if strings.Contains(text, bad) {
				t.Errorf("%s still has Note prose %q", f, bad)
			}
		}
	}
	if checked < 5 {
		t.Fatalf("only %d examples checked; the extractor lost the prompts", checked)
	}
}
