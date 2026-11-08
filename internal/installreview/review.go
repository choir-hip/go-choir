// Package installreview is the install review pipeline of plan slice MA7.5
// and the trust draft's section 7 (docs/design/bayesian-trust-2026-11-04.md):
// one pipeline for every install and update whose ceiling is widened, whoever
// wrote the code. Mechanical checks run first; then, when the risk (the
// highest impact class of the requested grants) and the publisher's carried
// record call for it, management judges on its desk (choir.InstallPacket and
// choir.InstallVerdict, docs/design/desk-install-review-2026-11-08.md). The
// desk's answer is a precommitment record (a typed prediction that later
// evidence settles), and the pipeline's verdict is a record sealed by the
// computer's guest-core signer. The install gate honours only a sealed
// verdict for exactly the release, tree, manifest and behaviour surface being
// installed.
//
// Nothing here calls a model: the judgment is management's, made in a cell,
// and the runtime seals it.
package installreview

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/apppackage"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/trust"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const (
	// AgentID is the pipeline's agent on the ledger. A seat's record is
	// AgentID + ":" + seat.
	AgentID = "system:install-review"
	// ReceiptKind and ReceiptIssuer name the guest-core receipt that seals a
	// verdict (receiptsigner allows exactly this pair).
	ReceiptKind   = "InstallReviewVerdict"
	ReceiptIssuer = "choir-autoputer"
	// AddresseePrefix + release id addresses a review's records, so the gate
	// lists them by one indexed field.
	AddresseePrefix = "install-review:"
	// WindowDays is how long a reviewer's predictions run before they settle.
	WindowDays = 30

	// SeatDesk names the one seat in a plan: the desk the registry says holds
	// the install review (the runtime names that desk on the seat record; no
	// desk name is hard-coded here). Its seat record's precommitment is the
	// desk's own prediction.
	SeatDesk = "desk"

	VerdictAccept = "accept"
	VerdictReject = "reject"

	RoleVerdict = "verdict"
	RoleSeat    = "seat"

	// SubjectRefs keys.
	RefRole       = "install_review"
	RefRelease    = "release_id"
	RefPatch      = "patch_sha256"
	RefManifest   = "manifest_sha256"
	RefSurface    = "behaviour_surface"
	RefPackage    = "package"
	RefPublisher  = "publisher"
	RefSeat       = "seat"
	RefSeats      = "seats"
	RefVerdict    = "verdict"
	RefRisk       = "risk"
	RefAdded      = "added_grants"
	RefResolvesBy = "resolves_by"
	RefSeal       = "seal"

	// Gate refusal reasons.
	ReasonRequired = "install_review_required"
	ReasonRejected = "install_review_rejected"
)

// StandardQuestions are the typed predictions every accepting seat states
// (trust draft 7.3). They settle from evidence over WindowDays.
var StandardQuestions = []string{
	"No owner-private data reaches a publisher-reachable sink in the first 30 days after install.",
	"No act outside the declared ceiling in the first 30 days after install.",
}

// Subject is what one review is of: the exact bytes being installed.
type Subject struct {
	ReleaseID        string
	PatchSHA256      string
	ManifestSHA256   string
	BehaviourSurface string
	Package          string
	// Publisher is the release's track: its publisher's owner id.
	Publisher string
	Manifest  apppackage.Manifest
	// PriorCeiling is the ceiling of the release in force when this is an
	// update (HasPrior).
	PriorCeiling []string
	HasPrior     bool
}

// CapabilityKey is one ceiling entry: grant(args), args sorted.
func CapabilityKey(c apppackage.Capability) string {
	args := append([]string(nil), c.Args...)
	sort.Strings(args)
	if len(args) == 0 {
		return c.Grant
	}
	return c.Grant + "(" + strings.Join(args, ",") + ")"
}

// Ceiling is a manifest's ceiling, sorted.
func Ceiling(m apppackage.Manifest) []string {
	out := make([]string, 0, len(m.Capabilities))
	for _, c := range m.Capabilities {
		out = append(out, CapabilityKey(c))
	}
	sort.Strings(out)
	return out
}

