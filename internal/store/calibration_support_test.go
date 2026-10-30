package store

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// Texture's citation self-score (docs/archive/reports/icrl-calibration-experiment-2026-10-14.md):
// a Write's pSupported rides the revision metadata, and the turn that
// lists a new pair copies it, with the authoring model, onto the pair's
// check memo, where Jev's verdict on the same key settles it. Ways it can
// fail: the score is lost between revision and memo; a revision without
// one gets one; a malformed value is stored; the memo lists lose their
// times, so the digest cannot order them.
func TestCitationChecksCarryTheWritesSelfScore(t *testing.T) {
	s, start, caller, _ := setupLifecycleTextureTargetFixture(t)
	ctx := context.Background()
	owner, computer := start.OwnerID, start.ComputerID
	turn := func(revisionID, meta string, pairs ...types.CitationPair) {
		t.Helper()
		req := textureTurnBaseRequest(t, s, start, caller, types.TextureTurnRevision)
		req.CommandID = "cal-turn-" + revisionID
		rev := start.InitialRevision
		rev.RevisionID, rev.AuthorKind, rev.AuthorLabel, rev.CreatedAt = revisionID, types.AuthorAppAgent, "appagent", time.Time{}
		rev.ParentRevisionID = req.ExpectedHeadRevisionID
		rev.BodyDoc, _ = json.Marshal(userAuthoredTextStructuredTextureDoc(rev.DocID, rev.RevisionID, "The merger may proceed "+revisionID+"."))
		rev.Content = "The merger may proceed " + revisionID + "."
		if meta != "" {
			rev.Metadata = json.RawMessage(meta)
		}
		rev.Provenance = json.RawMessage(`{"schema_version":1,"authoring_model":{"provider":"chatgpt","model":"gpt-6-luna"},"authored_at":"2026-10-15T00:00:00Z"}`)
		req.Revision = rev
		req.Citations = pairs
		setTextureTurnDigest(t, &req, TextureSourceGraphWriteSet{})
		if _, err := s.ApplyTextureTurn(ctx, req); err != nil {
			t.Fatalf("turn %s: %v", revisionID, err)
		}
	}
	scored := supportPair("cal-rev-1", "p-1", "The merger may proceed.", "merger may proceed")
	turn("cal-rev-1", `{"texture_write_p_supported":0.85}`, scored)
	plain := supportPair("cal-rev-2", "p-2", "An appeal was filed.", "appeal was filed")
	turn("cal-rev-2", `{"texture_write_why":"no score"}`, plain)
	bad := supportPair("cal-rev-3", "p-3", "Shares fell.", "shares fell")
	turn("cal-rev-3", `{"texture_write_p_supported":1.7}`, bad)

	checks, err := s.ListCitationChecks(ctx, owner, computer, 100)
	if err != nil || len(checks) != 3 {
		t.Fatalf("checks = %d, %v", len(checks), err)
	}
	byKey := map[string]SupportMemo[types.CitationCheck]{}
	for _, c := range checks {
		byKey[c.Value.Pair.Key] = c
		if c.At.IsZero() {
			t.Fatalf("check %s has no time", c.Value.Pair.Key)
		}
	}
	if c := byKey[scored.Key].Value; c.SelfP == nil || *c.SelfP != 0.85 || c.ModelID != "chatgpt/gpt-6-luna" {
		t.Fatalf("scored check = %+v", c)
	}
	if c := byKey[plain.Key].Value; c.SelfP != nil {
		t.Fatalf("a Write without pSupported got %v", *c.SelfP)
	}
	if c := byKey[bad.Key].Value; c.SelfP != nil {
		t.Fatalf("pSupported 1.7 stored as %v", *c.SelfP)
	}
	if _, err := commitVerdicts(t, s, start, types.SupportSystemAgentID, "cal-verdict-1",
		types.SupportVerdict{Key: scored.Key, Verdict: types.SupportSupported, Judge: "jev:m"}); err != nil {
		t.Fatal(err)
	}
	verdicts, err := s.ListSupportVerdicts(ctx, owner, computer, 100)
	if err != nil || len(verdicts) != 1 || verdicts[0].Value.Key != scored.Key || verdicts[0].Value.Verdict != types.SupportSupported {
		t.Fatalf("verdicts = %+v, %v", verdicts, err)
	}
	if other, _ := s.ListCitationChecks(ctx, owner, "another-computer", 100); len(other) != 0 {
		t.Fatalf("another computer lists %d checks", len(other))
	}
}
