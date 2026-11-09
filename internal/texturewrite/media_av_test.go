package texturewrite

// The video and audio tokens (station L8, last slice). Ways they could fail,
// written before the code:
//  1. an arbitrary URL (a look-alike host, http, javascript:, data:, a
//     userinfo trick) reaches a document that plays it;
//  2. the video stores the URL instead of the canonical id, so the renderer
//     must trust a string;
//  3. an audio URL that is not https or not audio is stored;
//  4. inline use or a trailing word silently drops text or nests an embed;
//  5. an @all that copies the frame line changes the block, or the desk
//     cannot remove its own block;
//  6. a caption or title is unbounded, or a refusal does not name the line.

import (
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/texturedoc"
)

func TestVideoTokenStoresTheCanonicalID(t *testing.T) {
	for _, u := range []string{
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		"https://youtube.com/watch?v=dQw4w9WgXcQ&t=42s",
		"https://youtu.be/dQw4w9WgXcQ?si=abc",
		"https://www.youtube.com/shorts/dQw4w9WgXcQ",
		"https://m.youtube.com/watch?v=dQw4w9WgXcQ",
	} {
		res, err := applyMedia(t, baseDoc(paragraph("p1", "Intro.")), "@insert end\n{{video "+u+` "The talk"}}`, nil)
		if err != nil {
			t.Fatalf("%s: %v", u, err)
		}
		e := firstEmbed(t, res.Doc)
		if e.Kind != texturedoc.EmbedKindVideo || e.VideoID != "dQw4w9WgXcQ" || e.Caption != "The talk" || e.Alt != "The talk" {
			t.Fatalf("%s: embed %+v", u, e)
		}
		if err := texturedoc.Validate(res.Doc, res.Entities); err != nil {
			t.Fatalf("result invalid: %v", err)
		}
	}
	// No caption: alt falls back so the block stays accessible.
	res, err := applyMedia(t, baseDoc(paragraph("p1", "Intro.")), "@insert end\n{{video https://youtu.be/dQw4w9WgXcQ}}", nil)
	if err != nil || firstEmbed(t, res.Doc).Alt == "" {
		t.Fatalf("no caption: %v", err)
	}
}

func TestVideoTokenRefusals(t *testing.T) {
	base := baseDoc(paragraph("p1", "Intro."))
	for name, tc := range map[string]struct{ script, want string }{
		"http":         {"@insert end\n{{video http://www.youtube.com/watch?v=dQw4w9WgXcQ \"c\"}}", "https"},
		"javascript":   {"@insert end\n{{video javascript:alert(1) \"c\"}}", "https"},
		"data":         {"@insert end\n{{video data:text/html,x \"c\"}}", "https"},
		"look-alike":   {"@insert end\n{{video https://youtube.com.evil.example/watch?v=dQw4w9WgXcQ \"c\"}}", "not a YouTube URL"},
		"userinfo":     {"@insert end\n{{video https://youtube.com@evil.example/watch?v=dQw4w9WgXcQ \"c\"}}", "credentials"},
		"vimeo":        {"@insert end\n{{video https://vimeo.com/12345 \"c\"}}", "not a YouTube URL"},
		"bad id":       {"@insert end\n{{video https://www.youtube.com/watch?v=short \"c\"}}", "video id"},
		"no id":        {"@insert end\n{{video https://www.youtube.com/ \"c\"}}", "video id"},
		"two quotes":   {"@insert end\n{{video https://youtu.be/dQw4w9WgXcQ \"a\" \"b\"}}", "one caption"},
		"inline":       {"@insert end\nWatch {{video https://youtu.be/dQw4w9WgXcQ \"c\"}} now", "block of its own"},
		"trailing":     {"@insert end\n{{video https://youtu.be/dQw4w9WgXcQ \"c\"}} more", "block of its own"},
		"no url":       {"@insert end\n{{video}}", "needs a YouTube URL"},
		"long caption": {"@insert end\n{{video https://youtu.be/dQw4w9WgXcQ \"" + strings.Repeat("x", 1001) + "\"}}", "longer than"},
		"unterminated": {"@insert end\n{{video https://youtu.be/dQw4w9WgXcQ \"c\"", "}}"},
		"long url":     {"@insert end\n{{video https://youtu.be/dQw4w9WgXcQ?" + strings.Repeat("a", 3000) + " \"c\"}}", "longer than"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := applyMedia(t, base, tc.script, nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.want)
			}
			if !strings.Contains(err.Error(), "line 2") && !strings.Contains(err.Error(), "2:") && !strings.Contains(err.Error(), "ine") {
				t.Fatalf("refusal does not name a line: %v", err)
			}
		})
	}
}