// Added is the part of the ceiling the release in force did not hold (the
// whole ceiling on a fresh install).
func (s Subject) Added() []string {
	prior := map[string]bool{}
	for _, c := range s.PriorCeiling {
		prior[c] = true
	}
	var out []string
	for _, c := range Ceiling(s.Manifest) {
		if !prior[c] {
			out = append(out, c)
		}
	}
	return out
}

// Widened reports whether installing s widens the computer's ceiling: the
// release is above the default level or carries a desk (the listing rule of
// apppackage.Manifest.ReviewRequired), or it is an update that adds any
// ceiling entry the release in force did not hold. A default-level fresh
// install is not widened: it takes the mechanical checks only (the table's
// "I0 to I1, thin publisher: one seat" cell waits for a seat that can run,
// see the MA7.5 ledger entry).
func (s Subject) Widened() bool {
	if s.Manifest.ReviewRequired() {
		return true
	}
	return s.HasPrior && len(s.Added()) > 0
}

// Risk is the highest impact class of the requested grants. An unknown grant
// is I4 (fail closed); a desk is at least I2 (its prompt is behaviour).
func Risk(m apppackage.Manifest) types.ImpactClass {
	risk := types.ImpactI0
	for _, c := range m.Capabilities {
		impact, ok := trust.GrantImpact(c.Grant)
		if !ok {
			impact = types.ImpactI4
		}
		risk = maxImpact(risk, impact)
	}
	if len(m.Contributes.Desks) > 0 {
		risk = maxImpact(risk, types.ImpactI2)
	}
	return risk
}

func maxImpact(a, b types.ImpactClass) types.ImpactClass {
	ra, _ := a.Rank()
	rb, _ := b.Rank()
	if rb > ra {
		return b
	}
	return a
}

// Plan is the sized review of a subject.
type Plan struct {
	Widened bool              `json:"widened"`
	Risk    types.ImpactClass `json:"risk"`
	Strong  bool              `json:"publisher_strong"`
	Added   []string          `json:"added,omitempty"`
	// Seats names who judges: management, or nobody (mechanical checks only).
	Seats []string `json:"seats,omitempty"`
	// Refusal is set when no review can admit the release.
	Refusal string `json:"refusal,omitempty"`
}

// NeedsReview reports whether management's verdict is owed.
func (p Plan) NeedsReview() bool { return len(p.Seats) > 0 && p.Refusal == "" }

// PlanFor sizes the review by the risk table of trust draft 7.2, amended by
// docs/design/desk-install-review-2026-11-08.md to one desk verdict at every
// reviewable level:
//
//	risk \ publisher record   strong          thin or new
//	I0 to I1                  mechanical      management's verdict
//	I2                        management's verdict (both)
//	I3                        management's verdict (both)
//	I4                        first-party only: refused for a Library release
//
// The second model family a thin publisher at I3 used to get is gone, a
// deliberate reduction: management's chain already fails over across
// families, and the publisher's trust row still gates the act.
func PlanFor(s Subject, strong bool) Plan {
	p := Plan{Widened: s.Widened(), Risk: Risk(s.Manifest), Strong: strong, Added: s.Added()}
	one := []string{SeatDesk}
	switch p.Risk {
	case types.ImpactI0, types.ImpactI1:
		if !strong {
			p.Seats = one
		}
	case types.ImpactI2, types.ImpactI3:
		p.Seats = one
	default:
		p.Refusal = "an I4 grant is first-party only; a Library release cannot be reviewed into one"
	}
	return p
}

// Flow renders the source-to-sink paths the ceiling opens as sentences (the
// flow check of trust draft 7.1): every owner-private read a grant opens,
// against every outbound sink it declares.
func Flow(m apppackage.Manifest) []string {
	var sources, sinks []string
	for _, c := range m.Capabilities {
		switch c.Grant {
		case "ui.read.kind", "desk.read.kind":
			for _, a := range c.Args {
				sources = append(sources, "the "+a+" kind")
			}
		case "desk.mail.read":
			sources = append(sources, "the owner's mail")
		case "desk.texture.search":
			sources = append(sources, "Texture snippets")
		case "ui.captures.read", "desk.captures.read":
			sources = append(sources, "the owner's captures")
		case "ui.net.egress", "desk.net.egress":
			sinks = append(sinks, c.Args...)
		}
	}
	sort.Strings(sources)
	sort.Strings(sinks)
	var out []string
	for _, src := range uniq(sources) {
		for _, sink := range uniq(sinks) {
			out = append(out, fmt.Sprintf("Owner-private data from %s could reach %s.", src, sink))
		}
	}
	return out
}

