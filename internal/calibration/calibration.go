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
// Added with the wiring changes of
// docs/design/calibration-wiring-permutations-2026-11-04.md, written first:
//   - pooled bins mix easy and hard claim types (Simpson), and near-certain
//     filler pads n (C4) -> bins per claim type; paths_confined is left out
//     of the digest;
//   - a desk gets calibrated by turning vague (C5) -> resolution beside
//     Brier, overall and per type;
//   - the clock's bias is invisible behind "inside the interval" (C5) -> a
//     clock kind's line carries the median of estimate over actual minutes
//     from at least MinBin timed claims, and the censored count it left out;
//   - a desk that states late reads a record that looks fine (C11) -> its
//     own late share is a line of its digest.
//
// The package has no store, clock or context: the runtime collects the
// observations and passes them in.
package calibration

import (
	"crypto/hmac"
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
	// Arm is the A/B arm stamped on the record when it was stated, when
	// it carries one (an engineering round's predictions).
	Arm string `json:"stated_arm,omitempty"`
	// A clock claim's stated estimate and, when its span was measured
	// (not censored), the actual seconds (C5).
	EstimateMinutes int    `json:"estimate_minutes,omitempty"`
	ActualS         *int64 `json:"actual_s,omitempty"`
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
	// Censored counts, per claim type, the desk's clock claims left out
	// because their round ended before the interval closed (C5, C7).
	Censored map[string]int
	// Late is the desk's own late share (C11).
	Late *types.CalibrationLate
}

// DigestExcluded are the claim types left out of every digest (C4):
// paths_confined held 96% of the time, so it pads n and flatters Brier.
var DigestExcluded = map[string]bool{string(types.PrecommitRulePathsConfined): true}

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

// SaltBytes is the length of a computer's secret salt.
const SaltBytes = 32

// A computer's held-out set and experiment arms hash with its own secret
// salt (docs/problems/selfdev-rounds-record-no-predictions-2026-11-04.md):
// a desk that reads the source cannot tell which of its records are held
// out, or which arm it is in. The runtime derives the salt from the guest
// key; the readout takes the arm and held-out mark from the records and the
// view, never by recomputing them.
func saltedByte(salt []byte, domain, value string) byte {
	mac := hmac.New(sha256.New, salt)
	mac.Write([]byte(domain + "\x00" + value))
	return mac.Sum(nil)[0]
}

// HeldOut reports whether an observation key is in the 10% held out of
// every digest: those outcomes are measured but never shown.
func HeldOut(salt []byte, key string) bool {
	return saltedByte(salt, "choir-calibration-heldout-v2", key)%10 == 0
}

// Arm names the experiment arm of a desk thread (a run's agent id; an
// engineering round's is engineering:<assignment>): "digest" or "control",
// fixed for the thread on its computer.
func Arm(salt []byte, thread string) string {
	if saltedByte(salt, "choir-calibration-arm-v2", strings.TrimSpace(thread))%2 == 0 {
		return ArmDigest
	}
	return ArmControl
}

// ArmOfCast names the arm of one management Cast (C2 of
// docs/design/calibration-wiring-permutations-2026-11-04.md): management
// has one long-lived thread per computer, so a split by thread puts every
// management act on a computer in one arm. The unit is the Cast record id,
// in its own hash domain, so a Cast's arm is independent of its engineer's.
func ArmOfCast(salt []byte, castRecordID string) string {
	if saltedByte(salt, "choir-calibration-arm-cast-v1", strings.TrimSpace(castRecordID))%2 == 0 {
		return ArmDigest
	}
	return ArmControl
}

// The two arms.
const (
	ArmDigest  = "digest"
	ArmControl = "control"
)

