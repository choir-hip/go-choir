package vocabmigrate

// Contracts: every V1 desk spelling forward-maps exactly once; every V2 name
// inverts to its canonical V1 representative; every row class round-trips
// V1→V2→V1; provenance restores exact spellings where many-to-one collapse
// would lose them; frozen protocol is skipped, never mapped; unknowns are
// reported, never silently passed or elevated.

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestForwardMapCoversAllDesksOnce(t *testing.T) {
	cases := map[string]string{
		"super":    "management",
		"co-super": "engineering", "cosuper": "engineering",
		"coagent": "engineering", "co-agent": "engineering",
		"co_super": "engineering", "cosuper_coding": "engineering",
		"co-super-coding": "engineering", "engineering": "engineering",
		"researcher": "research", "researchers": "research",
		"research": "research", "research-agent": "research",
		"web-research": "research", "web-researcher": "research",
		"texture": "texture", "conductor": "conductor",
		"processor": "processor", "reconciler": "reconciler",
		"email": "email", "verifier": "verifier",
	}
	seen := map[string]int{}
	for v1, want := range cases {
		got, ok := ForwardV1ToV2(v1)
		if !ok || got != want {
			t.Errorf("ForwardV1ToV2(%q) = (%q, %v), want (%q, true)", v1, got, ok, want)
		}
		seen[got]++
		// Case and whitespace tolerance matches live canonicalizers.
		got2, ok2 := ForwardV1ToV2("  " + v1 + " ")
		if !ok2 || got2 != want {
			t.Errorf("ForwardV1ToV2(padded %q) = (%q, %v)", v1, got2, ok2)
		}
	}
	if seen["management"] != 1 || seen["engineering"] != 8 || seen["research"] != 6 {
		t.Errorf("forward fan-in changed: %v (map edit needs mapping-doc update)", seen)
	}
}

func TestForwardRefusesUnknownAndSkipsProtocol(t *testing.T) {
	for _, unknown := range []string{"", "root", "admin", "management", "ownerition"} {
		if v2, ok := ForwardV1ToV2(unknown); ok || v2 != "" {
			t.Errorf("ForwardV1ToV2(%q) = (%q, %v), want (\"\", false)", unknown, v2, ok)
		}
	}
	for _, proto := range []string{"owner", "trusted-core", " Owner "} {
		if !IsFrozenProtocol(proto) {
			t.Errorf("IsFrozenProtocol(%q) = false, want true", proto)
		}
		if _, ok := ForwardV1ToV2(proto); ok {
			t.Errorf("ForwardV1ToV2(%q) mapped frozen protocol", proto)
		}
	}
}

func TestInverseCanonicalRepresentatives(t *testing.T) {
	cases := map[string]string{
		"management": "super", "engineering": "co-super", "research": "researcher",
		"texture": "texture", "conductor": "conductor",
	}
	for v2, want := range cases {
		got, ok := InverseV2ToV1Canonical(v2)
		if !ok || got != want {
			t.Errorf("InverseV2ToV1Canonical(%q) = (%q, %v), want (%q, true)", v2, got, ok, want)
		}
	}
	if _, ok := InverseV2ToV1Canonical("vapor"); ok {
		t.Error("InverseV2ToV1Canonical(vapor) accepted")
	}
	if _, err := RevertRowV1("bogus"); err == nil {
		t.Error("RevertRowV1(bogus) accepted")
	}
}

