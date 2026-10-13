package texturewrite

// Inline emphasis in a Write (docs/problems/autopaper-story-prose-markdown-
// and-meta-2026-10-13.md). Published stories showed "**Status.**" as
// literal asterisks: Write parsed block markdown but not inline emphasis.
// Ways it can fail, written before the code:
//
//  1. **bold**, *italic* or `code` reach a text node as characters instead
//     of strong, emphasis or code marks.
//  2. A {{cite}} inside bold text is swallowed, marked, or left unresolved;
//     the citation is an atom with no marks and the words around it keep
//     their emphasis.
//  3. The quote check, the carried-citation check or the support pair hash
//     read the delimiters: plain text must be the words without ** or *.
//  4. The frame shows a marked block without its markdown, so a block
//     written back unchanged no longer equals its frame text and loses its
//     id, or a rewrite silently drops the emphasis.
//  5. Asterisks that are not emphasis ("5 * 3", "a ** b", an unclosed "**")
//     vanish or turn half a sentence bold.
//  6. A code span's contents are read as emphasis or as a citation token.
//  7. The marked document does not validate against the Texture schema.

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/texturedoc"
)

type run struct {
	text  string
	marks string // comma-joined mark types, "" for none; "@ref" for a citation
}

func runsOf(n texturedoc.Node) []run {
	var out []run
	for _, c := range n.Content {
		switch c.Type {
		case "source_ref":
			out = append(out, run{marks: "@ref"})
		case "text":
			var ms []string
			for _, m := range c.Marks {
				ms = append(ms, m.Type)
			}
			out = append(out, run{text: c.Text, marks: strings.Join(ms, ",")})
		}
	}
	return out
}

func writeOne(t *testing.T, body string) (Result, texturedoc.Node) {
	t.Helper()
	doc, entities := citedBase(t)
	res, err := Apply(context.Background(), Input{
		OwnerID: "owner", DocID: "doc", Seed: "rev-em", Base: doc, Entities: entities,
		Script: "@insert end\n" + body, Resolver: newFakeCaptures(),
	})
	if err != nil {
		t.Fatalf("Apply(%q): %v", body, err)
	}
	if err := texturedoc.Validate(res.Doc, res.Entities); err != nil {
		t.Fatalf("failure mode 7: marked document does not validate: %v", err)
	}
	return res, res.Doc.Doc.Content[len(res.Doc.Doc.Content)-1]
}

// Failure modes 1 and 7.
func TestWriteMarksBoldItalicAndCode(t *testing.T) {
	_, p := writeOne(t, "**Status.** The deal is *pending* under rule `14a-8`.")
	want := []run{
		{"Status.", "strong"},
		{" The deal is ", ""},
		{"pending", "emphasis"},
		{" under rule ", ""},
		{"14a-8", "code"},
		{".", ""},
	}
	if got := runsOf(p); !reflect.DeepEqual(got, want) {
		t.Fatalf("runs = %+v\nwant   %+v", got, want)
	}
	raw, _ := json.Marshal(p)
	if strings.Contains(string(raw), "*") || strings.Contains(string(raw), "`") {
		t.Fatalf("delimiters reached the text: %s", raw)
	}

	_, p = writeOne(t, "***Both*** and **bold with *italic* inside**")
	want = []run{
		{"Both", "strong,emphasis"},
		{" and ", ""},
		{"bold with ", "strong"},
		{"italic", "strong,emphasis"},
		{" inside", "strong"},
	}
	if got := runsOf(p); !reflect.DeepEqual(got, want) {
		t.Fatalf("nested runs = %+v\nwant          %+v", got, want)
	}
}

