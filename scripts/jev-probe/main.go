//go:build jevprobe

// Command jev-probe measures Jev latency against question count and
// concurrency, and answer consistency on known-answer questions
// (docs/evidence/jev-measurement-2026-11-10.md, section 3, "Proposed probe").
// It is built only with the jevprobe tag, so no CI job compiles or runs it.
//
// Run it inside the d@e guest, which holds the gateway bearer, never against
// the paper or an owner's computer:
//
//	go run -tags jevprobe ./scripts/jev-probe -gateway "$GATEWAY_URL" -token-file /path/to/bearer
//
// It issues 75 calls (12 size-matrix calls and 63 concurrency calls), paced
// 25 s between trials. With the shared limiter in place the pacing is not
// needed for the rate, but it keeps the cells independent. The gateway log
// line to read afterwards is the hour summary,
// "gateway: jev hour summary: calls=... p50=... p90=...", and the per-call
// "gateway: jev call computer=<d@e> class=bulk questions=N queue_ms=... upstream_ms=...".
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/gatewayruntime"
)

// question is a known-answer question: the parity of a number.
func question(n int) json.RawMessage {
	q, _ := json.Marshal(map[string]any{
		"type":         "choice",
		"instructions": fmt.Sprintf("Is the integer in state.numbers.n%d even or odd?", n),
		"criteria": map[string]string{
			"even": fmt.Sprintf("state.numbers.n%d is divisible by 2.", n),
			"odd":  fmt.Sprintf("state.numbers.n%d is not divisible by 2.", n),
		},
		"choices": []string{"even", "odd"},
	})
	return q
}

func request(k, base int) gatewayruntime.JudgmentRequest {
	nums, qs := map[string]int{}, map[string]json.RawMessage{}
	for i := 0; i < k; i++ {
		n := base + i*7 + 3 // a mix of parities
		nums[fmt.Sprintf("n%d", n)] = n
		qs[fmt.Sprintf("q%d", n)] = question(n)
	}
	state, _ := json.Marshal(map[string]any{"task": "Answer each parity question.", "numbers": nums})
	return gatewayruntime.JudgmentRequest{State: state, Questions: qs}
}

type outcome struct {
	lat                  time.Duration
	right, wrong, missed int
	err                  error
}

func run(ctx context.Context, c *gatewayruntime.JudgmentClient, k, base int) outcome {
	req := request(k, base)
	started := time.Now()
	resp, err := c.Decide(ctx, req)
	o := outcome{lat: time.Since(started), err: err}
	if err != nil {
		return o
	}
	var d struct {
		Answers map[string]struct {
			Choice string `json:"choice"`
		} `json:"answers"`
	}
	_ = json.Unmarshal(resp.Distribution, &d)
	for name := range req.Questions {
		var n int
		_, _ = fmt.Sscanf(strings.TrimPrefix(name, "q"), "%d", &n)
		want := map[bool]string{true: "even", false: "odd"}[n%2 == 0]
		switch a, ok := d.Answers[name]; {
		case !ok:
			o.missed++
		case a.Choice == want:
			o.right++
		default:
			o.wrong++
		}
	}
	return o
}

func percentile(lats []time.Duration, p float64) time.Duration {
	if len(lats) == 0 {
		return 0
	}
	sorted := append([]time.Duration(nil), lats...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	return sorted[min(len(sorted)-1, int(p*float64(len(sorted))))]
}

func main() {
	gateway := flag.String("gateway", os.Getenv("GATEWAY_URL"), "gateway base URL")
	tokenFile := flag.String("token-file", "", "file holding the guest bearer")
	pace := flag.Duration("pace", 25*time.Second, "sleep between trials")
	flag.Parse()
	token, err := os.ReadFile(*tokenFile)
	if err != nil || *gateway == "" {
		fmt.Fprintln(os.Stderr, "usage: jev-probe -gateway URL -token-file FILE")
		os.Exit(2)
	}
	client := gatewayruntime.NewJudgmentClient(*gateway, strings.TrimSpace(string(token)))
	ctx := context.Background()

	// Size matrix: latency and accuracy against questions per call. Sizes
	// 32 and 64 exceed System1MaxQuestions, which is a cell-side bound; the
	// gateway does not cap them. A body refusal or a 400 is recorded as an
	// error: copy its text into the lane A ledger.
	for _, k := range []int{1, 8, 32, 64} {
		var lats []time.Duration
		for trial := 0; trial < 3; trial++ {
			o := run(ctx, client, k, 1000*(trial+1))
			lats = append(lats, o.lat)
			fmt.Printf("size k=%d trial=%d lat=%s right=%d wrong=%d missed=%d err=%v\n", k, trial, o.lat, o.right, o.wrong, o.missed, o.err)
			time.Sleep(*pace)
		}
		fmt.Printf("size k=%d p50=%s p90=%s\n", k, percentile(lats, 0.5), percentile(lats, 0.9))
	}

	// Concurrency: N single-question calls at once.
	for _, n := range []int{1, 4, 16} {
		for trial := 0; trial < 3; trial++ {
			results := make([]outcome, n)
			var wg sync.WaitGroup
			started := time.Now()
			for i := 0; i < n; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					results[i] = run(ctx, client, 1, 5000+i*13)
				}(i)
			}
			wg.Wait()
			var lats []time.Duration
			failed := 0
			for _, r := range results {
				lats = append(lats, r.lat)
				if r.err != nil {
					failed++
				}
			}
			fmt.Printf("conc N=%d trial=%d wall=%s p50=%s p90=%s failed=%d\n", n, trial, time.Since(started), percentile(lats, 0.5), percentile(lats, 0.9), failed)
			time.Sleep(*pace)
		}
	}
}
