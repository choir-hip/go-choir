package trustedoutbox

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/decisionpolicy"
	"github.com/yusefmosiah/go-choir/internal/platform"
)

var (
	ErrModeOff         = errors.New("trusted outbox: computer mode is off")
	ErrNotArmed        = errors.New("trusted outbox: live send is not armed")
	ErrMissingIntent   = errors.New("trusted outbox: dispatch intent receipt missing")
	ErrUnknownOutcome  = errors.New("trusted outbox: provider outcome unknown; reconciliation required")
	ErrRejected        = errors.New("trusted outbox: provider rejected send")
	ErrRevoked         = errors.New("trusted outbox: policy revoked immediately before dispatch")
	ErrSubjectMismatch = errors.New("trusted outbox: payload or subject mismatch")
	ErrNotIrreversible = errors.New("trusted outbox: receipt is not an irreversible-email authorization")
	ErrPartialSuccess  = errors.New("trusted outbox: partial success never greens")
)

const (
	ProviderAccepted   = "accepted"
	ProviderRejected   = "rejected"
	ProviderUnknown    = "unknown"
	ReceiptIntent      = "DispatchIntentReceipt"
	ReceiptConsequence = "ConsequenceReceipt"
)

type Intent struct {
	ComputerID      string
	OperationID     string
	Recipient       string
	Payload         []byte
	PayloadDigest   string
	AcceptanceInbox string
	Actuator        string
	PolicyDigest    string
	ReceiptDigest   string
	IdempotencyKey  string
}

type ProviderResult struct {
	Outcome            string
	ProviderDeliveryID string
	Err                error
}

type Provider interface {
	Send(intent Intent) (ProviderResult, error)
}

type RecordingProvider struct {
	mu              sync.Mutex
	Sends           []Intent
	Result          ProviderResult
	FailAfterAccept bool
}

func (p *RecordingProvider) Send(intent Intent) (ProviderResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Sends = append(p.Sends, intent)
	if p.Result.Outcome == "" {
		p.Result.Outcome = ProviderAccepted
		if p.Result.ProviderDeliveryID == "" {
			p.Result.ProviderDeliveryID = "recording:" + intent.IdempotencyKey
		}
	}
	return p.Result, p.Result.Err
}

type IntentReceipt struct {
	ReceiptKind    string `json:"receipt_kind"`
	IdempotencyKey string `json:"idempotency_key"`
	SubjectDigest  string `json:"subject_digest"`
	PayloadDigest  string `json:"payload_digest"`
	Recipient      string `json:"recipient"`
	PolicyDigest   string `json:"policy_digest"`
	ReceiptDigest  string `json:"consensus_receipt_digest"`
	RecordedAt     string `json:"recorded_at"`
	Digest         string `json:"digest,omitempty"`
}

type ConsequenceReceipt struct {
	ReceiptKind                    string `json:"receipt_kind"`
	IdempotencyKey                 string `json:"idempotency_key"`
	IntentDigest                   string `json:"intent_digest"`
	ProviderOutcome                string `json:"provider_outcome"`
	ProviderDeliveryID             string `json:"provider_delivery_id,omitempty"`
	PayloadDigest                  string `json:"payload_digest"`
	Recipient                      string `json:"recipient"`
	AcceptanceInbox                string `json:"acceptance_inbox"`
	UncertainOutcomeReconciliation string `json:"uncertain_outcome_reconciliation,omitempty"`
	CrashWindow                    string `json:"crash_window,omitempty"`
	CompensationIntent             string `json:"compensation_intent_if_correction_required,omitempty"`
	Greened                        bool   `json:"greened"`
	RecordedAt                     string `json:"recorded_at"`
	Digest                         string `json:"digest,omitempty"`
}

type Outbox struct {
	mu             sync.Mutex
	Store          *decisionpolicy.Store
	Provider       Provider
	Armed          bool
	Revoked        map[string]bool
	intents        map[string]IntentReceipt
	consequences   map[string]ConsequenceReceipt
	acceptedOrphan map[string]ProviderResult
}

