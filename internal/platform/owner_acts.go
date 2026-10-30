package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/owneract"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

// S1 owner-act receipts and the effect_accepted admission gate
// (docs/problems/effect-accepted-admitted-without-authority-2026-10-15.md;
// docs/reports/principal-identity-and-signed-acts-2026-10-15.md §5, §6 S1).
//
// An owner mode write must carry a receipt the proxy signed with its own
// host-owner-act key; corpusd records it single-use and names it in the
// ModeReceipt. Every ModeReceipt and every platform update offer corpusd
// issues goes into an append-only log, and an effect_accepted append is
// admitted only when its authority traces to one of them.

// EnvOwnerActMode selects the rollout mode: observe (the default, also when
// unset) logs what enforce would refuse and allows it; enforce refuses. Any
// other value is treated as enforce (fail closed, as hostauth does).
const EnvOwnerActMode = "CHOIR_OWNER_ACT_MODE"

type OwnerActMode string

const (
	OwnerActObserve OwnerActMode = "observe"
	OwnerActEnforce OwnerActMode = "enforce"
)

// Authority prefixes an effect_accepted may carry.
const (
	AuthorityPlatformUpdate     = "platform-control:update"
	AuthorityExternalOwner      = "external-owner:"
	AuthorityQualifiedConsensus = "qualified-consensus:"
)

var (
	// ErrOwnerActRefused refuses an owner mode write whose owner-act
	// receipt is missing, invalid or replayed (enforce only).
	ErrOwnerActRefused = errors.New("owner act refused")
	// ErrEffectAdmissionRefused refuses an effect_accepted whose authority
	// corpusd cannot trace (enforce only).
	ErrEffectAdmissionRefused = errors.New("effect_accepted admission refused")
)

const ownerActSchemaDDL = `
CREATE TABLE IF NOT EXISTS owner_act_receipts (
  receipt_id VARCHAR(64) NOT NULL,
  nonce CHAR(32) NOT NULL,
  computer_id VARCHAR(128) NOT NULL,
  act_kind VARCHAR(64) NOT NULL,
  owner_user_id VARCHAR(255) NOT NULL,
  auth_method VARCHAR(32) NOT NULL,
  credential_ref VARCHAR(255) NOT NULL,
  receipt_digest CHAR(64) NOT NULL,
  receipt_json TEXT NOT NULL,
  mode_receipt_digest CHAR(64) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (receipt_id),
  CONSTRAINT owner_act_receipts_nonce_uq UNIQUE (nonce),
  CONSTRAINT owner_act_receipts_digest_uq UNIQUE (receipt_digest)
);
CREATE TABLE IF NOT EXISTS self_development_mode_receipts (
  receipt_digest CHAR(64) NOT NULL,
  computer_id VARCHAR(128) NOT NULL,
  generation BIGINT NOT NULL,
  old_generation BIGINT NOT NULL,
  old_mode VARCHAR(32) NOT NULL,
  new_mode VARCHAR(32) NOT NULL,
  mode_writer VARCHAR(32) NOT NULL,
  owner_act_receipt_digest CHAR(64) NULL,
  receipt_json TEXT NOT NULL,
  admitted_event_digest CHAR(64) NULL,
  created_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (receipt_digest),
  CONSTRAINT self_development_mode_receipts_generation_uq UNIQUE (computer_id, generation)
);
CREATE TABLE IF NOT EXISTS platform_update_offers (
  offer_digest CHAR(64) NOT NULL,
  computer_id VARCHAR(128) NOT NULL,
  update_id VARCHAR(255) NOT NULL,
  base_event_head CHAR(64) NOT NULL,
  content_digest CHAR(64) NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  admitted_event_digest CHAR(64) NULL,
  created_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (offer_digest)
);
CREATE INDEX IF NOT EXISTS platform_update_offers_computer_idx ON platform_update_offers (computer_id, created_at);`

// platformUpdateOfferAdmissionGrace tolerates the seconds between the
// guest's own expiry check and its append.
const platformUpdateOfferAdmissionGrace = 2 * time.Minute

// OwnerActAuthority is corpusd's S1 policy: the rollout mode and the
// proxy's pinned public key.
type OwnerActAuthority struct {
	mode     OwnerActMode
	verifier *owneract.Verifier
	note     string
	now      func() time.Time
}

// NewOwnerActAuthority builds a policy. A nil verifier refuses every
// receipt (fail closed in enforce).
func NewOwnerActAuthority(mode OwnerActMode, verifier *owneract.Verifier) *OwnerActAuthority {
	if mode != OwnerActObserve {
		mode = OwnerActEnforce
	}
	return &OwnerActAuthority{mode: mode, verifier: verifier, now: func() time.Time { return time.Now().UTC() }}
}

