// Command choir is the headless control surface for Choir. It wraps the
// /api/ HTTP surface with API key (Bearer choir_sk_...) auth so agents and
// scripts can read Texture documents, observe trajectories, search,
// start runs, and verify the Universal Wire news feed without a browser.
//
// Auth: CHOIR_API_KEY env var or --api-key-file. Host: CHOIR_HOST env var
// or --host flag (defaults to https://choir.news). Request timeout:
// CHOIR_TIMEOUT env var or --timeout flag (defaults to 75 seconds).
//
// This is Phase 1 of nucleus-cli-v0: it targets the existing /api/ routes
// that the proxy already auth-gates with API keys. The graph-native
// /api/v1/ surface (agent-api-graph-native-v0) is Phase 2 and will migrate
// these commands once live.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	defaultHost      = "https://choir.news"
	apiKeyEnvVar     = "CHOIR_API_KEY"
	hostEnvVar       = "CHOIR_HOST"
	timeoutEnvVar    = "CHOIR_TIMEOUT"
	apiKeyPrefix     = "choir_sk_"
	defaultTimeout   = 75 * time.Second
	defaultListLimit = 50
)

var cliStdin io.Reader = os.Stdin

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 2
	}
	cmd := args[0]
	rest := args[1:]

	switch cmd {
	case "wire":
		return runWire(rest, stdout, stderr)
	case "trajectories":
		return runTrajectories(rest, stdout, stderr)
	case "trajectory":
		return runTrajectory(rest, stdout, stderr)
	case "texture":
		return runTexture(rest, stdout, stderr)
	case "search":
		return runSearch(rest, stdout, stderr)
	case "run":
		return runRun(rest, stdout, stderr)
	case "computer":
		return runComputer(rest, stdout, stderr)
	case "api-key":
		return runAPIKey(rest, stdout, stderr)
	case "self-dev":
		return runSelfDevelopment(rest, stdout, stderr)
	case "version":
		fmt.Fprintln(stdout, "choir v0 (Phase 1: existing /api/ routes)")
		return 0
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "choir: unknown command %q\n", cmd)
		printUsage(stderr)
		return 2
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `choir — headless Choir control surface

Usage:
  choir <command> [flags]

Commands:
  wire stories        List Universal Wire stories (real articles on the feed)
  wire diagnostics    Print wire feed diagnostics (edition substrate state)
  trajectories        List recent trajectories (ingestion/run state)
  trajectory <id>     Show one trajectory's obligations
  trajectory cancel <id>  Cancel an owner-scoped trajectory
  texture read <doc>  Read a Texture document's metadata (title, current revision id)
  texture history <doc>  List revision history for a document (metadata only)
  texture revisions <doc>  List revisions with full content bodies
  search <query>      Search the corpus
  run start <text>    Submit a prompt to the conductor (starts a run)
  run status <id>     Get the status of a prompt-bar submission
  run list            List recent owner-scoped runs
  run cancel <id>     Cancel an owner-scoped pending or running run
  computer status      Observe the current computer through the product API
  computer stop        Stop the current computer through owner-scoped vmctl
  computer start       Start or resume the current computer
  api-key list        List your API keys
  api-key create      Create a delegated API key (requires manage:keys or admin)
  api-key revoke <id> Revoke this key, or a delegated key with manage:keys/admin
  self-dev mode get|set  Read or generation-CAS the explicit computer mode
  version             Print CLI version
  help                Print this usage

Auth:
  --api-key-file path  Read API key from a mode-0600 file; "-" reads stdin.
                       Defaults to $CHOIR_API_KEY when omitted.
  --host string        Choir host. Defaults to $CHOIR_HOST or https://choir.news.
  --timeout duration   Request timeout. Defaults to $CHOIR_TIMEOUT or 75s.

Output is JSON to stdout; diagnostics and errors go to stderr.`)
}

// client holds shared CLI state.
type client struct {
	host   string
	apiKey string
	http   *http.Client
	stdout io.Writer
	stderr io.Writer
}

func newClient(flags *flag.FlagSet, args []string, stdout, stderr io.Writer) (*client, error) {
	apiKeyFile := flags.String("api-key-file", "", "Read API key from a mode-0600 file; '-' reads stdin; defaults to $"+apiKeyEnvVar)
	host := flags.String("host", envOr(hostEnvVar, defaultHost), "Choir host")
	timeout := flags.String("timeout", "", "Request timeout (for example 75s or 2m)")
	if err := flags.Parse(args); err != nil {
		return nil, err
	}
	key := strings.TrimSpace(os.Getenv(apiKeyEnvVar))
	if strings.TrimSpace(*apiKeyFile) != "" {
		var err error
		key, err = readCLISecretFile(*apiKeyFile, cliStdin)
		if err != nil {
			return nil, err
		}
	}
	if key == "" {
		return nil, fmt.Errorf("api key required: set --api-key-file or $%s", apiKeyEnvVar)
	}
	if !strings.HasPrefix(key, apiKeyPrefix) {
		return nil, fmt.Errorf("api key must start with %q", apiKeyPrefix)
	}
	h := strings.TrimRight(strings.TrimSpace(*host), "/")
	if h == "" {
		h = defaultHost
	}
	requestTimeout, err := resolveTimeout(*timeout, os.Getenv(timeoutEnvVar))
	if err != nil {
		return nil, err
	}
	return &client{
		host:   h,
		apiKey: key,
		http:   &http.Client{Timeout: requestTimeout},
		stdout: stdout,
		stderr: stderr,
	}, nil
}

