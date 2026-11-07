package trust

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/modelcatalog"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
)

// Anchor settlement by the strongest judge, and the sampler's durable log
// (plan 10.2, MA7.10). Nothing here is shown to a desk. A judge is asked the
// item's closed question and nothing else: its prompt does not say the item
// was drawn by a sampler, or that a scorer is being graded.

// ErrUnsettled means the strongest judge did not produce a typed answer, or
// the top strength split. The question stays unknown; it is never settled by
// a weaker judge in the strongest's place.
var ErrUnsettled = errors.New("trust: the anchor is unsettled")

// ErrJudgeUnavailable marks an unsettled anchor whose cause may pass: the
// judge's call failed (a transport error, a stop, a cancelled context). The
// item is not decided; a later pass may ask again. Every other cause of
// ErrUnsettled (a split, an answer outside the closed set, prose) is final.
var ErrJudgeUnavailable = errors.New("trust: the judge is unavailable")

// ErrBadAnswer marks a judge's answer that is not a typed answer. It is a
// property of the answer, not of the call, so it is final.
var ErrBadAnswer = errors.New("trust: the judge's answer is not typed")

// JudgeQuestion is a closed question about one item.
type JudgeQuestion struct {
	ItemID string
	Prompt string
	// Answers is the closed set of admissible answers.
	Answers []string
}

// Judge answers a JudgeQuestion. Strength orders judges (a panel above a
// single model, the owner on preference questions above both).
type Judge interface {
	Name() string
	Strength() int
	Judge(ctx context.Context, q JudgeQuestion) (string, error)
}

// SettleByJudges settles the item by the strongest configured judges: only
// the judges at the top strength are asked. Each must give an answer from
// the closed set; an error or an answer outside it from any of them leaves
// the item unsettled, as does a split. A weaker judge never stands in.
func SettleByJudges(ctx context.Context, q JudgeQuestion, kind AnchorKind, judges []Judge, at time.Time) (Anchor, error) {
	if len(q.Answers) == 0 {
		return Anchor{}, errors.New("trust: a judge question needs a closed answer set")
	}
	if len(judges) == 0 {
		return Anchor{}, fmt.Errorf("%w: no judge", ErrUnsettled)
	}
	top := judges[0].Strength()
	for _, j := range judges {
		if j.Strength() > top {
			top = j.Strength()
		}
	}
	var votes []AnchorVote
	for _, j := range judges {
		if j.Strength() != top {
			continue
		}
		if err := ctx.Err(); err != nil {
			return Anchor{}, fmt.Errorf("%w: %w: %v", ErrUnsettled, ErrJudgeUnavailable, err)
		}
		ans, err := j.Judge(ctx, q)
		if err != nil {
			if errors.Is(err, ErrBadAnswer) {
				return Anchor{}, fmt.Errorf("%w: %s: %v", ErrUnsettled, j.Name(), err)
			}
			return Anchor{}, fmt.Errorf("%w: %w: %s: %v", ErrUnsettled, ErrJudgeUnavailable, j.Name(), err)
		}
		if !hasAnswer(q.Answers, ans) {
			return Anchor{}, fmt.Errorf("%w: %s answered outside the closed set", ErrUnsettled, j.Name())
		}
		votes = append(votes, AnchorVote{Judge: j.Name(), Strength: top, Answer: ans})
	}
	a, ok := SettleAnchor(q.ItemID, kind, votes, at)
	if !ok {
		return Anchor{}, fmt.Errorf("%w: the top strength split", ErrUnsettled)
	}
	return a, nil
}

// JudgeCaller is the provider surface a judge calls: the gateway-backed
// ToolLoopProvider, the same one the install review seats use.
type JudgeCaller interface {
	CallWithTools(ctx context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error)
}

// ProviderJudge is a judge that calls one catalogued (provider, model)
// through the gateway. An uncatalogued model or a missing caller never
// answers.
type ProviderJudge struct {
	name, provider, model string
	strength              int
	caller                JudgeCaller
}

// NewProviderJudge names a judge on (provider, model).
func NewProviderJudge(name, provider, model string, strength int, caller JudgeCaller) *ProviderJudge {
	return &ProviderJudge{name: name, provider: provider, model: model, strength: strength, caller: caller}
}

