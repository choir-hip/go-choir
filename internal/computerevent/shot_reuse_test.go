package computerevent

import (
	"strings"
	"testing"
)

func reuseIDs(seed string) ShotInputIDs {
	list := make([]string, len(ShotInputPaths))
	for i, p := range ShotInputPaths {
		list[i] = DigestBytes([]byte(seed + p))[:40]
	}
	ids, _ := ShotInputIDsFromList(list)
	return ids
}


func goodReuse() ShotReusePayload {
	ids := reuseIDs("same")
	return ShotReusePayload{
		OperationID: "clean-rebase-new", BundleDigest: testDigestA, HostBuildJobID: testDigestB, ShotJobID: "liveness-1", PlanSHA256: testDigestA,
		Verdict: ShotVerdictPass, Reason: ShotReuseReason, RebaseOf: "op-prior", PriorEvent: testDigestA, PriorKind: string(EventShotVerdict),
		OriginEvent: testDigestA, OriginKey: ShotVerdictKey("op-prior", testDigestB), ChainDepth: 1,
		JudgedBase: strings.Repeat("1", 40), Judged: ids, BootedBase: strings.Repeat("2", 40), Booted: ids,
		Driver:   ShotDriverEnv{Browser: "/nix/store/b", Fontconfig: "/nix/store/f"},
		Liveness: ShotLiveness{Passed: true, Requests: 2, Marker: "m"},
	}
}

// A reuse that does not validate must never be readable as a pass (F2, F3).
func TestShotReusePayloadValidate(t *testing.T) {
	if err := goodReuse().Validate(); err != nil {
		t.Fatal(err)
	}
	for name, fn := range map[string]func(p *ShotReusePayload){
		"no operation":        func(p *ShotReusePayload) { p.OperationID = "" },
		"bad bundle":          func(p *ShotReusePayload) { p.BundleDigest = "x" },
		"no liveness job":     func(p *ShotReusePayload) { p.ShotJobID = "" },
		"a fail":              func(p *ShotReusePayload) { p.Verdict = ShotVerdictFail },
		"wrong reason":        func(p *ShotReusePayload) { p.Reason = "ok" },
		"rebase of itself":    func(p *ShotReusePayload) { p.RebaseOf = p.OperationID },
		"no prior":            func(p *ShotReusePayload) { p.PriorEvent = "" },
		"no origin":           func(p *ShotReusePayload) { p.OriginEvent = "" },
		"origin key not verd": func(p *ShotReusePayload) { p.OriginKey = "other:key" },
		"prior kind":          func(p *ShotReusePayload) { p.PriorKind = "candidate_shot" },
		"depth zero":          func(p *ShotReusePayload) { p.ChainDepth = 0 },
		"depth over bound":    func(p *ShotReusePayload) { p.PriorKind, p.ChainDepth = string(EventShotReuse), ShotReuseMaxChain+1 },
		"verdict prior deep":  func(p *ShotReusePayload) { p.ChainDepth = 2 },
		"reuse prior depth 1": func(p *ShotReusePayload) { p.PriorKind = string(EventShotReuse) },
		"empty judged ids":    func(p *ShotReusePayload) { p.Judged, p.Booted = ShotInputIDs{}, ShotInputIDs{} },
		"one empty id":        func(p *ShotReusePayload) { p.Judged.Nix, p.Booted.Nix = "", "" },
		"ids differ":          func(p *ShotReusePayload) { p.Booted = reuseIDs("other") },
		"short base":          func(p *ShotReusePayload) { p.JudgedBase = "abc" },
		"no browser":          func(p *ShotReusePayload) { p.Driver.Browser = "" },
		"no fontconfig":       func(p *ShotReusePayload) { p.Driver.Fontconfig = "" },
		"liveness failed":     func(p *ShotReusePayload) { p.Liveness.Passed = false },
		"liveness no calls":   func(p *ShotReusePayload) { p.Liveness.Requests = 0 },
		"liveness errors":     func(p *ShotReusePayload) { p.Liveness.Failures = 1 },
		"liveness no marker":  func(p *ShotReusePayload) { p.Liveness.Marker = "" },
	} {
		p := goodReuse()
		fn(&p)
		if p.Validate() == nil {
			t.Errorf("%s: validated", name)
		}
	}
	// "" == "" is not a match: empty ids are never complete.
	if (ShotInputIDs{}).Complete() {
		t.Error("empty source ids are complete")
	}
	if IsGitObjectID("") || IsGitObjectID(strings.Repeat("A", 40)) || !IsGitObjectID(strings.Repeat("a", 40)) || !IsGitObjectID(strings.Repeat("a", 64)) {
		t.Error("IsGitObjectID is wrong")
	}
	if _, ok := validEventKinds[EventShotReuse]; !ok {
		t.Error("shot_reuse is not a registered event kind")
	}
}