// LoadOwnerActAuthorityFromEnv reads EnvOwnerActMode and the proxy's
// public key (owneract.LoadVerifierFromEnv). A missing or bad key never
// stops corpusd: in observe it is logged, in enforce every owner act is
// refused until the key is fixed.
func LoadOwnerActAuthorityFromEnv() *OwnerActAuthority {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv(EnvOwnerActMode)))
	mode := OwnerActEnforce
	note := ""
	switch OwnerActMode(raw) {
	case "", OwnerActObserve:
		mode = OwnerActObserve
	case OwnerActEnforce:
	default:
		note = fmt.Sprintf("unknown %s %q treated as enforce", EnvOwnerActMode, raw)
	}
	verifier, path, err := owneract.LoadVerifierFromEnv()
	authority := NewOwnerActAuthority(mode, verifier)
	switch {
	case err != nil:
		authority.note = joinNote(note, err.Error())
	case verifier == nil && path == "":
		authority.note = joinNote(note, "no owner-act public key configured")
	case verifier == nil:
		authority.note = joinNote(note, "owner-act public key absent at "+path)
	default:
		authority.note = joinNote(note, "owner-act key "+verifier.KeyID()+" from "+path)
	}
	return authority
}

func joinNote(a, b string) string {
	if a == "" {
		return b
	}
	return a + "; " + b
}

// Status describes the policy for the startup log.
func (a *OwnerActAuthority) Status() string {
	if a == nil {
		return "owner-act gate not configured"
	}
	return fmt.Sprintf("mode=%s; %s", a.mode, a.note)
}

// Enforcing reports whether refusals refuse.
func (a *OwnerActAuthority) Enforcing() bool { return a != nil && a.mode == OwnerActEnforce }

// refuse logs a refusal and returns the error enforce returns; in observe
// it returns nil and the caller proceeds.
func (a *OwnerActAuthority) refuse(sentinel error, what, computerID, reason string) error {
	if a.Enforcing() {
		log.Printf("owner-act: enforce: refused %s computer=%s: %s", what, computerID, reason)
		return fmt.Errorf("%w: %s", sentinel, reason)
	}
	log.Printf("owner-act: observe: would refuse %s computer=%s: %s", what, computerID, reason)
	return nil
}

// OwnerActKindForMode names the owner act a mode write is: arming
// accept_once is the owner's approval of exactly one operation and bundle.
func OwnerActKindForMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case SelfDevelopmentModeAcceptOnce:
		return owneract.ActSelfDevApprove
	case SelfDevelopmentModeStandingConsensus:
		return owneract.ActSelfDevAutonomyArm
	}
	return owneract.ActSelfDevModeSet
}

// OwnerActModeSubject is the subject the proxy signs for a mode write and
// corpusd checks: the request commitment binds every field of the write.
func OwnerActModeSubject(computerID string, request SetSelfDevelopmentModeRequest) (map[string]any, error) {
	return OwnerActModeSubjectWithTier(computerID, request, "")
}

// OwnerActModeSubjectWithTier adds the computer tier the proxy read from
// vmctl, which a standing_consensus arm must carry (P4).
func OwnerActModeSubjectWithTier(computerID string, request SetSelfDevelopmentModeRequest, tier string) (map[string]any, error) {
	commitment, err := SelfDevelopmentModeRequestCommitment(computerID, request)
	if err != nil {
		return nil, err
	}
	subject := map[string]any{
		"mode":                     strings.TrimSpace(request.Mode),
		"expected_generation":      request.ExpectedGeneration,
		"operation_id":             request.OperationID,
		"bundle_digest":            request.BundleDigest,
		"policy_digest":            request.PolicyDigest,
		"consensus_receipt_digest": request.ConsensusReceiptDigest,
		"request_commitment":       commitment,
	}
	if tier = strings.TrimSpace(tier); tier != "" {
		subject["computer_tier"] = tier
	}
	return subject, nil
}

// verifyModeWrite checks the owner-act receipt on a host-internal mode
// write: signature, audience, computer, act kind, window and subject.
func (a *OwnerActAuthority) verifyModeWrite(r *http.Request, computerID string, request SetSelfDevelopmentModeRequest) (*owneract.Verified, error) {
	receipt, err := owneract.Decode(r.Header.Get(owneract.Header))
	if err != nil {
		return nil, err
	}
	verified, err := a.verifier.Verify(receipt, computerID, OwnerActKindForMode(request.Mode), a.now())
	if err != nil {
		return nil, err
	}
	want, err := OwnerActModeSubject(computerID, request)
	if err != nil {
		return nil, err
	}
	if verified.SubjectString("request_commitment") != want["request_commitment"] || verified.SubjectString("mode") != want["mode"] {
		return nil, fmt.Errorf("owner act: subject does not bind this mode write")
	}
	return &verified, nil
}