func readCLISecretFile(path string, stdin io.Reader) (string, error) {
	path = strings.TrimSpace(path)
	if path == "-" {
		if stdin == nil {
			return "", fmt.Errorf("api key stdin is unavailable")
		}
		raw, err := io.ReadAll(io.LimitReader(stdin, 64<<10))
		if err != nil {
			return "", fmt.Errorf("read api key from stdin: %w", err)
		}
		return strings.TrimSpace(string(raw)), nil
	}
	info, err := os.Lstat(path)
	if err != nil {
		return "", fmt.Errorf("read api key file: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		return "", fmt.Errorf("api key file must be a regular mode-0600 file")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read api key file: %w", err)
	}
	if len(raw) > 64<<10 {
		return "", fmt.Errorf("api key file is too large")
	}
	return strings.TrimSpace(string(raw)), nil
}

func resolveTimeout(flagValue, envValue string) (time.Duration, error) {
	raw := strings.TrimSpace(flagValue)
	source := "--timeout"
	if raw == "" {
		raw = strings.TrimSpace(envValue)
		source = "$" + timeoutEnvVar
	}
	if raw == "" {
		return defaultTimeout, nil
	}
	timeout, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid duration: %w", source, err)
	}
	if timeout <= 0 {
		return 0, fmt.Errorf("%s must be greater than zero", source)
	}
	return timeout, nil
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

// do performs an authenticated request and decodes the JSON response into
// out. If body is non-nil it is JSON-encoded and sent as the request body.
// On non-2xx it returns an error with the response body.
func (c *client) do(method, path string, body any, out any) error {
	url := c.host + path
	var reqBody io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request body: %w", err)
		}
		reqBody = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &apiErrorResp{Status: resp.StatusCode, Body: string(respBody)}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("decode response: %w (body: %s)", err, truncate(string(respBody), 200))
	}
	return nil
}

type apiErrorResp struct {
	Status int
	Body   string
}

func (e *apiErrorResp) Error() string {
	return fmt.Sprintf("http %d: %s", e.Status, truncate(strings.TrimSpace(e.Body), 300))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// writeJSON pretty-prints v to stdout.
func writeJSON(w io.Writer, v any) int {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(os.Stderr, "choir: encode output: %v\n", err)
		return 1
	}
	return 0
}

// ---- wire ----

func runWire(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "choir wire: subcommand required (stories|diagnostics)")
		return 2
	}
	sub := args[0]
	fs := flag.NewFlagSet("choir wire "+sub, flag.ContinueOnError)
	fs.SetOutput(stderr)
	c, err := newClient(fs, args[1:], stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir wire: %v\n", err)
		return 2
	}
	switch sub {
	case "stories":
		var resp wireStoriesResponse
		if err := c.do(http.MethodGet, "/api/universal-wire/stories", nil, &resp); err != nil {
			fmt.Fprintf(stderr, "choir wire stories: %v\n", err)
			return 1
		}
		return writeJSON(stdout, resp)
	case "diagnostics":
		var resp wireStoriesResponse
		if err := c.do(http.MethodGet, "/api/universal-wire/stories", nil, &resp); err != nil {
			fmt.Fprintf(stderr, "choir wire diagnostics: %v\n", err)
			return 1
		}
		return writeJSON(stdout, resp.Diagnostics)
	default:
		fmt.Fprintf(stderr, "choir wire: unknown subcommand %q\n", sub)
		return 2
	}
}

// wireStoriesResponse mirrors the wire API response without importing its
// Dolt-backed owner package.
type wireStoriesResponse struct {
	Stories      []wireStory          `json:"stories"`
	StyleSources []json.RawMessage    `json:"style_sources"`
	Source       string               `json:"source"`
	Edition      *json.RawMessage     `json:"edition,omitempty"`
	Diagnostics  *wireFeedDiagnostics `json:"diagnostics,omitempty"`
}

type wireStory struct {
	ID                string            `json:"id"`
	Headline          string            `json:"headline"`
	Dek               string            `json:"dek"`
	Freshness         string            `json:"freshness"`
	Prominence        int               `json:"prominence"`
	StoryTextureDoc   string            `json:"story_texture_doc_id,omitempty"`
	TextureContent    string            `json:"texture_content,omitempty"`
	PlatformRoutePath string            `json:"platform_route_path,omitempty"`
	SourceState       string            `json:"source_state"`
	CreatedAt         time.Time         `json:"created_at,omitempty"`
	UpdatedAt         time.Time         `json:"updated_at,omitempty"`
	Projections       map[string]string `json:"projections"`
}

// wireFeedDiagnostics mirrors the diagnostics field shape. Kept loose
// (RawMessage) because the substrate-state schema is owned by the runtime
// and may evolve; the CLI prints it verbatim.
type wireFeedDiagnostics json.RawMessage

func (d *wireFeedDiagnostics) UnmarshalJSON(b []byte) error {
	*d = wireFeedDiagnostics(b)
	return nil
}

func (d wireFeedDiagnostics) MarshalJSON() ([]byte, error) {
	return []byte(d), nil
}

// ---- trajectories ----

