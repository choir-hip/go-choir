package entitytext

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"
)

// The extractor's candidate recall on slice e's labelled set
// (docs/evidence/related-entities-labelled-set-2026-10-26.json, labelled
// before any extractor run). Cold start: no gazetteer, no library IDF, so
// this is the floor the learning loop starts from. A label is found when a
// proposal's key, or one of its merged short forms' keys, equals the key of
// one of the label's forms. The doc names candidate recall below 0.8 as the
// trigger for the NER escalation.

type labelledSet struct {
	Stories []struct {
		PublicationID string `json:"publication_id"`
		Title         string `json:"title"`
		Labels        []struct {
			Name  string   `json:"name"`
			Type  string   `json:"type"`
			Role  string   `json:"role"`
			Forms []string `json:"forms"`
		} `json:"labels"`
	} `json:"stories"`
}

type labelledTexts struct {
	Stories []struct {
		PublicationID string `json:"publication_id"`
		Title         string `json:"title"`
		Content       string `json:"content"`
	} `json:"stories"`
}

func loadJSON(t *testing.T, path string, v any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

type recall struct{ found, total int }

func (r recall) rate() float64 {
	if r.total == 0 {
		return 0
	}
	return float64(r.found) / float64(r.total)
}

func TestCandidateRecallOnTheLabelledSet(t *testing.T) {
	var set labelledSet
	var texts labelledTexts
	loadJSON(t, "../../docs/evidence/related-entities-labelled-set-2026-10-26.json", &set)
	loadJSON(t, "../../docs/evidence/related-entities-labelled-set-stories-2026-10-26.json", &texts)
	if len(set.Stories) != 30 || len(texts.Stories) != 30 {
		t.Fatalf("labelled set has %d stories and %d texts, want 30", len(set.Stories), len(texts.Stories))
	}
	var all, central, top, topCentral recall
	byType := map[string]*recall{}
	var missed []string
	proposals := 0
	for i, s := range set.Stories {
		tx := texts.Stories[i]
		if tx.PublicationID != s.PublicationID {
			t.Fatalf("story %d: text %s, labels %s", i, tx.PublicationID, s.PublicationID)
		}
		// The body without its title heading, as the host sends it.
		body := tx.Content
		if strings.HasPrefix(body, "# ") {
			if nl := strings.Index(body, "\n"); nl >= 0 {
				body = body[nl+1:]
			}
		}
		ps := Propose(Parse(tx.Title, body), nil)
		proposals += len(ps)
		keys, topKeys := map[string]bool{}, map[string]bool{}
		for rank, p := range ps {
			ks := []string{p.Key}
			for _, a := range p.Aliases {
				ks = append(ks, Key(a))
			}
			for _, k := range ks {
				keys[k] = true
				if rank < 24 {
					topKeys[k] = true
				}
			}
		}
		for _, l := range s.Labels {
			found, foundTop := false, false
			for _, f := range l.Forms {
				k := Key(f)
				found = found || keys[k]
				foundTop = foundTop || topKeys[k]
			}
			if byType[l.Type] == nil {
				byType[l.Type] = &recall{}
			}
			for _, r := range []*recall{&all, byType[l.Type]} {
				r.total++
				if found {
					r.found++
				}
			}
			top.total++
			if foundTop {
				top.found++
			}
			if l.Role == "central" {
				central.total++
				topCentral.total++
				if found {
					central.found++
				}
				if foundTop {
					topCentral.found++
				}
			}
			if !found {
				missed = append(missed, s.PublicationID[:12]+" "+l.Role+" "+l.Name)
			}
		}
	}
	sort.Strings(missed)
	t.Logf("candidate recall: all %d/%d = %.3f; central %d/%d = %.3f", all.found, all.total, all.rate(), central.found, central.total, central.rate())
	t.Logf("within the top 24: all %.3f, central %.3f; proposals %d over 30 stories", top.rate(), topCentral.rate(), proposals)
	types := make([]string, 0, len(byType))
	for k := range byType {
		types = append(types, k)
	}
	sort.Strings(types)
	for _, k := range types {
		t.Logf("  %-13s %d/%d = %.3f", k, byType[k].found, byType[k].total, byType[k].rate())
	}
	for _, m := range missed {
		t.Logf("  missed: %s", m)
	}
}
