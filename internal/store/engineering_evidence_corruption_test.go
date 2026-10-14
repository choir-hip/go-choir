package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// tamperObject rewrites one graph object's body in place while keeping its
// canonical identity, metadata, and updated timestamp. Writes go to the main
// object store exactly like the existing direct-write tests; evidence reads
// see committed state on the read pool.
func tamperObject(t *testing.T, s *Store, ctx context.Context, canonical string, body any) {
	t.Helper()
	obj, err := s.ogStore.GetObject(ctx, canonical)
	if err != nil {
		t.Fatalf("tamper get %s: %v", canonical, err)
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("tamper marshal %s: %v", canonical, err)
	}
	obj.Body = raw
	obj.ContentHash = objectgraph.ContentHash(obj.ObjectKind, raw, obj.Metadata)
	if err := s.ogStore.PutObject(ctx, obj); err != nil {
		t.Fatalf("tamper put %s: %v", canonical, err)
	}
}

// fabricatedObject builds a valid graph object for direct injection without
// going through a reducer.
func fabricatedObject(t *testing.T, kind objectgraph.ObjectKind, ownerID, computerID, key string, body any, metadata map[string]any) objectgraph.Object {
	t.Helper()
	obj, err := lifecycleObject(kind, ownerID, computerID, key, body, metadata, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("fabricate %s %s: %v", kind, key, err)
	}
	return obj
}

// engineeringEvidenceScenario is the base used by the corruption table:
// implementation assignment A (records authority admits implementation
// only) records a changed-subject report producing candidate C. The
// evidence projection is requested for A, so A's assignment/report/event
// and the candidate's source lineage are the tamper targets.
type engineeringEvidenceScenario struct {
	ctx context.Context
	s   *Store
	f   engineeringAssignmentStoreFixture

	open        types.OpenEngineeringAssignmentRequest
	reportID    string
	reportRef   string
	eventID     string
	candidateID string
}

const corruptionAssignmentID = "assignment-corruption-impl"

func buildEngineeringEvidenceScenario(t *testing.T) *engineeringEvidenceScenario {
	t.Helper()
	s := openTestStore(t)
	ctx := context.Background()
	f := installEngineeringAssignmentAuthority(t, s, 1)
	sc := &engineeringEvidenceScenario{ctx: ctx, s: s, f: f}

	impl := engineeringOpenRequest(f, corruptionAssignmentID, "cap-corruption-impl", "capsule-corruption-impl")
	if _, err := s.OpenEngineeringAssignment(ctx, impl); err != nil {
		t.Fatal(err)
	}
	if _, err := s.BindEngineeringAssignment(ctx, bindEngineeringRequest(impl, f.assignedRunIDs[0], "cap-corruption-impl")); err != nil {
		t.Fatal(err)
	}
	changedDigest := objectgraph.SHA256([]byte("corruption changed subject"))
	implReport := assignmentReportRequest(impl, 2, "report-corruption-impl", changedDigest, types.EngineeringResultCompleted, types.EngineeringVerdictNone)
	result, err := s.RecordEngineeringAssignmentReport(ctx, implReport)
	if err != nil || result.Candidate == nil || result.Report == nil || result.Report.CandidateID == "" {
		t.Fatalf("impl changed report: %+v, %v", result, err)
	}
	sc.open = impl
	sc.reportID = result.Report.ReportID
	sc.eventID = implReport.CommandID + ":1"
	sc.candidateID = result.Candidate.CandidateID
	sc.reportRef = reportRefFor(t, s, ctx, f, impl, sc.reportID)
	return sc
}

// evidence reads the requested projection.
func (sc *engineeringEvidenceScenario) evidence(t *testing.T) (EngineeringCapsuleEvidence, error) {
	t.Helper()
	return sc.s.GetEngineeringCapsuleEvidence(sc.ctx, sc.f.ownerID, sc.f.computerID, sc.f.trajectoryID, corruptionAssignmentID, 1)
}

func (sc *engineeringEvidenceScenario) canonical(t *testing.T, kind objectgraph.ObjectKind, key string) string {
	t.Helper()
	id, err := lifecycleCanonicalID(kind, sc.f.ownerID, sc.f.computerID, key)
	if err != nil {
		t.Fatalf("canonical %s %s: %v", kind, key, err)
	}
	return id
}

