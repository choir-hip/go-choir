package maild

import (
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"
	"unicode"
)

// The digest message (design section 4.4): a fixed template over the selected
// stories. All story text is model-written, so it is escaped, stripped of
// control characters and URL schemes, and never reaches a link: every link is
// https://<primary domain>/autopaper/<slug> built from the slug.
const (
	digestSubjectMaxRunes = 120
	digestTitleMaxRunes   = 200
	digestDekMaxRunes     = 400
	digestVersionHeader   = "v0-autopaper-digest"
	digestDisclosure      = "Written by AI agents from cited sources. Open any story to see them."
)

// digestSectionLabels mirrors AUTOPAPER_SECTIONS in autopaper.ts (the front
// page entry omitted), in the paper's order.
var digestSectionLabels = []struct{ ID, Label string }{
	{"world", "World"},
	{"us_politics", "U.S. Politics"},
	{"business", "Business & Finance"},
	{"tech_ai", "Tech & AI"},
}

var urlSchemeRe = regexp.MustCompile(`(?i)\bhttps?://`)

type digestMail struct {
	Subject string
	Text    string
	HTML    string
	Headers map[string]any
}

// cleanDigestText makes model text safe for one line of a message: control
// characters and line breaks become spaces, runs of space collapse, URL
// schemes are dropped so a mail client has nothing to autolink, and the
// result is cut to max runes.
func cleanDigestText(s string, max int) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == 0x2028 || r == 0x2029 {
			return ' '
		}
		return r
	}, s)
	s = urlSchemeRe.ReplaceAllString(s, "")
	s = strings.Join(strings.Fields(s), " ")
	if max > 0 {
		if runes := []rune(s); len(runes) > max {
			s = strings.TrimSpace(string(runes[:max-1])) + "…"
		}
	}
	return s
}

func digestSectionLabel(id string) string {
	for _, s := range digestSectionLabels {
		if s.ID == id {
			return s.Label
		}
	}
	return ""
}

func digestPlural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

// digestAgeText says how long ago t was, for the story meta line.
func digestAgeText(t, now time.Time) string {
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return digestPlural(int(d/time.Minute), "minute", "minutes") + " ago"
	case d < 24*time.Hour:
		return digestPlural(int(d/time.Hour), "hour", "hours") + " ago"
	default:
		return digestPlural(int(d/(24*time.Hour)), "day", "days") + " ago"
	}
}

func digestBase(domain string) string {
	return "https://" + strings.ToLower(strings.TrimSpace(domain))
}

func digestStoryLink(domain, slug string) string {
	return digestBase(domain) + "/autopaper/" + slug
}

func digestMeta(s digestStory, now time.Time) string {
	var parts []string
	if label := digestSectionLabel(s.Section); label != "" {
		parts = append(parts, label)
	}
	if s.SourceCount != nil {
		parts = append(parts, digestPlural(*s.SourceCount, "source", "sources"))
	}
	parts = append(parts, "updated "+digestAgeText(s.UpdatedAt, now))
	return strings.Join(parts, ", ")
}

// digestGroups orders the stories under the lead: the paper's sections in
// order, then the rest as "More stories".
func digestGroups(stories []digestStory) []struct {
	Label   string
	Stories []digestStory
} {
	type group = struct {
		Label   string
		Stories []digestStory
	}
	var groups []group
	used := map[int]bool{}
	for _, sec := range digestSectionLabels {
		g := group{Label: sec.Label}
		for i, s := range stories {
			if !used[i] && s.Section == sec.ID {
				g.Stories = append(g.Stories, s)
				used[i] = true
			}
		}
		if len(g.Stories) > 0 {
			groups = append(groups, g)
		}
	}
	rest := group{Label: "More stories"}
	for i, s := range stories {
		if !used[i] {
			rest.Stories = append(rest.Stories, s)
		}
	}
	if len(rest.Stories) > 0 {
		groups = append(groups, rest)
	}
	return groups
}