func runTrajectories(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir trajectories", flag.ContinueOnError)
	fs.SetOutput(stderr)
	c, err := newClient(fs, args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir trajectories: %v\n", err)
		return 2
	}
	var resp trajectoriesListResponse
	if err := c.do(http.MethodGet, "/api/trajectories", nil, &resp); err != nil {
		fmt.Fprintf(stderr, "choir trajectories: %v\n", err)
		return 1
	}
	if len(resp.Trajectories) > defaultListLimit {
		resp.Trajectories = resp.Trajectories[:defaultListLimit]
	}
	return writeJSON(stdout, resp)
}

func runTrajectory(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "cancel" {
		return runTrajectoryCancel(args[1:], stdout, stderr)
	}
	fs := flag.NewFlagSet("choir trajectory", flag.ContinueOnError)
	fs.SetOutput(stderr)
	c, err := newClient(fs, args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir trajectory: %v\n", err)
		return 2
	}
	rest := fs.Args()
	if len(rest) == 0 || strings.TrimSpace(rest[0]) == "" {
		fmt.Fprintln(stderr, "choir trajectory: trajectory id required")
		return 2
	}
	id := strings.TrimSpace(rest[0])
	var resp json.RawMessage
	if err := c.do(http.MethodGet, "/api/trajectories/"+id, nil, &resp); err != nil {
		fmt.Fprintf(stderr, "choir trajectory %s: %v\n", id, err)
		return 1
	}
	return writeJSON(stdout, resp)
}
func runTrajectoryCancel(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir trajectory cancel", flag.ContinueOnError)
	fs.SetOutput(stderr)
	c, err := newClient(fs, args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir trajectory cancel: %v\n", err)
		return 2
	}
	rest := fs.Args()
	if len(rest) != 1 || strings.TrimSpace(rest[0]) == "" {
		fmt.Fprintln(stderr, "choir trajectory cancel: trajectory id required")
		return 2
	}
	id := strings.TrimSpace(rest[0])
	var resp json.RawMessage
	path := "/api/trajectories/" + url.PathEscape(id) + "/cancel"
	if err := c.do(http.MethodPost, path, nil, &resp); err != nil {
		fmt.Fprintf(stderr, "choir trajectory cancel %s: %v\n", id, err)
		return 1
	}
	return writeJSON(stdout, resp)
}

type trajectoriesListResponse struct {
	Trajectories []trajectoryRecord `json:"trajectories"`
}

// trajectoryRecord mirrors the fields the CLI needs from
// internal/types.TrajectoryRecord. Kept minimal to avoid importing the
// types package (and its transitive cgo deps).
type trajectoryRecord struct {
	TrajectoryID   string          `json:"trajectory_id"`
	OwnerID        string          `json:"owner_id"`
	Kind           string          `json:"kind"`
	SubjectRefs    json.RawMessage `json:"subject_refs,omitempty"`
	Status         string          `json:"status,omitempty"`
	SettlementRule json.RawMessage `json:"settlement_rule,omitempty"`
	CreatedAt      time.Time       `json:"created_at,omitempty"`
	UpdatedAt      time.Time       `json:"updated_at,omitempty"`
}

// ---- texture ----

func runTexture(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "choir texture: subcommand required (read|history|revisions)")
		return 2
	}
	sub := args[0]
	fs := flag.NewFlagSet("choir texture "+sub, flag.ContinueOnError)
	fs.SetOutput(stderr)
	c, err := newClient(fs, args[1:], stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir texture: %v\n", err)
		return 2
	}
	rest := fs.Args()
	if len(rest) == 0 || strings.TrimSpace(rest[0]) == "" {
		fmt.Fprintf(stderr, "choir texture %s: document id required\n", sub)
		return 2
	}
	docID := strings.TrimSpace(rest[0])
	switch sub {
	case "read":
		var resp json.RawMessage
		if err := c.do(http.MethodGet, "/api/texture/documents/"+docID, nil, &resp); err != nil {
			fmt.Fprintf(stderr, "choir texture read %s: %v\n", docID, err)
			return 1
		}
		return writeJSON(stdout, resp)
	case "history":
		var resp json.RawMessage
		if err := c.do(http.MethodGet, "/api/texture/documents/"+docID+"/history", nil, &resp); err != nil {
			fmt.Fprintf(stderr, "choir texture history %s: %v\n", docID, err)
			return 1
		}
		return writeJSON(stdout, resp)
	case "revisions":
		var resp json.RawMessage
		if err := c.do(http.MethodGet, "/api/texture/documents/"+docID+"/revisions", nil, &resp); err != nil {
			fmt.Fprintf(stderr, "choir texture revisions %s: %v\n", docID, err)
			return 1
		}
		return writeJSON(stdout, resp)
	default:
		fmt.Fprintf(stderr, "choir texture: unknown subcommand %q\n", sub)
		return 2
	}
}

// ---- search ----

func runSearch(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir search", flag.ContinueOnError)
	fs.SetOutput(stderr)
	c, err := newClient(fs, args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir search: %v\n", err)
		return 2
	}
	rest := fs.Args()
	if len(rest) == 0 || strings.TrimSpace(strings.Join(rest, " ")) == "" {
		fmt.Fprintln(stderr, "choir search: query required")
		return 2
	}
	q := strings.TrimSpace(strings.Join(rest, " "))
	// The proxy owns /api/platform/retrieval/search; it expects the query
	// in the q parameter.
	var resp json.RawMessage
	if err := c.do(http.MethodGet, "/api/platform/retrieval/search?q="+url.QueryEscape(q), nil, &resp); err != nil {
		fmt.Fprintf(stderr, "choir search: %v\n", err)
		return 1
	}
	return writeJSON(stdout, resp)
}