func (sc *engineeringEvidenceScenario) body(t *testing.T, canonical string) []byte {
	t.Helper()
	obj, err := sc.s.ogStore.GetObject(sc.ctx, canonical)
	if err != nil {
		t.Fatalf("get object %s: %v", canonical, err)
	}
	return obj.Body
}

// reportRefFor resolves the canonical object id of a committed report.
func reportRefFor(t *testing.T, s *Store, ctx context.Context, f engineeringAssignmentStoreFixture, open types.OpenEngineeringAssignmentRequest, reportID string) string {
	t.Helper()
	assignment, err := s.GetEngineeringAssignment(ctx, f.ownerID, f.computerID, open.AssignmentID, open.Binding.Attempt)
	if err != nil || len(assignment.ReportRefs) == 0 {
		t.Fatalf("assignment report refs: %+v, %v", assignment, err)
	}
	return assignment.ReportRefs[len(assignment.ReportRefs)-1]
}

func isCorrupt(err error) bool {
	return strings.Contains(err.Error(), ErrEngineeringEvidenceCorrupt.Error())
}

func TestEngineeringCapsuleEvidenceCandidateSourceLineage(t *testing.T) {
	sc := buildEngineeringEvidenceScenario(t)
	ev, err := sc.evidence(t)
	if err != nil {
		t.Fatalf("evidence: %v", err)
	}
	if len(ev.Candidates) != 1 {
		t.Fatalf("candidates = %d, want 1: %+v", len(ev.Candidates), ev.Candidates)
	}
	c := ev.Candidates[0]
	if c.CandidateID != sc.candidateID || c.SourceReportID != sc.reportID {
		t.Fatalf("candidate join = %+v, want source %s from the implementation report", c, sc.reportID)
	}
	// The source event is joined with its reducer seq.
	if c.ReducerSeq <= 0 {
		t.Fatalf("candidate reducer_seq=%d, want the implementation report event seq", c.ReducerSeq)
	}
	if ev.EvidenceComplete {
		t.Fatalf("expected incomplete verifier contract")
	}
}

func TestEngineeringCapsuleEvidenceRejectsTamperedAssignment(t *testing.T) {
	sc := buildEngineeringEvidenceScenario(t)
	ctx, s, f := sc.ctx, sc.s, sc.f
	assignmentCanonical := sc.canonical(t, ogKindEngineeringAssignment, engineeringAttemptKey(corruptionAssignmentID, 1))
	var a types.EngineeringAssignment
	if err := json.Unmarshal(sc.body(t, assignmentCanonical), &a); err != nil {
		t.Fatal(err)
	}
	t.Run("duplicate report ref", func(t *testing.T) {
		dup := a
		dup.ReportRefs = append(append([]string(nil), a.ReportRefs...), a.ReportRefs[len(a.ReportRefs)-1])
		tamperObject(t, s, ctx, assignmentCanonical, dup)
		if _, err := sc.evidence(t); !isCorrupt(err) {
			t.Fatalf("duplicate ref: err = %v", err)
		}
	})
	t.Run("too large", func(t *testing.T) {
		big := a
		refs := make([]string, 0, 300)
		for i := 0; i < 300; i++ {
			refs = append(refs, fmt.Sprintf("capsule-report:sha256:%d", i))
		}
		big.ReportRefs = refs
		tamperObject(t, s, ctx, assignmentCanonical, big)
		if _, err := sc.evidence(t); err != ErrEngineeringEvidenceTooLarge {
			t.Fatalf("too large: err = %v", err)
		}
	})
	// Cross-owner and cross-computer reads must stay NotFound even after
	// the scope is populated.
	if _, err := s.GetEngineeringCapsuleEvidence(ctx, "other-owner", f.computerID, f.trajectoryID, corruptionAssignmentID, 1); err != ErrNotFound {
		t.Fatalf("cross owner = %v", err)
	}
}

