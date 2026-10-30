package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Support judgement memo (Cut 2 2a (v)). Two objects per hash pair, both in
// the computer's store beside the records they ride with:
//
//   - choir.citation_check: the pair's quote check, written by the Texture
//     turn that first lists the pair (ApplyTextureTurn), in the same
//     transaction as the revision and the list record;
//   - choir.support_verdict: the pair's verdict, written by the runtime's
//     Jev job as a system act (CommitInboxRecords with Judge
//     system:support), in the same transaction as the failure record to
//     the Texture (support-judgement-to-jev-design-2026-10-14).
//
// A pair with a check and no verdict is pending: the commit listed it and
// Jev has not judged it yet.
//
// The definition puts check results in Store B behind corpusd. They live
// here instead because both writes must be atomic with this store (the
// check with the revision, the failure record with the verdict), and a
// guest reads Store B only through collection routes, not by key. Unit 2b
// publishes them with each version's overlay, so public reads still never
// reach a paper computer.

const (
	ogKindCitationCheck  = objectgraph.ObjectKind("choir.citation_check")
	ogKindSupportVerdict = objectgraph.ObjectKind("choir.support_verdict")
)

// CitationCheck reads one pair's check memo.
func (s *Store) CitationCheck(ctx context.Context, ownerID, computerID, key string) (types.CitationCheck, bool, error) {
	return getSupportMemo[types.CitationCheck](ctx, s, ogKindCitationCheck, ownerID, computerID, key)
}

// SupportVerdict reads one pair's verdict memo.
func (s *Store) SupportVerdict(ctx context.Context, ownerID, computerID, key string) (types.SupportVerdict, bool, error) {
	return getSupportMemo[types.SupportVerdict](ctx, s, ogKindSupportVerdict, ownerID, computerID, key)
}

func getSupportMemo[T any](ctx context.Context, s *Store, kind objectgraph.ObjectKind, ownerID, computerID, key string) (T, bool, error) {
	var zero T
	if strings.TrimSpace(key) == "" {
		return zero, false, nil
	}
	obj, err := s.lifecycleGetObject(ctx, kind, strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(key))
	if errors.Is(err, ErrNotFound) {
		return zero, false, nil
	}
	if err != nil {
		return zero, false, err
	}
	value, err := decodeLifecycleObject[T](obj)
	return value, err == nil, err
}

// validSupportPair checks a pair's shape and that its key is its hash pair.
func validSupportPair(p types.CitationPair) bool {
	return p.Key != "" && p.DocID != "" && p.RevisionID != "" && p.BlockID != "" && p.BlockHash != "" &&
		p.Cited.Doc != "" && p.Cited.Hash != "" && p.Cited.Quote != "" && p.Matcher != "" &&
		p.Key == types.SupportPairKey(p.BlockHash, p.Cited.Hash, p.Cited.Quote, p.Matcher)
}

// supportPairKnown reports a pair already listed (a check memo) or judged.
func (s *Store) supportPairKnown(ctx context.Context, ownerID, computerID, key string) (bool, error) {
	for _, kind := range []objectgraph.ObjectKind{ogKindCitationCheck, ogKindSupportVerdict} {
		_, err := s.lifecycleGetObject(ctx, kind, ownerID, computerID, key)
		if err == nil {
			return true, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return false, err
		}
	}
	return false, nil
}

