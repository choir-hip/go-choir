package trust

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// Publisher reputation in the Library (MA7.6, trust draft section 8): the
// publisher's record shown as track-record sentences per class, each with its
// n, never as stars or one number (PSA 5.5). A computer reads its own settled
// outcomes; it may sign them as a bundle of settled outcomes only (class,
// impact, quality, severity, release, timestamps; no content), and a reader
// aggregates bundles that verify against each computer's key, so a
// publisher's record crosses computers without anyone's data.

// Outcome kinds.
const (
	OutcomeAct              = "act"               // a settled act of one of the publisher's desks (E1 to E8)
	OutcomeReview           = "review"            // a sealed install review verdict on one of its releases
	OutcomeReviewPrediction = "review_prediction" // a settled prediction of a review seat about one of its releases
)

// OutcomeBundleSchema names the signed bundle's shape.
const OutcomeBundleSchema = "choir.publisher_outcomes/v1"

// OutcomeReceiptKind and OutcomeReceiptIssuer name the guest-core receipt
// that signs a bundle (receiptsigner allows exactly this pair).
const (
	OutcomeReceiptKind   = "PublisherOutcomes"
	OutcomeReceiptIssuer = "choir-autoputer"
)

// SettledOutcome is one settled outcome about a publisher. It carries opaque
// ids and the draft's fields only, never a record's text.
type SettledOutcome struct {
	Kind      string      `json:"kind"`
	RecordID  string      `json:"record_id"`
	Publisher string      `json:"publisher"`
	Package   string      `json:"package,omitempty"`
	Release   string      `json:"release,omitempty"`
	Class     ActionClass `json:"class"`
	// Evidence is the evidence class of an act.
	Evidence EvidenceClass `json:"evidence,omitempty"`
	// Within: the act turned out within its commitment, or the review
	// prediction held.
	Within bool `json:"within"`
	// Severity is the evidence-strength multiplier of a failure (1, 5, 20).
	Severity float64 `json:"severity,omitempty"`
	// Fidelity marks an E3 or E7 event.
	Fidelity      bool   `json:"fidelity,omitempty"`
	ReviewVerdict string `json:"review_verdict,omitempty"`
	SettledAt     string `json:"settled_at"`
}

// PublisherOutcomes extracts the settled acts of publisher's desks and
// packages from in (which needs a Resolver to place desks under packages and
// publishers). Unsettled and excluded evidence is not an outcome.
func PublisherOutcomes(in Inputs, p Params, publisher string) []SettledOutcome {
	if publisher == "" {
		return nil
	}
	if p.Lambda == 0 {
		p = DefaultParams()
	}
	ex := Extract(in, p)
	var out []SettledOutcome
	for _, it := range ex.Items {
		hit := false
		pkg := ""
		for _, s := range subjectsOf(it, in) {
			if s.Kind == SubjectPublisher && s.ID == publisher {
				hit = true
			}
			if s.Kind == SubjectPackage {
				pkg = s.ID
			}
		}
		if !hit {
			continue
		}
		o := SettledOutcome{Kind: OutcomeAct, RecordID: it.RecordID, Publisher: publisher, Package: pkg, Class: it.Action,
			Evidence: it.Class, Within: !it.Failure(), Fidelity: it.Class == E3 || it.Class == E7}
		if !o.Within {
			o.Severity = it.M
		}
		if !it.At.IsZero() {
			o.SettledAt = it.At.UTC().Format(time.RFC3339)
		}
		out = append(out, o)
	}
	return out
}

// Sentence is one line of the reputation block: its text and the n it rests
// on. There is no score.
type Sentence struct {
	Text string `json:"text"`
	N    int    `json:"n"`
}

// Sentences renders outcomes as track-record sentences per class, each with
// its n. With nothing settled it says so, with n = 0 and no rate.
func Sentences(outcomes []SettledOutcome) []Sentence {
	type tally struct{ n, within int }
	acts := map[string]*tally{}
	var classes []string
	var fidelity []time.Time
	fidelityN := 0
	actsN := 0
	reviews, accepted := 0, 0
	preds, contradicted := 0, 0
	for _, o := range outcomes {
		switch o.Kind {
		case OutcomeAct:
			actsN++
			key := o.Class.Grant + " (" + string(o.Class.Impact) + ")"
			if acts[key] == nil {
				acts[key] = &tally{}
				classes = append(classes, key)
			}
			acts[key].n++
			if o.Within {
				acts[key].within++
			}
			if o.Fidelity {
				fidelityN++
				if t := parseTime(o.SettledAt); !t.IsZero() {
					fidelity = append(fidelity, t)
				}
			}
		case OutcomeReview:
			reviews++
			if o.ReviewVerdict == "accept" {
				accepted++
			}
		case OutcomeReviewPrediction:
			preds++
			if !o.Within {
				contradicted++
			}
		}
	}
	if actsN+reviews+preds == 0 {
		return []Sentence{{Text: "No settled outcomes for this maker yet.", N: 0}}
	}
	var out []Sentence
	sort.Strings(classes)
	for _, c := range classes {
		t := acts[c]
		out = append(out, Sentence{Text: fmt.Sprintf("%s: %d of %d settled acts within commitment.", c, t.within, t.n), N: t.n})
	}
	if actsN > 0 {
		sort.Slice(fidelity, func(i, j int) bool { return fidelity[i].Before(fidelity[j]) })
		switch {
		case fidelityN == 0:
			out = append(out, Sentence{Text: fmt.Sprintf("No fidelity events in %d settled acts.", actsN), N: actsN})
		case fidelityN == 1 && len(fidelity) == 1:
			out = append(out, Sentence{Text: fmt.Sprintf("One fidelity event (%s) in %d settled acts.", fidelity[0].Format("January 2006"), actsN), N: actsN})
		case len(fidelity) > 0:
			out = append(out, Sentence{Text: fmt.Sprintf("%d fidelity events in %d settled acts, the latest in %s.", fidelityN, actsN, fidelity[len(fidelity)-1].Format("January 2006")), N: actsN})
		default:
			out = append(out, Sentence{Text: fmt.Sprintf("%d fidelity events in %d settled acts.", fidelityN, actsN), N: actsN})
		}
	}
	if reviews > 0 {
		out = append(out, Sentence{Text: fmt.Sprintf("Reviewed: %d of %d accepted.", accepted, reviews), N: reviews})
	}
	if preds > 0 {
		out = append(out, Sentence{Text: fmt.Sprintf("%d of %d review predictions contradicted.", contradicted, preds), N: preds})
	}
	return out
}

