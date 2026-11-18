package calibration

import (
	"context"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// Autoradio's forecasts live in the calibration claims store as claims of one
// reserved agent (learning-engine design 3.3, 4.3, 12). The generic claims
// routes refuse that namespace; the server writes it through the constructor
// and the settle method below, and the run sheet's op path
// (internal/agentcore/api_autoradio_settle.go) is the settle method's only
// caller.

const (
	// AutoradioAgent is the one claimant of the namespace.
	AutoradioAgent = "autoradio-dj"
	// AutoradioNamespace is the prefix of every reserved agent slug and tag.
	AutoradioNamespace = "autoradio"
	// AutoradioHeardRule is the one rule of the closed registry, bound to
	// AutoradioAgent.
	AutoradioHeardRule = "autoradio_heard"
	// AutoradioDesk is the desk subject ref of every Autoradio record.
	AutoradioDesk = "app:autoradio"
	// AutoradioLapse is how long after a stretch's start its forecasts fall
	// overdue (design 12.7).
	AutoradioLapse = 12 * time.Hour

	// The cause classes of design 3.1 and 12.1.
	CauseInstruction = "instruction"
	CauseEdition     = "edition"
	CauseImportance  = "importance"
	CauseLesson      = "lesson"
	CauseExplore     = "explore"
	// CauseTrying is a forecast a "What Autoradio is trying" line caused
	// (ref try:T<n>); a lesson line's is CauseLesson, a listener's own line
	// (mine:<8 hex>) or a tap (tap:<entry id>) is CauseInstruction.
	CauseTrying = "trying"

	maxCriterionRef = maxTextLen - len("caused_by=")
	maxTitleBytes   = 200
)

var (
	stretchIDRe    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,31}$`)
	eventIDRe      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`)
	segmentSuffix  = regexp.MustCompile(`^\.s[0-9]{1,6}$`)
	kindRe         = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)
	lessonIDRe     = regexp.MustCompile(`^L[0-9]{1,9}$`)
	tryIDRe        = regexp.MustCompile(`^T[0-9]{1,9}$`)
	sha256HexRe    = regexp.MustCompile(`^[0-9a-f]{64}$`)
	instructionRef = regexp.MustCompile(`^(instruction:(turn/[A-Za-z0-9][A-Za-z0-9_.-]{0,63}|note/[a-z0-9]{1,32})|mine:[0-9a-f]{8}|tap:[A-Za-z0-9][A-Za-z0-9_.-]{0,63})$`)
	editionRef     = regexp.MustCompile(`^edition:P[1-9][0-9]{0,5}$`)

	// autoradioRules is the closed registry of rule names the reserved settle
	// method accepts, each bound to the one claimant agent it may settle.
	autoradioRules = map[string]string{AutoradioHeardRule: AutoradioAgent}
)

// IsReservedAgent reports whether an agent name falls in the reserved
// namespace: its cleaned, slugged form begins "autoradio". Slug lower-cases,
// trims and maps every other character to '-', so a padded, cased or
// hyphen-led variant (" Autoradio-DJ", "-autoradio-dj") is reserved
// while "my-autoradio" is not. Slug trims only hyphens, so "_autoradio" is not.
func IsReservedAgent(name string) bool {
	return strings.HasPrefix(Slug(name), AutoradioNamespace)
}

// IsReservedTag reports whether a tag begins "autoradio" in any case, once
// trimmed.
func IsReservedTag(tag string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(tag)), AutoradioNamespace)
}

// IsReservedRecord reports whether a calibration record (a claim or a
// settlement) belongs to the namespace, judged on the STORED claimant agent
// and, for a claim, its stored tags: never on a request or on the record id.
func IsReservedRecord(rec *types.CommitmentRecord) bool {
	if rec == nil || rec.Calibration == nil {
		return false
	}
	if IsReservedAgent(rec.Calibration.Agent) {
		return true
	}
	if c := rec.Calibration.Claim; c != nil {
		for _, tag := range c.Tags {
			if IsReservedTag(tag) {
				return true
			}
		}
	}
	return false
}

// IsReservedClaim is IsReservedRecord for a record that is a claim.
func IsReservedClaim(rec *types.CommitmentRecord) bool {
	return IsClaim(rec) && IsReservedRecord(rec)
}