func TestEngineeringCapsuleEvidenceRejectsTamperedReport(t *testing.T) {
	sc := buildEngineeringEvidenceScenario(t)
	ctx, s, f := sc.ctx, sc.s, sc.f
	reportCanonical := sc.canonical(t, ogKindEngineeringReport, sc.reportID)
	var r types.EngineeringAssignmentReport
	if err := json.Unmarshal(sc.body(t, reportCanonical), &r); err != nil {
		t.Fatal(err)
	}
	t.Run("corrupt report body", func(t *testing.T) {
		obj, err := s.ogStore.GetObject(ctx, reportCanonical)
		if err != nil {
			t.Fatal(err)
		}
		obj.Body = []byte(`{not json`)
		obj.ContentHash = objectgraph.ContentHash(obj.ObjectKind, obj.Body, obj.Metadata)
		if err := s.ogStore.PutObject(ctx, obj); err != nil {
			t.Fatal(err)
		}
		if _, err := sc.evidence(t); !isCorrupt(err) {
			t.Fatalf("corrupt body: err = %v", err)
		}
	})
	t.Run("unjoined assignment report", func(t *testing.T) {
		fake := r
		fake.ReportID = "report-corruption-extra"
		fakeObj := fabricatedObject(t, ogKindEngineeringReport, f.ownerID, f.computerID, fake.ReportID, fake, engineeringReportMetadata(fake))
		if err := s.ogStore.PutObject(ctx, fakeObj); err != nil {
			t.Fatal(err)
		}
		if _, err := sc.evidence(t); !isCorrupt(err) {
			t.Fatalf("unjoined report: err = %v", err)
		}
	})
	t.Run("referenced report missing", func(t *testing.T) {
		obj, err := s.ogStore.GetObject(ctx, reportCanonical)
		if err != nil {
			t.Fatal(err)
		}
		obj.Tombstone = true
		if err := s.ogStore.PutObject(ctx, obj); err != nil {
			t.Fatal(err)
		}
		// A tombstoned report must be invisible to the evidence projection.
		if _, err := sc.evidence(t); !isCorrupt(err) {
			t.Fatalf("missing report: err = %v", err)
		}
	})
}

func TestEngineeringCapsuleEvidenceRejectsTamperedReportEvent(t *testing.T) {
	sc := buildEngineeringEvidenceScenario(t)
	ctx, s, f := sc.ctx, sc.s, sc.f
	eventCanonical := sc.canonical(t, ogKindLifecycleEvent, sc.eventID)
	var ev types.LifecycleEvent
	if err := json.Unmarshal(sc.body(t, eventCanonical), &ev); err != nil {
		t.Fatal(err)
	}
	reportCanonical := sc.canonical(t, ogKindEngineeringReport, sc.reportID)
	expect := func(name string, mutated types.LifecycleEvent) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			tamperObject(t, s, ctx, eventCanonical, mutated)
			if _, err := sc.evidence(t); !isCorrupt(err) {
				t.Fatalf("%s: err = %v", name, err)
			}
		})
	}
	kindMismatch := ev
	kindMismatch.Kind = types.LifecycleEngineeringAssignmentBound
	expect("report event kind or work scope", kindMismatch)

	scopeMismatch := ev
	scopeMismatch.WorkItemID = "work-other"
	expect("report event work scope mismatch", scopeMismatch)

	dupRef := ev
	dupRef.ArtifactRefs = append(dupRef.ArtifactRefs, reportCanonical)
	expect("duplicate report artifact ref in event", dupRef)

	missingRef := ev
	missingRef.ArtifactRefs = nil
	expect("report event missing", missingRef)

	runMismatch := ev
	runMismatch.RunID = "run-other"
	expect("report event run or agent scope", runMismatch)

	agentMismatch := ev
	agentMismatch.AgentID = "engineering:other"
	expect("report event agent scope mismatch", agentMismatch)

	t.Run("ambiguous report event", func(t *testing.T) {
		fake := ev
		fake.EventID = "fabricated-event:1"
		fake.CommandID = "command-fabricated"
		fake.CommandDigest = objectgraph.SHA256([]byte("fabricated"))
		fake.ReducerSeq = ev.ReducerSeq + 1
		fake.ArtifactRefs = []string{reportCanonical}
		fakeObj := fabricatedObject(t, ogKindLifecycleEvent, f.ownerID, f.computerID, fake.EventID, fake,
			lifecycleMetadata("event_id", fake.EventID, f.computerID, f.trajectoryID, fake.ReducerSeq))
		if err := s.ogStore.PutObject(ctx, fakeObj); err != nil {
			t.Fatal(err)
		}
		if _, err := sc.evidence(t); !isCorrupt(err) {
			t.Fatalf("ambiguous event: err = %v", err)
		}
	})
}

