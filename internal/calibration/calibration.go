// Package calibration computes a desk's calibration digest from its own
// settled self-scores: the in-context feedback of the ICRL experiment
// (docs/archive/reports/icrl-calibration-experiment-2026-10-14.md).
//
// Ways it can fail, written before the code (experiment §5):
//   - a pending item, or a held-out one, enters the digest (P4, held-out);
//   - a tiny sample reads as a finding (P5): no digest under MinN, no bin
//     under MinBin;
//   - the rendered text changes when nothing changed (P2): the text has no
//     time, and observations are ordered deterministically;
//   - the math differs from E6a's (Brier over continuous outcomes in
//     [0,1], qualified as 0.5; ECE over E6a's five bins), so the digest and
//     the offline measurement disagree.
//
// The package has no store, clock or context: the runtime collects the
// observations and passes them in.
package calibration

import (
	"crypto/sha256"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// Observation is one settled self-score: the probability a desk stated
// and the outcome independent settlement gave it, in [0, 1].
type Observation struct {
	Desk      string `json:"desk"`
	ClaimType string `json:"claim_type"`
	Thread    string `json:"thread"`
	// Story is the inbox the claim is about (texture:<doc>): the cluster
	// of the experiment's story-clustered bootstrap.
	Story    string  `json:"story,omitempty"`
	ModelID  string  `json:"model_id,omitempty"`
	RecordID string  `json:"record_id"`
	Key      string  `json:"key"`
	P        float64 `json:"p"`
	Y        float64 `json:"y"`
	// Scorers name who settled it; several scorers' outcomes are averaged.
	Scorers     []string `json:"scorers,omitempty"`
	CommittedAt string   `json:"committed_at,omitempty"`
	HeldOut     bool     `json:"held_out,omitempty"`
}

// Options bound a digest.
type Options struct {
	// Window is how many of the most recent settled observations count.
	Window int
	// MinN is the fewest observations a digest is shown with.
	MinN int
	// MinBin is the fewest observations a bin, claim type or model line
	// is shown with.
	MinBin int
	// ModelID is the model of the run reading the digest; its own line is
	// shown when it has at least MinN observations.
	ModelID string
}

// DefaultOptions are the experiment's preregistered bounds.
var DefaultOptions = Options{Window: 400, MinN: 20, MinBin: 5}

// Bins are E6a's five bands of stated probability (claims-e6a box score).
var Bins = []struct {
	Label  string
	Lo, Hi float64
}{
	{"<0.5", 0, 0.5}, {"0.5-0.7", 0.5, 0.7}, {"0.7-0.85", 0.7, 0.85}, {"0.85-0.95", 0.85, 0.95}, {">=0.95", 0.95, 1.0000001},
}

// BinOf returns the index of p's bin.
func BinOf(p float64) int {
	for i, b := range Bins {
		if p >= b.Lo && p < b.Hi {
			return i
		}
	}
	if p < 0 {
		return 0
	}
	return len(Bins) - 1
}

// calibratedBand is the |mean p - observed| under which a desk reads as
// calibrated rather than over- or under-confident.
const calibratedBand = 0.03

// HeldOut reports whether an observation key is in the 10% held out of
// every digest: those outcomes are measured but never shown.
func HeldOut(key string) bool {
	sum := sha256.Sum256([]byte("choir-icrl-heldout-v1:" + key))
	return sum[0]%10 == 0
}

// Arm names the experiment arm of a desk thread (a run's agent id):
// "digest" or "control", fixed for the thread
// (sha256("choir-icrl-v1:" + thread), first byte even is digest).
func Arm(thread string) string {
	sum := sha256.Sum256([]byte("choir-icrl-v1:" + strings.TrimSpace(thread)))
	if sum[0]%2 == 0 {
		return "digest"
	}
	return "control"
}

// Stats is calibration over a set of observations.
type Stats struct {
	N        int
	Brier    float64
	ECE      float64
	MeanP    float64
	Observed float64
	Bins     []types.CalibrationBin
}

// Measure computes Brier, ECE (n-weighted |mean p - observed| over Bins),
// mean p and observed share. Bins with no observation are omitted.
func Measure(obs []Observation) Stats {
	s := Stats{N: len(obs)}
	if s.N == 0 {
		return s
	}
	type acc struct {
		n      int
		sp, sy float64
	}
	bins := make([]acc, len(Bins))
	for _, o := range obs {
		d := o.P - o.Y
		s.Brier += d * d
		s.MeanP += o.P
		s.Observed += o.Y
		b := &bins[BinOf(o.P)]
		b.n++
		b.sp += o.P
		b.sy += o.Y
	}
	n := float64(s.N)
	s.Brier /= n
	s.MeanP /= n
	s.Observed /= n
	for i, b := range bins {
		if b.n == 0 {
			continue
		}
		mp, my := b.sp/float64(b.n), b.sy/float64(b.n)
		s.ECE += float64(b.n) / n * math.Abs(mp-my)
		s.Bins = append(s.Bins, types.CalibrationBin{Label: Bins[i].Label, N: b.n, MeanP: mp, Observed: my})
	}
	return s
}

// Compute builds a desk's digest from its observations: held-out ones are
// counted and excluded, the rest ordered newest first (by commit time,
// then key) and cut to the window.
func Compute(desk string, obs []Observation, opts Options) types.CalibrationDigest {
	if opts.Window <= 0 {
		opts.Window = DefaultOptions.Window
	}
	if opts.MinBin <= 0 {
		opts.MinBin = DefaultOptions.MinBin
	}
	if opts.MinN <= 0 {
		opts.MinN = DefaultOptions.MinN
	}
	d := types.CalibrationDigest{Desk: desk}
	var shown []Observation
	for _, o := range obs {
		if o.Desk != desk {
			continue
		}
		if o.HeldOut {
			d.HeldOut++
			continue
		}
		shown = append(shown, o)
	}
	sort.SliceStable(shown, func(i, j int) bool {
		if shown[i].CommittedAt != shown[j].CommittedAt {
			return shown[i].CommittedAt > shown[j].CommittedAt
		}
		return shown[i].Key < shown[j].Key
	})
	if len(shown) > opts.Window {
		shown = shown[:opts.Window]
	}
	all := Measure(shown)
	d.N, d.Brier, d.ECE, d.MeanP, d.Observed = all.N, all.Brier, all.ECE, all.MeanP, all.Observed
	for _, b := range all.Bins {
		if b.N >= opts.MinBin {
			d.Bins = append(d.Bins, b)
		}
	}
	d.Direction = direction(all.MeanP, all.Observed)
	byType := map[string][]Observation{}
	for _, o := range shown {
		byType[o.ClaimType] = append(byType[o.ClaimType], o)
	}
	types_ := make([]string, 0, len(byType))
	for k := range byType {
		types_ = append(types_, k)
	}
	sort.Strings(types_)
	worst := -1.0
	for _, k := range types_ {
		st := Measure(byType[k])
		if st.N < opts.MinBin {
			continue
		}
		d.ClaimTypes = append(d.ClaimTypes, types.CalibrationStat{Key: k, N: st.N, Brier: st.Brier, ECE: st.ECE, MeanP: st.MeanP, Observed: st.Observed})
		if st.ECE > worst {
			worst, d.Worst = st.ECE, k
		}
	}
	if model := strings.TrimSpace(opts.ModelID); model != "" {
		var mine []Observation
		for _, o := range shown {
			if o.ModelID == model {
				mine = append(mine, o)
			}
		}
		if st := Measure(mine); st.N >= opts.MinN {
			d.Model = &types.CalibrationStat{Key: model, N: st.N, Brier: st.Brier, ECE: st.ECE, MeanP: st.MeanP, Observed: st.Observed}
		}
	}
	// Under MinN the record is noise (P5): the digest is computed, for the
	// owner's view, but has no text for a desk to read.
	if d.N >= opts.MinN {
		d.Text = Render(d)
	}
	return d
}

func direction(meanP, observed float64) string {
	switch gap := meanP - observed; {
	case gap <= -calibratedBand:
		return "under-confident"
	case gap >= calibratedBand:
		return "over-confident"
	}
	return "calibrated"
}

// claimNoun is how the digest names a claim type to the desk.
var claimNoun = map[string]string{
	types.CalibrationClaimHypothesis:      "hypothesis probabilities",
	types.CalibrationClaimCompleted:       "p_complete on final Reports",
	types.CalibrationClaimCitationSupport: "pSupported on Writes (one per Write, scored per citation)",
}

// Render is the block the desk reads. It has no time and no per-item
// outcome, so it is stable while the record is unchanged.
func Render(d types.CalibrationDigest) string {
	if d.N == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Your track record (%s desk): your %d most recent self-scored predictions that independent settlement has scored since you made them.\n", d.Desk, d.N)
	fmt.Fprintf(&b, "- Brier %.3f (0 is perfect); calibration error %.3f.\n", d.Brier, d.ECE)
	switch d.Direction {
	case "under-confident":
		fmt.Fprintf(&b, "- Under-confident: your mean probability was %.2f and %.2f of these held.\n", d.MeanP, d.Observed)
	case "over-confident":
		fmt.Fprintf(&b, "- Over-confident: your mean probability was %.2f and only %.2f of these held.\n", d.MeanP, d.Observed)
	default:
		fmt.Fprintf(&b, "- Calibrated on average: mean probability %.2f, %.2f held.\n", d.MeanP, d.Observed)
	}
	if len(d.Bins) > 0 {
		parts := make([]string, 0, len(d.Bins))
		for _, bin := range d.Bins {
			parts = append(parts, fmt.Sprintf("you said %.2f, %.2f held (n=%d)", bin.MeanP, bin.Observed, bin.N))
		}
		b.WriteString("- By stated probability: " + strings.Join(parts, "; ") + ".\n")
	}
	if len(d.ClaimTypes) > 1 && d.Worst != "" {
		for _, st := range d.ClaimTypes {
			if st.Key == d.Worst {
				fmt.Fprintf(&b, "- Least calibrated: %s (said %.2f, %.2f held, n=%d).\n", nounOf(st.Key), st.MeanP, st.Observed, st.N)
			}
		}
	} else if len(d.ClaimTypes) == 1 {
		fmt.Fprintf(&b, "- Scored: %s.\n", nounOf(d.ClaimTypes[0].Key))
	}
	if d.Model != nil {
		fmt.Fprintf(&b, "- Your model (%s): said %.2f, %.2f held, Brier %.3f (n=%d).\n", d.Model.Key, d.Model.MeanP, d.Model.Observed, d.Model.Brier, d.Model.N)
	}
	b.WriteString("A probability is a frequency: of the claims you give 0.7, about 7 in 10 should hold. State your next probabilities with this record in mind.")
	return b.String()
}

func nounOf(claimType string) string {
	if n, ok := claimNoun[claimType]; ok {
		return n
	}
	return claimType
}
