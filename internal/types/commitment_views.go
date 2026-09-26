package types

import (
	"encoding/json"
	"sort"
	"strings"
	"time"
)

// Commitment derived views (mission R4): read-only projections over the
// append-only commitment_record bodies. Nothing here mutates or writes —
// accrual derives from stored score stamps, and rescoring is a separate
// concern (R5b adjacency, excluded). The views are pure functions so the
// texture evidence seam, the desk pack seam, and the learning-claims gate
// share one derivation.

// Materiality classes for the supervision projection. A record appears in
// exactly one class so the doc renders distinct, countable buckets.
const (
	// MaterialityFalsified: the governing resolution contradicted the claim.
	// Falsified claims stay visible forever — supervision does not airbrush
	// the log.
	MaterialityFalsified = "falsified"
	// MaterialityOverdue: still open past the caller's staleness bound.
	MaterialityOverdue = "overdue"
	// MaterialityTopClaim: still open within the staleness bound.
	MaterialityTopClaim = "top_claim"
)

// learning-claims gate verdicts. The gate flags rather than refuses:
// unbacked claims remain distinguishable on the durable verification event
// instead of blocking verifier flows that predate the ledger (refusal is an
// owner-tightenable follow-up).
const (
	LearningClaimBacked   = "backed"
	LearningClaimUnbacked = "unbacked"
)

// CommitmentAccrual is the derived tally for one agent or one document
// scope: act counts bucketed by effective discrepancy plus the summed
// materiality weight. Derived per read — never stored.
type CommitmentAccrual struct {
	// AgentID is set when accrued per committing agent; ContextRef when
	// accrued per document/channel scope. Exactly one carries a value.
	AgentID    string `json:"agent_id,omitempty"`
	ContextRef string `json:"context_ref,omitempty"`

	Committed    int `json:"committed"`
	Open         int `json:"open"`
	Confirmed    int `json:"confirmed"`
	Qualified    int `json:"qualified"`
	Contradicted int `json:"contradicted"`
	// UnresolvedVerdicts counts acts whose governing resolution declared the
	// outcome never became verifiable — distinct from Open (no resolution).
	UnresolvedVerdicts int `json:"unresolved_verdicts"`
	// Disagreements counts acts where resolutions conflicted or any score
	// flagged disagreement — preserved signal, never collapsed.
	Disagreements int `json:"disagreements"`
	// MaterialityWeight sums per-act materiality weights across the scope.
	MaterialityWeight float64 `json:"materiality_weight"`
}

// CommitmentMaterialityEntry is one projected supervision item: a claim a
// human should see because it is falsified, overdue, or a top-level open
// claim. The doc renders these under Texture's editorial discretion.
type CommitmentMaterialityEntry struct {
	RecordID    string           `json:"record_id"`
	AgentID     string           `json:"agent_id"`
	ContextRef  string           `json:"context_ref,omitempty"`
	Materiality string           `json:"materiality"`
	Claim       string           `json:"claim"`
	Discrepancy DiscrepancyClass `json:"discrepancy"`
	// ObservationExcerpt carries the governing resolution's observation for
	// resolved claims; empty while open.
	ObservationExcerpt string  `json:"observation_excerpt,omitempty"`
	Weight             float64 `json:"weight"`
	// AgeSeconds is how long an open claim has been standing; zero when the
	// act predates stamped commit times (pre-R4 records) — such records are
	// never overdue because their true age is unknowable.
	AgeSeconds int64  `json:"age_seconds,omitempty"`
	ResolvedAt string `json:"resolved_at,omitempty"`
}

// ActingPackItem is one entry of the pack injected into the acting desk's
// cell frame. It deliberately has NO score fields: the epistemic boundary
// (own-scores never re-enter the acting context — reward hacking) holds by
// construction, not by instruction. The desk sees honest feedback —
// observations and discrepancies — and nothing else.
type ActingPackItem struct {
	RecordID    string           `json:"record_id"`
	Claim       string           `json:"claim"`
	Discrepancy DiscrepancyClass `json:"discrepancy"`
	// ObservationExcerpt is the resolving observation excerpt for resolved
	// acts; empty while open.
	ObservationExcerpt string `json:"observation_excerpt,omitempty"`
	ResolvedAt         string `json:"resolved_at,omitempty"`
	Disagreement       bool   `json:"disagreement,omitempty"`
}

