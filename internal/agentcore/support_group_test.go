package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/quotematch"
	"github.com/yusefmosiah/go-choir/internal/texturewrite"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Side-by-side cites are judged together (docs/problems/unsupported-cites-
// duplicates-and-churn-2026-11-10.md, fix A). Ways it can fail, each tested:
//   - GJ1 a group of 2, 3 or 4 cites costs one question per cite, not one;
//   - GJ2 a group where one cite carries the claim fails because the others
//     are irrelevant;
//   - GJ3 a group where no cite supports the claim passes;
//   - GJ4 a group's verdict is not recorded on every member, or the record
//     does not say it was a group's;
//   - GJ5 a group with an unreadable member is judged on partial evidence;
//   - GJ6 a failed group sends the writer one failure per cite, not one for
//     the span;
//   - GJ7 a lone pair beside a group is lost or judged as a group;
//   - GJ8 the evidence of a heavily cited span grows without bound.

// groupJudge answers yes to a question when the evidence it was shown
// (cited_passage or every cited_passages entry) contains "may proceed", and
// records the questions it was asked.
type groupJudge struct {
	mu        sync.Mutex
	calls     int
	questions int
	states    []string
}

func (g *groupJudge) Judge(_ context.Context, state json.RawMessage, questions map[string]json.RawMessage) (json.RawMessage, error) {
	g.mu.Lock()
	g.calls++
	g.questions += len(questions)
	g.states = append(g.states, string(state))
	g.mu.Unlock()
	var parsed struct {
		Pairs map[string]struct {
			Passage  string `json:"cited_passage"`
			Passages []struct {
				Passage string `json:"cited_passage"`
			} `json:"cited_passages"`
		} `json:"pairs"`
	}
	_ = json.Unmarshal(state, &parsed)
	answers := map[string]any{}
	for name := range questions {
		p := parsed.Pairs[name]
		evidence := p.Passage
		for _, x := range p.Passages {
			evidence += " " + x.Passage
		}
		choice, yes := "no", 0.1
		if strings.Contains(evidence, "may proceed") {
			choice, yes = "yes", 0.9
		}
		answers[name] = map[string]any{"type": "choice", "choice": choice, "probabilities": map[string]float64{"yes": yes, "no": 1 - yes}, "confidence": 0.9}
	}
	return json.Marshal(map[string]any{"model": "typesafe/jev-1.13", "answers": answers})
}

// groupCaptures has one capture per outlet: "good" says the claim, the rest
// do not.
func groupCaptures(outlets ...string) supportCaptures {
	caps := supportCaptures{}
	for _, o := range outlets {
		text := "Outlet " + o + " covered the harbour dredging schedule."
		if o == "good" || strings.HasPrefix(o, "good") {
			text = "Outlet " + o + " reports the merger may proceed."
		}
		caps["cap-"+o] = texturewrite.Capture{Doc: "cap-" + o, Version: "v1", URL: "https://example.org/" + o, Title: "Outlet " + o,
			Blocks: []quotematch.Block{{ID: "b1", Text: text}}}
	}
	return caps
}

func groupPair(span string, outlets ...string) []types.SupportVerdict {
	var group []types.Ref
	caps := groupCaptures(outlets...)
	for _, o := range outlets {
		text := caps["cap-"+o].Blocks[0].Text
		group = append(group, types.Ref{Doc: "cap-" + o, Version: "v1", CID: "b1", Hash: texturewrite.TextHash(text), Quote: text[:20]})
	}
	var out []types.SupportVerdict
	for _, ref := range group {
		pair := types.CitationPair{DocID: "doc", RevisionID: "rev", BlockID: "p1", BlockHash: texturewrite.TextHash(span), Text: span, Cited: ref, Matcher: "qm1"}
		if len(group) > 1 {
			pair.Group = group
			pair.BlockHash = texturewrite.GroupHash(span, group)
		}
		pair.Key = texturewrite.PairKey(pair.BlockHash, ref.Hash, ref.Quote)
		out = append(out, types.SupportVerdict{Key: pair.Key, Pair: &pair})
	}
	return out
}

func TestGroupedCitesAreOneQuestionAndTakeTheGroupsVerdict(t *testing.T) {
	span := "The merger may proceed."
	cases := []struct {
		name    string
		outlets []string
		want    string
	}{
		{"two cites, one carries the claim (GJ2)", []string{"good", "other"}, types.SupportSupported},
		{"three cites, the claim is in the last", []string{"a", "b", "good"}, types.SupportSupported},
		{"four cites, two carry it", []string{"good1", "a", "good2", "b"}, types.SupportSupported},
		{"two cites, none supports it (GJ3)", []string{"a", "b"}, types.SupportUnsupported},
		{"four cites, none supports it", []string{"a", "b", "c", "d"}, types.SupportUnsupported},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			jev := &groupJudge{}
			rt, _ := supportRuntime(t, jev)
			rt.SetTextureCaptures(groupCaptures(tc.outlets...))
			judged, err := rt.askJevForSupport(context.Background(), "owner", "", groupPair(span, tc.outlets...))
			if err != nil {
				t.Fatal(err)
			}
			if jev.calls != 1 || jev.questions != 1 {
				t.Fatalf("GJ1: calls = %d, questions = %d; want one call with one question for the whole group", jev.calls, jev.questions)
			}
			if len(judged) != len(tc.outlets) {
				t.Fatalf("GJ4: judged %d of %d members", len(judged), len(tc.outlets))
			}
			for _, v := range judged {
				if v.Verdict != tc.want || v.GroupSize != len(tc.outlets) {
					t.Errorf("GJ4: verdict = %q group size %d; want %q as a group of %d", v.Verdict, v.GroupSize, tc.want, len(tc.outlets))
				}
				if tc.want == types.SupportUnsupported && !strings.Contains(v.Why, "together") {
					t.Errorf("why = %q; want it to say the passages were judged together", v.Why)
				}
			}
			// Every source rides as evidence in the one question.
			for _, o := range tc.outlets {
				if !strings.Contains(jev.states[0], "Outlet "+o) {
					t.Errorf("the question's evidence lacks outlet %s", o)
				}
			}
		})
	}
}