func uniq(in []string) []string {
	var out []string
	for i, s := range in {
		if i == 0 || s != in[i-1] {
			out = append(out, s)
		}
	}
	return out
}

// Check is one mechanical check's result.
type Check struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
}

// MechanicalInput is what the mechanical checks read: the manifest bytes the
// build read from the verified tree, and the record's summary of them.
type MechanicalInput struct {
	ManifestRaw    []byte
	RecordSHA256   string
	RecordLevel    string
	RecordDesks    int
	RecordSurface  string
	ModuleBuilt    bool
	SecretsRefused bool
}

// Mechanical runs the checks of trust draft 7.1 the computer can run on the
// bytes it built: the manifest is the record's, parses as a published v2
// manifest, states the level and desks the record summarizes, and every
// grant maps to an impact class. (Source-built and the secret scan ran in
// the build; the screenshot rule is the listing's.) It returns the parsed
// manifest when it parses.
func Mechanical(in MechanicalInput) ([]Check, apppackage.Manifest) {
	var checks []Check
	add := func(name string, ok bool, detail string) {
		checks = append(checks, Check{Name: name, OK: ok, Detail: detail})
	}
	digest := computerevent.DigestBytes(in.ManifestRaw)
	add("manifest_is_the_records", digest == in.RecordSHA256, "the built tree's manifest hashes to "+digest)
	m, err := apppackage.ParseManifest(in.ManifestRaw, apppackage.OriginPublished)
	if err != nil {
		add("manifest_parses", false, err.Error())
		return checks, apppackage.Manifest{}
	}
	add("manifest_parses", true, "")
	add("level_is_the_records", string(m.Level()) == in.RecordLevel && len(m.Contributes.Desks) == in.RecordDesks,
		fmt.Sprintf("manifest level %s with %d desks; the record says %s with %d", m.Level(), len(m.Contributes.Desks), in.RecordLevel, in.RecordDesks))
	var unknown []string
	for _, c := range m.Capabilities {
		if _, ok := trust.GrantImpact(c.Grant); !ok {
			unknown = append(unknown, c.Grant)
		}
	}
	add("grants_have_an_impact_class", len(unknown) == 0, strings.Join(unknown, ", "))
	add("source_built_by_the_platform", in.ModuleBuilt, "")
	return checks, m
}

// Passed reports whether every check passed.
func Passed(checks []Check) bool {
	for _, c := range checks {
		if !c.OK {
			return false
		}
	}
	return len(checks) > 0
}

// Packet is what a seat reviews.
type Packet struct {
	Subject Subject
	Risk    types.ImpactClass
	Added   []string
	Flow    []string
	Checks  []Check
	// ManifestJSON is the manifest; Source is the built module (the code the
	// shell would run), bounded by the caller.
	ManifestJSON string
	Source       string
}

// Prediction is one typed prediction of a seat: the probability that the
// question resolves yes.
type Prediction struct {
	Question string  `json:"question"`
	PYes     float64 `json:"p_yes"`
}

// SeatVerdict is a seat's typed answer.
type SeatVerdict struct {
	// Reason is management's stated reason (required on a rejection).
	Reason      string       `json:"reason,omitempty"`
	Accept      bool         `json:"accept"`
	Blocking    bool         `json:"blocking"`
	Findings    []string     `json:"findings"`
	Predictions []Prediction `json:"predictions"`
}

