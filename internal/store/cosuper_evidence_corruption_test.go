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

// coSuperEvidenceScenario is the rich base used by the corruption table.
// Implementation assignment A records a changed-subject report producing
// candidate C; verification assignment V is then bound over that candidate
// and records a certifying report. The evidence projection is requested for
// V, so V's assignment/report/event are the direct tamper targets while the
// candidate source lineage exercises A's report/event.
type coSuperEvidenceScenario struct {
	ctx context.Context
	s   *Store
	f   coSuperAssignmentStoreFixture

	verifyOpen     types.OpenCoSuperAssignmentRequest
	verifyReportID string
	verifyEventID  string

	implOpen      types.OpenCoSuperAssignmentRequest
	implReportID  string
	implReportRef string
	implEventID   string
	candidateID   string
}

func buildCoSuperEvidenceScenario(t *testing.T) *coSuperEvidenceScenario {
	t.Helper()
	s := openTestStore(t)
	ctx := context.Background()
	f := installCoSuperAssignmentAuthority(t, s, 2)
	sc := &coSuperEvidenceScenario{ctx: ctx, s: s, f: f}

	impl := coSuperOpenRequest(f, 0, "assignment-corruption-impl", 1, types.CoSuperAssignmentImplementation, true, "cap-corruption-impl", "capsule-corruption-impl")
	if _, err := s.OpenCoSuperAssignment(ctx, impl); err != nil {
		t.Fatal(err)
	}
	if _, err := s.BindCoSuperAssignment(ctx, bindCoSuperRequest(impl, f.assignedRunIDs[0], "cap-corruption-impl")); err != nil {
		t.Fatal(err)
	}
	changedDigest := objectgraph.SHA256([]byte("corruption changed subject"))
	implReport := assignmentReportRequest(impl, 2, "report-corruption-impl", changedDigest, types.CoSuperResultCompleted, types.CoSuperVerdictNone)
	result, err := s.RecordCoSuperAssignmentReport(ctx, implReport)
	if err != nil || result.Candidate == nil || result.Report == nil || result.Report.CandidateID == "" {
		t.Fatalf("impl changed report: %+v, %v", result, err)
	}
	sc.implOpen = impl
	sc.implReportID = result.Report.ReportID
	sc.implEventID = implReport.CommandID + ":1"
	sc.candidateID = result.Candidate.CandidateID
	sc.implReportRef = reportRefFor(t, s, ctx, f, impl, sc.implReportID)

	verify := coSuperOpenRequest(f, 1, "assignment-corruption-verify", 1, types.CoSuperAssignmentVerification, true, "cap-corruption-verify", "capsule-corruption-verify")
	verify.Binding.SubjectDigest = result.Candidate.SubjectDigest
	verify.Binding.SourceCandidateID = result.Candidate.CandidateID
	verify.Binding.SourceArtifactRef = "capsule-subject:" + result.Candidate.SubjectDigest
	verify.CommandDigest, _ = ComputeOpenCoSuperAssignmentDigest(verify)
	if _, err := s.OpenCoSuperAssignment(ctx, verify); err != nil {
		t.Fatal(err)
	}
	if _, err := s.BindCoSuperAssignment(ctx, bindCoSuperRequest(verify, f.assignedRunIDs[1], "cap-corruption-verify")); err != nil {
		t.Fatal(err)
	}
	verifyReport := assignmentReportRequest(verify, 2, "report-corruption-verify", verify.Binding.SubjectDigest, types.CoSuperResultCompleted, types.CoSuperVerdictPass)
	verifyResult, err := s.RecordCoSuperAssignmentReport(ctx, verifyReport)
	if err != nil || verifyResult.Report == nil {
		t.Fatalf("verify report: %+v, %v", verifyResult, err)
	}
	sc.verifyOpen = verify
	sc.verifyReportID = verifyResult.Report.ReportID
	sc.verifyEventID = verifyReport.CommandID + ":1"
	return sc
}

// evidence reads the requested verify projection.
func (sc *coSuperEvidenceScenario) evidence(t *testing.T) (CoSuperCapsuleEvidence, error) {
	t.Helper()
	return sc.s.GetCoSuperCapsuleEvidence(sc.ctx, sc.f.ownerID, sc.f.computerID, sc.f.trajectoryID, "assignment-corruption-verify", 1)
}