// renderDigest builds the message. runDate is the UTC day of the run (the
// subject's date); now is the clock the ages are measured against.
func renderDigest(sel digestSelection, domain string, runDate, now time.Time) digestMail {
	base := digestBase(domain)
	manage := base + "/?app=email"
	title := func(s digestStory) string {
		t := cleanDigestText(s.Title, digestTitleMaxRunes)
		if t == "" {
			t = "Untitled story"
		}
		if s.Updated {
			t += " (Updated)"
		}
		return t
	}

	var text, body strings.Builder
	date := runDate.UTC().Format("Mon 2 Jan")
	leadTitle := "Today's stories"
	if len(sel.Stories) > 0 {
		leadTitle = cleanDigestText(sel.Stories[0].Title, digestTitleMaxRunes)
	}
	subject := cleanDigestText("Autopaper, "+date+": "+leadTitle, digestSubjectMaxRunes)

	writeStory := func(s digestStory) {
		t := title(s)
		dek := cleanDigestText(s.Dek, digestDekMaxRunes)
		link := digestStoryLink(domain, s.Slug)
		meta := digestMeta(s, now)
		text.WriteString(t + "\n")
		if dek != "" {
			text.WriteString(dek + "\n")
		}
		text.WriteString(meta + "\n" + link + "\n\n")

		body.WriteString(`<p style="margin:0 0 4px;font-size:18px;font-weight:700;line-height:1.3;"><a href="` +
			html.EscapeString(link) + `" style="color:#111827;text-decoration:none;">` + html.EscapeString(t) + `</a></p>`)
		if dek != "" {
			body.WriteString(`<p style="margin:0 0 4px;color:#374151;">` + html.EscapeString(dek) + `</p>`)
		}
		body.WriteString(`<p style="margin:0 0 20px;font-size:13px;color:#6b7280;">` + html.EscapeString(meta) + `</p>`)
	}
	writeLabel := func(label string) {
		text.WriteString(strings.ToUpper(label) + "\n\n")
		body.WriteString(`<h2 style="margin:24px 0 12px;font-size:13px;letter-spacing:.06em;text-transform:uppercase;color:#6b7280;">` +
			html.EscapeString(label) + `</h2>`)
	}

	text.WriteString("Autopaper, " + date + "\n\n")
	body.WriteString(`<h1 style="margin:0 0 20px;font-size:14px;letter-spacing:.06em;text-transform:uppercase;color:#6b7280;">Autopaper, ` +
		html.EscapeString(date) + `</h1>`)
	if len(sel.Stories) > 0 {
		writeStory(sel.Stories[0])
		for _, g := range digestGroups(sel.Stories[1:]) {
			writeLabel(g.Label)
			for _, s := range g.Stories {
				writeStory(s)
			}
		}
	}

	footerText := digestDisclosure + "\n" +
		"Manage this email: " + manage + "\n" +
		"You get this because you turned on the daily digest in Mail. Turn it off there. Replies are not read yet."
	text.WriteString(footerText)

	body.WriteString(`<p style="margin:24px 0 8px;color:#4b5563;">` + html.EscapeString(digestDisclosure) + `</p>` +
		`<p style="margin:0 0 8px;color:#4b5563;">Manage this email: <a href="` + html.EscapeString(manage) + `" style="color:#4b5563;">` +
		html.EscapeString(manage) + `</a></p>` +
		`<p style="margin:0 0 16px;color:#6b7280;font-size:13px;">You get this because you turned on the daily digest in Mail. Turn it off there. Replies are not read yet.</p>`)
	htmlBody := `<!doctype html><html><body style="margin:0;padding:0;background:#ffffff;">` +
		`<main style="max-width:600px;margin:0 auto;padding:24px;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;font-size:16px;line-height:1.55;color:#111827;">` +
		body.String() + choirAutomatedHTMLSignature() + `</main></body></html>`

	return digestMail{
		Subject: subject,
		Text:    appendChoirAutomatedTextSignature(text.String()),
		HTML:    htmlBody,
		Headers: map[string]any{
			"X-Choir-Maild":    digestVersionHeader,
			"List-Unsubscribe": "<" + manage + "&digest=off>",
		},
	}
}
