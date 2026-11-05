package trust

// Failure modes (MA7.7), written first:
//  C1 a spike goes unnoticed because the baseline is the mean (one spike
//     inflates it): the baseline is robust (median and MAD).
//  C2 a desk with no history is flagged on its first dollar, or never: below
//     MinActive windows there is no baseline and no notice.
//  C3 noise on cents pages the owner: an absolute floor applies.
//  C4 one anomaly notifies every scan: the notice id is deterministic in the
//     desk and the window, so a repeat scan commits the same record.
//  C5 a ceiling: nothing here refuses or caps spend (R8); the anomaly only
//     notifies and names its class (I2).
//  C6 spend attributed to no desk is lost: an entry with no agent is grouped
//     under "unattributed", not dropped.

import (
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/llmcost"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func steady(desk string, hours int, usd float64, end time.Time) []CostPoint {
	var pts []CostPoint
	for h := 1; h <= hours; h++ {
		pts = append(pts, CostPoint{Desk: desk, USD: usd, At: end.Add(-time.Duration(h)*time.Hour - 30*time.Minute)})
	}
	return pts
}

func TestSpikeAgainstOwnHistoryIsAnAnomaly(t *testing.T) {
	now := t0.Add(100 * time.Hour)
	pts := steady("research", 48, 0.20, now)
	pts = append(pts, CostPoint{Desk: "research", USD: 4.0, At: now.Add(-10 * time.Minute)})
	an := DetectCostAnomalies(pts, now, DefaultAnomalyParams(), nil)
	if len(an) != 1 || an[0].Desk != "research" || an[0].USD < 4 || an[0].Multiple < 10 {
		t.Fatalf("%+v", an)
	}
	if an[0].Class != SpendAnomalyClass || an[0].Class.Impact != types.ImpactI2 {
		t.Fatalf("an anomaly is I2-material: %+v", an[0].Class)
	}
}

func TestSteadySpendThinHistoryAndCentsAreQuiet(t *testing.T) {
	now := t0.Add(100 * time.Hour)
	// steady
	pts := steady("research", 48, 0.20, now)
	pts = append(pts, CostPoint{Desk: "research", USD: 0.22, At: now.Add(-10 * time.Minute)})
	if an := DetectCostAnomalies(pts, now, DefaultAnomalyParams(), nil); len(an) != 0 {
		t.Fatalf("steady spend flagged: %+v", an)
	}
	// a new desk: no baseline
	thin := []CostPoint{{Desk: "newdesk", USD: 9, At: now.Add(-10 * time.Minute)}, {Desk: "newdesk", USD: 0.1, At: now.Add(-3 * time.Hour)}}
	if an := DetectCostAnomalies(thin, now, DefaultAnomalyParams(), nil); len(an) != 0 {
		t.Fatalf("no history, no anomaly: %+v", an)
	}
	// a hundredfold jump that is still cents
	cheap := steady("mail", 48, 0.0005, now)
	cheap = append(cheap, CostPoint{Desk: "mail", USD: 0.05, At: now.Add(-10 * time.Minute)})
	if an := DetectCostAnomalies(cheap, now, DefaultAnomalyParams(), nil); len(an) != 0 {
		t.Fatalf("cents flagged: %+v", an)
	}
}

func TestOneSpikeDoesNotRaiseTheBaseline(t *testing.T) {
	now := t0.Add(100 * time.Hour)
	pts := steady("research", 48, 0.20, now)
	// an earlier spike in history; a repeat of the same size now is still anomalous
	pts = append(pts, CostPoint{Desk: "research", USD: 5, At: now.Add(-20*time.Hour - 10*time.Minute)})
	pts = append(pts, CostPoint{Desk: "research", USD: 5, At: now.Add(-10 * time.Minute)})
	if an := DetectCostAnomalies(pts, now, DefaultAnomalyParams(), nil); len(an) != 1 {
		t.Fatalf("median baseline: %+v", an)
	}
}

