package shotsandbox

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/shot"
)

func stageSpec(t *testing.T, mode string) Spec {
	t.Helper()
	work := t.TempDir()
	return Spec{
		Binary: os.Args[0], Frontend: fixtureDist(t), StorePath: filepath.Join(work, "state"), FilesRoot: filepath.Join(work, "files"),
		OutDir: filepath.Join(work, "out"), WorkDir: filepath.Join(work, "w"), Browser: "chromium-fake",
		Plan: capsule.ShotPlan{Views: []capsule.ShotView{
			{Name: "home", Path: "/", Viewport: capsule.ShotViewportDesktop},
			{Name: "list", Path: "/list", Viewport: capsule.ShotViewportPhone},
		}},
		PassEnv: []string{"SHOTSANDBOX_FAKE=" + mode}, ReadyTimeoutS: 5, PlanTimeoutS: 30,
	}
}

// One pass: the autoputer starts over the private state, the frontend is
// served over it, the plan runs once with a desktop reset before every view,
// and the owner identity reaches the autoputer through the proxy.
func TestRunStageServesTheCandidateAndResetsEveryView(t *testing.T) {
	spec := stageSpec(t, "ok")
	var seen string
	fake := func(ctx context.Context, opts shot.Options) ([]shot.Result, error) {
		resp, err := http.Get(opts.BaseURL + "/api/ping")
		if err != nil {
			return nil, err
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		seen = string(b)
		var out []shot.Result
		for _, v := range opts.Plan.Views {
			if err := opts.BeforeView(ctx, v); err != nil {
				return nil, err
			}
			out = append(out, shot.Result{Name: v.Name, Kind: "png"})
		}
		return out, nil
	}
	res := RunStage(context.Background(), spec, fake)
	if res.Error != "" || len(res.Files) != 2 {
		t.Fatalf("result = %+v", res)
	}
	for _, want := range []string{"user=owner", "store=" + spec.StorePath, "files=" + spec.FilesRoot, "trust=1", "provider=stub"} {
		if !strings.Contains(seen, want) {
			t.Errorf("autoputer saw %q, want %s", seen, want)
		}
	}
	resets, _ := os.ReadFile(filepath.Join(spec.WorkDir, "resets"))
	if n := strings.Count(string(resets), "reset"); n != 2 {
		t.Fatalf("desktop resets = %d, want one per view (2)", n)
	}
}

// FM9: a candidate that panics on start fails the pass with its own words,
// not a timeout and not a blank screenshot.
func TestRunStageCandidatePanicOnStartFailsWithStderrTail(t *testing.T) {
	spec := stageSpec(t, "die")
	ran := false
	res := RunStage(context.Background(), spec, func(context.Context, shot.Options) ([]shot.Result, error) { ran = true; return nil, nil })
	if ran {
		t.Fatal("the browser ran against a candidate that never served")
	}
	if !strings.Contains(res.Error, "exited before port") || !strings.Contains(res.Error, "store schema is newer") || !strings.Contains(res.Tail, "panic:") {
		t.Fatalf("result = %+v", res)
	}
}

// FM9/FM6: a candidate that starts and never listens fails at the ready
// deadline naming the port.
func TestRunStageCandidateThatNeverServesFailsAtTheReadyDeadline(t *testing.T) {
	spec := stageSpec(t, "silent")
	spec.ReadyTimeoutS = 1
	res := RunStage(context.Background(), spec, func(context.Context, shot.Options) ([]shot.Result, error) { t.Fatal("ran"); return nil, nil })
	if !strings.Contains(res.Error, "did not accept connections") {
		t.Fatalf("result = %+v", res)
	}
}

// A binary that does not exist is a named failure, not a panic.
func TestRunStageMissingBinaryFails(t *testing.T) {
	spec := stageSpec(t, "ok")
	spec.Binary = filepath.Join(t.TempDir(), "nope")
	res := RunStage(context.Background(), spec, func(context.Context, shot.Options) ([]shot.Result, error) { return nil, nil })
	if !strings.Contains(res.Error, "did not start") {
		t.Fatalf("result = %+v", res)
	}
}

// A failed view fails the pass (no partial set) and the autoputer is stopped.
func TestRunStageShotFailureIsTheResultsError(t *testing.T) {
	spec := stageSpec(t, "ok")
	res := RunStage(context.Background(), spec, func(context.Context, shot.Options) ([]shot.Result, error) {
		return nil, errors.New("shot: view home: step 0 (click): selector #x not found")
	})
	if res.Files != nil || !strings.Contains(res.Error, "selector #x not found") {
		t.Fatalf("result = %+v", res)
	}
}

func TestRunStageRefusesAnIncompleteSpec(t *testing.T) {
	spec := stageSpec(t, "ok")
	spec.Browser = ""
	if res := RunStage(context.Background(), spec, nil); !strings.Contains(res.Error, "no browser") {
		t.Fatalf("result = %+v", res)
	}
	spec = stageSpec(t, "ok")
	spec.Frontend = t.TempDir() // no index.html
	if res := RunStage(context.Background(), spec, nil); !strings.Contains(res.Error, "index.html") {
		t.Fatalf("result = %+v", res)
	}
}