// planSupportPairs is a revision turn's share of support judging: each pair
// with neither a check nor a verdict gets its check memo, which lists it. A
// pair already listed or judged is skipped, so an unchanged citation is
// listed once. The commit sends no record: the runtime hands the revision's
// pairs to Jev after the commit returns
// (support-judgement-to-jev-design-2026-10-14).
func (s *Store) planSupportPairs(ctx context.Context, ownerID, computerID string, req types.ApplyTextureTurnRequest, now time.Time) ([]objectgraph.Object, []objectgraph.ObjectCondition, error) {
	if len(req.Citations) == 0 {
		return nil, nil, nil
	}
	var objects []objectgraph.Object
	var conditions []objectgraph.ObjectCondition
	selfP, modelID := revisionCitationSelfScore(req.Revision)
	seen := map[string]bool{}
	for _, p := range req.Citations {
		if !validSupportPair(p) || p.DocID != req.DocumentID || p.RevisionID != req.Revision.RevisionID {
			return nil, nil, inboxRefused("citation pair %q does not name a hash pair of revision %s", p.Key, req.Revision.RevisionID)
		}
		if seen[p.Key] {
			continue
		}
		seen[p.Key] = true
		known, err := s.supportPairKnown(ctx, ownerID, computerID, p.Key)
		if err != nil {
			return nil, nil, err
		}
		if known {
			continue
		}
		// Write matched every committed quote, so a listed pair's check is
		// a match at its tier. The Write's self-score, if it stated one,
		// rides the check: Jev's verdict on the same key settles it.
		check := types.CitationCheck{Pair: p, Outcome: "matched", Tier: p.Tier, ListedBy: req.CommandID, SelfP: selfP, ModelID: modelID}
		obj, err := lifecycleObject(ogKindCitationCheck, ownerID, computerID, p.Key, check, map[string]any{
			"pair_key": p.Key, "computer_id": computerID, "doc_id": p.DocID, "revision_id": p.RevisionID,
		}, now, now)
		if err != nil {
			return nil, nil, err
		}
		objects = append(objects, obj)
		conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: obj.CanonicalID})
	}
	return objects, conditions, nil
}

// planSupportVerdicts records the Jev job's verdicts, a system act: the
// judge must be types.SupportSystemAgentID. A verdict needs a listed pair
// (its check memo), whose pair it records; a pair already judged is a memo
// hit and records nothing. The new failures for each document become one
// system record from the judge to texture:<doc>, which wakes the Texture. A
// pass records only its memo: it mints no record and touches no document.
func (s *Store) planSupportVerdicts(ctx context.Context, ownerID, computerID, judge, commandID string, verdicts []types.SupportVerdict, now time.Time) ([]objectgraph.Object, []objectgraph.ObjectCondition, []types.CommitmentRecord, []types.SupportVerdict, error) {
	if len(verdicts) == 0 {
		return nil, nil, nil, nil, nil
	}
	judge = strings.TrimSpace(judge)
	if judge != types.SupportSystemAgentID {
		return nil, nil, nil, nil, inboxRefused("support verdicts are the system's Jev judgement: judge %q is not %s", judge, types.SupportSystemAgentID)
	}
	var objects []objectgraph.Object
	var conditions []objectgraph.ObjectCondition
	var recorded []types.SupportVerdict
	failures := map[string][]types.SupportVerdict{}
	seen := map[string]bool{}
	for _, v := range verdicts {
		v.Key, v.Why, v.Judge = strings.TrimSpace(v.Key), strings.TrimSpace(v.Why), strings.TrimSpace(v.Judge)
		if v.Verdict != types.SupportSupported && v.Verdict != types.SupportUnsupported {
			return nil, nil, nil, nil, inboxRefused("verdict on %s is %q, not %s or %s", v.Key, v.Verdict, types.SupportSupported, types.SupportUnsupported)
		}
		if v.Judge == "" || (v.Verdict == types.SupportUnsupported && v.Why == "") {
			return nil, nil, nil, nil, inboxRefused("verdict on %s needs its judge, and a failure needs why", v.Key)
		}
		if seen[v.Key] {
			continue
		}
		seen[v.Key] = true
		check, listed, err := s.CitationCheck(ctx, ownerID, computerID, v.Key)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		if !listed {
			return nil, nil, nil, nil, inboxRefused("pair %s was not listed by a Texture commit", v.Key)
		}
		if _, judged, err := s.SupportVerdict(ctx, ownerID, computerID, v.Key); err != nil {
			return nil, nil, nil, nil, err
		} else if judged {
			continue
		}
		pair := check.Pair
		v.Pair, v.Memo, v.RecordID = &pair, false, ""
		if v.Verdict == types.SupportUnsupported {
			v.RecordID = commandID + ":support:" + pair.DocID
			failures[pair.DocID] = append(failures[pair.DocID], v)
		}
		obj, err := lifecycleObject(ogKindSupportVerdict, ownerID, computerID, v.Key, v, map[string]any{
			"pair_key": v.Key, "computer_id": computerID, "doc_id": pair.DocID, "revision_id": pair.RevisionID, "verdict": v.Verdict,
		}, now, now)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		objects = append(objects, obj)
		conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: obj.CanonicalID})
		recorded = append(recorded, v)
	}
	docs := make([]string, 0, len(failures))
	for doc := range failures {
		docs = append(docs, doc)
	}
	sort.Strings(docs)
	var records []types.CommitmentRecord
	for _, doc := range docs {
		texture := agentprofile.Texture + ":" + doc
		records = append(records, types.CommitmentRecord{
			SchemaID: types.CommitmentRecordSchemaV1, RecordID: commandID + ":support:" + doc, Kind: types.CommitmentKindReport,
			Discrepancy: types.DiscrepancyUnresolved, Provenance: types.CommitmentProvenance{AgentID: judge},
			EmitterKind: types.CommitmentEmitterSystem, ComputerID: computerID,
			Inbox: texture, SenderInbox: judge, Addressee: texture,
			SubjectRefs: map[string]string{"doc_id": doc},
			Support:     &types.CommitmentSupport{Failures: failures[doc]},
		})
	}
	return objects, conditions, records, recorded, nil
}