func (p *ProviderJudge) Name() string  { return p.name }
func (p *ProviderJudge) Strength() int { return p.strength }

const judgeSystem = `Answer the question about the item below with exactly one of the allowed answers, as a bare word or as {"answer": "<word>"}. Say nothing else.`

// Judge asks the model and reads its typed answer.
func (p *ProviderJudge) Judge(ctx context.Context, q JudgeQuestion) (string, error) {
	if p.caller == nil || !modelcatalog.ProviderCatalogued(p.provider) {
		return "", fmt.Errorf("trust: judge %s has no provider", p.name)
	}
	if _, ok := modelcatalog.Lookup(p.provider, p.model); !ok {
		return "", fmt.Errorf("trust: judge %s: %s/%s is not catalogued", p.name, p.provider, p.model)
	}
	text := q.Prompt + "\n\nAllowed answers: " + strings.Join(q.Answers, ", ")
	msg, err := json.Marshal(map[string]any{"role": "user", "content": []any{map[string]string{"type": "text", "text": text}}})
	if err != nil {
		return "", err
	}
	resp, err := p.caller.CallWithTools(ctx, provideriface.ToolLoopRequest{
		Provider: p.provider, Model: p.model, ConversationID: opaqueConversationID(p.name),
		System: judgeSystem, Messages: []json.RawMessage{msg},
	})
	if err != nil {
		return "", fmt.Errorf("trust: judge %s: %w", p.name, err)
	}
	return ParseJudgeAnswer(resp.Text, q.Answers)
}

// opaqueConversationID names a call without naming the item it is about.
func opaqueConversationID(judge string) string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "judge-" + judge + "-" + hex.EncodeToString(b[:])
}

// ParseJudgeAnswer reads a typed answer: a bare allowed word (case and a
// closing full stop aside) or a JSON object with an "answer" field. Prose
// around an answer is not an answer.
func ParseJudgeAnswer(text string, allowed []string) (string, error) {
	t := strings.TrimSpace(text)
	if strings.HasPrefix(t, "{") {
		var o struct {
			Answer string `json:"answer"`
		}
		if err := json.Unmarshal([]byte(t), &o); err != nil {
			return "", ErrBadAnswer
		}
		t = o.Answer
	}
	t = strings.ToLower(strings.TrimSpace(strings.TrimRight(strings.TrimSpace(t), ".")))
	for _, a := range allowed {
		if strings.ToLower(a) == t {
			return a, nil
		}
	}
	return "", fmt.Errorf("%w: not one of the allowed answers", ErrBadAnswer)
}

// SpotLog is the sampler's append-only host log: one JSON line per decision,
// synced on write. It lives apart from every record and every desk.
type SpotLog struct {
	mu sync.Mutex
	f  *os.File
}

// ErrSpotLogCorrupt marks a log with a bad line that is not a torn tail.
var ErrSpotLogCorrupt = errors.New("trust: the spot log is corrupt")

// OpenSpotLog opens (creating) the log for appending. A torn tail (an
// unterminated final line that does not parse, left by a short write or a
// power loss) is truncated first; a valid unterminated last line gets its
// newline. A new file's directory entry is synced.
func OpenSpotLog(path string) (*SpotLog, error) {
	_, valid, torn, err := loadSpotLog(path)
	if err != nil {
		return nil, err
	}
	_, statErr := os.Stat(path)
	created := os.IsNotExist(statErr)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	if torn {
		if err := f.Truncate(valid); err != nil {
			f.Close()
			return nil, err
		}
	} else if st, err := f.Stat(); err == nil && st.Size() > 0 {
		last := make([]byte, 1)
		if rf, err := os.Open(path); err == nil {
			_, _ = rf.ReadAt(last, st.Size()-1)
			rf.Close()
			if last[0] != '\n' {
				if _, err := f.Write([]byte{'\n'}); err != nil {
					f.Close()
					return nil, err
				}
			}
		}
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return nil, err
	}
	if created {
		if d, err := os.Open(filepath.Dir(path)); err == nil {
			_ = d.Sync()
			d.Close()
		}
	}
	return &SpotLog{f: f}, nil
}