// AutoradioForecastInput is one content segment's forecast: what the handler
// knows when it appends the segment's claim (design 3.3).
type AutoradioForecastInput struct {
	StretchID string
	SegmentID string
	// Kind is the segment kind; it becomes the claim's project, so the
	// summary's group_by=project stratifies by kind.
	Kind  string
	Title string
	// Model is the model that wrote the stretch.
	Model string
	// P is p_heard.
	P float64
	// Cause is the cause class of design 12.1; CausedBy is the model's full
	// ref (empty or the class word for the classes without a ref); LessonID
	// is the lesson's id when, and only when, the cause is a lesson.
	Cause    string
	CausedBy string
	LessonID string
	// TakeID is the take a take segment leads in: required when, and only
	// when, Kind is "take", and tagged take:<id> (design 11.5).
	TakeID     string
	TextSHA256 string // 64 lower-case hex digits
	Blocks     int
	// StretchStart is the stretch start record's time; the deadline derives
	// from it, so a replay is byte-identical.
	StretchStart time.Time
}

func checkAutoradioInput(in AutoradioForecastInput) (criterionRef string, err error) {
	if !stretchIDRe.MatchString(in.StretchID) {
		return "", invalid("stretch id %q is outside the grammar of design 12.5", in.StretchID)
	}
	suffix, ok := strings.CutPrefix(in.SegmentID, in.StretchID)
	if !ok || !segmentSuffix.MatchString(suffix) || len(in.SegmentID) > 40 {
		return "", invalid("segment id %q is not the stretch id, \".s\" and an index within 40 bytes", in.SegmentID)
	}
	if !kindRe.MatchString(in.Kind) {
		return "", invalid("segment kind %q is outside the grammar", in.Kind)
	}
	// The title is trimmed (design 3.2 does not drop a padded one-line title);
	// an empty one is refused, and the programme validators drop it first.
	if t := strings.TrimSpace(in.Title); t == "" || len(t) > maxTitleBytes {
		return "", invalid("title must be a non-empty line of at most %d bytes", maxTitleBytes)
	}
	for _, r := range strings.TrimSpace(in.Title) {
		if unicode.IsControl(r) {
			return "", invalid("title contains a control character")
		}
	}
	if (in.Kind == AutoradioKindTake) != (in.TakeID != "") || (in.TakeID != "" && !types.TakeIDRe.MatchString(in.TakeID)) {
		return "", invalid("a take forecast names its take (take-<16 hex>), and no other kind does")
	}
	if !sha256HexRe.MatchString(in.TextSHA256) {
		return "", invalid("text hash must be 64 lower-case hex digits")
	}
	if in.Blocks < 1 {
		return "", invalid("blocks must be at least 1")
	}
	if in.StretchStart.IsZero() {
		return "", invalid("the stretch start time is required: the deadline derives from it")
	}
	if math.IsNaN(in.P) || math.IsInf(in.P, 0) || in.P < 0 || in.P > 1 {
		return "", invalid("p_heard must be between 0 and 1")
	}
	ref := in.CausedBy
	switch in.Cause {
	case CauseInstruction:
		if in.LessonID != "" || !instructionRef.MatchString(ref) {
			return "", invalid("an instruction cause needs a ref instruction:turn/<id> or instruction:note/<id>")
		}
	case CauseEdition:
		if in.LessonID != "" || !editionRef.MatchString(ref) {
			return "", invalid("an edition cause needs a ref edition:P<n>")
		}
	case CauseTrying:
		if !tryIDRe.MatchString(in.LessonID) || ref != "try:"+in.LessonID {
			return "", invalid("a trying cause needs line id T<n> and the ref try:<id>")
		}
	case CauseLesson:
		if !lessonIDRe.MatchString(in.LessonID) || ref != "lesson:"+in.LessonID {
			return "", invalid("a lesson cause needs lesson id L<n> and the ref lesson:<id>")
		}
	case CauseImportance, CauseExplore:
		if in.LessonID != "" || (ref != "" && ref != in.Cause) {
			return "", invalid("a %s cause has no ref", in.Cause)
		}
		ref = in.Cause
	default:
		return "", invalid("cause class %q is not instruction, edition, importance, lesson or explore", in.Cause)
	}
	if len(ref) > maxCriterionRef {
		return "", invalid("caused_by ref is longer than %d bytes", maxCriterionRef)
	}
	return ref, nil
}

