package types

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// The request record (modular architecture MA1.3, plan section 4.2 and 10.1).
//
// A request (cast, ask, escalate, feed, decline) is a staked act: the sender
// states p_success (the chance the request achieves its stated outcome), a
// proposed materiality weight, and how the request settles. The three are
// REQUIRED on write and OPTIONAL on read: a tape record written before the
// slice has none of them and still decodes and replays. The reducer stamps
// the provenance origin and freezes the materiality procedure; a cell can set
// neither.

// ImpactClass is the governance impact scale I0 (none) to I4 (severe).
type ImpactClass string

const (
	ImpactI0 ImpactClass = "I0"
	ImpactI1 ImpactClass = "I1"
	ImpactI2 ImpactClass = "I2"
	ImpactI3 ImpactClass = "I3"
	ImpactI4 ImpactClass = "I4"
)

// Rank returns 0 to 4 for a valid class; "" is I0 (an unstated class carries
// no impact).
func (c ImpactClass) Rank() (int, bool) {
	switch c {
	case "", ImpactI0:
		return 0, true
	case ImpactI1:
		return 1, true
	case ImpactI2:
		return 2, true
	case ImpactI3:
		return 3, true
	case ImpactI4:
		return 4, true
	}
	return 0, false
}

// Origin classes the reducer stamps on provenance. A cell never supplies one.
const (
	// OriginDeskRun: the act came from a desk's own cell.
	OriginDeskRun = "desk_run"
	// OriginTask: the act came from a management task's cell, sent as
	// management (provenance names the task and its run).
	OriginTask = "task"
	// OriginSystem: the platform authored the record.
	OriginSystem = "system"
	// OriginOwner: the owner's own act.
	OriginOwner = "owner"
	// OriginExternal: an outside agent through the product API.
	OriginExternal = "external"
)

// IsValidOrigin reports whether origin is in the closed set (or empty, an
// unstamped record written before MA1.3).
func IsValidOrigin(origin string) bool {
	switch origin {
	case "", OriginDeskRun, OriginTask, OriginSystem, OriginOwner, OriginExternal:
		return true
	}
	return false
}

// OriginClass is the record's origin class: the stamped one, or for a record
// written before the slice, system when a system emitter wrote it and
// "unstamped" otherwise.
func (rec CommitmentRecord) OriginClass() string {
	if o := strings.TrimSpace(rec.Provenance.Origin); o != "" {
		return o
	}
	if rec.EmitterKind == CommitmentEmitterSystem {
		return OriginSystem
	}
	return "unstamped"
}

// Request kinds: the typed request acts.
const (
	RequestCast     = "cast"
	RequestAsk      = "ask"
	RequestEscalate = "escalate"
	RequestFeed     = "feed"
	RequestDecline  = "decline"
	// RequestPrecommit is not a request, but its weight is catalogued and
	// frozen the same way.
	RequestPrecommit = "precommit"
)

// SettlementRecipient: the recipient settles the request (its Report or
// Reply). Any other settlement is a rule kind of the closed vocabulary.
const SettlementRecipient = "recipient"

// RequestFields are the fields a cell supplies with a request. Every field a
// cell may not set is absent here and refused by name at parse.
type RequestFields struct {
	PSuccess    *float64    `json:"p_success"`
	Weight      *float64    `json:"weight"`
	ImpactClass ImpactClass `json:"impact_class,omitempty"`
	Settlement  string      `json:"settlement"`
}

// RequestShapeExample is the shape the refusals name.
const RequestShapeExample = `{"p_success": 0.8, "weight": 0.3, "settlement": "recipient"}`

// requestShapeHint is appended to every refusal.
func requestShapeHint() string {
	return "; pass the request fields as the last argument: " + RequestShapeExample +
		" (p_success in [0,1] is your chance the request achieves its outcome; weight in [0,1] is how much it matters; settlement is \"recipient\" or a rule kind; optional impact_class I0..I4)"
}

// cellForbiddenRequestKeys are fields only the reducer stamps.
var cellForbiddenRequestKeys = []string{"provenance", "origin", "origin_class", "materiality_procedure", "procedure", "agent_id", "model_id", "desk", "run_id", "hash"}

