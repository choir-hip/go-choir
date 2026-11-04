package agentcore

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// An engineer that has nothing left to do but cannot find its next act can
// tick: the same cell, again and again, each a model call over the whole
// context (docs/problems/engineer-noop-spin-2026-11-04.md: 236 cells in 8
// minutes at 130k input tokens each, no job, no write, no Freeze). The tool
// loop had no check on a stream of cells that change nothing.
//
// The detector counts consecutive capsule_go_eval cells whose source and
// stdout are the same once numbers and string literals are masked, and that
// had no effect: no error, no staged intent (a write, a job, a report), no
// job look or park, no terminal fate. Any other cell resets the count. At
// noopSpinNudgeAt it appends one harness line naming the count and the next
// safe acts; at noopSpinEndAt it ends the run with a visible fate.
const (
	noopSpinNudgeAt = 6
	noopSpinEndAt   = 2 * noopSpinNudgeAt
)

var (
	noopStringLiteral = regexp.MustCompile("\"(?:[^\"\\\\]|\\\\.)*\"|`[^`]*`")
	noopNumber        = regexp.MustCompile(`[0-9]+`)
)

type noopSpinDetector struct {
	key   string
	count int
}

// noopSpinKey masks what a ticking model varies from cell to cell.
func noopSpinKey(source, stdout string) string {
	mask := func(s string) string {
		s = noopStringLiteral.ReplaceAllString(s, `""`)
		s = noopNumber.ReplaceAllString(s, "0")
		return strings.Join(strings.Fields(s), " ")
	}
	return mask(source) + "\x00" + noopNumber.ReplaceAllString(strings.TrimSpace(stdout), "0")
}

// noopCell is the part of a capsule_go_eval result that says whether the
// cell changed anything.
type noopCell struct {
	Error           string            `json:"error"`
	Stderr          string            `json:"stderr"`
	Stdout          string            `json:"stdout"`
	ExitCode        *int              `json:"exit_code"`
	StagedIntentIDs []string          `json:"staged_intent_ids"`
	Intents         []json.RawMessage `json:"intents"`
	FateTerminal    bool              `json:"fate_terminal"`
	JobPark         string            `json:"job_park"`
	JobsAwaited     []string          `json:"jobs_awaited"`
	JobResults      []json.RawMessage `json:"job_results"`
	FreezeResult    json.RawMessage   `json:"freeze_result"`
	VerifyResult    json.RawMessage   `json:"verify_result"`
}

func (c noopCell) hasEffect() bool {
	return c.Error != "" || c.ExitCode == nil || *c.ExitCode != 0 || len(c.StagedIntentIDs) > 0 || len(c.Intents) > 0 ||
		c.FateTerminal || c.JobPark != "" || len(c.JobsAwaited) > 0 || len(c.JobResults) > 0 ||
		len(c.FreezeResult) > 0 || len(c.VerifyResult) > 0
}

// reset starts the count over (a park on a job is a wait, not a tick).
func (d *noopSpinDetector) reset() {
	d.key, d.count = "", 0
}

// observe reads one tool batch. It returns the count of consecutive no-op
// cells, and nudge is true exactly once, at noopSpinNudgeAt. stop is true
// from noopSpinEndAt on.
func (d *noopSpinDetector) observe(calls []types.ToolCall, results []types.ToolResult) (int, bool, bool) {
	if d == nil {
		return 0, false, false
	}
	before := d.count
	for i := 0; i < len(calls) && i < len(results); i++ {
		if strings.TrimSpace(calls[i].Name) != "capsule_go_eval" {
			d.reset()
			continue
		}
		var args struct {
			Source string `json:"source"`
		}
		var cell noopCell
		if results[i].IsError || json.Unmarshal(calls[i].Arguments, &args) != nil || json.Unmarshal([]byte(results[i].Output), &cell) != nil || cell.hasEffect() {
			d.reset()
			continue
		}
		key := noopSpinKey(args.Source, cell.Stdout)
		if key != d.key {
			d.key, d.count = key, 1
			continue
		}
		d.count++
	}
	// The batch's end state: a streak that a later cell in the batch broke
	// is no streak. exit_code is always serialized, so a result without it
	// is not a capsule_go_eval result and counts as an effect (hasEffect).
	return d.count, d.count >= noopSpinNudgeAt && before < noopSpinNudgeAt, d.count >= noopSpinEndAt
}

// noopSpinNudge is the one harness line at the nudge. The engineering desk
// has no Wait verb: a job is waited on with choir.Poll, which parks the turn
// until it ends.
func noopSpinNudge(count int) string {
	return fmt.Sprintf("%d cells in a row changed nothing (the same cell, apart from its numbers and strings). If the work is done, call choir.Freeze now. If you are waiting on a job, call choir.Poll(jobID): the turn parks until it ends, and you are woken. If you are waiting on someone else, choir.Ask or choir.Report and end your turn. Otherwise make a change. The run ends after %d such cells.", count, noopSpinEndAt)
}

// noopSpinError is the run's fate when the spin goes on past the nudge.
func noopSpinError(count int) error {
	return fmt.Errorf("engineer spun %d no-op cells", count)
}