// ActingPack is the acting desk's score-free commitment context, injected
// into the cell frame (SessionFrame.Pack) and read inside the cell through
// choir.Pack(). Built for the desk's own committed and addressed acts.
type ActingPack struct {
	AgentID string           `json:"agent_id"`
	Items   []ActingPackItem `json:"items"`
}

// SupervisionPackItem is the supervising surface's full per-act view: the
// acting pack fields plus the score stamps the acting desk never sees.
type SupervisionPackItem struct {
	ActingPackItem
	Scores      []CommitmentScore `json:"scores"`
	Resolutions []string          `json:"resolutions,omitempty"`
}

// SupervisionPack is the score-carrying pack for supervision surfaces
// (texture evidence feed, learning-claims inspection). Never injected into
// an acting desk's frame.
type SupervisionPack struct {
	Items []SupervisionPackItem `json:"items"`
}

// resolutionTargets returns the record id a resolve record governs:
// ParentID carries the StagedIntent TargetRef; RelatedIDs preserves the
// same link. Either is authoritative; both are set on live records.
func resolutionTarget(rec CommitmentRecord) string {
	if id := strings.TrimSpace(rec.ParentID); id != "" {
		return id
	}
	for _, id := range rec.RelatedIDs {
		if id = strings.TrimSpace(id); id != "" {
			return id
		}
	}
	return ""
}

// isResolutionRecord identifies records committed by IntentResolve: they
// carry a governing link to the act they resolve and a ResolvedAt stamp.
func isResolutionRecord(rec CommitmentRecord) bool {
	return resolutionTarget(rec) != "" && strings.TrimSpace(rec.Provenance.ResolvedAt) != ""
}

// commitmentWeight sums the frozen materiality weights: typed-question
// weights (likelihood x impact x relevance) plus predicted-consequence
// weights. Omission is penalized by weight — a heavyweight claim outranks
// a lightweight one regardless of direction.
func commitmentWeight(rec CommitmentRecord) float64 {
	w := 0.0
	for _, q := range rec.Prediction.Questions {
		w += q.Weight
	}
	for _, c := range rec.Consequences {
		w += c.Weight
	}
	return w
}

// commitmentClaimText is the claim as recorded: the thin report's claim or
// the packet-bodied report's summary when the preserved packet carries one.
func commitmentClaimText(rec CommitmentRecord) string {
	h := strings.TrimSpace(rec.Prediction.Hypothesis)
	if strings.HasPrefix(h, "{") {
		var packet CoagentSourcePacketPayload
		if err := json.Unmarshal([]byte(h), &packet); err == nil {
			if s := strings.TrimSpace(packet.Summary); s != "" {
				return s
			}
		}
	}
	return h
}

func parseCommitmentTime(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t, true
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, true
	}
	return time.Time{}, false
}

// resolutionChronological orders governing candidates deterministically:
// by ResolvedAt where stamped, then record id so equal (or empty) stamps
// still pick one winner per replay.
func resolutionChronological(a, b CommitmentRecord) bool {
	at, aok := parseCommitmentTime(a.Provenance.ResolvedAt)
	bt, bok := parseCommitmentTime(b.Provenance.ResolvedAt)
	if aok && bok && !at.Equal(bt) {
		return at.Before(bt)
	}
	if aok != bok {
		return aok
	}
	return a.RecordID < b.RecordID
}

// ResolvedCommitment is one ledger act joined with the resolution records
// that govern it. The act itself stays unresolved — resolution is a linked
// append, not a rewrite — so the derived view is where verdicts surface.
type ResolvedCommitment struct {
	Act          CommitmentRecord
	Resolutions  []CommitmentRecord
	Discrepancy  DiscrepancyClass
	Observation  *CommitmentObservation
	Disagreement bool
	Weight       float64
	Age          time.Duration
	HasAge       bool
}