// ownerActReplayed reports whether a receipt id or nonce was used before.
func ownerActReplayed(ctx context.Context, tx *sql.Tx, verified *owneract.Verified) (bool, error) {
	var count int
	err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM owner_act_receipts WHERE receipt_id=? OR nonce=? OR receipt_digest=?`, verified.ReceiptID, verified.Nonce, verified.Digest).Scan(&count)
	return count > 0, err
}

func recordOwnerAct(ctx context.Context, tx *sql.Tx, verified *owneract.Verified, modeReceiptDigest string, now time.Time) error {
	receiptJSON, err := verified.Receipt.CanonicalBytes()
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO owner_act_receipts (receipt_id, nonce, computer_id, act_kind, owner_user_id, auth_method, credential_ref, receipt_digest, receipt_json, mode_receipt_digest, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		verified.ReceiptID, verified.Nonce, verified.ComputerID, verified.ActKind, verified.OwnerUserID, verified.AuthMethod, verified.CredentialRef, verified.Digest, string(receiptJSON), modeReceiptDigest, now)
	if err != nil {
		return fmt.Errorf("owner act: record receipt: %w", err)
	}
	return nil
}

// recordModeReceiptIssued appends a ModeReceipt to the issuance log.
func recordModeReceiptIssued(ctx context.Context, tx *sql.Tx, computerID string, current, next SelfDevelopmentMode, writer, ownerActDigest string, receiptJSON []byte, now time.Time) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO self_development_mode_receipts (receipt_digest, computer_id, generation, old_generation, old_mode, new_mode, mode_writer, owner_act_receipt_digest, receipt_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		computerevent.DigestBytes(receiptJSON), computerID, next.Generation, current.Generation, current.Mode, next.Mode, writer, nullableString(ownerActDigest), string(receiptJSON), now)
	if err != nil {
		return fmt.Errorf("self-development mode: record issuance: %w", err)
	}
	return nil
}

// recordPlatformUpdateOfferIssued appends a signed offer to the issuance log.
func recordPlatformUpdateOfferIssued(ctx context.Context, db *sql.DB, offer selfdevprotocol.PlatformUpdateOffer, offerDigest string, now time.Time) error {
	expiresAt, err := time.Parse(time.RFC3339Nano, offer.ExpiresAt)
	if err != nil {
		return fmt.Errorf("platform update offer: expiry: %w", err)
	}
	_, err = db.ExecContext(ctx, `INSERT INTO platform_update_offers (offer_digest, computer_id, update_id, base_event_head, content_digest, expires_at, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		offerDigest, offer.ComputerID, offer.UpdateID, offer.BaseEventHead, offer.Manifest.ContentDigest, expiresAt.UTC(), now)
	if err != nil {
		return fmt.Errorf("platform update offer: record issuance: %w", err)
	}
	return nil
}

type modeReceiptLogRow struct {
	digest         string
	generation     uint64
	oldGeneration  uint64
	oldMode        string
	newMode        string
	writer         string
	ownerActDigest string
	admitted       string
	receipt        computerevent.Receipt
}

func readModeReceiptLogRow(ctx context.Context, tx *sql.Tx, where string, args ...any) (modeReceiptLogRow, bool, error) {
	var row modeReceiptLogRow
	var receiptJSON string
	err := tx.QueryRowContext(ctx, `SELECT receipt_digest, generation, old_generation, old_mode, new_mode, mode_writer, COALESCE(owner_act_receipt_digest, ''), COALESCE(admitted_event_digest, ''), receipt_json FROM self_development_mode_receipts WHERE `+where, args...).
		Scan(&row.digest, &row.generation, &row.oldGeneration, &row.oldMode, &row.newMode, &row.writer, &row.ownerActDigest, &row.admitted, &receiptJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return modeReceiptLogRow{}, false, nil
	}
	if err != nil {
		return modeReceiptLogRow{}, false, err
	}
	if err := json.Unmarshal([]byte(receiptJSON), &row.receipt); err != nil {
		return modeReceiptLogRow{}, false, err
	}
	return row, true, nil
}

func (r modeReceiptLogRow) field(name string) string {
	value, _ := r.receipt.KindFields[name].(string)
	return value
}