func TestAttributionGroupsByDeskPackagePublisherAndKeepsUnattributed(t *testing.T) {
	res := func(d string) SubjectInfo { return SubjectInfo{Package: "pkg-" + d, Publisher: "pub"} }
	pts := []CostPoint{{Desk: "a", USD: 1, InputTokens: 10}, {Desk: "a", USD: 2, OutputTokens: 5}, {Desk: "b", USD: 4}, {USD: 0.5}}
	rep := AttributeCost(pts, res)
	by := func(rows []SpendRow, k string) SpendRow {
		for _, r := range rows {
			if r.Key == k {
				return r
			}
		}
		return SpendRow{}
	}
	if r := by(rep.ByDesk, "a"); r.USD != 3 || r.Calls != 2 || r.InputTokens != 10 || r.OutputTokens != 5 {
		t.Fatalf("a: %+v", r)
	}
	if r := by(rep.ByDesk, UnattributedDesk); r.USD != 0.5 {
		t.Fatalf("unattributed: %+v", rep.ByDesk)
	}
	if r := by(rep.ByPackage, "pkg-b"); r.USD != 4 {
		t.Fatalf("package: %+v", rep.ByPackage)
	}
	if r := by(rep.ByPublisher, "pub"); r.USD != 7 {
		t.Fatalf("publisher: %+v", rep.ByPublisher)
	}
	if rep.TotalUSD != 7.5 {
		t.Fatalf("total %v", rep.TotalUSD)
	}
}

func TestPointsFromLLMCostEntries(t *testing.T) {
	entries := []llmcost.CostEntry{
		{AgentID: "research:doc1", Model: "m", Timestamp: t0, Cost: llmcost.Cost{USD: 0.5, InputTokens: 100, OutputTokens: 10}},
		{AgentID: "", Model: "m", Timestamp: t0, Cost: llmcost.Cost{USD: 0.1}},
	}
	pts := PointsFromEntries(entries)
	if len(pts) != 2 || pts[0].Desk != "research" || pts[0].USD != 0.5 || pts[1].Desk != "" {
		t.Fatalf("%+v", pts)
	}
}

func TestAnomalyNoticeIsDeterministicAndANotice(t *testing.T) {
	now := t0.Add(100 * time.Hour)
	pts := append(steady("research", 48, 0.20, now), CostPoint{Desk: "research", USD: 4.0, At: now.Add(-10 * time.Minute)})
	an := DetectCostAnomalies(pts, now, DefaultAnomalyParams(), func(string) SubjectInfo { return SubjectInfo{Package: "p", Publisher: "q"} })[0]
	n1 := an.Notice("management:owner-1", "comp-1")
	n2 := an.Notice("management:owner-1", "comp-1")
	if n1.RecordID != n2.RecordID || !strings.HasPrefix(n1.RecordID, "cost-anomaly:research:") {
		t.Fatalf("%s %s", n1.RecordID, n2.RecordID)
	}
	if n1.EmitterKind != types.CommitmentEmitterSystem || n1.Directive == nil || !n1.Directive.Subtype.IsNotice() || n1.Inbox != "management:owner-1" {
		t.Fatalf("%+v", n1)
	}
	if !strings.Contains(n1.Directive.Body, "research") || !strings.Contains(n1.Directive.Body, "no spend ceiling") {
		t.Fatalf("body %q", n1.Directive.Body)
	}
	if n1.SubjectRefs["package"] != "p" || n1.SubjectRefs["publisher"] != "q" {
		t.Fatalf("refs %+v", n1.SubjectRefs)
	}
	// the same desk in the next window is a new notice
	later := now.Add(2 * time.Hour)
	pts2 := append(pts, CostPoint{Desk: "research", USD: 4.0, At: later.Add(-10 * time.Minute)})
	an2 := DetectCostAnomalies(pts2, later, DefaultAnomalyParams(), nil)
	if len(an2) != 1 || an2[0].Notice("management:owner-1", "comp-1").RecordID == n1.RecordID {
		t.Fatalf("%+v", an2)
	}
	if err := types.ValidateRequestRecord(n1); err != nil {
		t.Fatal(err)
	}
}