func TestLonePairBesideAGroupIsStillItsOwnQuestion(t *testing.T) {
	jev := &groupJudge{}
	rt, _ := supportRuntime(t, jev)
	rt.SetTextureCaptures(groupCaptures("good", "other", "alone"))
	pending := append(groupPair("The merger may proceed.", "good", "other"), groupPair("Dredging starts in May.", "alone")...)
	judged, err := rt.askJevForSupport(context.Background(), "owner", "", pending)
	if err != nil {
		t.Fatal(err)
	}
	if jev.calls != 1 || jev.questions != 2 || len(judged) != 3 {
		t.Fatalf("GJ7: calls = %d, questions = %d, judged = %d; want one call, two questions, three verdicts", jev.calls, jev.questions, len(judged))
	}
	for _, v := range judged {
		lone := v.Pair.Text == "Dredging starts in May."
		if lone && (v.GroupSize != 0 || v.Verdict != types.SupportUnsupported) {
			t.Errorf("GJ7: the lone pair = %+v; want judged alone, unsupported", v)
		}
		if !lone && (v.GroupSize != 2 || v.Verdict != types.SupportSupported) {
			t.Errorf("GJ7: a group member = %+v", v)
		}
	}
}

func TestAGroupWithAnUnreadableMemberStaysPending(t *testing.T) {
	jev := &groupJudge{}
	rt, _ := supportRuntime(t, jev)
	rt.SetTextureCaptures(groupCaptures("good")) // "gone" has no capture
	pending := groupPair("The merger may proceed.", "good", "gone")
	judged, err := rt.askJevForSupport(context.Background(), "owner", "", pending)
	if err != nil {
		t.Fatal(err)
	}
	if len(judged) != 0 || jev.calls != 0 {
		t.Fatalf("GJ5: judged = %d, calls = %d; a group that cannot be read whole is not judged on part of its evidence", len(judged), jev.calls)
	}
}