// NewAutoradioForecast builds the claim of one content segment (design 3.3,
// table 12.1): agent autoradio-dj, type forecast, probability p_heard, the
// full cause ref in the criterion, the tags of 12.1, the deadline a lapse
// after the stretch's start, the desk and record_kind subject refs and the
// owner-private label of 12.2. It calls NewClaimRecord, so the claim's own
// bounds apply. The HTTP claim route does not reach it.
func NewAutoradioForecast(in AutoradioForecastInput, now time.Time) (types.CommitmentRecord, error) {
	ref, err := checkAutoradioInput(in)
	if err != nil {
		return types.CommitmentRecord{}, err
	}
	tags := []string{
		AutoradioNamespace, "stretch:" + in.StretchID, "seg:" + in.SegmentID, "kind:" + in.Kind, "cause:" + in.Cause,
	}
	if in.Cause == CauseLesson {
		tags = append(tags, "lesson:"+in.LessonID)
	}
	if in.TakeID != "" {
		tags = append(tags, "take:"+in.TakeID)
	}
	// A forecast a notebook line caused names the line (design 4: line:<ref>,
	// within 64 bytes), so the line's count is read from the ledger.
	if strings.HasPrefix(ref, "lesson:") || strings.HasPrefix(ref, "try:") || strings.HasPrefix(ref, "mine:") || strings.HasPrefix(ref, "tap:") {
		if t := "line:" + ref; len(t) <= 64 {
			tags = append(tags, t)
		}
	}
	tags = append(tags, "sha:"+in.TextSHA256[:32], fmt.Sprintf("blocks:%d", in.Blocks))
	p := in.P
	rec, err := NewClaimRecord(ClaimRequest{
		Agent:          AgentIdentity{Name: AutoradioAgent, Model: in.Model, SessionID: in.StretchID},
		Claim:          fmt.Sprintf("The listener hears segment %s (%s: %s) through.", in.SegmentID, in.Kind, strings.TrimSpace(in.Title)),
		Type:           types.CalibrationClaimForecast,
		P:              &p,
		Criterion:      "caused_by=" + ref,
		Deadline:       in.StretchStart.Add(AutoradioLapse).UTC().Format(time.RFC3339),
		Project:        in.Kind,
		Tags:           tags,
		IdempotencyKey: in.StretchID + "/" + in.SegmentID,
		SourceRef:      "autoradio:" + in.StretchID + "/" + in.SegmentID,
	}, now, "")
	if err != nil {
		return rec, err
	}
	return stampAutoradio(rec, "autoradio.forecast"), nil
}

// stampAutoradio sets the subject refs and the owner-private label of an
// Autoradio record kind (design 12.2).
func stampAutoradio(rec types.CommitmentRecord, kind string) types.CommitmentRecord {
	label, _ := types.AutoradioKindLabel(kind)
	rec.SubjectRefs = map[string]string{"desk": AutoradioDesk, "record_kind": kind}
	rec.Provenance.Labels = types.NewLabelSet(label).Strings()
	return rec
}

// IsAutoradioSettlement reports whether rec is an autoradio.settlement record
// the Autoradio desk stamped. Callers outside this file ask here instead of
// naming the kind (the reserved-writer guard keeps the literal in one place).
func IsAutoradioSettlement(rec types.CommitmentRecord) bool {
	return rec.SubjectRefs["desk"] == AutoradioDesk && rec.SubjectRefs["record_kind"] == "autoradio.settlement"
}

var errNotServerForecast = errors.New("not a forecast claim the Autoradio server wrote")

// checkServerForecast accepts only a claim whose STORED agent is exactly
// autoradio-dj, whose kind is the forecast record, and which carries the
// server-written tags of design 12.1 (a hand-written look-alike does not).
func checkServerForecast(claim types.CommitmentRecord, agent string) error {
	if !IsClaim(&claim) || claim.Calibration.Agent != agent || claim.Calibration.Claim.Type != types.CalibrationClaimForecast {
		return errNotServerForecast
	}
	if claim.SubjectRefs["desk"] != AutoradioDesk || claim.SubjectRefs["record_kind"] != "autoradio.forecast" {
		return errNotServerForecast
	}
	has := map[string]bool{}
	for _, tag := range claim.Calibration.Claim.Tags {
		if tag == AutoradioNamespace {
			has[tag] = true
			continue
		}
		if prefix, rest, ok := strings.Cut(tag, ":"); ok && rest != "" {
			has[prefix+":"] = true
		}
	}
	for _, need := range []string{AutoradioNamespace, "stretch:", "seg:", "kind:", "cause:", "sha:", "blocks:"} {
		if !has[need] {
			return fmt.Errorf("%w: tag %s is missing", errNotServerForecast, need)
		}
	}
	return nil
}

