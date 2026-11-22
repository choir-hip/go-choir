package provider

import (
	"strings"
	"testing"
)

// An artifact_ref's text is the guest's: it must not put a space, a field or a
// newline in the inference line's media summary (first design F0, seat 1).
func TestSummarizeMediaCleansAnArtifactRef(t *testing.T) {
	hostile := "artifact:sha256:ab desk=fake\n2026/11/22 05:00:00 gateway: x"
	msgs := []Message{{Role: "user", Content: []Block{{Type: "image", Source: &MediaSource{Kind: "artifact_ref", Ref: hostile}}}}}
	line := SummarizeMedia(msgs).String()
	if strings.ContainsAny(line, "\n\r\t") || strings.Contains(line, " desk=") || strings.Count(line, " ") != 2 {
		t.Fatalf("summary = %q", line)
	}
}

func TestLogFieldAndLogText(t *testing.T) {
	if got := LogField("a b\nc=d\x00é", 96); got != "abc=d" {
		t.Fatalf("LogField = %q", got)
	}
	if got := LogField("   ", 96); got != "-" {
		t.Fatalf("empty = %q", got)
	}
	if got := LogText("x\n2026/11/22 gateway: inference succeeded", 10); strings.Contains(got, "\n") || !strings.HasPrefix(got, `"`) {
		t.Fatalf("LogText = %q", got)
	}
}
