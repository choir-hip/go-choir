package agentcore

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The 2026-11-09 prompt cut for Texture, management and the management task
// (docs/evidence/token-efficiency-2026-11-09/followup/system-prompt-review.md).
// Ways it can fail, written before the code:
//   - P1 a prompt grows back (a desk's built system prompt passes its bound);
//   - P2 a prompt's prefix stops being byte-stable (two builds differ), which
//     throws away the provider's cache;
//   - P3 the cut drops text the model needs: Texture's Write grammar (now in
//     the Write verb's documentation, the one place), management's
//     procedures that moved onto their records (Shots owed, Install review
//     owed);
//   - P4 a management task spends its first cells fetching the records and
//     the story page the host can hand it, or the host's hand-over skips the
//     marks choir.Records() and choir.Stories() set (records shown, story
//     library head), so its acts are refused or its reads go untainted.

// Built-prompt sizes before the cut, on the same build (labels and event
// wakes on for management): texture 15,878, management 49,855, task 12,163.
const (
	texturePromptBound    = 15200
	managementPromptBound = 36000
	taskPromptBound       = 12100
)

func promptCutRuns(owner, computer string) map[string]*types.RunRecord {
	run := func(profile, agentID string) *types.RunRecord {
		return &types.RunRecord{RunID: "run-" + profile, AgentID: agentID, OwnerID: owner, ComputerID: computer, AgentProfile: profile, AgentRole: profile}
	}
	return map[string]*types.RunRecord{
		"texture":    run(agentprofile.Texture, "texture:doc-a"),
		"management": run(agentprofile.Management, "management:"+owner),
		"task":       run(agentprofile.ManagementTask, types.ManagementTaskInbox("task-a")),
	}
}

func TestPromptCutSizeBoundsAndByteStability(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	owner, computer := "owner-cut", rt.cfg.ComputerID
	// Management is measured as the paper builds it: intake labels and event
	// wakes both on.
	for _, key := range []string{store.SettingIntakeLabels, store.SettingIntakeEventWakes} {
		if err := s.PutComputerSetting(ctx, store.ComputerSetting{OwnerID: owner, ComputerID: computer, Key: key, Value: true}); err != nil {
			t.Fatal(err)
		}
	}
	bounds := map[string]int{"texture": texturePromptBound, "management": managementPromptBound, "task": taskPromptBound}
	runs := promptCutRuns(owner, computer)
	for name, rec := range runs {
		first, err := rt.systemPromptForRun(rec)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		time.Sleep(1100 * time.Millisecond)
		other := *rec
		other.RunID += "-later"
		second, err := rt.systemPromptForRun(&other)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if first != second {
			t.Errorf("%s prompt is not byte-stable across two builds: %s", name, firstDiff(first, second))
		}
		if n := len(first); n > bounds[name] {
			t.Errorf("%s prompt is %d characters; the bound is %d", name, n, bounds[name])
		}
	}
}

// P3: Texture's Write grammar lives in the Write verb's documentation only;
// the built prompt must still teach it.
func TestTexturePromptCutKeepsWriteGrammar(t *testing.T) {
	rt, _ := testRuntime(t)
	prompt, err := rt.systemPromptForRun(promptCutRuns("owner-cut", rt.cfg.ComputerID)["texture"])
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"@replace", "@insert", "@delete", "@move", "@section", "@all", `{{cite r2.c1 "`, "{{ref", "{{image", "{{file", "{{video", "{{audio", "pSupported",
		"A cite shows as a number, never its quote",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("Texture's built prompt does not teach %q", want)
		}
	}
	if strings.Contains(prompt, "ReportPacket") {
		t.Error("Texture's prompt names choir.ReportPacket, a verb its surface does not have")
	}
}

// P3: the Shots owed and Install review owed procedures ride their records,
// not management's prompt, and each holds the whole of what the prompt used
// to carry.
func TestOwedRecordsCarryTheirProcedures(t *testing.T) {
	rt, _ := testRuntime(t)
	prompt, err := rt.systemPromptForRun(promptCutRuns("owner-cut", rt.cfg.ComputerID)["management"])
	if err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{"q1 the focused surface", "Do it in this order, one step per cell"} {
		if strings.Contains(prompt, gone) {
			t.Errorf("management's prompt still carries %q; it rides the owed record", gone)
		}
	}
	shots := shotsOwedLine("op-1")
	for _, want := range []string{"choir.ShotStart(\"op-1\")", "choir.ShotWait(jobID, 60)", "restarted", "stale", "choir.ShotVerdict(", "q1 ", "q2 ", "q3 ", "q4 ", "q5 the size is the platform's", "q6 ", "choir.Screens(", "choir.VerifyFindings"} {
		if !strings.Contains(shots, want) {
			t.Errorf("the Shots owed line lacks %q", want)
		}
	}
	install := installReviewOwedLine("rel-1", "pkg")
	for _, want := range []string{"choir.InstallPacket(\"rel-1\")", "choir.InstallVerdict(\"rel-1\"", "packetRef", "blocking", "A failed mechanical check is a rejection"} {
		if !strings.Contains(install, want) {
			t.Errorf("the Install review owed line lacks %q", want)
		}
	}
}

// openingRecorder records the first model request of every management task.
type openingRecorder struct {
	*delegationModel
	rt        *Runtime
	mu2       sync.Mutex
	opening   []string
	recordsAt []int64
	storiesAt []int64
}

func (o *openingRecorder) CallWithTools(ctx context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	if strings.Contains(req.System, "You are a management task") {
		o.mu2.Lock()
		if len(o.opening) == 0 {
			var all strings.Builder
			for _, m := range req.Messages {
				all.Write(m)
			}
			o.opening = append(o.opening, all.String())
			o.recordsAt = append(o.recordsAt, o.rt.recordsShownMark(req.ConversationID))
			o.storiesAt = append(o.storiesAt, o.rt.storyLibraryShownMark(req.ConversationID))
		}
		o.mu2.Unlock()
	}
	return o.delegationModel.CallWithTools(ctx, req)
}
