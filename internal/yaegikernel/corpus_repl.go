package yaegikernel

import (
	"fmt"
	"strings"
)

// The corpus as values (owner, 2026-11-24: "we should definitely be able to
// have the whole corpus and new captures at a desks fingertips (repl)"). Each
// verb returns a typed Go value the cell works on; nothing is printed unless
// the cell prints it, and a value's String() is one summary line, never its
// rows. The host serves them (researchtools/corpus_repl.go, corpusd's
// /internal/platform/captures/{page,near,edges}); none calls a generative model (a text query is one embedding).

// CorpusCapturesPayload defines ActionCorpusCaptures.
type CorpusCapturesPayload struct {
	Since    string `json:"since,omitempty"`
	Until    string `json:"until,omitempty"`
	SourceID string `json:"source_id,omitempty"`
	Kind     string `json:"kind,omitempty"`
	Cursor   string `json:"cursor,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}

// CorpusTextsPayload defines ActionCorpusTexts.
type CorpusTextsPayload struct {
	IDs      []string `json:"ids"`
	MaxChars int      `json:"max_chars,omitempty"`
}

// CorpusNearPayload defines ActionCorpusNear.
type CorpusNearPayload struct {
	Of    string `json:"of"`
	K     int    `json:"k,omitempty"`
	Since string `json:"since,omitempty"`
	Until string `json:"until,omitempty"`
}

// CorpusEdgesPayload defines ActionCorpusEdges.
type CorpusEdgesPayload struct {
	Ref string `json:"ref"`
}

// CorpusRow is one capture's metadata: no text. Kind is "source" (a source's
// poll brought it in) or "read" (a research run fetched it with choir.Read).
type CorpusRow struct {
	CaptureID   string `json:"capture_id"`
	Title       string `json:"title,omitempty"`
	URL         string `json:"url"`
	Host        string `json:"host,omitempty"`
	SourceID    string `json:"source_id,omitempty"`
	Kind        string `json:"kind"`
	Language    string `json:"language,omitempty"`
	FetchedAt   string `json:"fetched_at,omitempty"`
	PublishedAt string `json:"published_at,omitempty"`
	HasText     bool   `json:"has_text"`
}

// CorpusPage is one page of captures, newest fetched first. Next, when set, is
// the cursor of the next page.
type CorpusPage struct {
	Captures []CorpusRow `json:"captures"`
	Next     string      `json:"next,omitempty"`
}

func (p CorpusPage) String() string {
	return fmt.Sprintf("corpus page: %d captures, next=%q (rows are p.Captures; nothing printed)", len(p.Captures), p.Next)
}

// IDs lists the page's capture ids.
func (p CorpusPage) IDs() []string {
	out := make([]string, 0, len(p.Captures))
	for _, c := range p.Captures {
		out = append(out, c.CaptureID)
	}
	return out
}

// CorpusBlock is one block of a capture's text, with the id a quote names.
type CorpusBlock struct {
	ID   string `json:"id,omitempty"`
	Text string `json:"text"`
}

// CorpusText is one capture's text. Err is set when it was not served (not
// this computer's, owner-private, gone).
type CorpusText struct {
	CaptureID string        `json:"capture_id"`
	Title     string        `json:"title,omitempty"`
	URL       string        `json:"url,omitempty"`
	Blocks    []CorpusBlock `json:"blocks,omitempty"`
	Body      string        `json:"body,omitempty"`
	Chars     int           `json:"chars"`
	Truncated bool          `json:"truncated,omitempty"`
	Err       string        `json:"error,omitempty"`
}

// Text is the capture's text: its blocks, a blank line apart.
func (t CorpusText) Text() string {
	if len(t.Blocks) == 0 {
		return t.Body
	}
	parts := make([]string, 0, len(t.Blocks))
	for _, b := range t.Blocks {
		parts = append(parts, b.Text)
	}
	return strings.Join(parts, "\n\n")
}

func (t CorpusText) String() string {
	if t.Err != "" {
		return fmt.Sprintf("%s: not served: %s", t.CaptureID, t.Err)
	}
	return fmt.Sprintf("%s: %q, %d chars (t.Text() is the text)", t.CaptureID, t.Title, t.Chars)
}

// CorpusTexts is the texts of the captures a call named, in order. Unreached
// are the ids left for a next call when the answer reached its byte bound.
type CorpusTexts struct {
	Texts     []CorpusText `json:"texts"`
	Unreached []string     `json:"unreached,omitempty"`
}

func (r CorpusTexts) String() string {
	failed := 0
	for _, t := range r.Texts {
		if t.Err != "" {
			failed++
		}
	}
	return fmt.Sprintf("corpus texts: %d captures (%d not served), %d unreached (r.Texts; nothing printed)", len(r.Texts), failed, len(r.Unreached))
}

// ByID maps a capture id to its text.
func (r CorpusTexts) ByID() map[string]CorpusText {
	m := make(map[string]CorpusText, len(r.Texts))
	for _, t := range r.Texts {
		m[t.CaptureID] = t
	}
	return m
}

// NearCapture is a capture near the query, with the cosine similarity.
type NearCapture struct {
	CaptureID   string  `json:"capture_id"`
	Title       string  `json:"title,omitempty"`
	URL         string  `json:"url"`
	Host        string  `json:"host,omitempty"`
	SourceID    string  `json:"source_id,omitempty"`
	Kind        string  `json:"kind"`
	Language    string  `json:"language,omitempty"`
	FetchedAt   string  `json:"fetched_at,omitempty"`
	PublishedAt string  `json:"published_at,omitempty"`
	HasText     bool    `json:"has_text"`
	Score       float64 `json:"score"`
}

// NearStory is a published story near the query: its latest version's title
// and the best similarity among its passages.
type NearStory struct {
	PublicationID string  `json:"publication_id"`
	VersionID     string  `json:"version_id"`
	Revision      int     `json:"revision"`
	Title         string  `json:"title,omitempty"`
	Day           string  `json:"day,omitempty"`
	Score         float64 `json:"score"`
}

// CorpusNear is the nearest captures and stories, best first. Unavailable
// says why the vectors could not answer (the lists are then empty); State is
// exact, approximate or partial.
type CorpusNear struct {
	Captures []NearCapture `json:"captures"`
	Stories  []NearStory   `json:"stories"`
	// Cut names the lists an answer over its byte bound was shortened in.
	Cut         []string `json:"cut,omitempty"`
	State       string   `json:"state,omitempty"`
	Partial     string   `json:"partial,omitempty"`
	Unavailable string   `json:"unavailable,omitempty"`
}

func (r CorpusNear) String() string {
	if r.Unavailable != "" {
		return "corpus near: unavailable: " + r.Unavailable
	}
	return fmt.Sprintf("corpus near: %d captures, %d stories, state %s (r.Captures, r.Stories; nothing printed)", len(r.Captures), len(r.Stories), r.State)
}

// IDs lists the near captures' ids, best first.
func (r CorpusNear) IDs() []string {
	out := make([]string, 0, len(r.Captures))
	for _, c := range r.Captures {
		out = append(out, c.CaptureID)
	}
	return out
}

// StoryRef is a published story or one of its versions.
type StoryRef struct {
	PublicationID string `json:"publication_id"`
	VersionID     string `json:"version_id"`
	Revision      int    `json:"revision"`
	Latest        bool   `json:"latest"`
	Title         string `json:"title,omitempty"`
}

// RelatedStory is a story joined by a recorded related_story edge. The edge is
// symmetric: later_development does not say which story is later.
type RelatedStory struct {
	PublicationID string `json:"publication_id"`
	Title         string `json:"title,omitempty"`
	Relation      string `json:"relation"`
}

// CorpusEdges is what one ref is joined to. Kind is capture, story or
// version; Missing says why nothing was found. A capture has Stories (the
// published versions that cite it) and SamePage (its other captures). A story
// or version has Story, Versions, Sources (captures the version cites that
// this computer holds) and Related.
type CorpusEdges struct {
	Ref         string         `json:"ref"`
	Kind        string         `json:"kind,omitempty"`
	Missing     string         `json:"missing,omitempty"`
	Capture     *CorpusRow     `json:"capture,omitempty"`
	Stories     []StoryRef     `json:"stories,omitempty"`
	SamePage    []CorpusRow    `json:"same_page,omitempty"`
	Story       *StoryRef      `json:"story,omitempty"`
	Versions    []StoryRef     `json:"versions,omitempty"`
	Sources     []CorpusRow    `json:"sources,omitempty"`
	SourcesMore int            `json:"sources_more,omitempty"`
	Related     []RelatedStory `json:"related,omitempty"`
	// Cut names the lists an answer over its byte bound was shortened in.
	Cut []string `json:"cut,omitempty"`
}

func (r CorpusEdges) String() string {
	if r.Missing != "" {
		return "corpus edges: nothing: " + r.Missing
	}
	return fmt.Sprintf("corpus edges of %s %s: %d stories, %d same-page, %d versions, %d sources, %d related (nothing printed)",
		r.Kind, r.Ref, len(r.Stories), len(r.SamePage), len(r.Versions), len(r.Sources), len(r.Related))
}

// SourceIDs lists the ids of a story's sources.
func (r CorpusEdges) SourceIDs() []string {
	out := make([]string, 0, len(r.Sources))
	for _, c := range r.Sources {
		out = append(out, c.CaptureID)
	}
	return out
}

// CorpusCaptures pages this computer's captures, newest fetched first, as
// metadata rows: since (inclusive) and until (exclusive), each a day or an
// RFC 3339 time on the fetch time; source filters one source id; kind "source"
// (arrived by a source's poll) or "read" (fetched by a research Read) or "";
// cursor is a page's Next. limit 0 means 1000, at most 5000; a page is also cut
// at about 2 MiB, and then Next is set.
func (s *ChoirScope) CorpusCaptures(since, until, source, kind, cursor string, limit int) (CorpusPage, error) {
	var out CorpusPage
	err := s.call(ActionCorpusCaptures, CorpusCapturesPayload{Since: since, Until: until, SourceID: source, Kind: kind, Cursor: cursor, Limit: limit}, &out)
	if out.Captures == nil {
		out.Captures = []CorpusRow{}
	}
	return out, err
}

// CorpusTexts reads the text of up to 200 captures in one call: each capture's
// blocks (id and text) up to maxChars characters (0 means 20000, at most
// 200000), the answer cut at about 3 MiB with Unreached naming the rest. A
// capture of another computer or an owner-private one is not served (Err).
func (s *ChoirScope) CorpusTexts(ids any, maxChars int) (CorpusTexts, error) {
	list, err := captureIDsArg(ids)
	if err != nil {
		return CorpusTexts{}, fmt.Errorf("choir: CorpusTexts: %w", err)
	}
	var out CorpusTexts
	if err := s.call(ActionCorpusTexts, CorpusTextsPayload{IDs: list, MaxChars: maxChars}, &out); err != nil {
		return CorpusTexts{}, err
	}
	for _, t := range out.Texts {
		if t.Err == "" && t.URL != "" {
			s.noteCaptureURL(t.URL)
		}
	}
	return out, nil
}

// CorpusNear finds the nearest captures and published stories to of: a capture
// id, a publication id (pub-...) or any other text, which the host embeds. k is
// how many of each (0 means 20, at most 500); since and until bound the day.
func (s *ChoirScope) CorpusNear(of string, k int, since, until string) (CorpusNear, error) {
	var out CorpusNear
	err := s.call(ActionCorpusNear, CorpusNearPayload{Of: of, K: k, Since: since, Until: until}, &out)
	if out.Captures == nil {
		out.Captures = []NearCapture{}
	}
	if out.Stories == nil {
		out.Stories = []NearStory{}
	}
	return out, err
}

// CorpusEdges reads the edges around a capture id, a publication id or a
// story version id.
func (s *ChoirScope) CorpusEdges(ref string) (CorpusEdges, error) {
	var out CorpusEdges
	err := s.call(ActionCorpusEdges, CorpusEdgesPayload{Ref: strings.TrimSpace(ref)}, &out)
	return out, err
}

func init() {
	choirFunctionDocs["CorpusCaptures"] = FunctionDoc{Args: []string{"since", "until", "source", "kind", "cursor", "limit"}, Summary: "A page of this computer's captures, newest fetched first, as values: p.Captures is a []CorpusRow (CaptureID, Title, URL, Host, SourceID, Kind, Language, FetchedAt, PublishedAt, HasText) and p.Next is the cursor of the next page (empty at the end). since (inclusive) and until (exclusive) are a day (2026-11-20) or an RFC 3339 time on the fetch time; source keeps one source id; kind is \"source\" (a source's poll brought it in), \"read\" (a research Read fetched it) or \"\" for both. limit 0 means 1000, at most 5000; a page is also cut at 2 MiB as written (the cursor is set). Owner-private and withdrawn captures are never listed. Metadata only, and nothing prints unless you print it: p.IDs() is the ids; read text with CorpusTexts.", Example: "p, err := choir.CorpusCaptures(\"2026-11-23\", \"\", \"\", \"source\", \"\", 0)\nfmt.Println(p, err)\nids := p.IDs()"}
	choirFunctionDocs["CorpusTexts"] = FunctionDoc{Args: []string{"ids", "maxChars"}, Summary: "The text of up to 200 captures of this computer in one call: r.Texts holds, in order, a CorpusText for each (CaptureID, Title, URL, Blocks [{ID, Text}], Chars, Truncated, Err; t.Text() joins the blocks). maxChars is per capture (0 means 20000, at most 200000). The answer is cut at its byte bound (2 MiB as written): r.Unreached names the ids to ask for next. A capture that is not this computer's shareable corpus (another computer's, owner-private, withdrawn, unknown) comes back with Err set, always the same Err: read an owner-private one with ReadCapture. Reading text this way taints the run as ReadCapture does. Held by the research desk only: a web child does not have these verbs, so pass it the ids or texts it should read. Quote a block by its ID and the capture's id, as after ReadCapture.", Example: "r, err := choir.CorpusTexts(ids[:20], 4000)\nfor _, t := range r.Texts {\n\tfmt.Println(t.CaptureID, len(t.Text()), t.Err)\n}"}
	choirFunctionDocs["CorpusNear"] = FunctionDoc{Args: []string{"of", "k", "since", "until"}, Summary: "The nearest captures and published stories to of by meaning, best first, each with its cosine similarity (Score, 1 is the same direction): of is a capture id of this computer, a publication id (pub-...) or any text (the host embeds it). r.Captures is a []NearCapture (the CorpusRow fields and Score) and r.Stories a []NearStory (PublicationID, VersionID, Revision, Title, Day, Score: the latest version). k 0 means 20, at most 500 of each (r.Cut names a list shortened at the byte bound; stories are one row each, so fewer than k can come back); since and until are days. r.Unavailable says why the vectors could not answer (a capture not embedded yet has none); r.State is exact, approximate or partial. The capture or story asked about is never in its own list. No generative model is called: a text is embedded, once.", Example: "r, err := choir.CorpusNear(\"harbor strike talks\", 200, \"2026-11-17\", \"\")\nfmt.Println(r, err)\nfor _, c := range r.Captures[:min(5, len(r.Captures))] {\n\tfmt.Println(c.Score, c.Title)\n}"}
	choirFunctionDocs["CorpusEdges"] = FunctionDoc{Args: []string{"ref"}, Summary: "The edges around a capture id, a publication id or a story version id: the only edges that exist. For a capture: r.Stories (the published story versions that cite it, with Latest) and r.SamePage (its other captures of the same page). For a story or version: r.Story, r.Versions, r.Sources (the captures that version cites which this computer holds), and r.Related (stories joined by a recorded related_story edge, with Relation same_incident, later_development or same_subject: the edge has no direction). r.Missing is one answer for every way nothing was found (not held, not shareable, not published, restricted). r.Cut names lists shortened at the byte bound. A story the Texture custodian holds as a document is not here.", Example: "e, err := choir.CorpusEdges(r.Stories[0].PublicationID)\nfmt.Println(e, err)\nids := e.SourceIDs()"}
}