// ResolveCommitments joins acts with their linked resolutions in one
// deterministic pass: resolutions sort chronologically, the latest governs,
// and conflicting verdicts or flagged scores surface as Disagreement —
// preserved, never collapsed.
func ResolveCommitments(records []CommitmentRecord, asOf time.Time) []ResolvedCommitment {
	resolutions := map[string][]CommitmentRecord{}
	var acts []CommitmentRecord
	for _, rec := range records {
		if isResolutionRecord(rec) {
			target := resolutionTarget(rec)
			resolutions[target] = append(resolutions[target], rec)
		} else {
			acts = append(acts, rec)
		}
	}
	out := make([]ResolvedCommitment, 0, len(acts))
	for _, act := range acts {
		rc := ResolvedCommitment{Act: act, Weight: commitmentWeight(act)}
		if t, ok := parseCommitmentTime(act.Provenance.CommittedAt); ok {
			rc.Age = asOf.Sub(t)
			rc.HasAge = true
		}
		res := resolutions[act.RecordID]
		sort.SliceStable(res, func(i, j int) bool { return resolutionChronological(res[i], res[j]) })
		rc.Resolutions = res
		if len(res) == 0 {
			rc.Discrepancy = DiscrepancyUnresolved
		} else {
			governing := res[len(res)-1]
			rc.Discrepancy = governing.Discrepancy
			obs := governing.Observation
			rc.Observation = &obs
			verdicts := map[DiscrepancyClass]bool{}
			for _, r := range res {
				verdicts[r.Discrepancy] = true
				for _, sc := range r.Scores {
					if sc.Disagreement {
						rc.Disagreement = true
					}
				}
			}
			if len(verdicts) > 1 {
				rc.Disagreement = true
			}
		}
		out = append(out, rc)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Act.RecordID != out[j].Act.RecordID {
			return out[i].Act.RecordID < out[j].Act.RecordID
		}
		return out[i].Act.Provenance.AgentID < out[j].Act.Provenance.AgentID
	})
	return out
}

// accrue folds one joined act into a bucket. Key selection is the caller's:
// the same derivation serves per-agent and per-doc accrual.
func accrue(view map[string]*CommitmentAccrual, key string, setKey func(*CommitmentAccrual), rc ResolvedCommitment) {
	acc := view[key]
	if acc == nil {
		acc = &CommitmentAccrual{}
		setKey(acc)
		view[key] = acc
	}
	acc.Committed++
	acc.MaterialityWeight += rc.Weight
	if rc.Disagreement {
		acc.Disagreements++
	}
	switch rc.Discrepancy {
	case DiscrepancyConfirmed:
		acc.Confirmed++
	case DiscrepancyQualified:
		acc.Qualified++
	case DiscrepancyContradicted:
		acc.Contradicted++
	default:
		if len(rc.Resolutions) == 0 {
			acc.Open++
		} else {
			acc.UnresolvedVerdicts++
		}
	}
}

func sortedAccruals(view map[string]*CommitmentAccrual) []CommitmentAccrual {
	keys := make([]string, 0, len(view))
	for k := range view {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]CommitmentAccrual, 0, len(keys))
	for _, k := range keys {
		out = append(out, *view[k])
	}
	return out
}

// AccrualByAgent derives the per-agent tally over the full record set.
// Acts by an agent with no committed acts do not appear — the view is
// derived, so absent keys mean zero, not missing rows.
func AccrualByAgent(records []CommitmentRecord, asOf time.Time) []CommitmentAccrual {
	view := map[string]*CommitmentAccrual{}
	for _, rc := range ResolveCommitments(records, asOf) {
		key := rc.Act.Provenance.AgentID
		accrue(view, key, func(a *CommitmentAccrual) { a.AgentID = key }, rc)
	}
	return sortedAccruals(view)
}

