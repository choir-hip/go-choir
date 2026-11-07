package trust

// Failure modes (MA7.10 wiring), written first:
//  W1 the anchored sample cannot be read back as scorer observations: a
//     stored item must feed ScorerProfiles and ErrorCorrelation unchanged.
//  W2 the served sample leaks content or the seed: the sample carries ids,
//     agent and scorer names, families, answers and the truth, never the
//     report text, the prompt or the seed.
//  W3 an unsettled item enters the sample as an anchor.

import (
	"encoding/json"
	"strings"
	"testing"
)

func sampleFixture() AnchorSample {
	var s AnchorSample
	for i := 0; i < 30; i++ {
		id := "cast-" + itoa(i) + "#q0"
		truth := []string{"yes", "no"}[i%2]
		wrong := map[string]string{"yes": "no", "no": "yes"}[truth]
		judge, agent := truth, truth
		if i%10 == 0 {
			judge = wrong
		}
		if i%3 == 0 {
			agent = wrong
		}
		s.Items = append(s.Items, AnchorItem{
			ItemID: id, Agent: "research:doc", QuestionType: "research_child", Choices: []string{"yes", "no"},
			Answers: []ScorerAnswer{
				{Scorer: "system:judge", Family: "alpha", Answer: judge, Probs: map[string]float64{judge: 0.9, map[string]string{"yes": "no", "no": "yes"}[judge]: 0.1}},
				{Scorer: "self:research:doc", Family: "beta", Answer: agent, Probs: map[string]float64{agent: 0.7, map[string]string{"yes": "no", "no": "yes"}[agent]: 0.3}},
			},
			Truth: truth, Kind: AnchorSpotCheck, Judges: []string{"deepseek"}, Agreement: 1, SettledAt: t0,
			Draw: 0.01, Rate: 0.1, SeedID: "abc123", SampledAt: t0,
		})
	}
	return s
}

func TestAnchorSampleFeedsProfilesAndCorrelation(t *testing.T) {
	s := sampleFixture()
	obs, anchors := s.Observations()
	if len(obs) != 60 || len(anchors) != 30 {
		t.Fatalf("%d observations, %d anchors", len(obs), len(anchors))
	}
	by := map[string]ScorerProfile{}
	for _, p := range ScorerProfiles(obs, anchors) {
		by[p.Scorer] = p
	}
	j, a := by["system:judge"].Type("research_child"), by["self:research:doc"].Type("research_child")
	if !j.Known || j.N != 30 || !(j.Accuracy > a.Accuracy) {
		t.Fatalf("judge %+v agent %+v", j, a)
	}
	if got := ErrorCorrelation(obs, anchors); len(got) != 1 {
		t.Fatalf("one scorer pair: %+v", got)
	}
}

func TestAnchorSampleCarriesNoContentAndNoSeed(t *testing.T) {
	b, _ := json.Marshal(sampleFixture())
	low := strings.ToLower(string(b))
	for _, bad := range []string{"seed\"", "prompt", "report_text", "\"text\""} {
		if strings.Contains(low, bad) {
			t.Fatalf("the served sample names %q", bad)
		}
	}
}

func TestAnchorSampleSkipsItemsWithoutATruth(t *testing.T) {
	s := sampleFixture()
	s.Items[0].Truth = ""
	obs, anchors := s.Observations()
	if len(anchors) != 29 || len(obs) != 58 {
		t.Fatalf("an item with no truth entered as an anchor: %d %d", len(obs), len(anchors))
	}
}