func TestAGroupsEvidenceIsBounded(t *testing.T) {
	jev := &groupJudge{}
	rt, _ := supportRuntime(t, jev)
	outlets := []string{"a", "b", "c", "d", "e", "f"}
	caps := groupCaptures(outlets...)
	pending := groupPair("The merger may proceed.", outlets...)
	for i, o := range outlets {
		long := "Outlet " + o + " opens. " + strings.Repeat("filler words about the harbour. ", 3000) + "The merger may proceed here. " + strings.Repeat("more filler. ", 3000)
		c := caps["cap-"+o]
		c.Blocks = []quotematch.Block{{ID: "b1", Text: long}}
		caps["cap-"+o] = c
		pending[i].Pair.Cited.Hash = texturewrite.TextHash(long)
		pending[i].Pair.Cited.Quote = "The merger may proceed here"
		for j := range pending {
			pending[j].Pair.Group[i] = pending[i].Pair.Cited
		}
	}
	rt.SetTextureCaptures(caps)
	if _, err := rt.askJevForSupport(context.Background(), "owner", "", pending); err != nil {
		t.Fatal(err)
	}
	if got := len([]rune(jev.states[0])); got > supportGroupEvidenceRunes+4000 {
		t.Fatalf("GJ8: the question state is %d runes; want evidence bounded near %d", got, supportGroupEvidenceRunes)
	}
	if !strings.Contains(jev.states[0], "The merger may proceed here") {
		t.Fatal("GJ8: bounding cut the cited words out of the evidence")
	}
}

func TestBoundPassageKeepsTheQuote(t *testing.T) {
	passage := strings.Repeat("x", 500) + " the cited words " + strings.Repeat("y", 500)
	got := boundPassage(passage, "the cited words", 100)
	if len([]rune(got)) != 100 || !strings.Contains(got, "the cited words") {
		t.Fatalf("bounded = %q", got)
	}
	if boundPassage("short", "short", 100) != "short" {
		t.Fatal("a short passage changed")
	}
}

func TestAFailedGroupSendsTheWriterOneFailureNamingTheSpan(t *testing.T) {
	jev := &fakeSupportJudge{choice: "no"}
	rt, s := supportRuntime(t, jev)
	ctx := context.Background()
	owner, computer, docID := "owner-group-fail", rt.cfg.ComputerID, "doc-group-fail"
	seeded := seedSupportPairs(t, s, owner, computer, docID, true, "cid-1", "cid-2")
	if len(seeded.Pairs) != 2 || len(seeded.Pairs[0].Group) != 2 {
		t.Fatalf("seed = %+v", seeded.Pairs)
	}
	rt.JudgeRevisionSupport(owner, computer, seeded.RunID, seeded.RevisionID, seeded.Pairs)
	rt.WaitSupportJudgements()
	if jev.callCount() != 1 {
		t.Fatalf("Jev calls = %d", jev.callCount())
	}
	got, err := s.ListInboxRecords(ctx, owner, computer, agentprofile.Texture+":"+docID, 0, 10)
	if err != nil || len(got) != 1 || got[0].Support == nil {
		t.Fatalf("texture inbox = %+v, %v", got, err)
	}
	if n := len(got[0].Support.Failures); n != 1 {
		t.Fatalf("GJ6: %d failures; want one for the span", n)
	}
	for _, p := range seeded.Pairs {
		v, found, _ := s.SupportVerdict(ctx, owner, computer, p.Key)
		if !found || v.Verdict != types.SupportUnsupported || v.GroupSize != 2 {
			t.Errorf("GJ4: verdict for %s = %+v, %v; want unsupported as a group of 2", p.Key, v, found)
		}
	}
	frame := texturewrite.Frame{DocID: docID, Records: texturewrite.RecordsFrom([]texturewrite.RecordInput{{Seq: 1, RecordID: "r", Kind: "report", From: types.SupportSystemAgentID,
		Support: got[0].Support}})}.Render()
	if !strings.Contains(frame, "cites together") || !strings.Contains(frame, "the merger may proceed") || !strings.Contains(frame, "regulator appealed") {
		t.Fatalf("the send-back does not name the span and the group's quotes:\n%s", frame)
	}
	_ = fmt.Sprint
}