func (sc *coSuperEvidenceScenario) canonical(t *testing.T, kind objectgraph.ObjectKind, key string) string {
	t.Helper()
	id, err := lifecycleCanonicalID(kind, sc.f.ownerID, sc.f.computerID, key)
	if err != nil {
		t.Fatalf("canonical %s %s: %v", kind, key, err)
	}
	return id
}

func (sc *coSuperEvidenceScenario) body(t *testing.T, canonical string) []byte {
	t.Helper()
	obj, err := sc.s.ogStore.GetObject(sc.ctx, canonical)
	if err != nil {
		t.Fatalf("get object %s: %v", canonical, err)
	}
	return obj.Body
}

// reportRefFor resolves the canonical object id of a committed report.
func reportRefFor(t *testing.T, s *Store, ctx context.Context, f coSuperAssignmentStoreFixture, open types.OpenCoSuperAssignmentRequest, reportID string) string {
	t.Helper()
	assignment, err := s.GetCoSuperAssignment(ctx, f.ownerID, f.computerID, open.AssignmentID, open.Binding.Attempt)
	if err != nil || len(assignment.ReportRefs) == 0 {
		t.Fatalf("assignment report refs: %+v, %v", assignment, err)
	}
	return assignment.ReportRefs[len(assignment.ReportRefs)-1]
}

func isCorrupt(err error) bool {
	return strings.Contains(err.Error(), ErrCoSuperEvidenceCorrupt.Error())
}

func TestCoSuperCapsuleEvidenceCrossAssignmentCandidateSourceLineage(t *testing.T) {
	sc := buildCoSuperEvidenceScenario(t)
	ev, err := sc.evidence(t)
	if err != nil {
		t.Fatalf("verify evidence: %v", err)
	}
	if len(ev.Candidates) != 1 {
		t.Fatalf("candidates = %d, want 1: %+v", len(ev.Candidates), ev.Candidates)
	}
	c := ev.Candidates[0]
	if c.CandidateID != sc.candidateID || c.SourceReportID != sc.implReportID {
		t.Fatalf("candidate join = %+v, want source %s from implementation report", c, sc.implReportID)
	}
	// The implementation source event must be joined from the cross-assignment
	// scan (not from the verification's own report refs) with its seq.
	if c.ReducerSeq <= 0 {
		t.Fatalf("candidate reducer_seq=%d, want the implementation report event seq", c.ReducerSeq)
	}
	// The verifier contract reports that the candidate's source evidence came
	// from an implementation assignment bind.
	if ev.EvidenceComplete {
		t.Fatalf("expected incomplete verifier contract")
	}
}

func TestCoSuperCapsuleEvidenceRejectsTamperedVerifyAssignment(t *testing.T) {
	sc := buildCoSuperEvidenceScenario(t)
	ctx, s, f := sc.ctx, sc.s, sc.f
	assignmentCanonical := sc.canonical(t, ogKindCoSuperAssignment, coSuperAttemptKey("assignment-corruption-verify", 1))
	var a types.CoSuperAssignment
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
		if _, err := sc.evidence(t); err != ErrCoSuperEvidenceTooLarge {
			t.Fatalf("too large: err = %v", err)
		}
	})
	// Cross-owner and cross-computer reads must stay NotFound even after
	// the scope is populated.
	if _, err := s.GetCoSuperCapsuleEvidence(ctx, "other-owner", f.computerID, f.trajectoryID, "assignment-corruption-verify", 1); err != ErrNotFound {
		t.Fatalf("cross owner = %v", err)
	}
}