// OutcomeBundle is what one computer signs about one publisher.
type OutcomeBundle struct {
	Schema     string           `json:"schema"`
	ComputerID string           `json:"computer_id"`
	Publisher  string           `json:"publisher"`
	IssuedAt   string           `json:"issued_at"`
	Outcomes   []SettledOutcome `json:"outcomes"`
}

// Digest is the sha256 of the bundle's JSON.
func (b OutcomeBundle) Digest() string {
	raw, _ := json.Marshal(b)
	return computerevent.DigestBytes(raw)
}

// SignedBundle is a bundle and the guest-core receipt over its digest.
type SignedBundle struct {
	Bundle  OutcomeBundle   `json:"bundle"`
	Receipt json.RawMessage `json:"receipt"`
}

// BundleSigner is the guest-core signer.
type BundleSigner interface {
	SignReceipt(ctx context.Context, kind, issuer string, fields map[string]any, issuedAt time.Time) (computerevent.Receipt, error)
}

func bundleFields(b OutcomeBundle) map[string]any {
	return map[string]any{"computer_id": b.ComputerID, "publisher": b.Publisher, "bundle_digest": b.Digest(), "outcomes_n": fmt.Sprint(len(b.Outcomes))}
}

// SignBundle signs b as of at.
func SignBundle(ctx context.Context, signer BundleSigner, b OutcomeBundle, at time.Time) (SignedBundle, error) {
	if signer == nil {
		return SignedBundle{}, errors.New("trust: no outcome signer")
	}
	b.Schema = OutcomeBundleSchema
	b.IssuedAt = at.UTC().Format(time.RFC3339Nano)
	receipt, err := signer.SignReceipt(ctx, OutcomeReceiptKind, OutcomeReceiptIssuer, bundleFields(b), at.UTC())
	if err != nil {
		return SignedBundle{}, err
	}
	raw, err := json.Marshal(receipt)
	if err != nil {
		return SignedBundle{}, err
	}
	return SignedBundle{Bundle: b, Receipt: raw}, nil
}

type bundlePin ed25519.PublicKey

func (k bundlePin) ResolveReceiptKey(_, _, _ string, _ uint64, _ time.Time) (ed25519.PublicKey, error) {
	return ed25519.PublicKey(k), nil
}

// VerifyBundle checks sb against the key of the computer it names.
func VerifyBundle(sb SignedBundle, key func(computerID string) (ed25519.PublicKey, bool)) error {
	if sb.Bundle.Schema != OutcomeBundleSchema || len(sb.Receipt) == 0 {
		return errors.New("not a signed outcome bundle")
	}
	pub, ok := key(sb.Bundle.ComputerID)
	if !ok || len(pub) != ed25519.PublicKeySize {
		return fmt.Errorf("no key for computer %s", sb.Bundle.ComputerID)
	}
	var receipt computerevent.Receipt
	if err := json.Unmarshal(sb.Receipt, &receipt); err != nil {
		return err
	}
	if receipt.ReceiptKind != OutcomeReceiptKind || receipt.Issuer != OutcomeReceiptIssuer {
		return errors.New("not an outcome receipt")
	}
	if err := receipt.Verify(bundlePin(pub)); err != nil {
		return err
	}
	for name, want := range bundleFields(sb.Bundle) {
		if fmt.Sprint(receipt.KindFields[name]) != fmt.Sprint(want) {
			return fmt.Errorf("receipt field %s does not match the bundle", name)
		}
	}
	return nil
}

// Aggregate verifies bundles about publisher and returns their outcomes, each
// (computer, kind, record) once. A bundle that fails verification, or is
// about another publisher, contributes nothing and is named in rejected.
func Aggregate(bundles []SignedBundle, publisher string, key func(computerID string) (ed25519.PublicKey, bool)) ([]SettledOutcome, []string) {
	seen := map[string]bool{}
	var out []SettledOutcome
	var rejected []string
	for _, sb := range bundles {
		if sb.Bundle.Publisher != publisher {
			continue
		}
		if err := VerifyBundle(sb, key); err != nil {
			rejected = append(rejected, sb.Bundle.ComputerID+": "+err.Error())
			continue
		}
		for _, o := range sb.Bundle.Outcomes {
			if o.Publisher != publisher {
				continue
			}
			k := sb.Bundle.ComputerID + "\x00" + o.Kind + "\x00" + o.RecordID + "\x00" + string(o.Evidence)
			if seen[k] {
				continue
			}
			seen[k] = true
			out = append(out, o)
		}
	}
	return out, rejected
}
