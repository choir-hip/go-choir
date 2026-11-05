package disclosure

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// Ways the rules table could fail (written before the code):
//
//  1. A cell changes by accident. TestTableCellsPinned pins every cell.
//  2. The paper stops publishing: a run labelled only world_untrusted (or
//     platform, publisher) is asked or denied at a sink.
//  3. An owner-private label reaches publication or a release without an
//     ask (egress is allowed, owner 2026-11-05), or an unadmitted desk gets a free message.
//  4. A mixed label set decides by its most permissive label, not its
//     strictest.
//  5. System 1 decides alone: a distribution that is merely likely, or a
//     failed call, allows.
//  6. An unknown label atom is treated as harmless.

func TestTableCellsPinned(t *testing.T) {
	want := map[Class]map[Sink]Cell{
		ClassOwnerPrivate: {
			SinkPublication:    {Ask, Ask},
			SinkMailSend:       {Ask, Allow},
			SinkEgress:         {Allow, Allow},
			SinkDeskMessage:    {Ask, Allow},
			SinkLibraryRelease: {Ask, Ask},
		},
		ClassWorldUntrust: {SinkPublication: {Allow, Allow}, SinkMailSend: {Allow, Allow}, SinkEgress: {Allow, Allow}, SinkDeskMessage: {Allow, Allow}, SinkLibraryRelease: {Allow, Allow}},
		ClassPlatform:     {SinkPublication: {Allow, Allow}, SinkMailSend: {Allow, Allow}, SinkEgress: {Allow, Allow}, SinkDeskMessage: {Allow, Allow}, SinkLibraryRelease: {Allow, Allow}},
		ClassPublisher:    {SinkPublication: {Allow, Allow}, SinkMailSend: {Allow, Allow}, SinkEgress: {Allow, Allow}, SinkDeskMessage: {Allow, Allow}, SinkLibraryRelease: {Allow, Allow}},
	}
	if len(Table) != len(want) {
		t.Fatalf("table has %d classes, want %d", len(Table), len(want))
	}
	for _, c := range Classes {
		if len(Table[c]) != len(Sinks) {
			t.Errorf("class %s covers %d sinks, want %d", c, len(Table[c]), len(Sinks))
		}
		for _, s := range Sinks {
			got, ok := CellFor(c, s)
			if !ok || got != want[c][s] {
				t.Errorf("cell %s x %s = %+v (present %v), want %+v", c, s, got, ok, want[c][s])
			}
		}
	}
}

func TestEverySinkForEveryLabelClassGetsTheTablesDecision(t *testing.T) {
	cases := []struct {
		label    string
		sink     Sink
		admitted bool
		want     Decision
	}{
		{types.LabelWorldUntrusted, SinkPublication, false, Allow},
		{types.LabelWorldUntrusted, SinkEgress, false, Allow},
		{types.LabelPlatform, SinkLibraryRelease, false, Allow},
		{types.PublisherLabel("example-news"), SinkPublication, false, Allow},
		{types.OwnerPrivateLabel("document"), SinkPublication, false, Ask},
		{types.OwnerPrivateLabel("document"), SinkPublication, true, Ask},
		{types.OwnerPrivateLabel("document"), SinkEgress, false, Allow},
		{types.OwnerPrivateLabel("document"), SinkLibraryRelease, false, Ask},
		{types.OwnerPrivateLabel("document"), SinkDeskMessage, false, Ask},
		{types.OwnerPrivateLabel("document"), SinkDeskMessage, true, Allow},
		{types.OwnerPrivateLabel("mail"), SinkMailSend, false, Ask},
		{types.OwnerPrivateLabel("mail"), SinkMailSend, true, Allow},
	}
	for _, c := range cases {
		v := Decide(types.NewLabelSet(c.label), c.sink, Facts{AdmittedFor: func(string) bool { return c.admitted }})
		if v.Decision != c.want {
			t.Errorf("%s at %s (admitted %v) = %s, want %s", c.label, c.sink, c.admitted, v.Decision, c.want)
		}
	}
}