// SubjectDigest is the digest of what a review is of: the release, tree
// patch, manifest and behaviour surface. A pending install and its owed record
// are keyed on it, and it is computable from the request's bytes before any
// build.
func SubjectDigest(releaseID, patchSHA256, manifestSHA256, behaviourSurface string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{"install-review-subject", releaseID, patchSHA256, manifestSHA256, behaviourSurface}, "\x00")))
	return hex.EncodeToString(sum[:])
}

// Digest is the subject's digest.
func (s Subject) Digest() string {
	return SubjectDigest(s.ReleaseID, s.PatchSHA256, s.ManifestSHA256, s.BehaviourSurface)
}

// recordID derives a review record's id from its parts.
func recordID(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return "install-review-" + hex.EncodeToString(sum[:])[:32]
}

func baseRefs(s Subject, plan Plan) map[string]string {
	return map[string]string{
		RefRelease: s.ReleaseID, RefPatch: s.PatchSHA256, RefManifest: s.ManifestSHA256, RefSurface: s.BehaviourSurface,
		RefPackage: s.Package, RefPublisher: s.Publisher, RefRisk: string(plan.Risk), RefAdded: strings.Join(plan.Added, " "),
	}
}

// SeatHypothesis is the text of a seat record's hypothesis: its verdict, its
// findings and its reason. A replay of a verdict is matched on it, so it is a
// function of the content only (not of the record id, which embeds a time).
func SeatHypothesis(seat string, v SeatVerdict) string {
	verdict := VerdictReject
	if v.Accept {
		verdict = VerdictAccept
	}
	hyp := "install review seat " + seat + ": " + verdict
	if len(v.Findings) > 0 {
		hyp += "; findings: " + strings.Join(v.Findings, "; ")
	}
	if strings.TrimSpace(v.Reason) != "" {
		hyp += "; reason: " + strings.TrimSpace(v.Reason)
	}
	return hyp
}

// SeatRecord is one seat's answer as an open precommitment: the envelope
// question (StandardQuestions[1]) with the seat's probability, and every
// typed prediction, at the review's risk class. It resolves by rule from
// evidence once WindowDays have passed.
func SeatRecord(s Subject, plan Plan, seat, model string, v SeatVerdict, computerID string, at time.Time) types.CommitmentRecord {
	stamp := at.UTC().Format(time.RFC3339Nano)
	refs := baseRefs(s, plan)
	refs[RefRole], refs[RefSeat] = RoleSeat, seat
	refs[RefVerdict] = VerdictReject
	if v.Accept {
		refs[RefVerdict] = VerdictAccept
	}
	refs[RefResolvesBy] = at.UTC().Add(WindowDays * 24 * time.Hour).Format(time.RFC3339)
	var qs []types.TypedQuestion
	pEnvelope := 0.0
	for _, p := range v.Predictions {
		qs = append(qs, types.TypedQuestion{Question: p.Question, Answers: []string{"no", "yes"},
			Probabilities: map[string]float64{"yes": p.PYes, "no": 1 - p.PYes}})
		if p.Question == StandardQuestions[1] {
			pEnvelope = p.PYes
		}
	}
	weight := trust.ImpactNumber(plan.Risk) / trust.ImpactNumber(types.ImpactI4)
	if weight <= 0 {
		weight = 0.01
	}
	hyp := SeatHypothesis(seat, v)
	return types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: recordID("seat", s.ReleaseID, s.PatchSHA256, seat, stamp),
		Kind: types.CommitmentKindPrecommit, Discrepancy: types.DiscrepancyUnresolved,
		Prediction: types.CommitmentPrediction{Hypothesis: hyp, Questions: qs, CommittedAt: stamp},
		Precommit: &types.CommitmentPrecommit{Question: StandardQuestions[1], Distribution: map[string]float64{"yes": pEnvelope, "no": 1 - pEnvelope},
			Resolver: AgentID, Weight: &weight, ImpactClass: plan.Risk},
		Provenance:  types.CommitmentProvenance{AgentID: AgentID + ":" + seat, ModelID: model, CommittedAt: stamp, Origin: types.OriginSystem},
		EmitterKind: types.CommitmentEmitterSystem, ComputerID: computerID,
		Addressee: AddresseePrefix + s.ReleaseID, SubjectRefs: refs,
	}
}