// admitEffectAccepted is the S1 gate. It runs inside CompareAndSwap's
// transaction, after the idempotent-retry short-circuit and the reducer, so
// history is not re-judged and only well-formed events are. An admitted
// event marks the log row it consumed, so one grant or offer admits one
// event.
func (a *OwnerActAuthority) admitEffectAccepted(ctx context.Context, tx *sql.Tx, event computerevent.Event, eventDigest string) error {
	reason, mark, err := a.effectAcceptedRefusal(ctx, tx, event, eventDigest)
	// P4: the consensus branch always enforces. There is no pre-P4
	// consensus traffic to observe, and observe would admit exactly the
	// guest-forged quorum the branch exists to stop.
	enforce := a.Enforcing() || IsConsensusAuthority(event.AuthorityRef)
	if err != nil {
		if enforce {
			return fmt.Errorf("%w: authority lookup failed: %v", ErrEffectAdmissionRefused, err)
		}
		log.Printf("owner-act: observe: effect_accepted authority lookup failed computer=%s event=%s: %v", event.ComputerID, eventDigest, err)
		return nil
	}
	if reason != "" {
		if enforce && !a.Enforcing() {
			log.Printf("owner-act: enforce (consensus): refused effect_accepted %s computer=%s: %s", eventDigest, event.ComputerID, reason)
			return fmt.Errorf("%w: %s", ErrEffectAdmissionRefused, reason)
		}
		return a.refuse(ErrEffectAdmissionRefused, "effect_accepted "+eventDigest+" authority="+event.AuthorityRef, event.ComputerID, reason)
	}
	if mark != nil {
		if err := mark(); err != nil {
			if enforce {
				return fmt.Errorf("%w: record admission: %v", ErrEffectAdmissionRefused, err)
			}
			log.Printf("owner-act: observe: record admission failed computer=%s event=%s: %v", event.ComputerID, eventDigest, err)
		}
	}
	return nil
}

// effectAcceptedRefusal returns a refusal reason ("" when admitted) and the
// write that marks the consumed authority.
func (a *OwnerActAuthority) effectAcceptedRefusal(ctx context.Context, tx *sql.Tx, event computerevent.Event, eventDigest string) (string, func() error, error) {
	authority := strings.TrimSpace(event.AuthorityRef)
	switch {
	case authority == AuthorityPlatformUpdate:
		return a.platformUpdateRefusal(ctx, tx, event, eventDigest)
	case strings.HasPrefix(authority, AuthorityExternalOwner):
		return a.externalOwnerRefusal(ctx, tx, event, eventDigest, strings.TrimPrefix(authority, AuthorityExternalOwner))
	case strings.HasPrefix(authority, AuthorityQualifiedConsensus):
		// P4: admitted only for a ConsensusDecisionReceipt corpusd's
		// decision poster minted from signed ballots.
		return consensusDecisionRefusal(ctx, tx, event, eventDigest, a.now())
	case authority == "":
		return "no authority_ref", nil, nil
	default:
		return fmt.Sprintf("unknown authority %q", authority), nil, nil
	}
}

func (a *OwnerActAuthority) platformUpdateRefusal(ctx context.Context, tx *sql.Tx, event computerevent.Event, eventDigest string) (string, func() error, error) {
	var computerID, baseHead, contentDigest, admitted string
	var expiresAt time.Time
	err := tx.QueryRowContext(ctx, `SELECT computer_id, base_event_head, content_digest, expires_at, COALESCE(admitted_event_digest, '') FROM platform_update_offers WHERE offer_digest=?`, event.DecisionRef).
		Scan(&computerID, &baseHead, &contentDigest, &expiresAt, &admitted)
	if errors.Is(err, sql.ErrNoRows) {
		return "decision_ref is not a platform update offer corpusd minted", nil, nil
	}
	if err != nil {
		return "", nil, err
	}
	switch {
	case computerID != event.ComputerID:
		return "platform update offer was minted for another computer", nil, nil
	case contentDigest != event.ProposedEffectRef:
		return "proposed_effect_ref is not the offer's content digest", nil, nil
	case baseHead != event.PreviousHead:
		return "event does not follow the offer's base event head", nil, nil
	case a.now().After(expiresAt.UTC().Add(platformUpdateOfferAdmissionGrace)):
		return "platform update offer expired", nil, nil
	case admitted != "" && admitted != eventDigest:
		return "platform update offer already admitted another event", nil, nil
	}
	return "", func() error {
		_, err := tx.ExecContext(ctx, `UPDATE platform_update_offers SET admitted_event_digest=? WHERE offer_digest=? AND (admitted_event_digest IS NULL OR admitted_event_digest=?)`, eventDigest, event.DecisionRef, eventDigest)
		return err
	}, nil
}

