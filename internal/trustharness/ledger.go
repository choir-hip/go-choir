package trustharness

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// The mission's own precommitment ledger (MA7.9 (0);
// docs/definitions/choir-modular-architecture-2026-11-04.precommit.md). The
// records and their resolution classes are parsed from the ledger's text.
// The settled values a score needs and the ledger states only in prose (an
// observed number, the outcome of a qualified settlement, who settled) come
// from a transcription file, each entry quoting the ledger sentence it
// transcribes; an entry whose quote is not in the ledger, or that contradicts
// the parsed resolution, is refused and reported, never scored.

// LedgerInput is the parsed ledger.
type LedgerInput struct {
	Path     string         `json:"path"`
	SHA256   string         `json:"sha256"`
	Records  []LedgerRecord `json:"records"`
	Problems []string       `json:"problems,omitempty"`
}

// LedgerRecord is one record and, when settled, its settlement.
type LedgerRecord struct {
	ID string `json:"id"`
	// Kind is "probability", "interval" (a point estimate with an 80%
	// interval) or "other".
	Kind     string   `json:"kind"`
	P        *float64 `json:"p,omitempty"`
	Estimate *float64 `json:"estimate,omitempty"`
	Lo       *float64 `json:"lo,omitempty"`
	Hi       *float64 `json:"hi,omitempty"`
	// Resolution is open, confirmed, contradicted, qualified, unresolved or
	// void.
	Resolution string `json:"resolution"`
	// Settler is rule, devin or owner (from the transcription).
	Settler string `json:"settler,omitempty"`
	// Outcome is 1 or 0 for a settled probability record; Observed the
	// settled value of an interval record.
	Outcome  *float64 `json:"outcome,omitempty"`
	Observed *float64 `json:"observed,omitempty"`
	Quote    string   `json:"quote,omitempty"`
}

// Transcription is one entry of the transcription file.
type Transcription struct {
	ID       string   `json:"id"`
	Settler  string   `json:"settler"`
	Outcome  *float64 `json:"outcome,omitempty"`
	Observed *float64 `json:"observed,omitempty"`
	Quote    string   `json:"quote"`
}

var (
	reRecord     = regexp.MustCompile(`^\*\*(R\d+[a-z]?-\d+)\.\*\*`)
	reSettlement = regexp.MustCompile(`^\*\*(R\d+[a-z]?-\d+), (settled (confirmed|contradicted|qualified)|unresolved|void)`)
	reP          = regexp.MustCompile(`(?m)^- p = ([0-9.]+)`)
	reEstimate   = regexp.MustCompile(`(?m)^- Estimate: ([^\n]*?)80% interval \[([^\]]+)\]`)
	reNumber     = regexp.MustCompile(`-?[0-9]+(?:\.[0-9]+)?`)
	reSpace      = regexp.MustCompile(`\s+`)
)

func num(s string) (float64, bool) {
	m := reNumber.FindString(s)
	if m == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(m, 64)
	return v, err == nil
}

