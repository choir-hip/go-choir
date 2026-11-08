package disclosure

import (
	"fmt"
	"strings"
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
			SinkPublication:    {Ask, Allow},
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
		{types.OwnerPrivateLabel("document"), SinkPublication, true, Allow},
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

// Ways the sites tally could fail (written before the code): it records an
// allow; it records content (a URL with a query, an owner identifier, a body);
// it grows without bound; it changes a decision or the counts.

func TestSitesRecordOnlyAskAndDenyAsDeskKindAndLabelNames(t *testing.T) {
	ResetCounts()
	t.Cleanup(ResetCounts)
	labels := types.NewLabelSet(types.OwnerPrivateLabel("document"), types.LabelWorldUntrusted)
	ask := Decide(labels, SinkDeskMessage, Facts{})
	allow := Decide(types.NewLabelSet(types.LabelWorldUntrusted), SinkDeskMessage, Facts{})
	NoteSite(allow, Site{Sender: "texture", Recipient: "research", Kind: "cast"})
	if len(Sites()) != 0 {
		t.Fatalf("an allow was recorded: %v", Sites())
	}
	NoteSite(ask, Site{Sender: "texture", Recipient: "research", Kind: "cast"})
	NoteSite(ask, Site{Sender: "texture", Recipient: "research", Kind: "cast"})
	want := "desk_message|ask|texture->research|cast|owner_private:document"
	if got := Sites(); len(got) != 1 || got[want] != 2 {
		t.Fatalf("sites = %v, want %q x2", got, want)
	}
	// Non-desk sinks name the sink, never the target; a hostile field cannot
	// smuggle content or a separator.
	pub := Decide(labels, SinkPublication, Facts{})
	NoteSite(pub, Site{Sender: "texture", Recipient: "https://x.test/a?token=SECRET", Kind: "body text|with pipes"})
	for k := range Sites() {
		if strings.Contains(k, "SECRET") || strings.Contains(k, "x.test") || strings.Contains(k, " ") || strings.Count(k, "|") != 4 {
			t.Fatalf("a key carries content or a stray separator: %q", k)
		}
	}
	if Sites()["publication|ask|texture->publication|bodytextwithpipes|owner_private:document"] != 1 {
		t.Fatalf("sites = %v", Sites())
	}
	if c := Counts(); len(c) != 0 {
		t.Fatalf("NoteSite changed the counts: %v", c)
	}
}

func TestSitesAreBounded(t *testing.T) {
	ResetCounts()
	t.Cleanup(ResetCounts)
	v := Decide(types.NewLabelSet(types.OwnerPrivateLabel("document")), SinkDeskMessage, Facts{})
	for i := 0; i < 200; i++ {
		NoteSite(v, Site{Sender: "texture", Recipient: fmt.Sprintf("desk%d", i), Kind: "cast"})
	}
	got := Sites()
	if len(got) != maxSites+1 || got[sitesOverflowKey] != 200-maxSites {
		t.Fatalf("len=%d overflow=%d", len(got), got[sitesOverflowKey])
	}
}

// Owner ruling 2026-11-06: captures (and the evidence derived from them) move
// between the owner's Texture and research desks without an ask; nothing else
// about the table changes.
func TestOwnerFlowAllowsCapturesBetweenTextureAndResearchOnly(t *testing.T) {
	capture, evidence := types.OwnerPrivateLabel("capture"), types.OwnerPrivateLabel("evidence")
	both := types.NewLabelSet(capture, evidence, types.LabelWorldUntrusted)
	v := Decide(both, SinkDeskMessage, Facts{OwnerDesks: true})
	if v.Decision != Allow || v.Binding() != "-" || len(v.Hits) != 3 {
		t.Errorf("verdict %+v, want allow with every label recorded", v)
	}
	for _, c := range []struct {
		name       string
		labels     types.LabelSet
		sink       Sink
		ownerDesks bool
	}{
		{"another desk pair", both, SinkDeskMessage, false},
		{"other label", types.NewLabelSet(types.OwnerPrivateLabel("mail")), SinkDeskMessage, true},
		{"mixed with a mail label", types.NewLabelSet(capture, types.OwnerPrivateLabel("mail")), SinkDeskMessage, true},
		{"publication", both, SinkPublication, true},
		{"mail send", both, SinkMailSend, true},
		{"library release", both, SinkLibraryRelease, true},
	} {
		if v := Decide(c.labels, c.sink, Facts{OwnerDesks: c.ownerDesks}); v.Decision != Ask {
			t.Errorf("%s = %s, want ask", c.name, v.Decision)
		}
	}
	// Egress was already allowed and stays so.
	if v := Decide(both, SinkEgress, Facts{}); v.Decision != Allow {
		t.Errorf("egress = %s", v.Decision)
	}
}