// ---- self-development ----

func runSelfDevelopment(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "choir self-dev: subcommand required (start|status|inspect|approve|reject|rollback|wait|genesis|kernel-capabilities|mode)")
		return 2
	}
	switch args[0] {
	case "start":
		return runSelfDevelopmentStart(args[1:], stdout, stderr)
	case "status", "inspect":
		return runSelfDevelopmentStatus(args[0], args[1:], stdout, stderr)
	case "approve", "reject":
		return runSelfDevelopmentDecision(args[0], args[1:], stdout, stderr)
	case "wait":
		return runSelfDevelopmentWait(args[1:], stdout, stderr)
	case "genesis":
		return runSelfDevelopmentGenesis(args[1:], stdout, stderr)
	case "rollback":
		return runSelfDevelopmentRollback(args[1:], stdout, stderr)
	case "kernel-capabilities":
		return runSelfDevelopmentKernelCapabilities(args[1:], stdout, stderr)
	case "mode":
		if len(args) < 2 {
			fmt.Fprintln(stderr, "choir self-dev mode: subcommand required (get|set)")
			return 2
		}
		switch args[1] {
		case "get":
			return runSelfDevelopmentModeGet(args[2:], stdout, stderr)
		case "set":
			return runSelfDevelopmentModeSet(args[2:], stdout, stderr)
		default:
			fmt.Fprintf(stderr, "choir self-dev mode: unknown subcommand %q\n", args[1])
			return 2
		}
	default:
		fmt.Fprintf(stderr, "choir self-dev: unknown subcommand %q\n", args[0])
		return 2
	}
}

func runSelfDevelopmentStart(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir self-dev start", flag.ContinueOnError)
	fs.SetOutput(stderr)
	computerID := fs.String("computer", "", "Stable ComputerID")
	idempotencyKey := fs.String("idempotency-key", "", "Unique idempotency key")
	promptFile := fs.String("prompt-file", "", "Prompt file path; '-' reads stdin")
	c, err := newClient(fs, args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir self-dev start: %v\n", err)
		return 2
	}
	if strings.TrimSpace(*computerID) == "" || strings.TrimSpace(*idempotencyKey) == "" || strings.TrimSpace(*promptFile) == "" || len(fs.Args()) != 0 {
		fmt.Fprintln(stderr, "choir self-dev start: --computer, --idempotency-key, and --prompt-file are required; positional arguments are forbidden")
		return 2
	}
	prompt, err := readCLIInputFile(*promptFile, 1<<20)
	if err != nil || strings.TrimSpace(prompt) == "" {
		fmt.Fprintf(stderr, "choir self-dev start: invalid prompt file: %v\n", err)
		return 2
	}
	var response json.RawMessage
	path := "/api/computers/" + url.PathEscape(strings.TrimSpace(*computerID)) + "/self-development/operations"
	if err := c.do(http.MethodPost, path, map[string]any{"idempotency_key": strings.TrimSpace(*idempotencyKey), "prompt": prompt}, &response); err != nil {
		fmt.Fprintf(stderr, "choir self-dev start: %v\n", err)
		return 1
	}
	return writeJSON(stdout, response)
}

func runSelfDevelopmentStatus(command string, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir self-dev "+command, flag.ContinueOnError)
	fs.SetOutput(stderr)
	computerID := fs.String("computer", "", "Stable ComputerID")
	c, err := newClient(fs, args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir self-dev %s: %v\n", command, err)
		return 2
	}
	if strings.TrimSpace(*computerID) == "" || len(fs.Args()) != 1 || strings.TrimSpace(fs.Args()[0]) == "" {
		fmt.Fprintf(stderr, "choir self-dev %s: --computer and one operation id are required\n", command)
		return 2
	}
	var response json.RawMessage
	path := "/api/computers/" + url.PathEscape(strings.TrimSpace(*computerID)) + "/self-development/operations/" + url.PathEscape(strings.TrimSpace(fs.Args()[0]))
	if err := c.do(http.MethodGet, path, nil, &response); err != nil {
		fmt.Fprintf(stderr, "choir self-dev %s: %v\n", command, err)
		return 1
	}
	return writeJSON(stdout, response)
}