// RevisionMetaPSupported is the revision metadata key a Texture Write's
// self-score is stored under (choir.Write's pSupported).
const RevisionMetaPSupported = "texture_write_p_supported"

// revisionCitationSelfScore reads a revision's citation self-score and the
// model that wrote it: the Write's pSupported from the revision metadata
// and "<provider>/<model>" from its provenance
// (docs/archive/reports/icrl-calibration-experiment-2026-10-14.md). A missing or
// malformed value is no self-score.
func revisionCitationSelfScore(rev types.Revision) (*float64, string) {
	var meta map[string]any
	if len(rev.Metadata) == 0 || json.Unmarshal(rev.Metadata, &meta) != nil {
		return nil, ""
	}
	p, ok := meta[RevisionMetaPSupported].(float64)
	if !ok || !(p >= 0 && p <= 1) {
		return nil, ""
	}
	var prov types.Provenance
	model := ""
	if len(rev.Provenance) > 0 && json.Unmarshal(rev.Provenance, &prov) == nil {
		model = strings.TrimSpace(prov.AuthoringModel.Model)
		if provider := strings.TrimSpace(prov.AuthoringModel.Provider); provider != "" && model != "" {
			model = provider + "/" + model
		}
	}
	return &p, model
}

// SupportMemo is one support memo with the time it was written.
type SupportMemo[T any] struct {
	Value T
	At    time.Time
}

// ListCitationChecks lists the computer's citation check memos, newest
// first, at most limit.
func (s *Store) ListCitationChecks(ctx context.Context, ownerID, computerID string, limit int) ([]SupportMemo[types.CitationCheck], error) {
	return listSupportMemos[types.CitationCheck](ctx, s, ogKindCitationCheck, ownerID, computerID, limit)
}

// ListSupportVerdicts lists the computer's support verdict memos, newest
// first, at most limit.
func (s *Store) ListSupportVerdicts(ctx context.Context, ownerID, computerID string, limit int) ([]SupportMemo[types.SupportVerdict], error) {
	return listSupportMemos[types.SupportVerdict](ctx, s, ogKindSupportVerdict, ownerID, computerID, limit)
}

func listSupportMemos[T any](ctx context.Context, s *Store, kind objectgraph.ObjectKind, ownerID, computerID string, limit int) ([]SupportMemo[T], error) {
	ownerID, computerID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID)
	if ownerID == "" || computerID == "" {
		return nil, fmt.Errorf("support memo list requires owner and computer")
	}
	objs, err := s.ogListByOwnerAndBody(ctx, kind, ownerID, nil, limit)
	if err != nil {
		return nil, err
	}
	out := make([]SupportMemo[T], 0, len(objs))
	for _, obj := range objs {
		if obj.Tombstone || strings.TrimSpace(obj.ComputerID) != computerID {
			continue
		}
		value, err := decodeLifecycleObject[T](obj)
		if err != nil {
			return nil, fmt.Errorf("support memo decode %s: %w", obj.CanonicalID, err)
		}
		out = append(out, SupportMemo[T]{Value: value, At: obj.CreatedAt})
	}
	return out, nil
}