func TestEngineeringCapsuleEvidenceRejectsTamperedCandidate(t *testing.T) {
	sc := buildEngineeringEvidenceScenario(t)
	ctx, s := sc.ctx, sc.s
	var c types.EngineeringSubjectCandidate
	obj, err := s.ogStore.GetObject(ctx, sc.candidateID)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(obj.Body, &c); err != nil {
		t.Fatal(err)
	}
	t.Run("unjoined assignment candidate", func(t *testing.T) {
		fake := c
		// Scope the forged candidate to the requested assignment so the
		// unjoined-candidate check fires; no report references it.
		fake.AssignmentID = sc.open.AssignmentID
		fake.Attempt = sc.open.Binding.Attempt
		key := strings.Join([]string{sc.f.ownerID, sc.f.computerID, "fabricated", "candidate"}, "\x00")
		fakeObj := fabricatedObject(t, ogKindEngineeringCandidate, sc.f.ownerID, sc.f.computerID, key, fake,
			map[string]any{"candidate_id": fake.CandidateID, "computer_id": sc.f.computerID, "trajectory_id": sc.f.trajectoryID})
		fake.CandidateID = fakeObj.CanonicalID
		fakeObj.Body, _ = json.Marshal(fake)
		fakeObj.ContentHash = objectgraph.ContentHash(fakeObj.ObjectKind, fakeObj.Body, fakeObj.Metadata)
		if err := s.ogStore.PutObject(ctx, fakeObj); err != nil {
			t.Fatal(err)
		}
		if _, err := sc.evidence(t); !isCorrupt(err) {
			t.Fatalf("unjoined candidate: err = %v", err)
		}
	})
	t.Run("candidate validation", func(t *testing.T) {
		bad := c
		bad.ArtifactRef = "capsule-subject:" + objectgraph.SHA256([]byte("tampered"))
		tamperObject(t, s, ctx, sc.candidateID, bad)
		if _, err := sc.evidence(t); !isCorrupt(err) {
			t.Fatalf("candidate validation: err = %v", err)
		}
	})
	t.Run("referenced candidate missing", func(t *testing.T) {
		reportCanonical := sc.canonical(t, ogKindEngineeringReport, sc.reportID)
		var r types.EngineeringAssignmentReport
		if err := json.Unmarshal(sc.body(t, reportCanonical), &r); err != nil {
			t.Fatal(err)
		}
		bad := r
		bad.CandidateID = "capsule-candidate:sha256:missing"
		tamperObject(t, s, ctx, reportCanonical, bad)
		if _, err := sc.evidence(t); !isCorrupt(err) {
			t.Fatalf("referenced candidate missing: err = %v", err)
		}
	})
}

func TestEngineeringCapsuleEvidenceRejectsTamperedCandidateSource(t *testing.T) {
	sc := buildEngineeringEvidenceScenario(t)
	ctx, s := sc.ctx, sc.s
	var c types.EngineeringSubjectCandidate
	obj, err := s.ogStore.GetObject(ctx, sc.candidateID)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(obj.Body, &c); err != nil {
		t.Fatal(err)
	}
	// A candidate whose source report exists nowhere must fail the report join.
	t.Run("candidate report join missing source", func(t *testing.T) {
		bad := c
		bad.SourceReportID = "report-does-not-exist"
		tamperObject(t, s, ctx, sc.candidateID, bad)
		if _, err := sc.evidence(t); !isCorrupt(err) {
			t.Fatalf("candidate report join: err = %v", err)
		}
	})
	// A candidate whose source report validates but carries a different
	// candidate identity must fail closed.
	t.Run("candidate report join identity", func(t *testing.T) {
		reportCanonical := sc.canonical(t, ogKindEngineeringReport, sc.reportID)
		var rr types.EngineeringAssignmentReport
		if err := json.Unmarshal(sc.body(t, reportCanonical), &rr); err != nil {
			t.Fatal(err)
		}
		bad := rr
		bad.CandidateID = "capsule-candidate:sha256:other"
		tamperObject(t, s, ctx, reportCanonical, bad)
		if _, err := sc.evidence(t); !isCorrupt(err) {
			t.Fatalf("candidate report join identity: err = %v", err)
		}
	})
}
