package maild

// Failure modes this file guards (design sections 4.4 and 8). Written before
// digest_render.go.
//
//	M7  Hostile or broken text. Titles and deks are model-written. A title
//	    with <script>, a quote, an ampersand or a line break must not reach
//	    the HTML unescaped, must not split the subject into a second header
//	    (CR and LF stripped), and must not reach a link: links are built from
//	    the slug, never from model text.
//	R1  Remote content. The message carries no remote image, web font,
//	    tracking pixel, redirecting link or http: link. Every story link is
//	    https://<primary domain>/pub/texture/<slug>.
//	R2  The subject runs away. It is cut to 120 characters (runes, not bytes,
//	    so a multi-byte headline is not cut mid-character).
//	R3  The disclosure and the way out are missing. The footer carries the AI
//	    disclosure line, a manage link, and the automated-mail signature; the
//	    message carries the List-Unsubscribe header and the version header.
//	R4  A wrong base URL. The base is https:// plus the configured primary
//	    domain (M11), not a literal.

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

func renderTestSelection(t *testing.T, opts ...rowOpt) digestSelection {
	t.Helper()
	rows := []FrontRow{
		digestRow("one", 30, withImportance(9), withSection("world")),
		digestRow("two", 60, withImportance(8), withSection("business")),
		digestRow("three", 90, withImportance(7), withSection("tech_ai")),
		digestRow("four", 100, withImportance(6)),
	}
	for i := range rows {
		for _, opt := range opts {
			opt(&rows[i])
		}
	}
	sel := selectDigest(rows, digestTestNow, nil)
	if len(sel.Stories) != 4 {
		t.Fatalf("selection = %v", selIDs(sel))
	}
	return sel
}

func TestDigestRenderHostileTextIsEscapedAndSubjectHasNoLineBreaks(t *testing.T) {
	rows := []FrontRow{
		digestRow("one", 30, withImportance(9), withTitle("Bad <script>alert(1)</script> \"quote\" & more\r\nBcc: attacker@example.com"), func(r *FrontRow) {
			r.Dek = "dek with <img src=http://evil.example/x.png> and\nnewline"
		}),
		digestRow("two", 60, withImportance(8)),
		digestRow("three", 90, withImportance(7)),
	}
	sel := selectDigest(rows, digestTestNow, nil)
	m := renderDigest(sel, "choir.news", digestTestNow, digestTestNow)
	if strings.ContainsAny(m.Subject, "\r\n") {
		t.Fatalf("subject has a line break: %q", m.Subject)
	}
	if !strings.HasPrefix(m.Subject, "Autopaper, Tue 20 Oct: ") {
		t.Fatalf("subject = %q", m.Subject)
	}
	for _, raw := range []string{"<script", "<img", "onerror"} {
		if strings.Contains(strings.ToLower(m.HTML), raw) {
			t.Fatalf("html contains unescaped %q:\n%s", raw, m.HTML)
		}
	}
	if !strings.Contains(m.HTML, "&lt;script&gt;") || !strings.Contains(m.HTML, "&amp; more") || !strings.Contains(m.HTML, "&#34;quote&#34;") {
		t.Fatalf("html did not escape the title:\n%s", m.HTML)
	}
	// A URL inside model text must not survive as a link a mail client would
	// autolink (R1): the scheme is dropped.
	if strings.Contains(m.Text, "http://") || strings.Contains(m.HTML, "http://") {
		t.Fatalf("a model-written http:// URL reached the message:\n%s", m.Text)
	}
	// The injected "Bcc:" line must not become its own line in the text part
	// or the subject.
	for _, line := range strings.Split(m.Text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "Bcc:") {
			t.Fatalf("text has a line that starts with Bcc:\n%s", m.Text)
		}
	}
}

func TestDigestRenderHasNoRemoteContentAndOnlyOwnLinks(t *testing.T) {
	m := renderDigest(renderTestSelection(t), "choir.news", digestTestNow, digestTestNow)
	for _, body := range []string{m.Text, m.HTML} {
		lower := strings.ToLower(body)
		for _, banned := range []string{"<script", "<img", "<link", "<iframe", "http://", "@import", "url(", "<style"} {
			if strings.Contains(lower, banned) {
				t.Fatalf("message contains %q:\n%s", banned, body)
			}
		}
	}
	urlRe := regexp.MustCompile(`https?://[^\s"'<>)]+`)
	allowed := regexp.MustCompile(`^https://choir\.news/(pub/texture/[A-Za-z0-9._~-]+|\?app=email(&digest=off)?)$`)
	for _, body := range []string{m.Text, m.HTML} {
		for _, link := range urlRe.FindAllString(strings.ReplaceAll(body, "&amp;", "&"), -1) {
			if !allowed.MatchString(link) {
				t.Fatalf("unexpected link %q in:\n%s", link, body)
			}
		}
	}
	for _, s := range []string{"https://choir.news/pub/texture/slug-one", "https://choir.news/pub/texture/slug-four"} {
		if !strings.Contains(m.Text, s) || !strings.Contains(m.HTML, s) {
			t.Fatalf("missing link %s", s)
		}
	}
}

