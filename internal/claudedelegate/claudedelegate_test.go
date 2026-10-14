package claudedelegate

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// docs/design/claude-delegate-slice1-2026-10-14.md §5. Failure modes pinned
// here (written before the code):
//   F5  the token reaches events.jsonl, stderr.log, status.json, the result
//       or any other file of the run directory;
//   F7  ANTHROPIC_API_KEY (or base URL, auth token, cloud switches, other
//       credentials) reaches the claude process;
//   F8  --bare is used, or WebSearch/WebFetch is reachable;
//   F9  a run outlives its wall clock, or its children survive the kill;
//   F11 a flag the pinned 2.1.197 binary does not have is passed;
//   F12 two runs at once, or a spec that escapes /workspace/platform;
//   F14 user/project settings or hooks are loaded.

const fakeToken = "sk-ant-oat01-FAKE-test-token-0123456789abcdefghijklmnop"

// flagsIn2_1_197 are the long options found by string search in the
// nixpkgs-pinned Claude Code 2.1.197 linux-x64 binary on 2026-10-14
// (`--permission-prompts` was not there).
var flagsIn2_1_197 = map[string]bool{
	"-p": true, "--model": true, "--output-format": true, "--verbose": true, "--max-turns": true,
	"--max-budget-usd": true, "--tools": true, "--allowedTools": true, "--disallowedTools": true,
	"--strict-mcp-config": true, "--mcp-config": true, "--setting-sources": true, "--settings": true,
	"--append-system-prompt": true, "--resume": true, "--permission-mode": true,
}

func TestArgsArePinnedToTheVerifiedFlagSet(t *testing.T) {
	spec := Spec{Objective: "add a comment", Paths: []string{"internal/x"}, Test: []string{"go", "test", "./internal/x/..."}, ResumeSession: "123e4567-e89b-12d3-a456-426614174000"}
	if err := spec.Normalize(); err != nil {
		t.Fatal(err)
	}
	args := Args(spec)
	for _, a := range args {
		if strings.HasPrefix(a, "-") && !flagsIn2_1_197[a] {
			t.Errorf("flag %q is not in the verified 2.1.197 set", a)
		}
		if a == "--bare" || a == "--dangerously-skip-permissions" || a == "--permission-prompts" {
			t.Errorf("forbidden flag %q", a)
		}
		if strings.Contains(a, "add a comment") && !strings.HasPrefix(a, "You are running") {
			t.Errorf("the objective must go on stdin, not argv: %q", a)
		}
	}
	value := func(flag string) string {
		i := slices.Index(args, flag)
		if i < 0 || i+1 >= len(args) {
			t.Fatalf("missing %s in %v", flag, args)
		}
		return args[i+1]
	}
	if value("--permission-mode") != "dontAsk" || args[len(args)-2] != "--permission-mode" {
		t.Errorf("permission mode must be dontAsk and last: %v", args[len(args)-2:])
	}
	if value("--output-format") != "stream-json" || !slices.Contains(args, "--verbose") {
		t.Error("stream-json needs --verbose")
	}
	if value("--max-turns") != "40" || value("--model") != Model {
		t.Errorf("turns/model = %s/%s", value("--max-turns"), value("--model"))
	}
	tools := strings.Split(value("--tools"), ",")
	for _, banned := range []string{"WebSearch", "WebFetch", "Task", "Agent"} {
		if slices.Contains(tools, banned) {
			t.Errorf("--tools offers %s", banned)
		}
	}
	disallowed := strings.Split(value("--disallowedTools"), ",")
	if !slices.Contains(disallowed, "WebSearch") || !slices.Contains(disallowed, "WebFetch") {
		t.Errorf("--disallowedTools = %v", disallowed)
	}
	for _, tool := range strings.Split(value("--allowedTools"), ",") {
		if strings.HasPrefix(tool, "Bash(") && tool != "Bash(go:*)" && tool != "Bash(gofmt:*)" && tool != "Bash(make:*)" {
			t.Errorf("Bash allowance %q is wider than go/gofmt/make", tool)
		}
		if tool == "Bash" || strings.Contains(tool, "git push") || strings.Contains(tool, "curl") {
			t.Errorf("unbounded allowance %q", tool)
		}
	}
	if value("--setting-sources") != "user" || value("--settings") != `{"disableAllHooks":true}` || !slices.Contains(args, "--strict-mcp-config") {
		t.Error("settings/hooks/MCP must be pinned off")
	}
	if value("--resume") != spec.ResumeSession {
		t.Error("resume session not passed")
	}
	ctx := value("--append-system-prompt")
	for _, want := range []string{"Never commit, push, deploy", "internal/x", "go test ./internal/x/..."} {
		if !strings.Contains(ctx, want) {
			t.Errorf("system context lacks %q", want)
		}
	}
}