func runSelfDevelopmentDecision(command string, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir self-dev "+command, flag.ContinueOnError)
	fs.SetOutput(stderr)
	computerID := fs.String("computer", "", "Stable ComputerID")
	idempotencyKey := fs.String("idempotency-key", "", "Unique idempotency key")
	desiredHead := fs.String("expected-desired-head", "", "Expected desired event head")
	effectiveHead := fs.String("expected-effective-head", "", "Expected effective event head")
	desiredCommitment := fs.String("expected-desired-commitment", "", "Expected desired state commitment")
	effectiveCommitment := fs.String("expected-effective-commitment", "", "Expected effective state commitment")
	bundle := fs.String("bundle", "", "Exact frozen bundle digest")
	verifier := fs.String("verifier", "", "Exact verifier evidence reference")
	reasonFile := fs.String("reason-file", "", "Rejection reason file; '-' reads stdin")
	c, err := newClient(fs, args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir self-dev %s: %v\n", command, err)
		return 2
	}
	if strings.TrimSpace(*computerID) == "" || strings.TrimSpace(*idempotencyKey) == "" || strings.TrimSpace(*desiredHead) == "" ||
		strings.TrimSpace(*effectiveHead) == "" || strings.TrimSpace(*desiredCommitment) == "" || strings.TrimSpace(*effectiveCommitment) == "" ||
		strings.TrimSpace(*bundle) == "" || strings.TrimSpace(*verifier) == "" || len(fs.Args()) != 1 {
		fmt.Fprintf(stderr, "choir self-dev %s: exact computer, operation, heads, state commitments, bundle, verifier, and idempotency key are required\n", command)
		return 2
	}
	body := map[string]any{
		"decision": command, "idempotency_key": strings.TrimSpace(*idempotencyKey), "bundle_digest": strings.TrimSpace(*bundle),
		"verifier_ref": strings.TrimSpace(*verifier), "expected_desired_event_head": strings.TrimSpace(*desiredHead),
		"expected_effective_event_head":       strings.TrimSpace(*effectiveHead),
		"expected_desired_state_commitment":   strings.TrimSpace(*desiredCommitment),
		"expected_effective_state_commitment": strings.TrimSpace(*effectiveCommitment),
	}
	if command == "reject" {
		if strings.TrimSpace(*reasonFile) == "" {
			fmt.Fprintln(stderr, "choir self-dev reject: --reason-file is required")
			return 2
		}
		reason, readErr := readCLIInputFile(*reasonFile, 256<<10)
		if readErr != nil || strings.TrimSpace(reason) == "" {
			fmt.Fprintf(stderr, "choir self-dev reject: invalid reason file: %v\n", readErr)
			return 2
		}
		body["reason"] = reason
	}
	var response json.RawMessage
	path := "/api/computers/" + url.PathEscape(strings.TrimSpace(*computerID)) + "/self-development/operations/" + url.PathEscape(strings.TrimSpace(fs.Args()[0])) + "/decision"
	if err := c.do(http.MethodPost, path, body, &response); err != nil {
		fmt.Fprintf(stderr, "choir self-dev %s: %v\n", command, err)
		return 1
	}
	return writeJSON(stdout, response)
}

func runSelfDevelopmentWait(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir self-dev wait", flag.ContinueOnError)
	fs.SetOutput(stderr)
	computerID := fs.String("computer", "", "Stable ComputerID")
	pollInterval := fs.Duration("poll-interval", 2*time.Second, "Status polling interval")
	c, err := newClient(fs, args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir self-dev wait: %v\n", err)
		return 2
	}
	if strings.TrimSpace(*computerID) == "" || len(fs.Args()) != 1 || *pollInterval <= 0 {
		fmt.Fprintln(stderr, "choir self-dev wait: --computer, one operation id, and a positive --poll-interval are required")
		return 2
	}
	path := "/api/computers/" + url.PathEscape(strings.TrimSpace(*computerID)) + "/self-development/operations/" + url.PathEscape(strings.TrimSpace(fs.Args()[0]))
	for {
		var response json.RawMessage
		if err := c.do(http.MethodGet, path, nil, &response); err != nil {
			fmt.Fprintf(stderr, "choir self-dev wait: %v\n", err)
			return 1
		}
		var status struct {
			State string `json:"state"`
		}
		if err := json.Unmarshal(response, &status); err != nil {
			fmt.Fprintln(stderr, "choir self-dev wait: invalid operation response")
			return 1
		}
		switch status.State {
		case "applied", "rejected", "rolled_back", "failed", "degraded":
			return writeJSON(stdout, response)
		}
		time.Sleep(*pollInterval)
	}
}

func runSelfDevelopmentGenesis(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir self-dev genesis", flag.ContinueOnError)
	fs.SetOutput(stderr)
	computerID := fs.String("computer", "", "Stable ComputerID")
	baselineVersion := fs.String("baseline-version", "", "Baseline ComputerVersion digest")
	baselineState := fs.String("baseline-state", "", "Baseline effective state digest")
	expectedAbsent := fs.Bool("expected-absent", false, "Require absent event head")
	idempotencyKey := fs.String("idempotency-key", "", "Unique idempotency key")
	c, err := newClient(fs, args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir self-dev genesis: %v\n", err)
		return 2
	}
	if strings.TrimSpace(*computerID) == "" || strings.TrimSpace(*baselineVersion) == "" || strings.TrimSpace(*baselineState) == "" || strings.TrimSpace(*idempotencyKey) == "" || !*expectedAbsent || len(fs.Args()) != 0 {
		fmt.Fprintln(stderr, "choir self-dev genesis: --computer, --baseline-version, --baseline-state, --expected-absent, and --idempotency-key are required")
		return 2
	}
	body := map[string]any{"baseline_version": strings.TrimSpace(*baselineVersion), "baseline_state": strings.TrimSpace(*baselineState), "expected_absent": true, "idempotency_key": strings.TrimSpace(*idempotencyKey)}
	var response json.RawMessage
	path := "/api/computers/" + url.PathEscape(strings.TrimSpace(*computerID)) + "/self-development/genesis"
	if err := c.do(http.MethodPost, path, body, &response); err != nil {
		fmt.Fprintf(stderr, "choir self-dev genesis: %v\n", err)
		return 1
	}
	return writeJSON(stdout, response)
}

