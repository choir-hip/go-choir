//go:build linux

package shotsandbox

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/yusefmosiah/go-choir/internal/shot"
)

// testStageHook makes this test binary the sandbox stage when the launcher
// re-executes it, with a fake browser step in place of Chromium.
func testStageHook() {
	// The launcher sets HOME to the work directory: a marker file there makes
	// this stage's thread-id check fail (threads_linux_test.go).
	if home := os.Getenv("HOME"); home != "" && os.Getenv(StageEnv) != "" {
		if _, err := os.Stat(filepath.Join(home, "refuse-confine")); err == nil {
			confineCheck = func(uid, gid int) error { return fmt.Errorf("thread 7 still has Uid 0 0 0 0, want %d", uid) }
		}
	}
	stageRun = func(ctx context.Context, opts shot.Options) ([]shot.Result, error) {
		resp, err := http.Get(opts.BaseURL + "/api/ping")
		if err != nil {
			return nil, err
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		var out []shot.Result
		for _, v := range opts.Plan.Views {
			if err := opts.BeforeView(ctx, v); err != nil {
				return nil, err
			}
			p := filepath.Join(opts.OutDir, v.Name+".png")
			if err := os.WriteFile(p, body, 0o644); err != nil {
				return nil, err
			}
			out = append(out, shot.Result{Name: v.Name, Kind: "png", Path: p, Bytes: int64(len(body))})
		}
		// The stage must also be unable to leave: report what a dial to the
		// outside does from here, with the servers up.
		fmt.Fprintln(os.Stderr, "stage ran the fake shots")
		return out, nil
	}
	MaybeRunStage()
}