// ParseLedger parses the ledger text and applies the transcriptions.
func ParseLedger(path, text string, trans []Transcription) LedgerInput {
	sum := sha256.Sum256([]byte(text))
	out := LedgerInput{Path: path, SHA256: hex.EncodeToString(sum[:]), Records: []LedgerRecord{}}
	byID := map[string]int{}
	lines := strings.Split(text, "\n")
	for i := 0; i < len(lines); i++ {
		if m := reSettlement.FindStringSubmatch(lines[i]); m != nil {
			res := m[3]
			if res == "" {
				res = m[2]
			}
			j, ok := byID[m[1]]
			if !ok {
				out.Problems = append(out.Problems, fmt.Sprintf("%s: a settlement of no parsed record", m[1]))
				continue
			}
			if out.Records[j].Resolution != "open" {
				out.Problems = append(out.Problems, fmt.Sprintf("%s: settled twice (%s, then %s); the first stands", m[1], out.Records[j].Resolution, res))
				continue
			}
			out.Records[j].Resolution = res
			continue
		}
		m := reRecord.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}
		body := []string{lines[i]}
		for k := i + 1; k < len(lines); k++ {
			l := lines[k]
			if strings.HasPrefix(l, "**") || strings.HasPrefix(l, "#") {
				break
			}
			body = append(body, l)
		}
		b := strings.Join(body, "\n")
		rec := LedgerRecord{ID: m[1], Kind: "other", Resolution: "open"}
		if pm := reP.FindStringSubmatch(b); pm != nil {
			if v, ok := num(pm[1]); ok {
				rec.Kind, rec.P = "probability", &v
			}
		} else if em := reEstimate.FindStringSubmatch(b); em != nil {
			e, ok1 := num(em[1])
			parts := strings.Split(em[2], ",")
			if ok1 && len(parts) == 2 {
				lo, ok2 := num(parts[0])
				hi, ok3 := num(parts[1])
				if ok2 && ok3 {
					rec.Kind, rec.Estimate, rec.Lo, rec.Hi = "interval", &e, &lo, &hi
				}
			}
		}
		if _, dup := byID[rec.ID]; dup {
			out.Problems = append(out.Problems, fmt.Sprintf("%s: committed twice; the first stands", rec.ID))
			continue
		}
		byID[rec.ID] = len(out.Records)
		out.Records = append(out.Records, rec)
	}
	flat := reSpace.ReplaceAllString(text, " ")
	for _, t := range trans {
		j, ok := byID[t.ID]
		if !ok {
			out.Problems = append(out.Problems, fmt.Sprintf("%s: transcription of no parsed record", t.ID))
			continue
		}
		rec := &out.Records[j]
		if q := reSpace.ReplaceAllString(strings.TrimSpace(t.Quote), " "); q == "" || !strings.Contains(flat, q) {
			out.Problems = append(out.Problems, fmt.Sprintf("%s: transcription quote not found in the ledger; not scored", t.ID))
			continue
		}
		if err := applyTranscription(rec, t); err != nil {
			out.Problems = append(out.Problems, fmt.Sprintf("%s: %v; not scored", t.ID, err))
		}
	}
	for i := range out.Records {
		r := &out.Records[i]
		if r.Kind == "probability" && r.Outcome == nil && (r.Resolution == "confirmed" || r.Resolution == "contradicted") {
			v := 0.0
			if r.Resolution == "confirmed" {
				v = 1
			}
			r.Outcome = &v
		}
	}
	sort.Strings(out.Problems)
	return out
}

func applyTranscription(rec *LedgerRecord, t Transcription) error {
	switch rec.Resolution {
	case "confirmed", "contradicted", "qualified":
	default:
		return fmt.Errorf("transcribed but the ledger resolution is %s", rec.Resolution)
	}
	s := strings.ToLower(strings.TrimSpace(t.Settler))
	if s != "rule" && s != "devin" && s != "owner" {
		return fmt.Errorf("settler %q is not rule, devin or owner", t.Settler)
	}
	switch rec.Kind {
	case "probability":
		if t.Observed != nil {
			return fmt.Errorf("an observed value on a probability record")
		}
		if t.Outcome != nil {
			o := *t.Outcome
			if o != 0 && o != 1 {
				return fmt.Errorf("outcome %v is not 0 or 1", o)
			}
			if (rec.Resolution == "confirmed" && o != 1) || (rec.Resolution == "contradicted" && o != 0) {
				return fmt.Errorf("outcome %v contradicts the ledger's %s", o, rec.Resolution)
			}
		} else if rec.Resolution == "qualified" {
			return fmt.Errorf("a qualified settlement needs its outcome transcribed")
		}
		rec.Outcome = t.Outcome
	case "interval":
		if t.Observed == nil {
			return fmt.Errorf("no observed value")
		}
		x := *t.Observed
		inside := x >= *rec.Lo && x <= *rec.Hi
		if (rec.Resolution == "confirmed" && !inside) || (rec.Resolution == "contradicted" && inside) {
			return fmt.Errorf("observed %v contradicts the ledger's %s against [%v, %v]", x, rec.Resolution, *rec.Lo, *rec.Hi)
		}
		rec.Observed = t.Observed
	default:
		return fmt.Errorf("record states neither a probability nor an interval")
	}
	rec.Settler, rec.Quote = s, strings.TrimSpace(t.Quote)
	return nil
}