// ParseRequestFields decodes the request fields a cell passed. Unknown keys
// are refused, and the stamped ones (provenance, origin, the materiality
// procedure) are refused by name: a cell cannot set them.
func ParseRequestFields(raw string) (*RequestFields, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return nil, fmt.Errorf("a request needs p_success, weight and settlement%s", requestShapeHint())
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &keys); err != nil {
		return nil, fmt.Errorf("request fields are not a JSON object: %w%s", err, requestShapeHint())
	}
	for _, k := range cellForbiddenRequestKeys {
		if _, ok := keys[k]; ok {
			return nil, fmt.Errorf("request field %q is stamped by the reducer, never supplied by a cell", k)
		}
	}
	var f RequestFields
	dec := json.NewDecoder(bytes.NewReader([]byte(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("request fields: %w%s", err, requestShapeHint())
	}
	return &f, nil
}

func finiteUnit(p float64) bool { return !math.IsNaN(p) && !math.IsInf(p, 0) && p >= 0 && p <= 1 }

// ValidSettlement reports whether s is "recipient" or a rule kind.
func ValidSettlement(s string) bool {
	if s == SettlementRecipient {
		return true
	}
	for _, k := range PrecommitRuleKinds {
		if string(k) == s {
			return true
		}
	}
	return false
}

// EffectiveImpact is the higher of the stated class and the catalogue's floor
// for the kind. A sender cannot lower a kind's class by leaving it out.
func EffectiveImpact(kind string, stated ImpactClass) ImpactClass {
	r, _ := stated.Rank()
	floor := ImpactI0
	if e, ok := MaterialityEntryFor(kind); ok {
		floor = e.ImpactFloor
	}
	fr, _ := floor.Rank()
	if fr > r {
		return floor
	}
	if stated == "" {
		return ImpactI0
	}
	return stated
}

// ValidateImpactWeight is the I1+ rule: a weight of 0 on a class of I1 or
// above is refused, for a request or a precommit. weight nil counts as 0.
func ValidateImpactWeight(kind string, stated ImpactClass, weight *float64) error {
	if _, ok := stated.Rank(); !ok {
		return fmt.Errorf("impact_class %q is not one of I0..I4", stated)
	}
	if weight != nil && !finiteUnit(*weight) {
		return fmt.Errorf("weight %v is outside [0,1]", *weight)
	}
	class := EffectiveImpact(kind, stated)
	if rank, _ := class.Rank(); rank >= 1 && (weight == nil || *weight == 0) {
		return fmt.Errorf("a %s is class %s, so weight 0 is refused: state how much it matters, in (0,1]", kind, class)
	}
	return nil
}

// ValidateRequestFields checks the request fields of one request kind.
func ValidateRequestFields(kind string, f *RequestFields) error {
	if f == nil {
		return fmt.Errorf("%s needs p_success, weight and settlement%s", kind, requestShapeHint())
	}
	if f.PSuccess == nil {
		return fmt.Errorf("%s: p_success is required%s", kind, requestShapeHint())
	}
	if !finiteUnit(*f.PSuccess) {
		return fmt.Errorf("%s: p_success %v is outside [0,1]", kind, *f.PSuccess)
	}
	if f.Weight == nil {
		return fmt.Errorf("%s: weight is required%s", kind, requestShapeHint())
	}
	if strings.TrimSpace(f.Settlement) == "" {
		return fmt.Errorf("%s: settlement is required%s", kind, requestShapeHint())
	}
	if !ValidSettlement(f.Settlement) {
		return fmt.Errorf("%s: settlement %q is neither %q nor a rule kind", kind, f.Settlement, SettlementRecipient)
	}
	if err := ValidateImpactWeight(kind, f.ImpactClass, f.Weight); err != nil {
		return fmt.Errorf("%s: %w", kind, err)
	}
	return nil
}

// ValidateRequestRecord checks the request fields a record carries, for the
// store's write path. Fields are optional here (a record written before the
// slice has none); what is present must be in range.
func ValidateRequestRecord(rec CommitmentRecord) error {
	if rec.PSuccess != nil && !finiteUnit(*rec.PSuccess) {
		return fmt.Errorf("record %s: p_success %v is outside [0,1]", rec.RecordID, *rec.PSuccess)
	}
	if rec.Weight != nil && !finiteUnit(*rec.Weight) {
		return fmt.Errorf("record %s: weight %v is outside [0,1]", rec.RecordID, *rec.Weight)
	}
	if rec.Settlement != "" && !ValidSettlement(rec.Settlement) {
		return fmt.Errorf("record %s: settlement %q is neither %q nor a rule kind", rec.RecordID, rec.Settlement, SettlementRecipient)
	}
	if _, ok := rec.ImpactClass.Rank(); !ok {
		return fmt.Errorf("record %s: impact_class %q is not one of I0..I4", rec.RecordID, rec.ImpactClass)
	}
	if !IsValidOrigin(rec.Provenance.Origin) {
		return fmt.Errorf("record %s: provenance origin %q is not an origin class", rec.RecordID, rec.Provenance.Origin)
	}
	if rec.PSuccess != nil || rec.Weight != nil {
		kind := requestKindOfRecord(rec)
		if kind != "" {
			if err := ValidateImpactWeight(kind, rec.ImpactClass, rec.Weight); err != nil {
				return fmt.Errorf("record %s: %w", rec.RecordID, err)
			}
		}
	}
	return nil
}

func requestKindOfRecord(rec CommitmentRecord) string {
	if rec.Directive != nil {
		switch rec.Directive.Subtype {
		case CommitmentDirectiveCast:
			return RequestCast
		case CommitmentDirectiveEscalate:
			return RequestEscalate
		case CommitmentDirectiveFeed:
			return RequestFeed
		case CommitmentDirectiveDecline:
			return RequestDecline
		}
		return ""
	}
	if rec.Kind == CommitmentKindPrecommit {
		// An Ask and a Precommit share the record kind; the frozen
		// procedure names which the reducer stamped. A precommit with no
		// freeze is a Precommit.
		if rec.MaterialityProcedure != nil && rec.MaterialityProcedure.Kind == RequestAsk {
			return RequestAsk
		}
		return RequestPrecommit
	}
	return ""
}

// MaterialityFreeze is what the reducer freezes with a record at commit: the
// kind's procedure ref from the custodian's catalogue, the weights as stated,
// and the hash over both. Later outcomes never rewrite it (R32, R38).
type MaterialityFreeze struct {
	Kind         string `json:"kind"`
	Custodian    string `json:"custodian"`
	ProcedureRef string `json:"procedure_ref"`
	// NoProcedure says the catalogue has no realized-materiality procedure
	// for the kind: the record gets no importance score, which is not a
	// score of zero.
	NoProcedure bool `json:"no_procedure,omitempty"`
	// Window is the procedure's reading window, frozen with it.
	Window      string      `json:"window,omitempty"`
	Weight      float64     `json:"weight"`
	ImpactClass ImpactClass `json:"impact_class"`
	Hash        string      `json:"hash"`
}

// materialityHashInput is the hashed form: field order is fixed by the struct.
type materialityHashInput struct {
	Kind         string      `json:"kind"`
	Custodian    string      `json:"custodian"`
	ProcedureRef string      `json:"procedure_ref"`
	NoProcedure  bool        `json:"no_procedure"`
	Window       string      `json:"window"`
	Weight       float64     `json:"weight"`
	ImpactClass  ImpactClass `json:"impact_class"`
}

// Digest recomputes the hash over the procedure ref and the frozen weights.
func (m MaterialityFreeze) Digest() string {
	raw, _ := json.Marshal(materialityHashInput{m.Kind, m.Custodian, m.ProcedureRef, m.NoProcedure, m.Window, m.Weight, m.ImpactClass})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// Verify reports whether the stored hash matches the frozen contents.
func (m MaterialityFreeze) Verify() bool { return m.Hash != "" && m.Hash == m.Digest() }

// FreezeMateriality builds the freeze for a kind. ok is false for a kind the
// catalogue does not list.
func FreezeMateriality(kind string, weight float64, stated ImpactClass) (MaterialityFreeze, bool) {
	e, ok := MaterialityEntryFor(kind)
	if !ok {
		return MaterialityFreeze{}, false
	}
	m := MaterialityFreeze{Kind: kind, Custodian: e.Custodian, ProcedureRef: e.ProcedureRef, NoProcedure: e.ProcedureRef == "", Window: e.Window,
		Weight: weight, ImpactClass: EffectiveImpact(kind, stated)}
	m.Hash = m.Digest()
	return m, true
}

// StampRequest writes the request fields, the frozen materiality procedure
// and nothing about provenance onto rec. The caller (the reducer) stamps
// provenance itself.
func (rec *CommitmentRecord) StampRequest(kind string, f *RequestFields) error {
	if err := ValidateRequestFields(kind, f); err != nil {
		return err
	}
	p, w := *f.PSuccess, *f.Weight
	rec.PSuccess, rec.Weight = &p, &w
	rec.Settlement = f.Settlement
	rec.ImpactClass = EffectiveImpact(kind, f.ImpactClass)
	m, ok := FreezeMateriality(kind, w, f.ImpactClass)
	if !ok {
		return fmt.Errorf("%s has no entry in the materiality catalogue", kind)
	}
	rec.MaterialityProcedure = &m
	return nil
}