func New(store *decisionpolicy.Store, provider Provider) *Outbox {
	return &Outbox{
		Store: store, Provider: provider,
		Revoked: map[string]bool{}, intents: map[string]IntentReceipt{},
		consequences: map[string]ConsequenceReceipt{}, acceptedOrphan: map[string]ProviderResult{},
	}
}

type DispatchRequest struct {
	Mode    string
	Input   decisionpolicy.ConsensusInput
	Receipt decisionpolicy.QualifiedConsensusReceipt
	Payload []byte
	Now     string
}

func (o *Outbox) Dispatch(req DispatchRequest) (ConsequenceReceipt, error) {
	if o == nil || o.Store == nil {
		return ConsequenceReceipt{}, decisionpolicy.ErrNoPolicy
	}
	if strings.TrimSpace(req.Mode) == "" || req.Mode == platform.SelfDevelopmentModeOff {
		return ConsequenceReceipt{}, ErrModeOff
	}
	if err := decisionpolicy.Verify(o.Store, req.Input, req.Receipt); err != nil {
		return ConsequenceReceipt{}, err
	}
	if req.Receipt.PolicyID != decisionpolicy.PolicyIDIrreversibleEmailV1 &&
		req.Receipt.PolicyID != decisionpolicy.PolicyIDHumanRequiredV1 {
		return ConsequenceReceipt{}, ErrNotIrreversible
	}
	subject := req.Input.Subject
	if !subject.Irreversible() {
		return ConsequenceReceipt{}, ErrNotIrreversible
	}
	payloadDigest := computerevent.DigestBytes(req.Payload)
	if payloadDigest != subject.PayloadDigest {
		return ConsequenceReceipt{}, ErrSubjectMismatch
	}
	if o.Revoked[req.Receipt.PolicyDigest] {
		return ConsequenceReceipt{}, ErrRevoked
	}
	now := strings.TrimSpace(req.Now)
	if now == "" {
		now = time.Now().UTC().Format(time.RFC3339Nano)
	}
	key, err := exactSubjectKey(subject)
	if err != nil {
		return ConsequenceReceipt{}, err
	}

	o.mu.Lock()
	if existing, ok := o.consequences[key]; ok {
		o.mu.Unlock()
		return existing, nil
	}
	intent := IntentReceipt{
		ReceiptKind: ReceiptIntent, IdempotencyKey: key, SubjectDigest: req.Receipt.SubjectDigest,
		PayloadDigest: subject.PayloadDigest, Recipient: subject.Recipient,
		PolicyDigest: req.Receipt.PolicyDigest, ReceiptDigest: req.Receipt.ReceiptDigest, RecordedAt: now,
	}
	intent.Digest = computerevent.DigestBytes(mustJSON(intentWithoutDigest(intent)))
	o.intents[key] = intent
	o.mu.Unlock()

	if o.Provider == nil {
		return ConsequenceReceipt{}, ErrNotArmed
	}
	if _, isRecording := o.Provider.(*RecordingProvider); !isRecording && !o.Armed {
		return ConsequenceReceipt{}, ErrNotArmed
	}

	result, sendErr := o.Provider.Send(Intent{
		ComputerID: subject.ComputerID, OperationID: subject.OperationID, Recipient: subject.Recipient,
		Payload: req.Payload, PayloadDigest: subject.PayloadDigest, AcceptanceInbox: subject.AcceptanceInbox,
		Actuator: subject.Actuator, PolicyDigest: req.Receipt.PolicyDigest, ReceiptDigest: req.Receipt.ReceiptDigest,
		IdempotencyKey: key,
	})
	if sendErr != nil && result.Outcome == "" {
		result.Outcome = ProviderUnknown
		result.Err = sendErr
	}
	if result.Outcome == "" {
		result.Outcome = ProviderUnknown
	}

	o.mu.Lock()
	defer o.mu.Unlock()
	if existing, ok := o.consequences[key]; ok {
		return existing, nil
	}
	if _, ok := o.intents[key]; !ok {
		return ConsequenceReceipt{}, ErrMissingIntent
	}
	cons := ConsequenceReceipt{
		ReceiptKind: ReceiptConsequence, IdempotencyKey: key, IntentDigest: intent.Digest,
		ProviderOutcome: result.Outcome, ProviderDeliveryID: result.ProviderDeliveryID,
		PayloadDigest: subject.PayloadDigest, Recipient: subject.Recipient, AcceptanceInbox: subject.AcceptanceInbox,
		CrashWindow: "if process dies after provider acceptance and before consequence persistence, reconciliation must find or compensate the send",
		RecordedAt:  now,
	}
	switch result.Outcome {
	case ProviderAccepted:
		cons.Greened = true
	case ProviderRejected:
		cons.Greened = false
		cons.CompensationIntent = "none; provider rejected before delivery"
		cons.Digest = computerevent.DigestBytes(mustJSON(consequenceWithoutDigest(cons)))
		o.consequences[key] = cons
		return cons, ErrRejected
	case ProviderUnknown:
		cons.Greened = false
		cons.UncertainOutcomeReconciliation = "required"
		cons.Digest = computerevent.DigestBytes(mustJSON(consequenceWithoutDigest(cons)))
		o.consequences[key] = cons
		o.acceptedOrphan[key] = result
		return cons, ErrUnknownOutcome
	default:
		cons.Greened = false
		return cons, ErrPartialSuccess
	}
	cons.Digest = computerevent.DigestBytes(mustJSON(consequenceWithoutDigest(cons)))
	o.consequences[key] = cons
	return cons, nil
}