// Append writes one decision and syncs it.
func (l *SpotLog) Append(d SpotDecision) error {
	b, err := json.Marshal(d)
	if err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, err := l.f.Write(append(b, '\n')); err != nil {
		return err
	}
	return l.f.Sync()
}

// Close closes the log.
func (l *SpotLog) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.f.Close()
}

// LoadSpotLog reads every decision. A missing log is empty. A final line with
// no newline that does not parse is a torn tail and is ignored; a bad line
// anywhere else refuses the load, since a skipped line is a silently missing
// decision. Fields this build does not know are ignored, so a log a newer
// build wrote still loads after a rollback.
func LoadSpotLog(path string) ([]SpotDecision, error) {
	out, _, _, err := loadSpotLog(path)
	return out, err
}

// loadSpotLog also returns the byte length of the valid prefix and whether a
// torn tail follows it.
func loadSpotLog(path string) ([]SpotDecision, int64, bool, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, 0, false, nil
	}
	if err != nil {
		return nil, 0, false, err
	}
	var out []SpotDecision
	var valid int64
	for n, rest := 1, raw; len(rest) > 0; n++ {
		line, tail := rest, []byte(nil)
		terminated := false
		if i := bytes.IndexByte(rest, '\n'); i >= 0 {
			line, tail, terminated = rest[:i], rest[i+1:], true
		}
		var d SpotDecision
		if err := json.Unmarshal(line, &d); err != nil || d.RecordID == "" {
			if !terminated {
				return out, valid, true, nil
			}
			return nil, 0, false, fmt.Errorf("%w: line %d is not a decision", ErrSpotLogCorrupt, n)
		}
		out = append(out, d)
		valid += int64(len(line))
		if terminated {
			valid++
		}
		rest = tail
	}
	return out, valid, false, nil
}

// SpotLogStale reports whether the log was written under another seed or
// another rate than the sampler's: it cannot be replayed, and it is not
// evidence of tampering. A log with a draw that does not reproduce under the
// same seed is not stale.
func SpotLogStale(s *SpotSampler, entries []SpotDecision) bool {
	for _, e := range entries {
		if e.SeedID != s.SeedID() || e.Rate != s.Rate {
			return true
		}
	}
	return false
}

// RotateSpotLog renames the log aside to <path>.stale-<time> and returns the
// new name, so a fresh log starts and the old entries stay inspectable.
func RotateSpotLog(path string, at time.Time) (string, error) {
	aside := path + ".stale-" + at.UTC().Format("20060102T150405.000000000Z")
	for n := 1; ; n++ {
		if _, err := os.Lstat(aside); os.IsNotExist(err) {
			break
		}
		aside = fmt.Sprintf("%s.stale-%s-%d", path, at.UTC().Format("20060102T150405.000000000Z"), n)
	}
	if err := os.Rename(path, aside); err != nil {
		return "", err
	}
	if d, err := os.Open(filepath.Dir(path)); err == nil {
		_ = d.Sync()
		d.Close()
	}
	return aside, nil
}

// VerifySpotLog replays every logged decision against the sampler and names
// each that does not reproduce: another seed, a different draw, or a sampled
// flag that does not follow from the draw and the logged rate.
func VerifySpotLog(s *SpotSampler, entries []SpotDecision) []string {
	var bad []string
	decidedTwice := map[string]bool{}
	for _, e := range entries {
		if e.Deferral == 0 {
			if decidedTwice[e.RecordID] {
				bad = append(bad, e.RecordID+": decided twice")
				continue
			}
			decidedTwice[e.RecordID] = true
		}
		switch {
		case e.SeedID != s.SeedID():
			bad = append(bad, e.RecordID+": logged under another seed")
		case e.Rate != s.Rate:
			bad = append(bad, e.RecordID+": logged at another rate")
		case e.Draw != s.Draw(e.RecordID):
			bad = append(bad, e.RecordID+": the draw does not reproduce")
		case e.Sampled != (e.Draw < e.Rate):
			bad = append(bad, e.RecordID+": the sampled flag does not follow from the draw")
		}
	}
	sort.Strings(bad)
	return bad
}
