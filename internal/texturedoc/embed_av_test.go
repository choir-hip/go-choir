package texturedoc

// The video and audio embeds (station L8, last slice). Ways they could fail,
// written before the code:
//   - a video target holds anything but the 11-character YouTube id;
//   - an audio target is http, javascript:, data:, carries credentials, or is
//     not an audio URL;
//   - audio has no title, or video no alt;
//   - a stored doc with only image/file/evidence embeds stops validating;
//   - either kind reads as publishable, or pins an image digest.

import (
	"strings"
	"testing"
)

func videoNode(mutate func(attrs, target map[string]any)) Node {
	target := map[string]any{"video_id": "dQw4w9WgXcQ"}
	attrs := map[string]any{"id": "embed-v", "kind": EmbedKindVideo, "target": target, "alt": "A talk", "caption": "A talk"}
	if mutate != nil {
		mutate(attrs, target)
	}
	return Node{Type: "embed", Attrs: attrs}
}

func audioNode(mutate func(attrs, target map[string]any)) Node {
	target := map[string]any{"url": "https://example.org/ep/42.mp3"}
	attrs := map[string]any{"id": "embed-a", "kind": EmbedKindAudio, "target": target, "title": "Episode 42"}
	if mutate != nil {
		mutate(attrs, target)
	}
	return Node{Type: "embed", Attrs: attrs}
}

func TestVideoAndAudioEmbedsValidate(t *testing.T) {
	doc := docOf(para("p1", "See."), videoNode(nil), audioNode(nil))
	if err := Validate(doc, nil); err != nil {
		t.Fatalf("refused: %v", err)
	}
	if got := EmbedProjectionText(doc.Doc.Content[1]); got != "[video: A talk]" {
		t.Fatalf("video projection %q", got)
	}
	if got := EmbedProjectionText(doc.Doc.Content[2]); got != "[audio: Episode 42]" {
		t.Fatalf("audio projection %q", got)
	}
	if len(PrivateEmbeds(doc)) != 2 || len(EmbeddedDigests(doc)) != 0 {
		t.Fatalf("private and pin nothing")
	}
}

func TestVideoAndAudioEmbedRefusals(t *testing.T) {
	type tc struct {
		node Node
		want string
	}
	for name, c := range map[string]tc{
		"video url not id": {videoNode(func(a, t map[string]any) { t["video_id"] = "https://youtu.be/dQw4w9WgXcQ" }), "video id"},
		"video short id":   {videoNode(func(a, t map[string]any) { t["video_id"] = "abc" }), "video id"},
		"video bad chars":  {videoNode(func(a, t map[string]any) { t["video_id"] = "dQw4w9WgXc\"" }), "video id"},
		"video url field":  {videoNode(func(a, t map[string]any) { t["url"] = "https://x.example/a.mp3" }), "not a video target field"},
		"video no alt":     {videoNode(func(a, t map[string]any) { delete(a, "alt") }), "alt is required"},
		"video title attr": {videoNode(func(a, t map[string]any) { a["title"] = "x" }), "file and audio"},
		"audio http":       {audioNode(func(a, t map[string]any) { t["url"] = "http://example.org/x.mp3" }), "https"},
		"audio javascript": {audioNode(func(a, t map[string]any) { t["url"] = "javascript:alert(1)" }), "https"},
		"audio data":       {audioNode(func(a, t map[string]any) { t["url"] = "data:audio/mp3;base64,AAAA" }), "https"},
		"audio userinfo":   {audioNode(func(a, t map[string]any) { t["url"] = "https://u:p@example.org/x.mp3" }), "credentials"},
		"audio not audio":  {audioNode(func(a, t map[string]any) { t["url"] = "https://example.org/x.html" }), "not an audio"},
		"audio space":      {audioNode(func(a, t map[string]any) { t["url"] = "https://example.org/a b.mp3" }), "spaces"},
		"audio too long":   {audioNode(func(a, t map[string]any) { t["url"] = "https://example.org/" + strings.Repeat("a", 2100) + ".mp3" }), "longer than"},
		"audio no title":   {audioNode(func(a, t map[string]any) { delete(a, "title") }), "title is required"},
		"audio digest":     {audioNode(func(a, t map[string]any) { t["digest"] = strings.Repeat("a", 64) }), "not a audio target field"},
		"audio long title": {audioNode(func(a, t map[string]any) { a["title"] = strings.Repeat("x", 1001) }), "longer than"},
	} {
		t.Run(name, func(t *testing.T) {
			err := Validate(docOf(para("p1", "See."), c.node), nil)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want %q", err, c.want)
			}
		})
	}
}

func TestParseYouTubeURL(t *testing.T) {
	ok := map[string]string{
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ&list=x": "dQw4w9WgXcQ",
		"https://youtu.be/dQw4w9WgXcQ":                       "dQw4w9WgXcQ",
		"https://www.youtube.com/shorts/dQw4w9WgXcQ":         "dQw4w9WgXcQ",
	}
	for u, id := range ok {
		if got, err := ParseYouTubeURL(u); err != nil || got != id {
			t.Errorf("%s: %q %v", u, got, err)
		}
	}
	for _, u := range []string{"", "http://youtu.be/dQw4w9WgXcQ", "https://youtu.be/", "https://www.youtube.com/embed/dQw4w9WgXcQ", "https://evil.example/watch?v=dQw4w9WgXcQ", "javascript:alert(1)"} {
		if _, err := ParseYouTubeURL(u); err == nil {
			t.Errorf("%q accepted", u)
		}
	}
}