// 2: the paper keeps publishing.
func TestAWorldOnlyRunPublishesAndReachesEverySink(t *testing.T) {
	for _, labels := range []types.LabelSet{
		types.NewLabelSet(types.LabelWorldUntrusted),
		types.NewLabelSet(types.LabelWorldUntrusted, types.LabelPlatform, types.PublisherLabel("example-wire")),
		nil,
	} {
		for _, s := range Sinks {
			if v := Decide(labels, s, Facts{}); v.Decision != Allow {
				t.Errorf("labels %v at %s = %s, want allow", labels, s, v.Decision)
			}
		}
	}
}

// 4: the strictest label decides.
func TestAMixedSetDecidesByItsStrictestLabel(t *testing.T) {
	labels := types.NewLabelSet(types.LabelWorldUntrusted, types.OwnerPrivateLabel("capture"))
	v := Decide(labels, SinkPublication, Facts{})
	if v.Decision != Ask || v.Binding() != string(ClassOwnerPrivate) || len(v.Hits) != 2 {
		t.Fatalf("verdict = %+v", v)
	}
	// Admission is per label: one admitted label does not admit the other.
	two := types.NewLabelSet(types.OwnerPrivateLabel("document"), types.OwnerPrivateLabel("mail"))
	v = Decide(two, SinkDeskMessage, Facts{AdmittedFor: func(l string) bool { return l == types.OwnerPrivateLabel("document") }})
	if v.Decision != Ask {
		t.Fatalf("a desk admitted for one of two labels got %s", v.Decision)
	}
}

// Deny is supported by the lattice even though no default cell uses it.
func TestADenyCellBeatsAnAsk(t *testing.T) {
	old := Table[ClassWorldUntrust][SinkMailSend]
	Table[ClassWorldUntrust][SinkMailSend] = same(Deny)
	defer func() { Table[ClassWorldUntrust][SinkMailSend] = old }()
	v := Decide(types.NewLabelSet(types.LabelWorldUntrusted, types.OwnerPrivateLabel("mail")), SinkMailSend, Facts{})
	if v.Decision != Deny {
		t.Fatalf("decision = %s, want deny", v.Decision)
	}
}

func TestFromDistributionDecidesNotTheJudge(t *testing.T) {
	cases := []struct {
		dist map[string]float64
		want Outcome
	}{
		{map[string]float64{"yes": 0.9, "no": 0.05, "unsure": 0.05}, OutcomeAllow},
		{map[string]float64{"yes": 0.8, "no": 0.1, "unsure": 0.1}, OutcomeAllow},
		{map[string]float64{"yes": 0.79, "no": 0.01, "unsure": 0.2}, OutcomeEscalate},
		{map[string]float64{"yes": 0.85, "no": 0.11, "unsure": 0.04}, OutcomeEscalate},
		{map[string]float64{"yes": 0.2, "no": 0.7, "unsure": 0.1}, OutcomeEscalate},
		{map[string]float64{"no": 0.0}, OutcomeEscalate},
		{nil, OutcomeEscalate},
	}
	for _, c := range cases {
		if got := FromDistribution(c.dist); got != c.want {
			t.Errorf("%v = %s, want %s", c.dist, got, c.want)
		}
	}
}

func TestCountersKeyDecisionsBySinkClassAndOutcome(t *testing.T) {
	ResetCounts()
	defer ResetCounts()
	Note(Decide(types.NewLabelSet(types.LabelWorldUntrusted), SinkPublication, Facts{}), "allowed")
	own := Decide(types.NewLabelSet(types.OwnerPrivateLabel("document")), SinkPublication, Facts{})
	Note(own, "observed")
	Note(own, "observed")
	Note(own, "enforced")
	got := Counts()
	if got["publication|-|allow|allowed"] != 1 || got["publication|owner_private|ask|observed"] != 2 || got["publication|owner_private|ask|enforced"] != 1 {
		t.Fatalf("counts = %v", got)
	}
}

func TestModeDefaultsToObserveAndRestores(t *testing.T) {
	if CurrentMode() != ModeObserve || CurrentMode().String() != "observe" {
		t.Fatalf("default mode = %s", CurrentMode())
	}
	restore := SetMode(ModeEnforce)
	if CurrentMode().String() != "enforce" {
		t.Fatal("mode did not change")
	}
	restore()
	if CurrentMode() != ModeObserve {
		t.Fatal("mode did not restore")
	}
}