// NewAutoradioSettlementRecord builds the settlement of one forecast by a
// rule of the closed registry. The rule names the heard rule only; heard is
// the outcome (true: the final block began and the segment finished, or was
// skipped with it begun); airEventID is the air event the heard route wrote
// and the settlement cites as its evidence. The record is the generic
// settlement (method rule) with the desk, record_kind and label of 12.2.
func NewAutoradioSettlementRecord(claim types.CommitmentRecord, rule string, heard bool, airEventID string, now time.Time) (types.CommitmentRecord, error) {
	agent, ok := autoradioRules[rule]
	if !ok {
		return types.CommitmentRecord{}, invalid("rule %q is not in the closed registry of Autoradio rules", rule)
	}
	if err := checkServerForecast(claim, agent); err != nil {
		return types.CommitmentRecord{}, invalid("%v", err)
	}
	if !eventIDRe.MatchString(airEventID) {
		return types.CommitmentRecord{}, invalid("air event id %q is outside the grammar of design 12.5", airEventID)
	}
	outcome := types.CalibrationOutcomeFalse
	if heard {
		outcome = types.CalibrationOutcomeTrue
	}
	rec, err := NewSettlementRecord(claim, SettlementRequest{
		Outcome:  outcome,
		Evidence: []string{"air:" + airEventID},
		Scorer:   ScorerIdentity{Method: types.CalibrationScorerRule, Name: rule},
	}, now)
	if err != nil {
		return rec, err
	}
	return stampAutoradio(rec, "autoradio.settlement"), nil
}

// SettleLedger is the slice of the guest store the settle method uses.
type SettleLedger interface {
	AppendCommitmentRecord(ctx context.Context, ownerID, computerID string, rec types.CommitmentRecord) (string, error)
	GetCommitmentRecord(ctx context.Context, ownerID, computerID, recordID string) (*types.CommitmentRecord, error)
}

// SettleResult is the stored settlement and whether this call wrote it.
type SettleResult struct {
	Record types.CommitmentRecord
	// Won is true only when this call's append is the stored record. A replay,
	// a lost race and a later disagreeing event all return the stored
	// settlement with Won false: a settlement is evidence and is never
	// rewritten, and the caller records a disagreement as a conflict.
	Won bool
}

// SettleAutoradio is the reserved settle method: the only way a rule
// settlement of the Autoradio namespace is written. Its one caller is the
// run sheet's op path, acting on an op the owner's authenticated session posted with by: phone (a test holds
// that). It reads the claim by id, builds the
// settlement (refusing a rule outside the closed registry, a claim that is
// not a server-written forecast of autoradio-dj, an outcome that is not
// heard or not heard, or evidence that is not one air event), appends it
// conditionally on not existing and re-reads it: the store answers success
// on a conflict, so only the re-read says who won (design 4.2).
func SettleAutoradio(ctx context.Context, ledger SettleLedger, ownerID, computerID, claimID, rule string, heard bool, airEventID string, now time.Time) (SettleResult, error) {
	claim, err := ledger.GetCommitmentRecord(ctx, ownerID, computerID, strings.TrimSpace(claimID))
	if err != nil {
		return SettleResult{}, err
	}
	if claim == nil {
		return SettleResult{}, invalid("no calibration claim %s", claimID)
	}
	rec, err := NewAutoradioSettlementRecord(*claim, rule, heard, airEventID, now)
	if err != nil {
		return SettleResult{}, err
	}
	rec.ComputerID = computerID
	existing, err := ledger.GetCommitmentRecord(ctx, ownerID, computerID, rec.RecordID)
	if err != nil {
		return SettleResult{}, err
	}
	if existing != nil {
		return SettleResult{Record: *existing}, nil
	}
	if _, err := ledger.AppendCommitmentRecord(ctx, ownerID, computerID, rec); err != nil {
		return SettleResult{}, err
	}
	stored, err := ledger.GetCommitmentRecord(ctx, ownerID, computerID, rec.RecordID)
	if err != nil {
		return SettleResult{}, err
	}
	if stored == nil {
		return SettleResult{}, errors.New("settlement not readable after append")
	}
	// SameSettlement compares the outcome and the evidence (the air ref), and the
	// commit stamp tells our append from a concurrent identical one.
	won := stored.Provenance.ResolvedAt == rec.Provenance.ResolvedAt && SameSettlement(*stored, rec) &&
		len(stored.EvidenceRefs) == 1 && len(rec.EvidenceRefs) == 1 && stored.EvidenceRefs[0] == rec.EvidenceRefs[0]
	return SettleResult{Record: *stored, Won: won}, nil
}

// SplitBlocks makes the blocks of a segment's text (design 12.3): line
// endings are normalised to LF (a lone CR counts as one), the text is split
// on one or more blank lines (a line of only whitespace is blank), each part
// is trimmed and empty parts are dropped. It is the one function that counts
// a segment's blocks; the claim's `blocks` tag is its count.
func SplitBlocks(text string) []string {
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	var blocks, part []string
	flush := func() {
		if len(part) > 0 {
			if b := strings.TrimSpace(strings.Join(part, "\n")); b != "" {
				blocks = append(blocks, b)
			}
			part = nil
		}
	}
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		part = append(part, line)
	}
	flush()
	return blocks
}
