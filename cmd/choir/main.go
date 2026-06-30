// Command choir is the headless control surface for Choir. It wraps the
// /api/ HTTP surface with API key (Bearer choir_sk_...) auth so agents and
// scripts can read Texture documents, observe trajectories, search, and
// verify the Universal Wire news feed without a browser.
//
// Auth: CHOIR_API_KEY env var or --api-key flag. Host: CHOIR_HOST env var
// or --host flag (defaults to https://choir.news).
//
// This is Phase 1 of nucleus-cli-v0: it targets the existing /api/ routes
// that the proxy already auth-gates with API keys. The graph-native
// /api/v1/ surface (agent-api-graph-native-v0) is Phase 2 and will migrate
// these commands once live.
package main

import (
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
	apiKeyPrefix     = "choir_sk_"
	defaultTimeout   = 30 * time.Second
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
  texture read <doc>  Read a Texture document + current revision
  texture history <doc>  List revision history for a document
  search <query>      Search the corpus
  version             Print CLI version
  help                Print this usage

Auth:
  --api-key string    API key (choir_sk_...). Defaults to $CHOIR_API_KEY.
  --host string       Choir host. Defaults to $CHOIR_HOST or https://choir.news.

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
	return &client{
		host:   h,
		apiKey: key,
		http:   &http.Client{Timeout: defaultTimeout},
		stdout: stdout,
		stderr: stderr,
	}, nil
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

// do performs an authenticated GET and decodes the JSON response into out.
// On non-2xx it returns an error with the response body.
func (c *client) do(method, path string, out any) error {
	url := c.host + path
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &apiErrorResp{Status: resp.StatusCode, Body: string(body)}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode response: %w (body: %s)", err, truncate(string(body), 200))
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
		if err := c.do(http.MethodGet, "/api/universal-wire/stories", &resp); err != nil {
			fmt.Fprintf(stderr, "choir wire stories: %v\n", err)
			return 1
		}
		return writeJSON(stdout, resp)
	case "diagnostics":
		var resp wireStoriesResponse
		if err := c.do(http.MethodGet, "/api/universal-wire/stories", &resp); err != nil {
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
	if err := c.do(http.MethodGet, "/api/trajectories", &resp); err != nil {
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
	if err := c.do(http.MethodGet, "/api/trajectories/"+id, &resp); err != nil {
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
	TrajectoryID   string    `json:"trajectory_id"`
	OwnerID        string    `json:"owner_id"`
	Kind           string    `json:"kind"`
	SettlementRule string    `json:"settlement_rule,omitempty"`
	CreatedAt      time.Time `json:"created_at,omitempty"`
	UpdatedAt      time.Time `json:"updated_at,omitempty"`
}

// ---- texture ----

func runTexture(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "choir texture: subcommand required (read|history)")
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
		if err := c.do(http.MethodGet, "/api/texture/documents/"+docID, &resp); err != nil {
			fmt.Fprintf(stderr, "choir texture read %s: %v\n", docID, err)
			return 1
		}
		return writeJSON(stdout, resp)
	case "history":
		var resp json.RawMessage
		if err := c.do(http.MethodGet, "/api/texture/documents/"+docID+"/history", &resp); err != nil {
			fmt.Fprintf(stderr, "choir texture history %s: %v\n", docID, err)
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
	if err := c.do(http.MethodGet, "/api/platform/retrieval/search?q="+url.QueryEscape(q), &resp); err != nil {
		fmt.Fprintf(stderr, "choir search: %v\n", err)
		return 1
	}
	return writeJSON(stdout, resp)
}