// Failure modes 2 and 3: a cite inside bold resolves against the plain words
// and the pair hashes the words without delimiters.
func TestCiteInsideBoldResolvesAndPlainTextDropsDelimiters(t *testing.T) {
	res, p := writeOne(t, "**The court ruled that the merger may proceed {{cite https://example.org/court \"the court ruled that the merger may proceed\"}}.** Then more.")
	want := []run{
		{"The court ruled that the merger may proceed ", "strong"},
		{"", "@ref"},
		{".", "strong"},
		{" Then more.", ""},
	}
	if got := runsOf(p); !reflect.DeepEqual(got, want) {
		t.Fatalf("runs = %+v\nwant   %+v", got, want)
	}
	refs := sourceRefs(p)
	if ref, ok := RefOf(refs[0]); !ok || ref.CID != "c1" {
		t.Fatalf("cite inside bold did not resolve: %+v", refs[0])
	}
	if len(refs[0].Marks) != 0 {
		t.Fatalf("citation atom carries marks: %+v", refs[0].Marks)
	}
	if got := CitingText(p); got != "The court ruled that the merger may proceed. Then more." {
		t.Fatalf("citing text = %q", got)
	}
	// A carried citation checks its quote against plain text: bolding the
	// quoted words in an @all rewrite keeps the citation.
	blocks := DocBlocks(res.Doc)
	script := "@all\n" + blocks[0].Text + "\n\n**The court ruled** that the merger *may proceed*.\n\n" + blocks[2].Text
	again, err := Apply(context.Background(), Input{OwnerID: "owner", DocID: "doc", Seed: "rev-em2", Base: res.Doc, Entities: res.Entities, Script: script, Resolver: newFakeCaptures()})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(sourceRefs(again.Doc.Doc.Content[1])) != 1 || len(again.Dropped) != 0 {
		t.Fatalf("bolding the quoted words dropped the citation: %+v dropped %+v", again.Doc.Doc.Content[1], again.Dropped)
	}
}

// Failure mode 4: the frame shows marks as markdown and a block written back
// unchanged keeps its node byte for byte.
func TestMarkedBlockRoundTripsThroughTheFrame(t *testing.T) {
	src := "**Who searched.** Agents of *HSI* searched {{cite https://example.org/court \"the court ruled that the merger may proceed\"}}, with `code` and ***both***."
	res, p := writeOne(t, src)
	blocks := DocBlocks(res.Doc)
	last := blocks[len(blocks)-1]
	if !strings.HasPrefix(last.Text, "**Who searched.** Agents of *HSI* searched {{cite cap-court ") || !strings.HasSuffix(last.Text, ", with `code` and ***both***.") {
		t.Fatalf("frame text = %q", last.Text)
	}
	before, _ := json.Marshal(p)
	again, err := Apply(context.Background(), Input{OwnerID: "owner", DocID: "doc", Seed: "rev-em3", Base: res.Doc, Entities: res.Entities,
		Script: "@replace " + last.Alias + "\n" + last.Text, Resolver: newFakeCaptures()})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	after, _ := json.Marshal(again.Doc.Doc.Content[len(again.Doc.Doc.Content)-1])
	if string(after) != string(before) {
		t.Fatalf("unchanged marked block changed:\nbefore %s\nafter  %s", before, after)
	}
}

// Failure modes 5 and 6.
func TestNonEmphasisAsterisksAndCodeSpansStayLiteral(t *testing.T) {
	cases := []struct {
		in   string
		want []run
	}{
		{"Five * three is 15, and a ** b too.", []run{{"Five * three is 15, and a ** b too.", ""}}},
		{"An **unclosed bold stays literal.", []run{{"An **unclosed bold stays literal.", ""}}},
		{"Use `a*b*c` and `{{cite x}}` literally.", []run{{"Use ", ""}, {"a*b*c", "code"}, {" and ", ""}, {"{{cite x}}", "code"}, {" literally.", ""}}},
		{"snake*case*word stays.", []run{{"snake*case*word stays.", ""}}},
	}
	for _, tc := range cases {
		_, p := writeOne(t, tc.in)
		if got := runsOf(p); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%q: runs = %+v\nwant %+v", tc.in, got, tc.want)
		}
	}
}
