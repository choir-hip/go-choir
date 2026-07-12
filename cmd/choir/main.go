// Command choir is the headless control surface for Choir. It wraps the
// /api/ HTTP surface with API key (Bearer choir_sk_...) auth so agents and
// scripts can read Texture documents, observe trajectories, search,
// start runs, and verify the Universal Wire news feed without a browser.
//
// Auth: CHOIR_API_KEY env var or --api-key flag. Host: CHOIR_HOST env var
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
  version             Print CLI version
  help                Print this usage

Auth:
  --api-key string    API key (choir_sk_...). Defaults to $CHOIR_API_KEY.
  --host string       Choir host. Defaults to $CHOIR_HOST or https://choir.news.
  --timeout duration  Request timeout. Defaults to $CHOIR_TIMEOUT or 75s.

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
	apiKey := flags.String("api-key", os.Getenv(apiKeyEnvVar), "API key (choir_sk_...)")
	host := flags.String("host", envOr(hostEnvVar, defaultHost), "Choir host")
	timeout := flags.String("timeout", "", "Request timeout (for example 75s or 2m)")
	if err := flags.Parse(args); err != nil {
		return nil, err
	}
	key := strings.TrimSpace(*apiKey)
	if key == "" {
		return nil, fmt.Errorf("api key required: set --api-key or $%s", apiKeyEnvVar)
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

// wireStoriesResponse mirrors internal/runtime.universalWireStoriesResponse.
// Defined here to avoid importing the runtime package (which needs cgo/ICU).
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
	if len(rest) == 0 || strings.TrimSpace(rest[0]) == "" {
		fmt.Fprintln(stderr, "choir run status: submission id required")
		return 2
	}
	id := strings.TrimSpace(rest[0])
	var resp json.RawMessage
	if err := c.do(http.MethodGet, "/api/prompt-bar/submissions/"+id, nil, &resp); err != nil {
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
	path := fmt.Sprintf("/api/agent/loops?limit=%d", *limit)
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
	if err := c.do(http.MethodPost, "/api/agent/cancel", map[string]string{"loop_id": id}, &resp); err != nil {
		fmt.Fprintf(stderr, "choir run cancel %s: %v\n", id, err)
		return 1
	}
	return writeJSON(stdout, resp)
}

// promptBarSubmitResponse mirrors internal/runtime.promptBarSubmitResponse.
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