func runSelfDevelopmentRollback(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir self-dev rollback", flag.ContinueOnError)
	fs.SetOutput(stderr)
	computerID := fs.String("computer", "", "Stable ComputerID")
	expectedDesired := fs.String("expected-desired-head", "", "Expected desired event head")
	currentApplied := fs.String("current-applied-head", "", "Current applied event head")
	toApplied := fs.String("to-applied-head", "", "Prior applied event head")
	priorMaterialization := fs.String("prior-materialization", "", "Prior materialization receipt digest")
	priorCheckpoint := fs.String("prior-checkpoint", "", "Prior checkpoint digest")
	expectedRouteGeneration := fs.Uint64("expected-route-generation", 0, "Expected route generation")
	idempotencyKey := fs.String("idempotency-key", "", "Unique idempotency key")
	c, err := newClient(fs, args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir self-dev rollback: %v\n", err)
		return 2
	}
	if strings.TrimSpace(*computerID) == "" || strings.TrimSpace(*expectedDesired) == "" || strings.TrimSpace(*currentApplied) == "" || strings.TrimSpace(*toApplied) == "" || strings.TrimSpace(*priorMaterialization) == "" || strings.TrimSpace(*priorCheckpoint) == "" || strings.TrimSpace(*idempotencyKey) == "" || len(fs.Args()) != 0 {
		fmt.Fprintln(stderr, "choir self-dev rollback: complete computer, head, materialization, checkpoint, route generation, and idempotency bindings are required")
		return 2
	}
	body := map[string]any{
		"expected_desired_head": strings.TrimSpace(*expectedDesired), "current_applied_head": strings.TrimSpace(*currentApplied),
		"to_applied_head": strings.TrimSpace(*toApplied), "prior_materialization": strings.TrimSpace(*priorMaterialization),
		"prior_checkpoint": strings.TrimSpace(*priorCheckpoint), "expected_route_generation": *expectedRouteGeneration,
		"idempotency_key": strings.TrimSpace(*idempotencyKey),
	}
	var response json.RawMessage
	path := "/api/computers/" + url.PathEscape(strings.TrimSpace(*computerID)) + "/self-development/rollbacks"
	if err := c.do(http.MethodPost, path, body, &response); err != nil {
		fmt.Fprintf(stderr, "choir self-dev rollback: %v\n", err)
		return 1
	}
	return writeJSON(stdout, response)
}

func runSelfDevelopmentKernelCapabilities(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir self-dev kernel-capabilities", flag.ContinueOnError)
	fs.SetOutput(stderr)
	computerID := fs.String("computer", "", "Stable ComputerID")
	c, err := newClient(fs, args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir self-dev kernel-capabilities: %v\n", err)
		return 2
	}
	if strings.TrimSpace(*computerID) == "" || len(fs.Args()) != 0 {
		fmt.Fprintln(stderr, "choir self-dev kernel-capabilities: --computer is required")
		return 2
	}
	var response json.RawMessage
	path := "/api/computers/" + url.PathEscape(strings.TrimSpace(*computerID)) + "/self-development/kernel-capabilities"
	if err := c.do(http.MethodGet, path, nil, &response); err != nil {
		fmt.Fprintf(stderr, "choir self-dev kernel-capabilities: %v\n", err)
		return 1
	}
	return writeJSON(stdout, response)
}

func readCLIInputFile(path string, maximum int64) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("file path is required")
	}
	var reader io.Reader
	if path == "-" {
		if cliStdin == nil {
			return "", fmt.Errorf("stdin is unavailable")
		}
		reader = cliStdin
	} else {
		info, err := os.Lstat(path)
		if err != nil {
			return "", err
		}
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("input must be a regular file")
		}
		file, err := os.Open(path)
		if err != nil {
			return "", err
		}
		defer file.Close()
		reader = file
	}
	raw, err := io.ReadAll(io.LimitReader(reader, maximum+1))
	if err != nil {
		return "", err
	}
	if int64(len(raw)) > maximum {
		return "", fmt.Errorf("input exceeds %d bytes", maximum)
	}
	return string(raw), nil
}

func runSelfDevelopmentModeGet(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir self-dev mode get", flag.ContinueOnError)
	fs.SetOutput(stderr)
	computerID := fs.String("computer", "", "Stable ComputerID")
	c, err := newClient(fs, args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir self-dev mode get: %v\n", err)
		return 2
	}
	if strings.TrimSpace(*computerID) == "" || len(fs.Args()) != 0 {
		fmt.Fprintln(stderr, "choir self-dev mode get: --computer is required and positional arguments are forbidden")
		return 2
	}
	var response json.RawMessage
	path := "/api/computers/" + url.PathEscape(strings.TrimSpace(*computerID)) + "/self-development/mode"
	if err := c.do(http.MethodGet, path, nil, &response); err != nil {
		fmt.Fprintf(stderr, "choir self-dev mode get: %v\n", err)
		return 1
	}
	return writeJSON(stdout, response)
}