func (o *Outbox) SimulateCrashAfterAccept(key string, result ProviderResult) {
	o.mu.Lock()
	defer o.mu.Unlock()
	delete(o.consequences, key)
	o.acceptedOrphan[key] = result
}

func (o *Outbox) Reconcile(key string) (ConsequenceReceipt, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if existing, ok := o.consequences[key]; ok {
		return existing, nil
	}
	intent, ok := o.intents[key]
	if !ok {
		return ConsequenceReceipt{}, ErrMissingIntent
	}
	orphan, found := o.acceptedOrphan[key]
	cons := ConsequenceReceipt{
		ReceiptKind: ReceiptConsequence, IdempotencyKey: key, IntentDigest: intent.Digest,
		PayloadDigest: intent.PayloadDigest, Recipient: intent.Recipient,
		CrashWindow: "reconciled after provider-accepted / persistence-missing window",
		RecordedAt:  time.Now().UTC().Format(time.RFC3339Nano),
	}
	if !found || orphan.Outcome != ProviderAccepted {
		cons.ProviderOutcome = ProviderUnknown
		cons.UncertainOutcomeReconciliation = "required"
		cons.CompensationIntent = "compensation_or_new_forward_action; restore does not unsend"
		cons.Greened = false
		cons.Digest = computerevent.DigestBytes(mustJSON(consequenceWithoutDigest(cons)))
		o.consequences[key] = cons
		return cons, ErrUnknownOutcome
	}
	cons.ProviderOutcome = ProviderAccepted
	cons.ProviderDeliveryID = orphan.ProviderDeliveryID
	cons.Greened = true
	cons.Digest = computerevent.DigestBytes(mustJSON(consequenceWithoutDigest(cons)))
	o.consequences[key] = cons
	return cons, nil
}

func exactSubjectKey(subject decisionpolicy.EffectSubject) (string, error) {
	digest, err := subject.Digest()
	if err != nil {
		return "", err
	}
	return "outbox:" + digest, nil
}

func intentWithoutDigest(r IntentReceipt) IntentReceipt {
	r.Digest = ""
	return r
}

func consequenceWithoutDigest(r ConsequenceReceipt) ConsequenceReceipt {
	r.Digest = ""
	return r
}

func mustJSON(v any) []byte {
	body, err := computerevent.CanonicalJSON(v)
	if err != nil {
		return []byte("{}")
	}
	return body
}