// AccrualByDoc derives the per-context (document/channel) tally — the same
// derivation grouped on Provenance.ContextRef.
func AccrualByDoc(records []CommitmentRecord, asOf time.Time) []CommitmentAccrual {
	view := map[string]*CommitmentAccrual{}
	for _, rc := range ResolveCommitments(records, asOf) {
		key := rc.Act.Provenance.ContextRef
		accrue(view, key, func(a *CommitmentAccrual) { a.ContextRef = key }, rc)
	}
	return sortedAccruals(view)
}

// ProjectMateriality renders the supervision projection: falsified claims
// first (never dropped), then overdue, then top-level open claims — each
// record in exactly one class. overdueAfter bounds staleness; acts without
// a committed timestamp (pre-R4 bodies) are open but never overdue.
func ProjectMateriality(records []CommitmentRecord, asOf time.Time, overdueAfter time.Duration) []CommitmentMaterialityEntry {
	var out []CommitmentMaterialityEntry
	for _, rc := range ResolveCommitments(records, asOf) {
		entry := CommitmentMaterialityEntry{
			RecordID:    rc.Act.RecordID,
			AgentID:     rc.Act.Provenance.AgentID,
			ContextRef:  rc.Act.Provenance.ContextRef,
			Claim:       commitmentClaimText(rc.Act),
			Discrepancy: rc.Discrepancy,
			Weight:      rc.Weight,
		}
		switch {
		case rc.Discrepancy == DiscrepancyContradicted:
			entry.Materiality = MaterialityFalsified
			if rc.Observation != nil {
				entry.ObservationExcerpt = rc.Observation.Excerpt
			}
			entry.ResolvedAt = rc.Resolutions[len(rc.Resolutions)-1].Provenance.ResolvedAt
		case len(rc.Resolutions) > 0:
			// Resolved with a non-falsified verdict — settled evidence, not
			// a standing claim; the doc's materiality view excludes it.
			continue
		case rc.HasAge && overdueAfter > 0 && rc.Age >= overdueAfter:
			entry.Materiality = MaterialityOverdue
			entry.AgeSeconds = int64(rc.Age / time.Second)
		default:
			entry.Materiality = MaterialityTopClaim
			if rc.HasAge {
				entry.AgeSeconds = int64(rc.Age / time.Second)
			}
		}
		out = append(out, entry)
	}
	sort.SliceStable(out, func(i, j int) bool {
		oi, oj := materialityOrder(out[i].Materiality), materialityOrder(out[j].Materiality)
		if oi != oj {
			return oi < oj
		}
		if out[i].Weight != out[j].Weight {
			return out[i].Weight > out[j].Weight
		}
		return out[i].RecordID < out[j].RecordID
	})
	return out
}

func materialityOrder(class string) int {
	switch class {
	case MaterialityFalsified:
		return 0
	case MaterialityOverdue:
		return 1
	default:
		return 2
	}
}

// packEligible selects the acts a desk's pack carries: its own committed
// acts and acts addressed to it (the ledger-side target-desk binding).
func packEligible(rec CommitmentRecord, agentID string) bool {
	if agentID == "" {
		return false
	}
	return rec.Provenance.AgentID == agentID || rec.Addressee == agentID
}

func packOrder(items []ActingPackItem) {
	// Weight is intentionally absent from the acting item — order by
	// discrepancy class (unresolved last? no: falsified first for honesty)
	// then record id. Resolved-critical items lead.
	sort.SliceStable(items, func(i, j int) bool {
		fi, fj := items[i].Discrepancy == DiscrepancyContradicted, items[j].Discrepancy == DiscrepancyContradicted
		if fi != fj {
			return fi
		}
		return items[i].RecordID < items[j].RecordID
	})
}