func TestAudioTokenMakesAnAudioBlock(t *testing.T) {
	for _, u := range []string{
		"https://example.org/ep/42.mp3",
		"https://cdn.example.org/a/ep.M4A?token=1",
		"https://example.org/x.ogg",
		"https://example.org/x.opus",
		"https://example.org/x.wav",
		"https://traffic.megaphone.fm/ABC1234567890",
		"https://dts.podtrac.com/redirect.mp3/x/y",
	} {
		res, err := applyMedia(t, baseDoc(paragraph("p1", "Intro.")), "@insert end\n{{audio "+u+` "Episode 42"}}`, nil)
		if err != nil {
			t.Fatalf("%s: %v", u, err)
		}
		e := firstEmbed(t, res.Doc)
		if e.Kind != texturedoc.EmbedKindAudio || e.URL != u || e.Title != "Episode 42" {
			t.Fatalf("%s: embed %+v", u, e)
		}
		if err := texturedoc.Validate(res.Doc, res.Entities); err != nil {
			t.Fatalf("result invalid: %v", err)
		}
	}
	// Title defaults to the file name.
	res, err := applyMedia(t, baseDoc(paragraph("p1", "Intro.")), "@insert end\n{{audio https://example.org/ep/42.mp3}}", nil)
	if err != nil || firstEmbed(t, res.Doc).Title != "42.mp3" {
		t.Fatalf("default title: %v", err)
	}
}

func TestAudioTokenRefusals(t *testing.T) {
	base := baseDoc(paragraph("p1", "Intro."))
	for name, tc := range map[string]struct{ script, want string }{
		"http":           {"@insert end\n{{audio http://example.org/x.mp3 \"t\"}}", "https"},
		"javascript":     {"@insert end\n{{audio javascript:alert(1) \"t\"}}", "https"},
		"data":           {"@insert end\n{{audio data:audio/mp3;base64,AAAA \"t\"}}", "https"},
		"not audio":      {"@insert end\n{{audio https://example.org/page.html \"t\"}}", "not an audio file"},
		"no extension":   {"@insert end\n{{audio https://example.org/ep/42 \"t\"}}", "not an audio file"},
		"userinfo":       {"@insert end\n{{audio https://u:p@example.org/x.mp3 \"t\"}}", "credentials"},
		"two quotes":     {"@insert end\n{{audio https://example.org/x.mp3 \"a\" \"b\"}}", "one title"},
		"inline":         {"@insert end\nListen {{audio https://example.org/x.mp3 \"t\"}} now", "block of its own"},
		"long title":     {"@insert end\n{{audio https://example.org/x.mp3 \"" + strings.Repeat("x", 1001) + "\"}}", "longer than"},
		"no url":         {"@insert end\n{{audio}}", "needs a https audio URL"},
		"host lookalike": {"@insert end\n{{audio https://traffic.megaphone.fm.evil.example/x \"t\"}}", "not an audio file"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := applyMedia(t, base, tc.script, nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestStoredVideoAndAudioFramesAreTokensTheDeskCanWriteBack(t *testing.T) {
	script := "@insert end\n{{video https://youtu.be/dQw4w9WgXcQ \"Talk\"}}\n\n{{audio https://example.org/ep.mp3 \"Episode\"}}"
	res, err := applyMedia(t, baseDoc(paragraph("p1", "Intro.")), script, nil)
	if err != nil {
		t.Fatal(err)
	}
	blocks := DocBlocks(res.Doc)
	if len(blocks) != 3 || !strings.HasPrefix(blocks[1].Text, "{{video ") || blocks[2].Text != `{{audio https://example.org/ep.mp3 "Episode"}}` {
		t.Fatalf("frame blocks %+v", blocks)
	}
	before := embedJSON(t, res.Doc)
	res2, err := applyMedia(t, res.Doc, "@all\nIntro, rewritten.\n\n"+blocks[1].Text+"\n\n"+blocks[2].Text, nil)
	if err != nil {
		t.Fatal(err)
	}
	if after := embedJSON(t, res2.Doc); strings.Join(after, "|") != strings.Join(before, "|") || len(texturedoc.Embeds(res2.Doc)) != 2 {
		t.Fatalf("embeds changed: %v vs %v", after, before)
	}
	res3, err := applyMedia(t, res.Doc, "@delete b2 b3", nil)
	if err != nil || len(texturedoc.Embeds(res3.Doc)) != 0 {
		t.Fatalf("delete: %v", err)
	}
	checks, _ := Checks(res.Doc, "doc", "rev")
	if len(checks) != 0 || len(res.Entities) != 0 {
		t.Fatalf("av blocks made citations")
	}
}