func (a *OwnerActAuthority) externalOwnerRefusal(ctx context.Context, tx *sql.Tx, event computerevent.Event, eventDigest, ownerID string) (string, func() error, error) {
	if strings.TrimSpace(ownerID) == "" {
		return "external-owner authority names no owner", nil, nil
	}
	if len(event.InputArtifactRefs) == 0 {
		return "no ModeReceipt input artifact", nil, nil
	}
	ref, err := computerevent.ParseArtifactRef(event.InputArtifactRefs[0])
	if err != nil {
		return "input artifact 0 is not an artifact reference", nil, nil
	}
	modeDigest := ref.Digest().String()
	consumed, found, err := readModeReceiptLogRow(ctx, tx, `computer_id=? AND receipt_digest=?`, event.ComputerID, modeDigest)
	if err != nil {
		return "", nil, err
	}
	if !found {
		return "input ModeReceipt " + modeDigest + " was never issued by corpusd for this computer", nil, nil
	}
	if consumed.writer != ModeWriterGuestConsumption || consumed.oldMode != SelfDevelopmentModeAcceptOnce || consumed.newMode != SelfDevelopmentModeProposeOnly {
		return fmt.Sprintf("input ModeReceipt is not a guest consumption of accept_once (writer=%s %s->%s)", consumed.writer, consumed.oldMode, consumed.newMode), nil, nil
	}
	if !strings.HasPrefix(consumed.field("idempotency_key"), AcceptOnceConsumptionKeyPrefix+event.ParentEventID+":") {
		return "consumption key does not name this operation", nil, nil
	}
	pairs := [][2]string{
		{consumed.field("consumed_operation_id"), event.ParentEventID},
		{consumed.field("consumed_bundle_digest"), event.ProposedEffectRef},
		{consumed.field("consumed_desired_event_head"), event.ExpectedDesiredEventHead},
		{consumed.field("consumed_effective_event_head"), event.ExpectedEffectiveEventHead},
		{consumed.field("consumed_pending_transition_ref"), event.ExpectedPendingTransitionRef},
		{consumed.field("consumed_desired_state_commitment"), event.ExpectedDesiredStateCommitment},
		{consumed.field("consumed_effective_state_commitment"), event.ExpectedEffectiveStateCommitment},
	}
	for _, pair := range pairs {
		if pair[0] != pair[1] {
			return "consumed ModeReceipt does not bind this operation, bundle and heads", nil, nil
		}
	}
	if consumed.admitted != "" && consumed.admitted != eventDigest {
		return "consumed ModeReceipt already admitted another event", nil, nil
	}
	arm, found, err := readModeReceiptLogRow(ctx, tx, `computer_id=? AND generation=?`, event.ComputerID, consumed.oldGeneration)
	if err != nil {
		return "", nil, err
	}
	if !found {
		return "the accept_once arm the consumption names was never issued by corpusd", nil, nil
	}
	if arm.writer != ModeWriterHostInternal || arm.newMode != SelfDevelopmentModeAcceptOnce ||
		arm.field("operation_id") != event.ParentEventID || arm.field("bundle_digest") != event.ProposedEffectRef {
		return "the consumed grant was not an owner arm of accept_once for this operation", nil, nil
	}
	if arm.ownerActDigest == "" {
		return "the accept_once arm carries no owner-act receipt", nil, nil
	}
	var actComputer, actOwner, actKind string
	err = tx.QueryRowContext(ctx, `SELECT computer_id, owner_user_id, act_kind FROM owner_act_receipts WHERE receipt_digest=?`, arm.ownerActDigest).Scan(&actComputer, &actOwner, &actKind)
	if errors.Is(err, sql.ErrNoRows) {
		return "the arm's owner-act receipt is not in corpusd's log", nil, nil
	}
	if err != nil {
		return "", nil, err
	}
	if actComputer != event.ComputerID || actKind != owneract.ActSelfDevApprove {
		return "the arm's owner-act receipt is for another computer or act", nil, nil
	}
	if actOwner != ownerID {
		return fmt.Sprintf("authority names owner %q but the owner act was %q's", ownerID, actOwner), nil, nil
	}
	return "", func() error {
		_, err := tx.ExecContext(ctx, `UPDATE self_development_mode_receipts SET admitted_event_digest=? WHERE receipt_digest=? AND (admitted_event_digest IS NULL OR admitted_event_digest=?)`, eventDigest, consumed.digest, eventDigest)
		return err
	}, nil
}