// Stats is calibration over a set of observations.
type Stats struct {
	N        int
	Brier    float64
	ECE      float64
	MeanP    float64
	Observed float64
	// Resolution is Murphy's: sum over bins of n_k/N (observed_k -
	// observed)^2.
	Resolution float64
	Bins       []types.CalibrationBin
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
		s.Resolution += float64(b.n) / n * (my - s.Observed) * (my - s.Observed)
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
		if o.Desk != desk || DigestExcluded[o.ClaimType] {
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
	d.N, d.Brier, d.ECE, d.MeanP, d.Observed, d.Resolution = all.N, all.Brier, all.ECE, all.MeanP, all.Observed, all.Resolution
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
		line := types.CalibrationStat{Key: k, N: st.N, Brier: st.Brier, ECE: st.ECE, MeanP: st.MeanP, Observed: st.Observed, Resolution: st.Resolution, Censored: opts.Censored[k]}
		for _, b := range st.Bins {
			if b.N >= opts.MinBin {
				line.Bins = append(line.Bins, b)
			}
		}
		line.Timed, line.EstimateRatio = estimateRatio(byType[k], opts.MinBin)
		d.ClaimTypes = append(d.ClaimTypes, line)
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
	if opts.Late != nil && opts.Late.Stated > 0 {
		late := *opts.Late
		d.Late = &late
	}
	if d.N >= opts.MinN {
		d.Text = Render(d)
	}
	return d
}

// estimateRatio is the median of estimate over actual minutes across the
// timed clock claims, and their count; nil under minN timed claims (C5).
func estimateRatio(obs []Observation, minN int) (int, *float64) {
	var ratios []float64
	for _, o := range obs {
		if o.ActualS == nil || o.EstimateMinutes < 1 {
			continue
		}
		actual := math.Max(float64(*o.ActualS)/60, 1.0/60)
		ratios = append(ratios, float64(o.EstimateMinutes)/actual)
	}
	if len(ratios) == 0 || len(ratios) < minN {
		return len(ratios), nil
	}
	sort.Float64s(ratios)
	m := ratios[len(ratios)/2]
	if len(ratios)%2 == 0 {
		m = (ratios[len(ratios)/2-1] + ratios[len(ratios)/2]) / 2
	}
	return len(ratios), &m
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
	fmt.Fprintf(&b, "- Brier %.3f (0 is perfect); resolution %.3f (higher means your probabilities separate the claims that held from those that did not); calibration error %.3f.\n", d.Brier, d.Resolution, d.ECE)
	switch d.Direction {
	case "under-confident":
		fmt.Fprintf(&b, "- Under-confident: your mean probability was %.2f and %.2f of these held.\n", d.MeanP, d.Observed)
	case "over-confident":
		fmt.Fprintf(&b, "- Over-confident: your mean probability was %.2f and only %.2f of these held.\n", d.MeanP, d.Observed)
	default:
		fmt.Fprintf(&b, "- Calibrated on average: mean probability %.2f, %.2f held.\n", d.MeanP, d.Observed)
	}
	// C4: each claim type with its own bins, never pooled.
	for _, st := range d.ClaimTypes {
		fmt.Fprintf(&b, "- %s (n=%d): Brier %.3f, resolution %.3f", nounOf(st.Key), st.N, st.Brier, st.Resolution)
		for _, bin := range st.Bins {
			fmt.Fprintf(&b, "; you said %.2f, %.2f held (n=%d)", bin.MeanP, bin.Observed, bin.N)
		}
		if st.EstimateRatio != nil {
			fmt.Fprintf(&b, "; your estimates were a median %.1fx the actual minutes (n=%d)", *st.EstimateRatio, st.Timed)
		}
		if st.Censored > 0 {
			fmt.Fprintf(&b, "; %d more ended before the interval closed and are not scored", st.Censored)
		}
		b.WriteString(".\n")
	}
	if len(d.ClaimTypes) > 1 && d.Worst != "" {
		for _, st := range d.ClaimTypes {
			if st.Key == d.Worst {
				fmt.Fprintf(&b, "- Least calibrated: %s (said %.2f, %.2f held, n=%d).\n", nounOf(st.Key), st.MeanP, st.Observed, st.N)
			}
		}
	}
	if d.Late != nil && d.Late.Late > 0 {
		fmt.Fprintf(&b, "- Late: %d of your %d claims were stated late, after the work visibly progressed; they are kept and never scored.\n", d.Late.Late, d.Late.Stated)
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