// VerdictRecord is the pipeline's verdict over the mechanical checks and the
// seat records. It is sealed (Seal) before it is written.
func VerdictRecord(s Subject, plan Plan, checks []Check, seats []types.CommitmentRecord, accept bool, computerID string, at time.Time) types.CommitmentRecord {
	return VerdictRecordWithReason(s, plan, checks, seats, accept, "", computerID, at)
}

// VerdictRecordWithReason is VerdictRecord carrying management's reason in
// its hypothesis, which is what a refusal shows the owner.
func VerdictRecordWithReason(s Subject, plan Plan, checks []Check, seats []types.CommitmentRecord, accept bool, reason, computerID string, at time.Time) types.CommitmentRecord {
	stamp := at.UTC().Format(time.RFC3339Nano)
	refs := baseRefs(s, plan)
	refs[RefRole] = RoleVerdict
	refs[RefSeats] = strings.Join(plan.Seats, " ")
	refs[RefVerdict] = VerdictReject
	if accept {
		refs[RefVerdict] = VerdictAccept
	}
	var children []string
	for _, r := range seats {
		children = append(children, r.RecordID)
	}
	var findings []string
	for _, c := range checks {
		if !c.OK {
			findings = append(findings, "mechanical check "+c.Name+" failed: "+c.Detail)
		}
	}
	hyp := "install review verdict: " + refs[RefVerdict]
	if len(findings) > 0 {
		hyp += "; " + strings.Join(findings, "; ")
	}
	if strings.TrimSpace(reason) != "" {
		hyp += "; management: " + strings.TrimSpace(reason)
	}
	return types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: recordID("verdict", s.ReleaseID, s.PatchSHA256, stamp),
		Kind: types.CommitmentKindReport, Discrepancy: types.DiscrepancyUnresolved,
		Prediction:  types.CommitmentPrediction{Hypothesis: hyp, CommittedAt: stamp},
		Provenance:  types.CommitmentProvenance{AgentID: AgentID, CommittedAt: stamp, Origin: types.OriginSystem},
		EmitterKind: types.CommitmentEmitterSystem, ComputerID: computerID,
		Addressee: AddresseePrefix + s.ReleaseID, ChildIDs: children, SubjectRefs: refs,
	}
}

// Signer is the guest-core receipt signer (receiptsigner.Client).
type Signer interface {
	PublicKey(ctx context.Context) (computerevent.SignerRef, ed25519.PublicKey, error)
	SignReceipt(ctx context.Context, kind, issuer string, fields map[string]any, issuedAt time.Time) (computerevent.Receipt, error)
}

// bodyDigest is the sha256 of the record with its seal removed.
func bodyDigest(rec types.CommitmentRecord) (string, error) {
	rec.SubjectRefs = cloneRefs(rec.SubjectRefs)
	delete(rec.SubjectRefs, RefSeal)
	raw, err := json.Marshal(rec)
	if err != nil {
		return "", err
	}
	return computerevent.DigestBytes(raw), nil
}

