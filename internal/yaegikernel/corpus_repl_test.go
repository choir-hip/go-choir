package yaegikernel

// The corpus as values in a cell (corpus_repl.go). Failure list, written first:
//
//   Y1 a verb returns JSON bytes or prints rows: a cell cannot range over
//      p.Captures, index r.Texts, or read a score as a number;
//   Y2 printing a value dumps its rows into the prompt (String() must be one line);
//   Y3 a payload field is lost or renamed on the way to the host (since, kind,
//      cursor, limit, the ids list, of, k, ref);
//   Y4 the research desk and its web child cannot call the verbs, or a desk
//      that does not hold corpus.read can.

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
)

func corpusCell(t *testing.T, role, source string) (SessionResult, map[BrokerAction][]json.RawMessage) {
	t.Helper()
	var mu sync.Mutex
	seen := map[BrokerAction][]json.RawMessage{}
	issuer, err := NewHandleIssuer(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	broker, err := NewBroker(BrokerConfig{ComputerID: "c", CurrentEpoch: 1, AllowedRoot: t.TempDir(),
		Egress: func(ctx context.Context, action BrokerAction, payload json.RawMessage) (json.RawMessage, error) {
			mu.Lock()
			seen[action] = append(seen[action], payload)
			mu.Unlock()
			switch action {
			case ActionCorpusCaptures:
				return json.RawMessage(`{"captures":[{"capture_id":"obj:choir.web_capture:o:a","title":"SECRET TITLE A","url":"https://x/a","kind":"source","has_text":true},{"capture_id":"obj:choir.web_capture:o:b","title":"B","url":"https://x/b","kind":"read","has_text":true}],"next":"123|obj:choir.web_capture:o:b"}`), nil
			case ActionCorpusTexts:
				return json.RawMessage(`{"texts":[{"capture_id":"obj:choir.web_capture:o:a","title":"A","url":"https://x/a","blocks":[{"id":"b1","text":"first"},{"id":"b2","text":"second"}],"chars":11},{"capture_id":"obj:choir.web_capture:o:b","error":"owner-private: read it with ReadCapture","chars":0}],"unreached":["z"]}`), nil
			case ActionCorpusNear:
				return json.RawMessage(`{"captures":[{"capture_id":"obj:choir.web_capture:o:a","title":"A","url":"u","kind":"source","has_text":true,"score":0.91}],"stories":[{"publication_id":"pub-0123456789abcdef0123456789abcdef","version_id":"v","revision":2,"title":"S","score":0.8}],"state":"exact"}`), nil
			case ActionCorpusEdges:
				return json.RawMessage(`{"ref":"r","kind":"story","sources":[{"capture_id":"s1","url":"u","kind":"source","has_text":true}],"related":[{"publication_id":"p2","relation":"same_subject"}]}`), nil
			}
			return json.RawMessage(`{}`), nil
		}}, issuer)
	if err != nil {
		t.Fatal(err)
	}
	scope, err := NewChoirScope(broker, issuer, "c", "act", 1, role, "")
	if err != nil {
		t.Fatal(err)
	}
	sess, err := NewSession(NewAllowlist(DefaultSafeStdlibPackagesList()...), scope.ChoirExports())
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()
	hooks := scope.BindCell()
	out, err := serveCell(sess, SessionFrame{ID: "cell", Source: source}, nil, &hooks)
	if err != nil {
		t.Fatalf("serve cell: %v", err)
	}
	return out, seen
}

const corpusCellSource = `p, err := choir.CorpusCaptures("2026-11-20", "", "feed:a", "source", "", 3000)
if err != nil { panic(err) }
n := 0
for _, c := range p.Captures { if c.Kind == "source" && c.HasText { n++ } }
r, err := choir.CorpusTexts(p.IDs(), 5000)
if err != nil { panic(err) }
text := ""
for _, t := range r.Texts[:1] { text = t.Text() }
near, err := choir.CorpusNear("harbor strike", 300, "2026-11-17", "")
if err != nil { panic(err) }
e, err := choir.CorpusEdges(near.Stories[0].PublicationID)
if err != nil { panic(err) }
fmt.Println(p)
fmt.Println(r)
fmt.Println(near)
fmt.Println(e)
fmt.Println("n", n, "text", text[:5], "score", near.Captures[0].Score*100, "rev", near.Stories[0].Revision, "src", e.SourceIDs(), "rel", e.Related[0].Relation, "unreached", r.Unreached, "err", len(r.ByID()) == 2)`

func TestCorpusVerbsReturnValuesAndPrintOneLine(t *testing.T) {
	out, seen := corpusCell(t, SessionRoleResearch, corpusCellSource)
	if out.Error != "" {
		t.Fatalf("cell: %s", out.Error) // Y1
	}
	if strings.Contains(out.Stdout, "SECRET TITLE") || strings.Contains(out.Stdout, "second") { // Y2
		t.Fatalf("a value printed its rows: %s", out.Stdout)
	}
	for _, want := range []string{"corpus page: 2 captures", "corpus texts: 2 captures (1 not served), 1 unreached", "corpus near: 1 captures, 1 stories, state exact", "n 1 text first", "score 91", "rev 2", "same_subject", "err true"} {
		if !strings.Contains(out.Stdout, want) {
			t.Fatalf("stdout lacks %q:\n%s", want, out.Stdout)
		}
	}
	var page CorpusCapturesPayload // Y3
	if err := json.Unmarshal(seen[ActionCorpusCaptures][0], &page); err != nil || page.Since != "2026-11-20" || page.SourceID != "feed:a" || page.Kind != "source" || page.Limit != 3000 {
		t.Fatalf("page payload %+v %v", page, err)
	}
	var texts CorpusTextsPayload
	_ = json.Unmarshal(seen[ActionCorpusTexts][0], &texts)
	if len(texts.IDs) != 2 || texts.MaxChars != 5000 {
		t.Fatalf("texts payload %+v", texts)
	}
	var near CorpusNearPayload
	_ = json.Unmarshal(seen[ActionCorpusNear][0], &near)
	if near.Of != "harbor strike" || near.K != 300 || near.Since != "2026-11-17" {
		t.Fatalf("near payload %+v", near)
	}
	var edges CorpusEdgesPayload
	_ = json.Unmarshal(seen[ActionCorpusEdges][0], &edges)
	if edges.Ref != "pub-0123456789abcdef0123456789abcdef" {
		t.Fatalf("edges payload %+v", edges)
	}
}

func TestCorpusVerbsBelongToTheDesksThatHoldTheGrant(t *testing.T) { // Y4
	for _, desk := range []string{deskmanifest.ResearcherName()} {
		d, ok := deskmanifest.Lookup(desk)
		if !ok || !d.HasGrant("corpus.read") {
			t.Fatalf("%s does not hold corpus.read", desk)
		}
		for _, a := range []string{"corpus_captures", "corpus_texts", "corpus_near", "corpus_edges"} {
			if !d.AllowsFrame(a) {
				t.Fatalf("%s does not allow %s", desk, a)
			}
		}
	}
	for _, desk := range []string{deskmanifest.DocumentCustodianName(), deskmanifest.ResearchWeb, deskmanifest.ResearchTexture, deskmanifest.TaskRoleName()} {
		d, ok := deskmanifest.Lookup(desk)
		if ok && (d.HasGrant("corpus.read") || d.AllowsFrame("corpus_texts")) {
			t.Fatalf("%s holds the corpus verbs", desk)
		}
	}
}

// The worked cell of the research overlay compiles and runs against the verbs.
func TestResearchOverlayCorpusCellRuns(t *testing.T) {
	raw, err := os.ReadFile("../runtimeprompts/overlays/rlm_research_runtime.yaml")
	if err != nil {
		t.Skip(err)
	}
	s := string(raw)
	at := strings.Index(s, "near, _ := choir.CorpusNear")
	end := strings.Index(s[at:], "```")
	var lines []string
	for _, l := range strings.Split(s[at:at+end], "\n") {
		lines = append(lines, strings.TrimPrefix(l, "    "))
	}
	out, _ := corpusCell(t, SessionRoleResearch, strings.Join(lines, "\n"))
	if out.Error != "" {
		t.Fatalf("the overlay's cell fails: %s", out.Error)
	}
	if !strings.Contains(out.Stdout, "corpus texts") {
		t.Fatalf("stdout %s", out.Stdout)
	}
}

// A web child's cell, steered by a page it read, cannot enumerate the corpus:
// none of the four verbs exists for it, and its worker is refused the actions.
func TestResearchWebChildCannotCallAnyCorpusVerb(t *testing.T) {
	for _, call := range []string{
		`_, err := choir.CorpusCaptures("", "", "", "", "", 5000); fmt.Println(err)`,
		`_, err := choir.CorpusTexts([]string{"a"}, 0); fmt.Println(err)`,
		`_, err := choir.CorpusNear("x", 5, "", ""); fmt.Println(err)`,
		`_, err := choir.CorpusEdges("x"); fmt.Println(err)`,
	} {
		out, seen := corpusCell(t, SessionRoleResearchWeb, call)
		if out.Error == "" {
			t.Fatalf("the web child's cell ran %q: %s", call, out.Stdout)
		}
		if len(seen) != 0 {
			t.Fatalf("the host was asked: %v", seen)
		}
	}
	for _, a := range []string{"corpus_captures", "corpus_texts", "corpus_near", "corpus_edges"} {
		if deskmanifest.FrameAllowed(deskmanifest.ResearchWeb, a) {
			t.Fatalf("the web child's frame table allows %s", a)
		}
	}
}