// BuildActingPack derives the score-free pack for one desk: own committed
// acts plus acts addressed to it, joined with their resolutions. The type
// carries no score fields by construction.
func BuildActingPack(records []CommitmentRecord, agentID string, limit int) ActingPack {
	pack := ActingPack{AgentID: agentID, Items: []ActingPackItem{}}
	if agentID == "" {
		return pack
	}
	for _, rc := range ResolveCommitments(records, time.Now().UTC()) {
		if !packEligible(rc.Act, agentID) {
			continue
		}
		item := ActingPackItem{
			RecordID:     rc.Act.RecordID,
			Claim:        commitmentClaimText(rc.Act),
			Discrepancy:  rc.Discrepancy,
			Disagreement: rc.Disagreement,
		}
		if rc.Observation != nil {
			item.ObservationExcerpt = rc.Observation.Excerpt
		}
		if len(rc.Resolutions) > 0 {
			item.ResolvedAt = rc.Resolutions[len(rc.Resolutions)-1].Provenance.ResolvedAt
		}
		pack.Items = append(pack.Items, item)
	}
	packOrder(pack.Items)
	if limit > 0 && len(pack.Items) > limit {
		pack.Items = pack.Items[:limit]
	}
	return pack
}

// BuildSupervisionPack derives the score-carrying pack for a supervision
// surface: same selection and join as the acting pack, with the score
// stamps and resolution links included. This is the pack that may feed the
// texture evidence surface — never a desk frame.
func BuildSupervisionPack(records []CommitmentRecord, agentID string, limit int) SupervisionPack {
	pack := SupervisionPack{Items: []SupervisionPackItem{}}
	for _, rc := range ResolveCommitments(records, time.Now().UTC()) {
		if agentID != "" && !packEligible(rc.Act, agentID) {
			continue
		}
		item := SupervisionPackItem{
			ActingPackItem: ActingPackItem{
				RecordID:     rc.Act.RecordID,
				Claim:        commitmentClaimText(rc.Act),
				Discrepancy:  rc.Discrepancy,
				Disagreement: rc.Disagreement,
			},
		}
		if rc.Observation != nil {
			item.ObservationExcerpt = rc.Observation.Excerpt
		}
		if len(rc.Resolutions) > 0 {
			item.ResolvedAt = rc.Resolutions[len(rc.Resolutions)-1].Provenance.ResolvedAt
		}
		for _, r := range rc.Resolutions {
			item.Scores = append(item.Scores, r.Scores...)
			item.Resolutions = append(item.Resolutions, r.RecordID)
		}
		pack.Items = append(pack.Items, item)
	}
	sort.SliceStable(pack.Items, func(i, j int) bool {
		fi, fj := pack.Items[i].Discrepancy == DiscrepancyContradicted, pack.Items[j].Discrepancy == DiscrepancyContradicted
		if fi != fj {
			return fi
		}
		return pack.Items[i].RecordID < pack.Items[j].RecordID
	})
	if limit > 0 && len(pack.Items) > limit {
		pack.Items = pack.Items[:limit]
	}
	return pack
}

// commitmentRecordIDCandidates extracts the record-id candidates a
// verifier ref can name: the raw ref, any scheme-prefixed tail, and the
// tail after the object-kind segment of a canonical id.
func commitmentRecordIDCandidates(ref string) []string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil
	}
	out := []string{ref}
	if i := strings.Index(ref, "://"); i >= 0 {
		out = append(out, ref[i+3:])
	}
	if i := strings.Index(ref, "commitment_record:"); i >= 0 {
		out = append(out, ref[i+len("commitment_record:"):])
	}
	return out
}

// GateLearningClaim binds a self-development outcome claim to scored
// commitment evidence: "backed" when at least one cited verifier ref
// resolves to an existing commitment record carrying a verdict or score,
// else "unbacked". The caller supplies the scored check (ledger lookup);
// the gate stays pure so the verdict is replayable from the record alone.
func GateLearningClaim(refs []string, isScored func(recordID string) bool) string {
	if isScored == nil {
		return LearningClaimUnbacked
	}
	for _, ref := range refs {
		for _, candidate := range commitmentRecordIDCandidates(ref) {
			if isScored(candidate) {
				return LearningClaimBacked
			}
		}
	}
	return LearningClaimUnbacked
}