func runSelfDevelopmentModeSet(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir self-dev mode set", flag.ContinueOnError)
	fs.SetOutput(stderr)
	computerID := fs.String("computer", "", "Stable ComputerID")
	mode := fs.String("mode", "", "off, audit_only, propose_only, or accept_once")
	expectedGeneration := fs.Uint64("expected-generation", 0, "Expected mode generation")
	idempotencyKey := fs.String("idempotency-key", "", "Unique idempotency key")
	expiresAt := fs.String("expires-at", "", "Canonical UTC expiry for accept_once")
	operationID := fs.String("operation", "", "Exact operation ID for accept_once")
	desiredHead := fs.String("expected-desired-head", "", "Expected desired event head for accept_once")
	effectiveHead := fs.String("expected-effective-head", "", "Expected effective event head for accept_once")
	desiredCommitment := fs.String("expected-desired-commitment", "", "Expected desired state commitment for accept_once")
	effectiveCommitment := fs.String("expected-effective-commitment", "", "Expected effective state commitment for accept_once")
	bundle := fs.String("bundle", "", "Exact bundle digest for accept_once")
	c, err := newClient(fs, args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir self-dev mode set: %v\n", err)
		return 2
	}
	if strings.TrimSpace(*computerID) == "" || strings.TrimSpace(*mode) == "" || strings.TrimSpace(*idempotencyKey) == "" || len(fs.Args()) != 0 {
		fmt.Fprintln(stderr, "choir self-dev mode set: --computer, --mode, and --idempotency-key are required; positional arguments are forbidden")
		return 2
	}
	body := map[string]any{
		"mode": *mode, "expected_generation": *expectedGeneration, "idempotency_key": *idempotencyKey,
	}
	if *mode == "accept_once" {
		body["expires_at"] = *expiresAt
		body["operation_id"] = *operationID
		body["expected_desired_event_head"] = *desiredHead
		body["expected_effective_event_head"] = *effectiveHead
		body["expected_desired_state_commitment"] = *desiredCommitment
		body["expected_effective_state_commitment"] = *effectiveCommitment
		body["bundle_digest"] = *bundle
	}
	var response json.RawMessage
	path := "/api/computers/" + url.PathEscape(strings.TrimSpace(*computerID)) + "/self-development/mode"
	if err := c.do(http.MethodPost, path, body, &response); err != nil {
		fmt.Fprintf(stderr, "choir self-dev mode set: %v\n", err)
		return 1
	}
	return writeJSON(stdout, response)
}

// ---- computer ----

func runComputer(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "choir computer: subcommand required (status|stop|start)")
		return 2
	}
	switch args[0] {
	case "status":
		return runComputerStatus(args[1:], stdout, stderr)
	case "stop":
		return runComputerAction(args[1:], "stop_current_computer", "stop", stdout, stderr)
	case "start":
		return runComputerAction(args[1:], "wake_current_computer", "start", stdout, stderr)
	default:
		fmt.Fprintf(stderr, "choir computer: unknown subcommand %q\n", args[0])
		return 2
	}
}

func runComputerStatus(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir computer status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	c, err := newClient(fs, args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir computer status: %v\n", err)
		return 2
	}
	if len(fs.Args()) != 0 {
		fmt.Fprintln(stderr, "choir computer status: unexpected positional arguments")
		return 2
	}
	var resp json.RawMessage
	if err := c.do(http.MethodGet, "/api/compute/status", nil, &resp); err != nil {
		fmt.Fprintf(stderr, "choir computer status: %v\n", err)
		return 1
	}
	return writeJSON(stdout, resp)
}

func runComputerAction(args []string, action, command string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir computer "+command, flag.ContinueOnError)
	fs.SetOutput(stderr)
	c, err := newClient(fs, args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir computer %s: %v\n", command, err)
		return 2
	}
	if len(fs.Args()) != 0 {
		fmt.Fprintf(stderr, "choir computer %s: unexpected positional arguments\n", command)
		return 2
	}
	var resp json.RawMessage
	if err := c.do(http.MethodPost, "/api/compute/recovery", map[string]string{"action": action}, &resp); err != nil {
		fmt.Fprintf(stderr, "choir computer %s: %v\n", command, err)
		return 1
	}
	return writeJSON(stdout, resp)
}

// ---- run ----

func runRun(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "choir run: subcommand required (start|status|list|cancel)")
		return 2
	}
	sub := args[0]
	switch sub {
	case "start":
		return runRunStart(args[1:], stdout, stderr)
	case "status":
		return runRunStatus(args[1:], stdout, stderr)
	case "list":
		return runRunList(args[1:], stdout, stderr)
	case "cancel":
		return runRunCancel(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "choir run: unknown subcommand %q\n", sub)
		return 2
	}
}

func runRunStart(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir run start", flag.ContinueOnError)
	fs.SetOutput(stderr)
	c, err := newClient(fs, args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir run start: %v\n", err)
		return 2
	}
	rest := fs.Args()
	if len(rest) == 0 || strings.TrimSpace(strings.Join(rest, " ")) == "" {
		fmt.Fprintln(stderr, "choir run start: prompt text required")
		return 2
	}
	text := strings.TrimSpace(strings.Join(rest, " "))
	var resp promptBarSubmitResponse
	if err := c.do(http.MethodPost, "/api/prompt-bar", map[string]string{"text": text}, &resp); err != nil {
		fmt.Fprintf(stderr, "choir run start: %v\n", err)
		return 1
	}
	return writeJSON(stdout, resp)
}