func TestRowRoundTrips(t *testing.T) {
	log := &ProvenanceLog{}
	run := &types.RunRecord{AgentProfile: "co-super", AgentRole: "researcher"}
	out := MigrateRunRecord(run, log)
	if !out.Changed || len(out.Unknowns) != 0 || len(out.Skipped) != 0 {
		t.Fatalf("run outcome = %+v", out)
	}
	if run.AgentProfile != "engineering" || run.AgentRole != "research" {
		t.Fatalf("run not migrated: %+v", run)
	}
	back, ok := log.InverseExact("agent_profile", run.AgentProfile)
	if !ok || back != "co-super" {
		t.Fatalf("provenance inverse = (%q, %v), want (co-super, true)", back, ok)
	}
	// Canonical spellings need no provenance entry: canonical fallback.
	back, ok = log.InverseExact("agent_role", run.AgentRole)
	if !ok || back != "researcher" {
		t.Fatalf("canonical fallback = (%q, %v), want (researcher, true)", back, ok)
	}

	agent := &types.AgentRecord{Profile: "super", Role: "coagent"}
	if out := MigrateAgentRecord(agent, log); !out.Changed {
		t.Fatalf("agent outcome = %+v", out)
	}
	if agent.Profile != "management" || agent.Role != "engineering" {
		t.Fatalf("agent not migrated: %+v", agent)
	}
	if back, _ := log.InverseExact("role", agent.Role); back != "coagent" {
		t.Fatalf("many-to-one exact restore lost: %q", back)
	}

	work := &types.WorkItemRecord{AuthorityProfile: "researchers"}
	if out := MigrateWorkItemRecord(work, log); !out.Changed || work.AuthorityProfile != "research" {
		t.Fatalf("work outcome = %+v", out)
	}

	msg := &types.ChannelMessage{Role: "texture"}
	if out := MigrateChannelMessage(msg, log); out.Changed || len(out.Unknowns) != 0 {
		t.Fatalf("stays-live message outcome = %+v", out)
	}

	grant := &types.CoSuperGrantPolicyAttestation{Role: "co-super"}
	if out := MigrateGrantAttestation(grant, log); !out.Changed || grant.Role != "engineering" {
		t.Fatalf("grant outcome = %+v", out)
	}
}

func TestAgentIDPrefixMigration(t *testing.T) {
	log := &ProvenanceLog{}
	cases := map[string]string{
		"super:owner":            "management:owner",
		"co-super:assignment-01": "engineering:assignment-01",
		"researcher:doc-9":       "research:doc-9",
		"texture:doc-9":          "texture:doc-9",
		"work-super-assignment":  "work-super-assignment",
		"run:abc":                "run:abc",
	}
	for in, want := range cases {
		got, out := MigrateAgentID(in, log)
		if got != want || len(out.Unknowns) != 0 {
			t.Errorf("MigrateAgentID(%q) = (%q, %+v), want %q", in, got, out, want)
		}
	}
	if back, _ := log.InverseExact("agent_id", "engineering"); back != "co-super" {
		t.Fatalf("ID prefix canonical fallback = %q", back)
	}
}

func TestFrozenProtocolSkippedUnknownsReported(t *testing.T) {
	run := &types.RunRecord{AgentProfile: "owner", AgentRole: "trusted-core"}
	out := MigrateRunRecord(run, nil)
	if out.Changed || len(out.Unknowns) != 0 || len(out.Skipped) != 2 {
		t.Fatalf("protocol outcome = %+v", out)
	}
	if run.AgentProfile != "owner" || run.AgentRole != "trusted-core" {
		t.Fatalf("protocol values rewritten: %+v", run)
	}
	run2 := &types.RunRecord{AgentProfile: "root", AgentRole: "super"}
	out2 := MigrateRunRecord(run2, nil)
	if len(out2.Unknowns) != 1 || run2.AgentProfile != "root" {
		t.Fatalf("unknown outcome = %+v", out2)
	}
	if run2.AgentRole != "management" {
		t.Fatalf("known field alongside unknown must still migrate: %+v", run2)
	}
}

func TestMetadataMigration(t *testing.T) {
	meta := map[string]any{
		"agent_profile":        "co-super",
		"agent_role":           "researcher",
		"requested_by_profile": "super",
		"unrelated":            "co-super",
	}
	out := MigrateMetadata(meta, nil)
	if !out.Changed || len(out.Unknowns) != 0 {
		t.Fatalf("metadata outcome = %+v", out)
	}
	if meta["agent_profile"] != "engineering" || meta["agent_role"] != "research" ||
		meta["requested_by_profile"] != "management" || meta["unrelated"] != "co-super" {
		t.Fatalf("metadata mis-migrated: %+v", meta)
	}
}
