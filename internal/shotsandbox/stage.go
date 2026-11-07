package shotsandbox

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/shot"
)

// runFunc shoots a plan; shot.Run in production, a fake in tests.
type runFunc func(ctx context.Context, opts shot.Options) ([]shot.Result, error)

// stageSlack bounds the stage beyond the plan's own timeout: the autoputer's
// start, the servers and the teardown.
const stageSlack = 3 * time.Minute

// RunStage runs one pass in this process, which the caller has already
// confined (stage_linux.go): it starts the autoputer, serves the frontend,
// shoots the plan and stops what it started. It never returns an error: a
// failed pass is a Result with the cause (FM9, FM6).
func RunStage(ctx context.Context, spec Spec, run runFunc) (result Result) {
	started := time.Now()
	defer func() { result.ElapsedMS = time.Since(started).Milliseconds() }()
	fail := func(tail *tailBuffer, format string, args ...any) Result {
		r := Result{Error: fmt.Sprintf(format, args...)}
		if tail != nil {
			r.Tail = tail.String()
		}
		return r
	}
	if spec.Probe != "" {
		return runProbe(spec.Probe)
	}
	if err := spec.check(); err != nil {
		return fail(nil, "%v", err)
	}
	for _, dir := range []string{filepath.Join(spec.WorkDir, "home"), filepath.Join(spec.WorkDir, "tmp"), spec.OutDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fail(nil, "work directory: %v", err)
		}
	}
	timeouts := spec.timeouts()
	total := timeouts.Plan
	if total <= 0 {
		total = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, total+stageSlack)
	defer cancel()

	port, err := freeLoopbackPort()
	if err != nil {
		return fail(nil, "no loopback port: %v", err)
	}
	cmd, exited, tail, err := startAutoputer(spec, port)
	if err != nil {
		return fail(nil, "%v", err)
	}
	defer func() {
		killGroup(cmd)
		select {
		case <-exited:
		case <-time.After(5 * time.Second):
		}
	}()
	if err := waitForPort(ctx, port, exited, spec.readyTimeout(), tail); err != nil {
		return fail(tail, "%v", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fail(tail, "frontend server: %v", err)
	}
	server := &http.Server{Handler: NewFrontendHandler(spec.Frontend, port, OwnerUser), ReadHeaderTimeout: 30 * time.Second}
	go func() { _ = server.Serve(listener) }()
	defer func() { _ = server.Close() }()
	base := "http://" + listener.Addr().String()

	client := &http.Client{Timeout: 30 * time.Second}
	files, err := run(ctx, shot.Options{
		BaseURL: base, Plan: spec.Plan, OutDir: spec.OutDir,
		Browser: spec.Browser, Ffmpeg: spec.Ffmpeg, Fontconfig: spec.Fontconfig, Timeouts: timeouts,
		BeforeView: func(ctx context.Context, _ capsule.ShotView) error { return resetDesktop(ctx, client, base) },
	})
	if err != nil {
		select {
		case <-exited:
			return fail(tail, "the candidate autoputer died during the shots: %v", err)
		default:
		}
		return fail(tail, "%v", err)
	}
	return Result{Files: files}
}

// maxUnixSocketPath is the longest path a unix socket can bind (sun_path is
// 108 bytes with its NUL; capsule's executor uses the same bound), and
// chromiumSocketSuffix the longest the browser adds under TMPDIR, the stage's
// work directory, for its SingletonSocket: chromedp's chromedp-runnerN
// profile (at most 42 bytes) or Chromium's own org.chromium.Chromium.XXXXXX
// (45), the one d@e hit. Keep the larger. The check covers the browser's
// socket only, not the candidate autoputer's TMPDIR (work/tmp).
const (
	maxUnixSocketPath    = 107
	chromiumSocketSuffix = "/org.chromium.Chromium.XXXXXX/SingletonSocket"
)

// CheckWorkDirLength refuses a work directory too long for the browser's
// socket, which would otherwise exit FATAL before the first shot.
func CheckWorkDirLength(dir string) error {
	if n := len(filepath.Clean(dir)) + len(chromiumSocketSuffix); n > maxUnixSocketPath {
		return fmt.Errorf("the work directory %s is too long for the browser's socket: %d bytes, over the %d-byte unix socket limit", dir, n, maxUnixSocketPath)
	}
	return nil
}

func (s Spec) check() error {
	switch {
	case s.Browser == "":
		return errors.New("no browser")
	case s.Frontend == "" || s.StorePath == "" || s.FilesRoot == "" || s.OutDir == "" || s.WorkDir == "":
		return errors.New("the spec names no frontend, state, files, output or work directory")
	}
	if _, err := os.Stat(filepath.Join(s.Frontend, "index.html")); err != nil {
		return fmt.Errorf("the frontend has no index.html: %w", err)
	}
	return s.Plan.Validate()
}

// runProbe is the test-only stage action: "dial:<addr>" reports whether a
// TCP dial from inside the sandbox connected.
func runProbe(probe string) Result {
	kind, arg, _ := strings.Cut(probe, ":")
	switch kind {
	case "dial":
		conn, err := net.DialTimeout("tcp", arg, 3*time.Second)
		if err != nil {
			return Result{Probe: "refused: " + err.Error()}
		}
		_ = conn.Close()
		return Result{Probe: "connected"}
	case "listen":
		l, err := net.Listen("tcp", arg)
		if err != nil {
			return Result{Probe: "refused: " + err.Error()}
		}
		defer l.Close()
		return Result{Probe: "listening " + strconv.Itoa(l.Addr().(*net.TCPAddr).Port)}
	}
	if r, ok := platformProbe(kind, arg); ok {
		return r
	}
	return Result{Error: "unknown probe " + kind}
}