func runRunStatus(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir run status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	c, err := newClient(fs, args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir run status: %v\n", err)
		return 2
	}
	rest := fs.Args()
	if len(rest) != 1 || strings.TrimSpace(rest[0]) == "" {
		fmt.Fprintln(stderr, "choir run status: run id required")
		return 2
	}
	id := strings.TrimSpace(rest[0])
	var resp json.RawMessage
	if err := c.do(http.MethodGet, "/api/runs/"+url.PathEscape(id), nil, &resp); err != nil {
		fmt.Fprintf(stderr, "choir run status %s: %v\n", id, err)
		return 1
	}
	return writeJSON(stdout, resp)
}

func runRunList(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir run list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	limit := fs.Int("limit", defaultListLimit, "Maximum number of recent runs")
	c, err := newClient(fs, args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir run list: %v\n", err)
		return 2
	}
	if len(fs.Args()) != 0 {
		fmt.Fprintln(stderr, "choir run list: unexpected positional arguments")
		return 2
	}
	if *limit <= 0 || *limit > 500 {
		fmt.Fprintln(stderr, "choir run list: --limit must be between 1 and 500")
		return 2
	}
	var resp json.RawMessage
	path := fmt.Sprintf("/api/runs?limit=%d", *limit)
	if err := c.do(http.MethodGet, path, nil, &resp); err != nil {
		fmt.Fprintf(stderr, "choir run list: %v\n", err)
		return 1
	}
	return writeJSON(stdout, resp)
}

func runRunCancel(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir run cancel", flag.ContinueOnError)
	fs.SetOutput(stderr)
	c, err := newClient(fs, args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir run cancel: %v\n", err)
		return 2
	}
	rest := fs.Args()
	if len(rest) != 1 || strings.TrimSpace(rest[0]) == "" {
		fmt.Fprintln(stderr, "choir run cancel: run id required")
		return 2
	}
	id := strings.TrimSpace(rest[0])
	var resp json.RawMessage
	if err := c.do(http.MethodPost, "/api/runs/"+url.PathEscape(id)+"/cancel", nil, &resp); err != nil {
		fmt.Fprintf(stderr, "choir run cancel %s: %v\n", id, err)
		return 1
	}
	return writeJSON(stdout, resp)
}

// promptBarSubmitResponse mirrors textureowner.promptBarSubmitResponse.
type promptBarSubmitResponse struct {
	SubmissionID string `json:"submission_id"`
	State        string `json:"state"`
	CreatedAt    string `json:"created_at"`
	StatusURL    string `json:"status_url"`
}

// ---- api-key ----

func runAPIKey(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "choir api-key: subcommand required (list|create|revoke)")
		return 2
	}
	sub := args[0]
	switch sub {
	case "list":
		return runAPIKeyList(args[1:], stdout, stderr)
	case "create":
		return runAPIKeyCreate(args[1:], stdout, stderr)
	case "revoke":
		return runAPIKeyRevoke(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "choir api-key: unknown subcommand %q\n", sub)
		return 2
	}
}

func runAPIKeyList(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir api-key list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	c, err := newClient(fs, args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir api-key list: %v\n", err)
		return 2
	}
	var resp json.RawMessage
	if err := c.do(http.MethodGet, "/auth/api-keys", nil, &resp); err != nil {
		fmt.Fprintf(stderr, "choir api-key list: %v\n", err)
		return 1
	}
	return writeJSON(stdout, resp)
}

func runAPIKeyCreate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir api-key create", flag.ContinueOnError)
	fs.SetOutput(stderr)
	labelFlag := fs.String("label", "CLI key", "Label for the new API key")
	scopesFlag := fs.String("scopes", "read:texture,read:base,read:runtime", "Comma-separated child scopes (must be within the caller's delegated scopes)")
	c, err := newClient(fs, args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir api-key create: %v\n", err)
		return 2
	}
	scopes := []string{}
	for _, s := range strings.Split(*scopesFlag, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			scopes = append(scopes, s)
		}
	}
	body := map[string]any{
		"label":  strings.TrimSpace(*labelFlag),
		"scopes": scopes,
	}
	var resp json.RawMessage
	if err := c.do(http.MethodPost, "/auth/api-keys", body, &resp); err != nil {
		fmt.Fprintf(stderr, "choir api-key create: %v\n", err)
		return 1
	}
	return writeJSON(stdout, resp)
}

func runAPIKeyRevoke(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir api-key revoke", flag.ContinueOnError)
	fs.SetOutput(stderr)
	c, err := newClient(fs, args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir api-key revoke: %v\n", err)
		return 2
	}
	rest := fs.Args()
	if len(rest) == 0 || strings.TrimSpace(rest[0]) == "" {
		fmt.Fprintln(stderr, "choir api-key revoke: key id required")
		return 2
	}
	id := strings.TrimSpace(rest[0])
	if err := c.do(http.MethodDelete, "/auth/api-keys/"+id, nil, nil); err != nil {
		fmt.Fprintf(stderr, "choir api-key revoke %s: %v\n", id, err)
		return 1
	}
	fmt.Fprintf(stdout, `{"revoked":%q}`+"\n", id)
	return 0
}