func cloneRefs(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func sealFields(rec types.CommitmentRecord, computerID, digest string) map[string]any {
	return map[string]any{
		"computer_id": computerID, "record_id": rec.RecordID, "record_digest": digest,
		"release_id": rec.SubjectRefs[RefRelease], "patch_sha256": rec.SubjectRefs[RefPatch],
		"manifest_sha256": rec.SubjectRefs[RefManifest], "behaviour_surface": rec.SubjectRefs[RefSurface],
		"verdict": rec.SubjectRefs[RefVerdict],
	}
}

// Seal signs the verdict record's body with the guest-core signer and
// returns the record carrying the receipt.
func Seal(ctx context.Context, signer Signer, rec types.CommitmentRecord, computerID string, at time.Time) (types.CommitmentRecord, error) {
	if signer == nil {
		return rec, errors.New("install review: no verdict signer")
	}
	digest, err := bodyDigest(rec)
	if err != nil {
		return rec, err
	}
	receipt, err := signer.SignReceipt(ctx, ReceiptKind, ReceiptIssuer, sealFields(rec, computerID, digest), at.UTC())
	if err != nil {
		return rec, fmt.Errorf("install review: seal: %w", err)
	}
	raw, err := json.Marshal(receipt)
	if err != nil {
		return rec, err
	}
	rec.SubjectRefs = cloneRefs(rec.SubjectRefs)
	rec.SubjectRefs[RefSeal] = base64.RawStdEncoding.EncodeToString(raw)
	return rec, nil
}

type pinnedKey ed25519.PublicKey

func (k pinnedKey) ResolveReceiptKey(_, _, _ string, _ uint64, _ time.Time) (ed25519.PublicKey, error) {
	return ed25519.PublicKey(k), nil
}

// VerifySeal checks that rec is a pipeline verdict sealed by pub for
// computerID over exactly its body.
func VerifySeal(rec types.CommitmentRecord, computerID string, pub ed25519.PublicKey) error {
	if rec.Provenance.AgentID != AgentID || rec.SubjectRefs[RefRole] != RoleVerdict {
		return errors.New("not a pipeline verdict")
	}
	raw, err := base64.RawStdEncoding.DecodeString(rec.SubjectRefs[RefSeal])
	if err != nil || len(raw) == 0 {
		return errors.New("no seal")
	}
	var receipt computerevent.Receipt
	if err := json.Unmarshal(raw, &receipt); err != nil {
		return fmt.Errorf("seal: %w", err)
	}
	if receipt.ReceiptKind != ReceiptKind || receipt.Issuer != ReceiptIssuer {
		return errors.New("seal is not an install review receipt")
	}
	if len(pub) != ed25519.PublicKeySize {
		return errors.New("no verdict key")
	}
	if err := receipt.Verify(pinnedKey(pub)); err != nil {
		return err
	}
	digest, err := bodyDigest(rec)
	if err != nil {
		return err
	}
	for name, want := range sealFields(rec, computerID, digest) {
		if fmt.Sprint(receipt.KindFields[name]) != fmt.Sprint(want) {
			return fmt.Errorf("seal field %s does not match the record", name)
		}
	}
	return nil
}

// Decision is the install gate's answer.
type Decision struct {
	Allowed   bool
	Reason    string
	Detail    string
	VerdictID string
	// Ignored lists records that claimed to be a verdict for this release
	// and failed the seal or the digest, with why.
	Ignored []string
}

// Gate decides whether s may install from the review records on the ledger:
// the latest sealed verdict for exactly this release, tree, manifest and
// behaviour surface decides; a record that fails its seal or names another
// digest is ignored and reported.
func Gate(records []types.CommitmentRecord, s Subject, computerID string, pub ed25519.PublicKey) Decision {
	var best *types.CommitmentRecord
	var ignored []string
	for i := range records {
		r := records[i]
		if r.SubjectRefs[RefRole] != RoleVerdict || r.SubjectRefs[RefRelease] != s.ReleaseID {
			continue
		}
		if err := VerifySeal(r, computerID, pub); err != nil {
			ignored = append(ignored, r.RecordID+": "+err.Error())
			continue
		}
		if r.SubjectRefs[RefPatch] != s.PatchSHA256 || r.SubjectRefs[RefManifest] != s.ManifestSHA256 || r.SubjectRefs[RefSurface] != s.BehaviourSurface {
			ignored = append(ignored, r.RecordID+": a verdict for another tree, manifest or surface")
			continue
		}
		if best == nil || r.Provenance.CommittedAt > best.Provenance.CommittedAt ||
			(r.Provenance.CommittedAt == best.Provenance.CommittedAt && r.RecordID > best.RecordID) {
			best = &records[i]
		}
	}
	switch {
	case best == nil:
		return Decision{Reason: ReasonRequired, Detail: "this release widens the computer's ceiling and has no sealed review verdict", Ignored: ignored}
	case best.SubjectRefs[RefVerdict] != VerdictAccept:
		return Decision{Reason: ReasonRejected, Detail: best.Prediction.Hypothesis, VerdictID: best.RecordID, Ignored: ignored}
	}
	return Decision{Allowed: true, VerdictID: best.RecordID, Ignored: ignored}
}