func TestEnvCarriesTheTokenOnlyAndDropsProviderSwitches(t *testing.T) {
	base := []string{
		"PATH=/run/current-system/sw/bin", "GOPROXY=http://goproxy.choir.internal,off", "HTTPS_PROXY=http://127.0.0.1:3128",
		"CGO_CFLAGS=-I/nix/store/x/include",
		"ANTHROPIC_API_KEY=sk-ant-api-should-never-pass", "ANTHROPIC_BASE_URL=http://relay", "ANTHROPIC_AUTH_TOKEN=x",
		"CLAUDE_CODE_USE_BEDROCK=1", "CLAUDE_CODE_OAUTH_TOKEN=stale", "AWS_BEARER_TOKEN_BEDROCK=x", "GITHUB_TOKEN=x",
		"OPENAI_API_KEY=x", "CHOIR_CLAUDE_DELEGATE_TOKEN=x", "HOME=/root", "MY_SECRET=x", "PATH=/dup",
	}
	env, err := Env(base, "/tmp/choir-delegate/d1/home", fakeToken)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][]string{}
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		got[k] = append(got[k], v)
	}
	for _, banned := range []string{"ANTHROPIC_API_KEY", "ANTHROPIC_BASE_URL", "ANTHROPIC_AUTH_TOKEN", "CLAUDE_CODE_USE_BEDROCK", "AWS_BEARER_TOKEN_BEDROCK", "GITHUB_TOKEN", "OPENAI_API_KEY", "CHOIR_CLAUDE_DELEGATE_TOKEN", "MY_SECRET"} {
		if _, ok := got[banned]; ok {
			t.Errorf("%s reached the claude environment", banned)
		}
	}
	if v := got[TokenEnv]; len(v) != 1 || v[0] != fakeToken {
		t.Errorf("%s = %v, want exactly the token once", TokenEnv, len(v))
	}
	for k, want := range map[string]string{
		"HOME": "/tmp/choir-delegate/d1/home", "PATH": "/run/current-system/sw/bin", "GOPROXY": "http://goproxy.choir.internal,off",
		"CLAUDE_CODE_SUBPROCESS_ENV_SCRUB": "1", "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1", "DISABLE_AUTOUPDATER": "1", "DISABLE_UPDATES": "1",
	} {
		if v := got[k]; len(v) != 1 || v[0] != want {
			t.Errorf("%s = %v, want [%s]", k, v, want)
		}
	}
	if _, err := Env(base, "/h", "  "); err == nil {
		t.Error("an empty token must refuse")
	}
}