func TestCoSuperCapsuleEvidenceRejectsTamperedVerifyReport(t *testing.T) {
	sc := buildCoSuperEvidenceScenario(t)
	ctx, s, f := sc.ctx, sc.s, sc.f
	reportCanonical := sc.canonical(t, ogKindCoSuperReport, sc.verifyReportID)
	var r types.CoSuperAssignmentReport
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
		fakeObj := fabricatedObject(t, ogKindCoSuperReport, f.ownerID, f.computerID, fake.ReportID, fake, coSuperReportMetadata(fake))
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

func TestCoSuperCapsuleEvidenceRejectsTamperedVerifyEvent(t *testing.T) {
	sc := buildCoSuperEvidenceScenario(t)
	ctx, s, f := sc.ctx, sc.s, sc.f
	eventCanonical := sc.canonical(t, ogKindLifecycleEvent, sc.verifyEventID)
	var ev types.LifecycleEvent
	if err := json.Unmarshal(sc.body(t, eventCanonical), &ev); err != nil {
		t.Fatal(err)
	}
	reportCanonical := sc.canonical(t, ogKindCoSuperReport, sc.verifyReportID)
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
	kindMismatch.Kind = types.LifecycleCoSuperAssignmentBound
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
	agentMismatch.AgentID = "co-super:other"
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

func TestCoSuperCapsuleEvidenceRejectsTamperedCandidate(t *testing.T) {
	sc := buildCoSuperEvidenceScenario(t)
	ctx, s := sc.ctx, sc.s
	var c types.CoSuperSubjectCandidate
	obj, err := s.ogStore.GetObject(ctx, sc.candidateID)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(obj.Body, &c); err != nil {
		t.Fatal(err)
	}
	t.Run("unjoined assignment candidate", func(t *testing.T) {
		fake := c
		// Scope the forged candidate to the requested (verification)
		// assignment so the unjoined-candidate check fires; it is referenced
		// by neither the verification binding nor any report.
		fake.AssignmentID = sc.verifyOpen.AssignmentID
		fake.Attempt = sc.verifyOpen.Binding.Attempt
		key := strings.Join([]string{sc.f.ownerID, sc.f.computerID, "fabricated", "candidate"}, "\x00")
		fakeObj := fabricatedObject(t, ogKindCoSuperCandidate, sc.f.ownerID, sc.f.computerID, key, fake,
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
		verifyReportCanonical := sc.canonical(t, ogKindCoSuperReport, sc.verifyReportID)
		var r types.CoSuperAssignmentReport
		if err := json.Unmarshal(sc.body(t, verifyReportCanonical), &r); err != nil {
			t.Fatal(err)
		}
		bad := r
		bad.CandidateID = "capsule-candidate:sha256:missing"
		tamperObject(t, s, ctx, verifyReportCanonical, bad)
		if _, err := sc.evidence(t); !isCorrupt(err) {
			t.Fatalf("referenced candidate missing: err = %v", err)
		}
	})
}

func TestCoSuperCapsuleEvidenceRejectsTamperedCandidateSource(t *testing.T) {
	sc := buildCoSuperEvidenceScenario(t)
	ctx, s := sc.ctx, sc.s
	var c types.CoSuperSubjectCandidate
	obj, err := s.ogStore.GetObject(ctx, sc.candidateID)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(obj.Body, &c); err != nil {
		t.Fatal(err)
	}
	// Wiring the implementation candidate's source to the verification's own
	// report must fail closed on lineage: a verification report is not an
	// implementation source.
	t.Run("candidate source lineage", func(t *testing.T) {
		bad := c
		bad.SourceReportID = sc.verifyReportID
		tamperObject(t, s, ctx, sc.candidateID, bad)
		if _, err := sc.evidence(t); !isCorrupt(err) {
			t.Fatalf("candidate source lineage: err = %v", err)
		}
	})
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
		implReportCanonical := sc.canonical(t, ogKindCoSuperReport, sc.implReportID)
		var rr types.CoSuperAssignmentReport
		if err := json.Unmarshal(sc.body(t, implReportCanonical), &rr); err != nil {
			t.Fatal(err)
		}
		bad := rr
		bad.CandidateID = "capsule-candidate:sha256:other"
		tamperObject(t, s, ctx, implReportCanonical, bad)
		if _, err := sc.evidence(t); !isCorrupt(err) {
			t.Fatalf("candidate report join identity: err = %v", err)
		}
	})
}

func TestCoSuperCapsuleEvidenceRejectsTamperedImplEventSourceScope(t *testing.T) {
	// The cross-assignment source event join must validate run/agent scope
	// exactly like the direct report join.
	sc := buildCoSuperEvidenceScenario(t)
	ctx, s := sc.ctx, sc.s
	implEventCanonical := sc.canonical(t, ogKindLifecycleEvent, sc.implEventID)
	var ev types.LifecycleEvent
	if err := json.Unmarshal(sc.body(t, implEventCanonical), &ev); err != nil {
		t.Fatal(err)
	}
	runMismatch := ev
	runMismatch.RunID = "run-other"
	tamperObject(t, s, ctx, implEventCanonical, runMismatch)
	if _, err := sc.evidence(t); !isCorrupt(err) {
		t.Fatalf("impl event run scope: err = %v", err)
	}
}
