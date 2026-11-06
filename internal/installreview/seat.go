package installreview

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/modelcatalog"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
)

// The DeepSeek seat runs Synthetic's DeepSeek V4.1 Flash 1017, the catalog's
// DeepSeek (never through OpenRouter: owner, 2026-11-04).
const (
	DeepSeekProvider = "synthetic"
	DeepSeekModel    = "hf:deepseek-ai/DeepSeek-V4.1-Flash-1017"
	// devinUnwired is why the Devin seat cannot run: the platform has no
	// Devin provider. The seat stays in the roster so a plan that needs it
	// says so instead of substituting another model.
	devinUnwired = "no Devin provider is wired on the platform (the model catalog lists none and the gateway routes none)"
)

// Caller is the provider surface a seat calls (the gateway-backed
// ToolLoopProvider).
type Caller interface {
	CallWithTools(ctx context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error)
}

// ProviderSeat is a seat that calls one catalogued (provider, model) through
// the gateway. A seat whose provider is not catalogued, or that has no
// caller, is unavailable and gives no verdict.
type ProviderSeat struct {
	name, provider, model string
	unwired               string
	caller                Caller
}

// NewProviderSeat names a seat on (provider, model).
func NewProviderSeat(name, provider, model string, caller Caller) *ProviderSeat {
	return &ProviderSeat{name: name, provider: provider, model: model, caller: caller}
}

func (s *ProviderSeat) Name() string     { return s.name }
func (s *ProviderSeat) Model() string    { return s.model }
func (s *ProviderSeat) Provider() string { return s.provider }

// Roster is the review seats: Devin and DeepSeek, nothing else (trust draft
// 7.2). caller is the runtime's gateway provider; nil leaves every seat
// unavailable.
func Roster(caller Caller) map[string]Seat {
	return map[string]Seat{
		SeatDevin:    &ProviderSeat{name: SeatDevin, unwired: devinUnwired, caller: caller},
		SeatDeepSeek: NewProviderSeat(SeatDeepSeek, DeepSeekProvider, DeepSeekModel, caller),
	}
}

const seatSystem = `You are an install reviewer for Choir, a persistent personal computer. A package asks to be installed with the capability ceiling below. Review its manifest and code for whether it stays inside that ceiling and whether any owner-private data could reach a sink its publisher can read.

Answer with exactly one JSON object and nothing else:
{"accept": true|false, "blocking": true|false, "findings": ["..."], "predictions": [{"question": "<copy each question below verbatim>", "p_yes": <probability in [0,1]>}]}

"blocking" is true when a finding must stop the install; a blocking finding cannot accept. Give a probability for every question; it is a prediction that will be scored against what happens.`

// Review asks the seat's model for a typed verdict. Anything that is not the
// typed verdict, with a probability for every standard question, is an
// error, never an accept.
func (s *ProviderSeat) Review(ctx context.Context, p Packet) (SeatVerdict, error) {
	if s.unwired != "" {
		return SeatVerdict{}, fmt.Errorf("%w: %s: %s", ErrSeatUnavailable, s.name, s.unwired)
	}
	if s.caller == nil || !modelcatalog.ProviderCatalogued(s.provider) {
		return SeatVerdict{}, fmt.Errorf("%w: %s: no provider", ErrSeatUnavailable, s.name)
	}
	if _, ok := modelcatalog.Lookup(s.provider, s.model); !ok {
		return SeatVerdict{}, fmt.Errorf("%w: %s: %s/%s is not catalogued", ErrSeatUnavailable, s.name, s.provider, s.model)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Package %s, release %s, publisher %s.\nRisk class: %s.\n", p.Subject.Package, p.Subject.ReleaseID, p.Subject.Publisher, p.Risk)
	fmt.Fprintf(&b, "Ceiling entries this install adds: %s\n", strings.Join(p.Added, ", "))
	if len(p.Flow) > 0 {
		fmt.Fprintf(&b, "Source-to-sink paths the ceiling opens:\n- %s\n", strings.Join(p.Flow, "\n- "))
	}
	for _, c := range p.Checks {
		fmt.Fprintf(&b, "Mechanical check %s: %v %s\n", c.Name, c.OK, c.Detail)
	}
	b.WriteString("Questions:\n")
	for _, q := range StandardQuestions {
		b.WriteString("- " + q + "\n")
	}
	fmt.Fprintf(&b, "\nManifest:\n%s\n\nCode:\n%s\n", p.ManifestJSON, p.Source)
	msg, err := json.Marshal(map[string]any{"role": "user", "content": []any{map[string]string{"type": "text", "text": b.String()}}})
	if err != nil {
		return SeatVerdict{}, err
	}
	resp, err := s.caller.CallWithTools(ctx, provideriface.ToolLoopRequest{
		Provider: s.provider, Model: s.model, ConversationID: "install-review-" + s.name + "-" + p.Subject.ReleaseID,
		System: seatSystem, Messages: []json.RawMessage{msg},
	})
	if err != nil {
		return SeatVerdict{}, fmt.Errorf("install review: seat %s: %w", s.name, err)
	}
	return ParseSeatVerdict(resp.Text)
}

// ParseSeatVerdict reads a seat's typed verdict out of its text (a bare
// object, or one fenced block).
func ParseSeatVerdict(text string) (SeatVerdict, error) {
	text = strings.TrimSpace(text)
	if i := strings.Index(text, "{"); i >= 0 {
		if j := strings.LastIndex(text, "}"); j > i {
			text = text[i : j+1]
		}
	}
	var raw struct {
		Accept      *bool        `json:"accept"`
		Blocking    bool         `json:"blocking"`
		Findings    []string     `json:"findings"`
		Predictions []Prediction `json:"predictions"`
	}
	dec := json.NewDecoder(strings.NewReader(text))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil || raw.Accept == nil {
		return SeatVerdict{}, errors.New("install review: the seat's answer is not a typed verdict")
	}
	if *raw.Accept && raw.Blocking {
		return SeatVerdict{}, errors.New("install review: the seat accepted with a blocking finding")
	}
	got := map[string]float64{}
	for _, p := range raw.Predictions {
		if p.PYes < 0 || p.PYes > 1 {
			return SeatVerdict{}, fmt.Errorf("install review: probability %v out of range", p.PYes)
		}
		got[strings.TrimSpace(p.Question)] = p.PYes
	}
	out := SeatVerdict{Accept: *raw.Accept, Blocking: raw.Blocking, Findings: raw.Findings}
	for _, q := range StandardQuestions {
		p, ok := got[q]
		if !ok {
			return SeatVerdict{}, fmt.Errorf("install review: no probability for %q", q)
		}
		out.Predictions = append(out.Predictions, Prediction{Question: q, PYes: p})
	}
	return out, nil
}