func TestSpecBounds(t *testing.T) {
	for name, raw := range map[string]string{
		"empty objective": `{"objective":"  "}`,
		"escaping path":   `{"objective":"x","paths":["../../etc"]}`,
		"absolute path":   `{"objective":"x","paths":["/etc"]}`,
		"turns":           `{"objective":"x","max_turns":1000}`,
		"minutes":         `{"objective":"x","wall_clock_min":600}`,
		"budget":          `{"objective":"x","max_budget_usd":1000}`,
		"bad resume":      `{"objective":"x","resume_session":"; rm -rf /"}`,
		"unknown field":   `{"objective":"x","flags":["--bare"]}`,
		"empty test":      `{"objective":"x","test":[""]}`,
		"huge":            `{"objective":"` + strings.Repeat("a", MaxObjectiveBytes+1) + `"}`,
	} {
		if _, err := DecodeSpec([]byte(raw)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	spec, err := DecodeSpec([]byte(`{"objective":"x","paths":["/workspace/platform/internal/capsule/","cmd"]}`))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(spec.Paths, []string{"internal/capsule", "cmd"}) || spec.MaxTurns != DefaultMaxTurns || spec.WallClockMin != DefaultWallClockMin {
		t.Fatalf("normalized = %+v", spec)
	}
}

// fakeClaude writes a shell script standing in for claude. It echoes what a
// careless or hostile run could: its whole environment, the token on stdout
// and stderr, and a result record that quotes the token.
func fakeClaude(t *testing.T, body string) Launch {
	t.Helper()
	script := filepath.Join(t.TempDir(), "claude")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return func(runDir string) *exec.Cmd {
		cmd := exec.Command(script)
		cmd.Dir = runDir
		return cmd
	}
}

const leakyClaude = `
read prompt
echo "{\"type\":\"system\",\"subtype\":\"init\",\"prompt_seen\":\"$prompt\"}"
env | sed 's/"/ /g' | while read line; do echo "{\"type\":\"debug\",\"env\":\"$line\"}"; done
if [ -n "$ANTHROPIC_API_KEY" ]; then echo '{"type":"debug","api_key":"present"}'; else echo '{"type":"debug","api_key":"unset"}'; fi
echo "{\"type\":\"assistant\",\"message\":{\"content\":[{\"type\":\"text\",\"text\":\"token is $CLAUDE_CODE_OAUTH_TOKEN\"},{\"type\":\"tool_use\",\"name\":\"Edit\"}]}}"
echo "leaked to stderr: $CLAUDE_CODE_OAUTH_TOKEN" >&2
echo "{\"type\":\"result\",\"subtype\":\"success\",\"is_error\":false,\"num_turns\":3,\"session_id\":\"123e4567-e89b-12d3-a456-426614174000\",\"total_cost_usd\":0.42,\"usage\":{\"input_tokens\":10},\"modelUsage\":{\"claude-opus-5-5\":{\"inputTokens\":10}},\"permission_denials\":[],\"result\":\"done; by the way $CLAUDE_CODE_OAUTH_TOKEN\"}"
`

func TestServerRedactsTheTokenFromEveryRunFile(t *testing.T) {
	root := t.TempDir()
	base := []string{"PATH=" + os.Getenv("PATH"), "ANTHROPIC_API_KEY=sk-ant-api-should-never-pass"}
	srv := NewServer(root, fakeToken, base, fakeClaude(t, leakyClaude))
	st, err := srv.Start(Spec{Objective: "do it", MaxTurns: 5, MaxBudgetUSD: 1, WallClockMin: 1})
	if err != nil {
		t.Fatal(err)
	}
	if st.State != StateRunning || st.ID == "" {
		t.Fatalf("start = %+v", st)
	}
	final, _ := srv.Wait(10 * time.Second)
	if final.State != StateDone || final.Result == nil || final.Result.Subtype != "success" || final.Result.NumTurns != 3 {
		t.Fatalf("final = %+v", final)
	}
	if final.Result.TotalCostUSD == nil || *final.Result.TotalCostUSD != 0.42 || len(final.Result.ModelUsage) == 0 {
		t.Fatalf("cost/usage not recorded: %+v", final.Result)
	}
	if final.Progress.AssistantMessages != 1 || final.Progress.ToolUses != 1 || !slices.Equal(final.Progress.LastTools, []string{"Edit"}) {
		t.Fatalf("progress = %+v", final.Progress)
	}
	if !strings.HasPrefix(final.EventsSHA256, "sha256:") || final.EventsBytes == 0 {
		t.Fatalf("events digest = %q/%d", final.EventsSHA256, final.EventsBytes)
	}
	events, err := os.ReadFile(filepath.Join(final.Dir, EventsFile))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(events, []byte(TokenEnv+"="+Redacted)) {
		t.Fatal("the fake claude did not see the token in its environment (or the line was not redacted)")
	}
	if !bytes.Contains(events, []byte(`"api_key":"unset"`)) {
		t.Fatal("ANTHROPIC_API_KEY reached the claude process")
	}
	if !bytes.Contains(events, []byte(`"prompt_seen":"do it"`)) {
		t.Fatal("the prompt did not arrive on stdin")
	}
	// Nothing the run left behind carries the token.
	_ = filepath.Walk(final.Dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		data, _ := os.ReadFile(path)
		if bytes.Contains(data, []byte(fakeToken)) {
			t.Errorf("%s carries the token", path)
		}
		return nil
	})
	statusJSON, _ := json.Marshal(final)
	if bytes.Contains(statusJSON, []byte(fakeToken)) {
		t.Error("status carries the token")
	}
	if !strings.Contains(final.Result.Text, Redacted) {
		t.Errorf("result text = %q", final.Result.Text)
	}
	resp := srv.Handle(Request{Op: "status"})
	if b, _ := json.Marshal(resp); bytes.Contains(b, []byte(fakeToken)) || !resp.Available {
		t.Errorf("status response = %s", b)
	}
}

func TestServerTimeoutKillsTheWholeProcessGroup(t *testing.T) {
	root := t.TempDir()
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	srv := NewServer(root, fakeToken, []string{"PATH=" + os.Getenv("PATH")}, fakeClaude(t, "sleep 60 &\necho $! > "+pidFile+"\nwait\n"))
	srv.wallClock = 300 * time.Millisecond
	if _, err := srv.Start(Spec{Objective: "hang", MaxTurns: 1, MaxBudgetUSD: 1, WallClockMin: 1}); err != nil {
		t.Fatal(err)
	}
	final, _ := srv.Wait(15 * time.Second)
	if final.State != StateTimeout || !strings.Contains(final.Reason, "wall clock") {
		t.Fatalf("final = %+v", final)
	}
	raw, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid := strings.TrimSpace(string(raw))
	// A killed child may linger as a zombie when nothing reaps orphans (a
	// container's PID 1, or the capsule broker); a zombie is dead.
	alive := func() bool {
		if stat, err := os.ReadFile("/proc/" + pid + "/stat"); err == nil {
			fields := strings.Fields(string(stat[bytes.LastIndexByte(stat, ')')+1:]))
			return len(fields) > 0 && fields[0] != "Z" && fields[0] != "X"
		} else if _, statErr := os.Stat("/proc/self"); statErr == nil {
			return false
		}
		return exec.Command("kill", "-0", pid).Run() == nil
	}
	deadline := time.Now().Add(5 * time.Second)
	for alive() {
		if time.Now().After(deadline) {
			t.Fatalf("child %s outlived the run's kill", pid)
		}
		time.Sleep(50 * time.Millisecond)
	}
	onDisk, err := ReadStatus(root, final.ID)
	if err != nil || onDisk.State != StateTimeout {
		t.Fatalf("status.json = %+v, %v", onDisk, err)
	}
}

func TestServerRunsOneAtATimeAndStops(t *testing.T) {
	srv := NewServer(t.TempDir(), fakeToken, []string{"PATH=" + os.Getenv("PATH")}, fakeClaude(t, "sleep 60\n"))
	first, err := srv.Start(Spec{Objective: "one", MaxTurns: 1, MaxBudgetUSD: 1, WallClockMin: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := srv.Start(Spec{Objective: "two", MaxTurns: 1, MaxBudgetUSD: 1, WallClockMin: 1}); err == nil {
		t.Fatal("a second concurrent run was started")
	}
	if _, err := srv.Stop("other-id"); err == nil {
		t.Fatal("stop of an unknown id succeeded")
	}
	if _, err := srv.Stop(first.ID); err != nil {
		t.Fatal(err)
	}
	final, _ := srv.Wait(15 * time.Second)
	if final.State != StateStopped {
		t.Fatalf("final = %+v", final)
	}
	if _, err := srv.Start(Spec{Objective: "three", MaxTurns: 1, MaxBudgetUSD: 1, WallClockMin: 1}); err != nil {
		t.Fatalf("a run after the first ended was refused: %v", err)
	}
	_, _ = srv.Stop("")
	_, _ = srv.Wait(15 * time.Second)
}

func TestServerWithoutTokenIsUnavailable(t *testing.T) {
	srv := NewServer(t.TempDir(), "", nil, fakeClaude(t, "exit 0\n"))
	if resp := srv.Handle(Request{Op: "status"}); resp.Available {
		t.Fatal("available without a token")
	}
	if resp := srv.Handle(Request{Op: "start", Spec: json.RawMessage(`{"objective":"x"}`)}); resp.OK || resp.Available {
		t.Fatalf("start without a token = %+v", resp)
	}
}

func TestServerRecordsAFailedRun(t *testing.T) {
	srv := NewServer(t.TempDir(), fakeToken, []string{"PATH=" + os.Getenv("PATH")}, fakeClaude(t, "echo \"error: unknown option '--x' $CLAUDE_CODE_OAUTH_TOKEN\" >&2\nexit 1\n"))
	if _, err := srv.Start(Spec{Objective: "x", MaxTurns: 1, MaxBudgetUSD: 1, WallClockMin: 1}); err != nil {
		t.Fatal(err)
	}
	final, _ := srv.Wait(10 * time.Second)
	if final.State != StateFailed || final.ExitCode == nil || *final.ExitCode != 1 || !strings.Contains(final.StderrTail, "unknown option") {
		t.Fatalf("final = %+v", final)
	}
	if strings.Contains(final.StderrTail, fakeToken) || strings.Contains(final.Reason, fakeToken) {
		t.Fatal("the stderr tail carries the token")
	}
}

func TestContainsSecretFindsMatchesAcrossChunkEdges(t *testing.T) {
	secret := []byte(fakeToken)
	for _, offset := range []int{0, 1, (1 << 20) - 5, (1 << 20) - len(secret) + 1, (1 << 20) + 3, 3 << 20} {
		data := bytes.Repeat([]byte("x"), (3<<20)+len(secret)+10)
		copy(data[offset:], secret)
		if found, err := ContainsSecret(bytes.NewReader(data), secret); err != nil || !found {
			t.Errorf("offset %d: found=%v err=%v", offset, found, err)
		}
	}
	clean := bytes.Repeat([]byte("sk-ant-oat01-FAKE"), 200000)
	if found, _ := ContainsSecret(bytes.NewReader(clean), secret); found {
		t.Error("false positive")
	}
}

func TestValidToken(t *testing.T) {
	if !ValidToken(fakeToken) || ValidToken("short") || ValidToken(fakeToken+" x") || ValidToken(fakeToken+"\n") {
		t.Fatal("ValidToken")
	}
}
