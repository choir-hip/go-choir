// Command claimslab is the E0/E6a laboratory's arithmetic over JSON on
// stdin, so the study scripts reuse the runtime's quote checker and one
// implementation of the scoring math. It reads files, never a store.
//
//	claimslab verify < {"items":[{"key","blocks":[{"id","text"}],"quotes":[{"id","quote","hint"}]}]}
//	claimslab match  < {"min_overlap":0.5,"items":[{"key","gold":[Span],"pred":[Span]}]}
//	claimslab rate   < {"groups":{"name":{"w":[...],"y":[...]}}}
//	claimslab brier  < {"groups":{"name":{"p":[...],"y":[...]}}}
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/yusefmosiah/go-choir/internal/claimslab"
	"github.com/yusefmosiah/go-choir/internal/quotematch"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: claimslab verify|match|rate|brier < input.json")
		os.Exit(2)
	}
	dec := json.NewDecoder(os.Stdin)
	enc := json.NewEncoder(os.Stdout)
	var err error
	switch os.Args[1] {
	case "verify":
		err = verify(dec, enc)
	case "match":
		err = match(dec, enc)
	case "rate":
		err = rate(dec, enc)
	case "brier":
		err = brier(dec, enc)
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "claimslab:", err)
		os.Exit(1)
	}
}

func verify(dec *json.Decoder, enc *json.Encoder) error {
	var in struct {
		// NoCap checks quotes of any length (desk quote fidelity, E6a R1).
		NoCap bool `json:"no_cap"`
		Items []struct {
			Key    string `json:"key"`
			Blocks []struct {
				ID   string `json:"id"`
				Text string `json:"text"`
			} `json:"blocks"`
			Quotes []struct {
				ID    string `json:"id"`
				Quote string `json:"quote"`
				Hint  string `json:"hint"`
			} `json:"quotes"`
		} `json:"items"`
	}
	if err := dec.Decode(&in); err != nil {
		return err
	}
	type row struct {
		Key   string               `json:"key"`
		ID    string               `json:"id"`
		Check claimslab.QuoteCheck `json:"check"`
	}
	out := struct {
		Matcher string `json:"matcher"`
		Results []row  `json:"results"`
	}{Matcher: quotematch.Version, Results: []row{}}
	for _, it := range in.Items {
		blocks := make([]quotematch.Block, len(it.Blocks))
		for i, b := range it.Blocks {
			blocks[i] = quotematch.Block{ID: b.ID, Text: b.Text}
		}
		for _, q := range it.Quotes {
			capRunes := claimslab.QuoteCap
			if in.NoCap {
				capRunes = 0
			}
			out.Results = append(out.Results, row{Key: it.Key, ID: q.ID, Check: claimslab.CheckQuoteCap(q.Quote, q.Hint, blocks, capRunes)})
		}
	}
	return enc.Encode(out)
}

func match(dec *json.Decoder, enc *json.Encoder) error {
	var in struct {
		MinOverlap float64 `json:"min_overlap"`
		Items      []struct {
			Key  string           `json:"key"`
			Gold []claimslab.Span `json:"gold"`
			Pred []claimslab.Span `json:"pred"`
		} `json:"items"`
	}
	if err := dec.Decode(&in); err != nil {
		return err
	}
	if in.MinOverlap <= 0 {
		in.MinOverlap = 0.5
	}
	type row struct {
		Key string              `json:"key"`
		M   claimslab.SpanMatch `json:"match"`
	}
	var rows []row
	var tp, g, p int
	for _, it := range in.Items {
		m := claimslab.MatchSpans(it.Gold, it.Pred, in.MinOverlap)
		rows = append(rows, row{it.Key, m})
		tp, g, p = tp+m.TruePositives, g+m.Gold, p+m.Predicted
	}
	total := claimslab.SpanMatch{Gold: g, Predicted: p, TruePositives: tp}
	return enc.Encode(map[string]any{"items": rows, "total": total, "precision": total.Precision(), "recall": total.Recall()})
}

func rate(dec *json.Decoder, enc *json.Encoder) error {
	var in struct {
		Groups map[string]struct {
			W []float64 `json:"w"`
			Y []float64 `json:"y"`
		} `json:"groups"`
	}
	if err := dec.Decode(&in); err != nil {
		return err
	}
	out := map[string]claimslab.Rate{}
	for k, g := range in.Groups {
		out[k] = claimslab.WeightedRate(g.W, g.Y)
	}
	return enc.Encode(out)
}

func brier(dec *json.Decoder, enc *json.Encoder) error {
	var in struct {
		Groups map[string]struct {
			P []float64 `json:"p"`
			Y []float64 `json:"y"`
		} `json:"groups"`
	}
	if err := dec.Decode(&in); err != nil {
		return err
	}
	out := map[string]claimslab.BrierResult{}
	for k, g := range in.Groups {
		out[k] = claimslab.Brier(g.P, g.Y)
	}
	return enc.Encode(out)
}
