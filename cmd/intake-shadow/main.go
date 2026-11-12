// Command intake-shadow compares the two ways intake labelling can ask Jev:
// the old one (stories cut into 15,000-byte slices, a call closed at 30,000
// bytes and 6 captures) against one state sized by the vendor's token limits.
//
//	intake-shadow -input flush.json                  # plan only: free, no call
//	intake-shadow -synthetic-stories 450 -synthetic-captures 6
//	intake-shadow -input flush.json -run             # PAID: both arms ask Jev
//
// The plan form prints, for each arm, the calls, questions, request bytes and
// estimated input tokens of the flush, and the cost of that at $0.042 per
// million input tokens. The run form also asks Jev through the gateway
// (RUNTIME_GATEWAY_URL and the guest's gateway token, as the runtime does; run
// it on d@e, never on the paper) and reports the billed tokens and the label
// agreement per question. -json prints the report as JSON.
//
// The input is one flush: {"stories":[{"doc_id","headline","lede",
// "updated_at"}],"captures":[{"capture_id","title","summary","outlet",
// "language"}]}. The jev-calls record keeps digests, not state, so a recorded
// input is regenerated from the story index and the held captures.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/gatewayruntime"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
)

func main() {
	input := flag.String("input", "", "a flush as JSON (stories and captures)")
	stories := flag.Int("synthetic-stories", 0, "plan a synthetic window of this many stories instead of -input")
	captures := flag.Int("synthetic-captures", 6, "captures in a synthetic flush")
	run := flag.Bool("run", false, "PAID: ask Jev with both arms and report agreement (default: plan only, no call)")
	asJSON := flag.Bool("json", false, "print the report as JSON")
	flag.Parse()

	in, err := load(*input, *stories, *captures)
	if err != nil {
		fmt.Fprintln(os.Stderr, "intake-shadow:", err)
		os.Exit(2)
	}
	var rep agentcore.IntakeShadowReport
	if *run {
		url := strings.TrimSpace(os.Getenv("RUNTIME_GATEWAY_URL"))
		if url == "" {
			fmt.Fprintln(os.Stderr, "intake-shadow: -run needs RUNTIME_GATEWAY_URL (and the guest's gateway token); it is a paid run for d@e")
			os.Exit(2)
		}
		judge := gatewayJudge{client: gatewayruntime.NewJudgmentClient(url, provideriface.GatewayToken())}
		rep, err = agentcore.RunIntakeShadow(context.Background(), judge, in)
	} else {
		rep, err = agentcore.PlanIntakeShadow(in)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "intake-shadow:", err)
		os.Exit(1)
	}
	if *asJSON {
		out, _ := json.MarshalIndent(rep, "", "  ")
		fmt.Println(string(out))
		return
	}
	printReport(rep, len(in.Stories), len(in.Captures))
}

func load(path string, stories, captures int) (agentcore.IntakeShadowInput, error) {
	var in agentcore.IntakeShadowInput
	switch {
	case path != "":
		raw, err := os.ReadFile(path)
		if err != nil {
			return in, err
		}
		if err := json.Unmarshal(raw, &in); err != nil {
			return in, fmt.Errorf("%s: %w", path, err)
		}
	case stories > 0:
		now := time.Now().UTC()
		for i := 0; i < stories; i++ {
			in.Stories = append(in.Stories, agentcore.IntakeShadowStory{DocID: fmt.Sprintf("doc-%03d", i), Headline: fmt.Sprintf("Story headline %03d about the matter at hand", i), UpdatedAt: now.Add(-time.Duration(i) * time.Minute)})
		}
		for i := 0; i < captures; i++ {
			in.Captures = append(in.Captures, agentcore.IntakeShadowCapture{CaptureID: fmt.Sprintf("cap-%02d", i), Title: fmt.Sprintf("Capture %d", i), Summary: "A short summary of the capture.", Outlet: "Wire", Language: "en"})
		}
	default:
		return in, fmt.Errorf("give -input flush.json or -synthetic-stories N")
	}
	if len(in.Captures) == 0 {
		return in, fmt.Errorf("the flush has no captures")
	}
	return in, nil
}

func printReport(rep agentcore.IntakeShadowReport, stories, captures int) {
	fmt.Printf("flush: %d captures over %d stories\n\n", captures, stories)
	for _, arm := range []agentcore.IntakeShadowArm{rep.Old, rep.New} {
		fmt.Printf("%s\n  calls %d, questions %d (most in a call %d), request %d bytes, estimated input %d tokens = $%.5f at $0.042 per million\n",
			arm.Name, arm.Calls, arm.Questions, arm.MostQuestions, arm.Bytes, arm.EstTokens, arm.EstCostUSD)
		if rep.Run {
			fmt.Printf("  billed: %d input tokens, $%.5f\n", arm.InputTokens, arm.CostUSD)
		}
	}
	if rep.Old.Calls > 0 {
		fmt.Printf("\ncalls %d -> %d, estimated input tokens %d -> %d\n", rep.Old.Calls, rep.New.Calls, rep.Old.EstTokens, rep.New.EstTokens)
	}
	if !rep.Run {
		fmt.Printf("\nplan only: no call was made. A run asks Jev with both arms, about $%.5f for this flush (the estimate overstates).\n", rep.Old.EstCostUSD+rep.New.EstCostUSD)
		return
	}
	if a := rep.Agreement; a != nil {
		fmt.Printf("\nagreement over %d labelled captures: event %.3f (%d), topic %.3f (%d), mean |event p diff| %.3f\n", a.Captures, a.Event, a.EventAgree, a.Topic, a.TopicAgree, a.MeanAbsEventP)
		for _, d := range a.Disagreements {
			fmt.Println("  differs:", d)
		}
	}
}

// gatewayJudge asks Jev through the gateway, as the runtime does.
type gatewayJudge struct {
	client *gatewayruntime.JudgmentClient
}

func (j gatewayJudge) Judge(ctx context.Context, state json.RawMessage, questions map[string]json.RawMessage) (json.RawMessage, error) {
	return j.JudgeScoped(ctx, "", state, questions)
}

func (j gatewayJudge) JudgeScoped(ctx context.Context, scope string, state json.RawMessage, questions map[string]json.RawMessage) (json.RawMessage, error) {
	resp, err := j.client.Decide(gatewayruntime.WithJevSite(ctx, "intake_shadow"), gatewayruntime.JudgmentRequest{State: state, Questions: questions, Scope: scope})
	if err != nil {
		return nil, err
	}
	return resp.Distribution, nil
}
