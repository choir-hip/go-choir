package trust

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sort"
	"time"
)

// Anchors and scorer grading (plan 10.2, MA7.10). Every score comes from some
// scorer, and scorers are scored. The regress stops at an anchor: a rule, an
// unannounced random spot-check settled slowly by the strongest judge
// available, or a world-resolved prediction. Two levels, no more: scorers are
// graded against anchors; anchors are not graded.
//
// Nothing in this file touches a desk's context, a pack or a record. The
// sampler returns a log entry that lives apart from the records it draws
// from; any marker a scorer can see would defeat it.

// hostDraw is a uniform draw in [0,1) that is a pure function of the host
// secret, a purpose and an id: HMAC-SHA256, first 53 bits.
func hostDraw(seed []byte, purpose, id string) float64 {
	m := hmac.New(sha256.New, seed)
	m.Write([]byte(purpose))
	m.Write([]byte{0})
	m.Write([]byte(id))
	sum := m.Sum(nil)
	return float64(binary.BigEndian.Uint64(sum[:8])>>11) / float64(1<<53)
}

func hostIndex(seed []byte, purpose, id string, n int) int {
	m := hmac.New(sha256.New, seed)
	m.Write([]byte(purpose))
	m.Write([]byte{0})
	m.Write([]byte(id))
	sum := m.Sum(nil)
	return int(binary.BigEndian.Uint64(sum[8:16]) % uint64(n))
}

func seedID(seed []byte) string {
	sum := sha256.Sum256(append([]byte("seed-id:"), seed...))
	return hex.EncodeToString(sum[:6])
}

// MinSeedBytes is the shortest host seed accepted.
const MinSeedBytes = 16

// SpotSampler draws the unannounced spot-check sample from live records. It
// is seeded by a host secret no desk holds, so which records are checked is
// not predictable, and it is a pure function of (seed, record id), so a replay
// draws the same sample.
type SpotSampler struct {
	seed []byte
	Rate float64
}

// NewSpotSampler builds a sampler with the host seed and the share of live
// records to draw, in (0, 1].
func NewSpotSampler(seed []byte, rate float64) (*SpotSampler, error) {
	if len(seed) < MinSeedBytes {
		return nil, fmt.Errorf("trust: spot-check seed needs at least %d bytes", MinSeedBytes)
	}
	if !(rate > 0 && rate <= 1) {
		return nil, fmt.Errorf("trust: spot-check rate %v is outside (0, 1]", rate)
	}
	return &SpotSampler{seed: append([]byte(nil), seed...), Rate: rate}, nil
}

// SeedID is a fingerprint of the seed for the log; the seed itself is never
// logged.
func (s *SpotSampler) SeedID() string { return seedID(s.seed) }

// Draw is the record's draw in [0,1).
func (s *SpotSampler) Draw(recordID string) float64 { return hostDraw(s.seed, "spot", recordID) }

// SpotDecision is the sampler's log entry for one record. It is the host's
// own log: it is never written onto the record and never shown to a desk.
type SpotDecision struct {
	RecordID string    `json:"record_id"`
	Sampled  bool      `json:"sampled"`
	Draw     float64   `json:"draw"`
	Rate     float64   `json:"rate"`
	SeedID   string    `json:"seed_id"`
	At       time.Time `json:"at"`
}

// Decide draws for a record. at is when the host drew (logged, and not part
// of the draw).
func (s *SpotSampler) Decide(recordID string, at time.Time) SpotDecision {
	d := s.Draw(recordID)
	return SpotDecision{RecordID: recordID, Sampled: d < s.Rate, Draw: d, Rate: s.Rate, SeedID: s.SeedID(), At: at}
}

// AnchorKind says what kind of anchor settled an item.
type AnchorKind string

const (
	AnchorRule      AnchorKind = "rule"
	AnchorSpotCheck AnchorKind = "spot_check"
	AnchorWorld     AnchorKind = "world"
)

// AnchorVote is one judge's answer on an anchored item. Strength orders the
// judges: a panel above a single model, the owner on preference questions
// above both.
type AnchorVote struct {
	Judge    string
	Strength int
	Answer   string
}

// Anchor is the settled truth of an item that scorers are graded against.
type Anchor struct {
	ItemID string     `json:"item_id"`
	Kind   AnchorKind `json:"kind"`
	Truth  string     `json:"truth"`
	Judges []string   `json:"judges,omitempty"`
	// Agreement is the share of all judges that matched the truth.
	Agreement float64   `json:"agreement"`
	SettledAt time.Time `json:"settled_at"`
}

// SettleAnchor settles an item by the strongest judge available: among the
// votes at the highest strength, the plurality answer. A split at the top
// strength, or no votes, settles nothing, and the question stays "unknown".
func SettleAnchor(itemID string, kind AnchorKind, votes []AnchorVote, at time.Time) (Anchor, bool) {
	if len(votes) == 0 {
		return Anchor{}, false
	}
	top := votes[0].Strength
	for _, v := range votes {
		if v.Strength > top {
			top = v.Strength
		}
	}
	tally := map[string]int{}
	for _, v := range votes {
		if v.Strength == top {
			tally[v.Answer]++
		}
	}
	best, bn, tie := "", 0, false
	answers := make([]string, 0, len(tally))
	for a := range tally {
		answers = append(answers, a)
	}
	sort.Strings(answers)
	for _, a := range answers {
		switch {
		case tally[a] > bn:
			best, bn, tie = a, tally[a], false
		case tally[a] == bn:
			tie = true
		}
	}
	if tie {
		return Anchor{}, false
	}
	a := Anchor{ItemID: itemID, Kind: kind, Truth: best, SettledAt: at}
	match := 0
	for _, v := range votes {
		a.Judges = append(a.Judges, v.Judge)
		if v.Answer == best {
			match++
		}
	}
	sort.Strings(a.Judges)
	a.Agreement = float64(match) / float64(len(votes))
	return a, true
}