func TestDigestRenderSubjectIsCutToTheLimitByRunes(t *testing.T) {
	long := strings.Repeat("é", 300)
	rows := []FrontRow{
		digestRow("one", 30, withImportance(9), withTitle(long)),
		digestRow("two", 60, withImportance(8)),
		digestRow("three", 90, withImportance(7)),
	}
	m := renderDigest(selectDigest(rows, digestTestNow, nil), "choir.news", digestTestNow, digestTestNow)
	if n := len([]rune(m.Subject)); n > 120 {
		t.Fatalf("subject has %d runes, want at most 120", n)
	}
	if !strings.Contains(m.Subject, "éé") {
		t.Fatalf("subject lost the headline: %q", m.Subject)
	}
}

func TestDigestRenderFooterHeadersAndBase(t *testing.T) {
	m := renderDigest(renderTestSelection(t), "news.example.test", digestTestNow, digestTestNow)
	if !strings.Contains(m.Text, "Written by AI agents from cited sources.") || !strings.Contains(m.HTML, "Written by AI agents from cited sources.") {
		t.Fatalf("disclosure line missing")
	}
	if !strings.Contains(m.Text, "https://news.example.test/?app=email") {
		t.Fatalf("manage link missing or built from a literal domain:\n%s", m.Text)
	}
	if strings.Contains(m.Text, "choir.news") {
		t.Fatalf("base is not from the configured primary domain:\n%s", m.Text)
	}
	if !strings.Contains(m.Text, choirAutomatedEmailSignature) || !strings.Contains(m.HTML, choirAutomatedEmailSignature) {
		t.Fatalf("automated signature missing")
	}
	if !strings.Contains(strings.ToLower(m.Text), "replies are not read") {
		t.Fatalf("no-reply notice missing")
	}
	if got := m.Headers["X-Choir-Maild"]; got != "v0-autopaper-digest" {
		t.Fatalf("X-Choir-Maild = %v", got)
	}
	if got, _ := m.Headers["List-Unsubscribe"].(string); got != "<https://news.example.test/?app=email&digest=off>" {
		t.Fatalf("List-Unsubscribe = %q", got)
	}
}

func TestDigestRenderGroupsLeadThenSectionsAndMarksUpdated(t *testing.T) {
	sel := renderTestSelection(t)
	sel.Stories[2].Updated = true
	m := renderDigest(sel, "choir.news", digestTestNow, digestTestNow)
	lead := strings.Index(m.Text, "Story one")
	world := strings.Index(m.Text, "Story two")
	if lead < 0 || world < 0 || lead > world {
		t.Fatalf("lead is not first:\n%s", m.Text)
	}
	for _, label := range []string{"Business & Finance", "Tech & AI", "More stories"} {
		// The text part sets labels in capitals; the HTML part keeps case.
		if !strings.Contains(strings.ToLower(m.Text), strings.ToLower(label)) || !strings.Contains(m.HTML, strings.ReplaceAll(label, "&", "&amp;")) {
			t.Fatalf("text missing section label %q:\n%s", label, m.Text)
		}
	}
	if !strings.Contains(m.Text, "Updated") {
		t.Fatalf("revised story not marked Updated:\n%s", m.Text)
	}
	if !strings.Contains(m.Text, "4 sources") || !strings.Contains(m.Text, "30 minutes ago") && !strings.Contains(m.Text, "1 hour ago") {
		t.Fatalf("source count or age missing:\n%s", m.Text)
	}
}

func TestDigestAgeText(t *testing.T) {
	now := digestTestNow
	cases := map[time.Duration]string{
		10 * time.Second: "just now",
		5 * time.Minute:  "5 minutes ago",
		time.Hour:        "1 hour ago",
		3 * time.Hour:    "3 hours ago",
		30 * time.Hour:   "1 day ago",
	}
	for d, want := range cases {
		if got := digestAgeText(now.Add(-d), now); got != want {
			t.Errorf("age %v = %q, want %q", d, got, want)
		}
	}
}
