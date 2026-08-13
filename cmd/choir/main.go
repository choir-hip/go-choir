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
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/buildinfo"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
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
var executionIdentityPlatformTrustDigest = computerevent.PlatformControlTrustDigest

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
	case "lifecycle":
		return runLifecycle(rest, stdout, stderr)
	case "texture":
		return runTexture(rest, stdout, stderr)
	case "search":
		return runSearch(rest, stdout, stderr)
	case "run":
		return runRun(rest, stdout, stderr)
	case "computer":
		return runComputer(rest, stdout, stderr)
	case "identity":
		return runExecutionIdentity(rest, stdout, stderr)
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
  lifecycle snapshot <id>  Reconstruct one durable lifecycle from canonical state
  lifecycle events <id>  Read reducer events after a durable cursor
  lifecycle <snapshot|events>  Observe the narrow durable-work protocol
  identity            Verify a nonce-bound signed execution identity
  texture create --request-id id --title title --content text  Create a Texture document
  texture read <doc>  Read a Texture document's metadata (title, current revision id)
  texture history <doc>  List revision history for a document (metadata only)
  texture revisions <doc>  List revisions with full content bodies
  texture show [--revision id] <doc>  Show the current or exact historical version as JSON
  texture watch [--after cursor] <doc>  Watch durable version/control events as JSONL
  texture open-source --revision id --source-ref id --source-ref-version id <doc>
  texture tell|correct --request-id id <doc> <instruction>
  search <query>      Search the corpus
  run start <text>    Submit a prompt to the conductor (starts a run)
  run status <id>     Get the status of a prompt-bar submission
  run list            List recent owner-scoped runs
  run cancel <id>     Cancel an owner-scoped pending or running run
  computer replay-completeness  Capture live-versus-event-replay state evidence
  computer replace-workspace  Quarantine the VM-local workspace onto current DDL
  computer rematerialize-from-tape  Rebuild VM-local state from the event tape
  computer restore    Restore VM-local state and served SPA from a checkpoint
  computer bootstrap-chain    Establish a canonical event chain on a pre-genesis computer
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
	idempotencyKey := fs.String("idempotency-key", "", "Stable caller-supplied command key for replay/conflict detection")
	expectedLifecycleVersion := fs.Int64("expected-lifecycle-version", 0, "Lifecycle version observed before cancellation")
	expectedHeadRevisionID := fs.String("expected-head-revision-id", "", "Artifact head observed before cancellation")
	reason := fs.String("reason", "owner cancellation", "Cancellation reason included in the request commitment")
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
	if strings.TrimSpace(*idempotencyKey) == "" || *expectedLifecycleVersion <= 0 || strings.TrimSpace(*expectedHeadRevisionID) == "" {
		fmt.Fprintln(stderr, "choir trajectory cancel: --idempotency-key, --expected-lifecycle-version, and --expected-head-revision-id are required")
		return 2
	}
	request := map[string]any{
		"idempotency_key":            strings.TrimSpace(*idempotencyKey),
		"expected_lifecycle_version": *expectedLifecycleVersion,
		"expected_head_revision_id":  strings.TrimSpace(*expectedHeadRevisionID),
		"reason":                     strings.TrimSpace(*reason),
	}
	var resp json.RawMessage
	path := "/api/trajectories/" + url.PathEscape(id) + "/cancel"
	if err := c.do(http.MethodPost, path, request, &resp); err != nil {
		fmt.Fprintf(stderr, "choir trajectory cancel %s: %v\n", id, err)
		return 1
	}
	return writeJSON(stdout, resp)
}

type executionIdentityCLIPlatformAttestation struct {
	Receipt         computerevent.Receipt `json:"receipt"`
	SignerPublicKey string                `json:"signer_public_key"`
}

type executionIdentityCLIEnvelope struct {
	Schema              string                                   `json:"schema"`
	Identity            map[string]any                           `json:"identity"`
	Receipt             computerevent.Receipt                    `json:"receipt"`
	SignerPublicKey     string                                   `json:"signer_public_key"`
	Joined              bool                                     `json:"joined,omitempty"`
	Guest               *executionIdentityCLIEnvelope            `json:"guest,omitempty"`
	VMCTL               map[string]any                           `json:"vmctl,omitempty"`
	RouteDigest         string                                   `json:"route_digest,omitempty"`
	HostBuild           json.RawMessage                          `json:"host_build,omitempty"`
	DeploymentReceipt   json.RawMessage                          `json:"deployment_receipt,omitempty"`
	PlatformAttestation *executionIdentityCLIPlatformAttestation `json:"platform_attestation,omitempty"`
}

type executionIdentityCLIResolver struct {
	ref computerevent.SignerRef
	key ed25519.PublicKey
}

func (r executionIdentityCLIResolver) ResolveReceiptKey(domain, _ string, keyID string, _ uint64, _ time.Time) (ed25519.PublicKey, error) {
	if domain != r.ref.SignerDomain || keyID != r.ref.KeyID {
		return nil, fmt.Errorf("execution identity signer mismatch")
	}
	return r.key, nil
}

func sameJSONValue(left, right any) bool {
	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftJSON, rightJSON)
}

func executionIdentityCLIDigest(value any) (string, error) {
	canonical, err := computerevent.CanonicalJSON(value)
	if err != nil {
		return "", err
	}
	return "sha256:" + computerevent.DigestBytes(canonical), nil
}

const executionIdentityCLIAudience = "choir.news/acceptance/execution-identity"

func executionIdentityCLIFullCommit(commit string) bool {
	if len(commit) != 40 {
		return false
	}
	for _, r := range commit {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

func executionIdentityCLICommonCommit(signed *executionIdentityCLIEnvelope, hostRaw, deploymentRaw json.RawMessage) (string, bool) {
	if signed == nil {
		return "", false
	}
	var host buildinfo.Info
	var deployment struct {
		TargetCommit string `json:"target_commit"`
		Artifacts    map[string]struct {
			Commit string `json:"commit"`
			Status string `json:"status"`
		} `json:"artifacts"`
		HostIdentity struct {
			CanonicalRef       string `json:"canonical_ref"`
			NixOSClosureDigest string `json:"nixos_closure_digest"`
			Services           map[string]struct {
				Role           string `json:"role"`
				PackageDigest  string `json:"package_digest"`
				EmbeddedCommit string `json:"embedded_commit"`
			} `json:"services"`
		} `json:"host_identity"`
	}
	if json.Unmarshal(hostRaw, &host) != nil || json.Unmarshal(deploymentRaw, &deployment) != nil {
		return "", false
	}
	target := strings.TrimSpace(deployment.TargetCommit)
	build, ok := signed.Identity["build"].(map[string]any)
	proxy := deployment.HostIdentity.Services["proxy"]
	guestCommit := strings.TrimSpace(fmt.Sprint(build["commit"]))
	guestDeployedCommit := strings.TrimSpace(fmt.Sprint(build["deployed_commit"]))
	if !ok || !executionIdentityCLIFullCommit(target) ||
		!executionIdentityCLIFullCommit(host.Commit) ||
		!executionIdentityCLIFullCommit(proxy.EmbeddedCommit) ||
		host.Service != "proxy" || host.Commit != proxy.EmbeddedCommit ||
		!executionIdentityCLIFullCommit(guestCommit) || guestDeployedCommit != guestCommit ||
		deployment.HostIdentity.CanonicalRef != "refs/heads/main@"+target ||
		!strings.HasPrefix(deployment.HostIdentity.NixOSClosureDigest, "sha256:") ||
		proxy.Role != "proxy" || !strings.HasPrefix(proxy.PackageDigest, "sha256:") {
		return "", false
	}
	if artifact, selected := deployment.Artifacts["proxy"]; selected {
		if artifact.Commit != target || artifact.Status != "active" ||
			host.Commit != target || host.DeployedCommit != target {
			return "", false
		}
	} else if strings.TrimSpace(host.DeployedCommit) != "" {
		return "", false
	}
	for role, service := range deployment.HostIdentity.Services {
		if service.Role != role || !strings.HasPrefix(service.PackageDigest, "sha256:") ||
			!executionIdentityCLIFullCommit(service.EmbeddedCommit) {
			return "", false
		}
	}
	return target, true
}

func runExecutionIdentity(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir identity", flag.ContinueOnError)
	fs.SetOutput(stderr)
	c, err := newClient(fs, args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir identity: %v\n", err)
		return 2
	}
	if len(fs.Args()) != 0 {
		fmt.Fprintln(stderr, "choir identity: no positional arguments allowed")
		return 2
	}
	expectedPlatformSignerKeyDigest, trustErr := executionIdentityPlatformTrustDigest()
	if trustErr != nil || !strings.HasPrefix(expectedPlatformSignerKeyDigest, "sha256:") {
		fmt.Fprintln(stderr, "choir identity: platform trust configuration unavailable")
		return 1
	}
	nonceBytes := make([]byte, 24)
	if _, err := rand.Read(nonceBytes); err != nil {
		fmt.Fprintf(stderr, "choir identity: generate nonce: %v\n", err)
		return 1
	}
	nonce := base64.RawURLEncoding.EncodeToString(nonceBytes)
	var envelope executionIdentityCLIEnvelope
	if err := c.do(http.MethodGet, "/api/acceptance/execution-identity?nonce="+url.QueryEscape(nonce), nil, &envelope); err != nil {
		fmt.Fprintf(stderr, "choir identity: %v\n", err)
		return 1
	}
	if !envelope.Joined || envelope.Guest == nil || envelope.PlatformAttestation == nil {
		fmt.Fprintln(stderr, "choir identity: platform identity join refused")
		return 1
	}
	signed := envelope.Guest
	if signed.Schema != "choir.execution_identity.v1" || signed.Identity["schema"] != signed.Schema || signed.Identity["nonce"] != nonce || signed.Identity["audience"] != executionIdentityCLIAudience {
		fmt.Fprintln(stderr, "choir identity: schema or nonce binding refused")
		return 1
	}
	publicKey, err := base64.RawStdEncoding.DecodeString(signed.SignerPublicKey)
	if err != nil || len(publicKey) != ed25519.PublicKeySize || len(signed.Receipt.RequiredSigners) != 1 {
		fmt.Fprintln(stderr, "choir identity: invalid signer public key")
		return 1
	}
	actualSignerKeyDigest := "sha256:" + computerevent.DigestBytes(publicKey)
	if signed.Receipt.ReceiptKind != "ExecutionIdentity" {
		fmt.Fprintln(stderr, "choir identity: unexpected receipt kind")
		return 1
	}
	ref := signed.Receipt.RequiredSigners[0]
	if ref.SignerDomain != "guest-core" || signed.Receipt.Verify(executionIdentityCLIResolver{ref: ref, key: ed25519.PublicKey(publicKey)}) != nil {
		fmt.Fprintln(stderr, "choir identity: signature verification failed")
		return 1
	}
	if !sameJSONValue(signed.Receipt.IssuedAt, signed.Identity["issued_at"]) {
		fmt.Fprintln(stderr, "choir identity: signed field issued_at mismatch")
		return 1
	}
	for key, value := range signed.Identity {
		if key == "issued_at" {
			continue
		}
		if !sameJSONValue(signed.Receipt.KindFields[key], value) {
			fmt.Fprintf(stderr, "choir identity: signed field %s mismatch\n", key)
			return 1
		}
	}
	expiresAt, expiresErr := time.Parse(time.RFC3339Nano, fmt.Sprint(signed.Identity["expires_at"]))
	issuedAt, issuedErr := time.Parse(time.RFC3339Nano, fmt.Sprint(signed.Identity["issued_at"]))
	now := time.Now().UTC()
	if expiresErr != nil || issuedErr != nil || !expiresAt.After(now) || issuedAt.After(now.Add(30*time.Second)) || expiresAt.Sub(issuedAt) > 2*time.Minute {
		fmt.Fprintln(stderr, "choir identity: expired or invalid validity window")
		return 1
	}
	build, ok := signed.Identity["build"].(map[string]any)
	if !ok || strings.TrimSpace(fmt.Sprint(build["commit"])) == "" || build["commit"] != build["deployed_commit"] {
		fmt.Fprintln(stderr, "choir identity: build/deploy identity conflict")
		return 1
	}
	targetCommit, commonCommit := executionIdentityCLICommonCommit(signed, envelope.HostBuild, envelope.DeploymentReceipt)
	if !commonCommit {
		fmt.Fprintln(stderr, "choir identity: host, guest, route, and deployment commit join refused")
		return 1
	}
	if envelope.PlatformAttestation != nil {
		platformKey, decodeErr := base64.RawStdEncoding.DecodeString(envelope.PlatformAttestation.SignerPublicKey)
		if decodeErr != nil || len(platformKey) != ed25519.PublicKeySize ||
			!strings.EqualFold(expectedPlatformSignerKeyDigest, "sha256:"+computerevent.DigestBytes(platformKey)) {
			fmt.Fprintln(stderr, "choir identity: platform signer key does not match the repository trust manifest")
			return 1
		}
		guestDigest, guestDigestErr := executionIdentityCLIDigest(signed.Receipt)
		routeDigest := strings.TrimSpace(envelope.RouteDigest)
		hostBuildDigest, hostBuildDigestErr := executionIdentityCLIDigest(envelope.HostBuild)
		deploymentDigest, deploymentDigestErr := executionIdentityCLIDigest(envelope.DeploymentReceipt)
		if guestDigestErr != nil || !strings.HasPrefix(routeDigest, "sha256:") || hostBuildDigestErr != nil || deploymentDigestErr != nil {
			fmt.Fprintln(stderr, "choir identity: platform join digest unavailable")
			return 1
		}
		expectedFields := map[string]any{
			"schema": signed.Schema, "nonce": signed.Identity["nonce"], "audience": executionIdentityCLIAudience,
			"deployed_commit": targetCommit,
			"computer_id":     signed.Identity["computer_id"], "realization_id": signed.Identity["realization_id"],
			"vm_epoch": signed.Identity["vm_epoch"], "guest_receipt_digest": guestDigest,
			"guest_signer_key_digest": actualSignerKeyDigest,
			"vmctl":                   envelope.VMCTL, "route_digest": routeDigest, "host_build_digest": hostBuildDigest,
			"deployment_receipt_digest": deploymentDigest,
		}
		platformReceipt := envelope.PlatformAttestation.Receipt
		expectedKeyID := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(expectedPlatformSignerKeyDigest)), "sha256:")
		if platformReceipt.ReceiptKind != "ExecutionIdentityJoin" || platformReceipt.Issuer != "corpusd" ||
			len(platformReceipt.RequiredSigners) != 1 || platformReceipt.RequiredSigners[0].SignerDomain != "platform-control" ||
			len(expectedKeyID) < 16 || !strings.EqualFold(platformReceipt.RequiredSigners[0].KeyID, expectedKeyID[:16]) ||
			!sameJSONValue(platformReceipt.KindFields, expectedFields) ||
			platformReceipt.Verify(executionIdentityCLIResolver{ref: platformReceipt.RequiredSigners[0], key: ed25519.PublicKey(platformKey)}) != nil {
			fmt.Fprintln(stderr, "choir identity: platform identity join verification failed")
			return 1
		}
	}
	return writeJSON(stdout, envelope)
}

func validateDurableWorkResponse(raw json.RawMessage) error {
	var envelope struct {
		Schema string `json:"schema"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return err
	}
	if envelope.Schema != "choir.durable_work.v1" {
		return fmt.Errorf("unsupported lifecycle schema %q", envelope.Schema)
	}
	return nil
}

func runLifecycle(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "choir lifecycle: subcommand required (snapshot|events|capsule-evidence)")
		return 2
	}
	subcommand := args[0]
	fs := flag.NewFlagSet("choir lifecycle "+subcommand, flag.ContinueOnError)
	fs.SetOutput(stderr)
	after := fs.Int64("after", 0, "Event cursor (events only)")
	limit := fs.Int("limit", 100, "Maximum events per page (events only)")
	attempt := fs.Uint64("attempt", 0, "Positive assignment attempt (capsule-evidence only)")
	c, err := newClient(fs, args[1:], stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir lifecycle %s: %v\n", subcommand, err)
		return 2
	}
	if subcommand == "capsule-evidence" {
		rest := fs.Args()
		if len(rest) != 2 || rest[0] == "" || rest[1] == "" || rest[0] != strings.TrimSpace(rest[0]) || rest[1] != strings.TrimSpace(rest[1]) || *attempt == 0 {
			fmt.Fprintln(stderr, "choir lifecycle capsule-evidence: trajectory id, assignment id, and positive --attempt required")
			return 2
		}
		var response json.RawMessage
		path := fmt.Sprintf("/api/trajectories/%s/capsule-evidence/%s?attempt=%d", url.PathEscape(rest[0]), url.PathEscape(rest[1]), *attempt)
		if err := c.do(http.MethodGet, path, nil, &response); err != nil {
			fmt.Fprintf(stderr, "choir lifecycle capsule-evidence: %v\n", err)
			return 1
		}
		var envelope struct {
			Schema string `json:"schema"`
		}
		if err := json.Unmarshal(response, &envelope); err != nil || envelope.Schema != "choir.co_super_capsule_evidence/v1" {
			fmt.Fprintln(stderr, "choir lifecycle capsule-evidence: invalid capsule evidence response")
			return 1
		}
		return writeJSON(stdout, response)
	}
	if subcommand == "events" {
		rest := fs.Args()
		if len(rest) != 1 || strings.TrimSpace(rest[0]) == "" || *after < 0 || *limit <= 0 {
			fmt.Fprintln(stderr, "choir lifecycle events: trajectory id, non-negative --after, and positive --limit required")
			return 2
		}
		var response json.RawMessage
		path := fmt.Sprintf("/api/trajectories/%s/events?after=%d&limit=%d", url.PathEscape(strings.TrimSpace(rest[0])), *after, *limit)
		if err := c.do(http.MethodGet, path, nil, &response); err != nil {
			fmt.Fprintf(stderr, "choir lifecycle events: %v\n", err)
			return 1
		}
		if err := validateDurableWorkResponse(response); err != nil {
			fmt.Fprintf(stderr, "choir lifecycle events: %v\n", err)
			return 1
		}
		return writeJSON(stdout, response)
	}
	if subcommand == "snapshot" {
		rest := fs.Args()
		if len(rest) != 1 || strings.TrimSpace(rest[0]) == "" {
			fmt.Fprintln(stderr, "choir lifecycle snapshot: trajectory id required")
			return 2
		}
		var response json.RawMessage
		path := "/api/trajectories/" + url.PathEscape(strings.TrimSpace(rest[0]))
		if err := c.do(http.MethodGet, path, nil, &response); err != nil {
			fmt.Fprintf(stderr, "choir lifecycle snapshot: %v\n", err)
			return 1
		}
		if err := validateDurableWorkResponse(response); err != nil {
			fmt.Fprintf(stderr, "choir lifecycle snapshot: %v\n", err)
			return 1
		}
		return writeJSON(stdout, response)
	}
	fmt.Fprintf(stderr, "choir lifecycle: unknown subcommand %q\n", subcommand)
	return 2
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

type textureShowCLIResponse struct {
	Schema   string          `json:"schema"`
	Document json.RawMessage `json:"document"`
	Revision json.RawMessage `json:"revision"`
	Current  bool            `json:"current"`
}

type textureWatchCLIPage struct {
	Schema         string            `json:"schema"`
	Events         []json.RawMessage `json:"events"`
	NextCursor     int64             `json:"next_cursor"`
	Watermark      int64             `json:"watermark"`
	CursorExpired  bool              `json:"cursor_expired,omitempty"`
	ReplayRequired bool              `json:"replay_required,omitempty"`
}

func textureWatchEventTerminal(raw json.RawMessage) bool {
	var event struct {
		WorkState string `json:"work_state"`
	}
	return json.Unmarshal(raw, &event) == nil && event.WorkState == "terminal"
}

func runTexture(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "choir texture: subcommand required (create|read|history|revisions|show|watch|open-source|tell|correct)")
		return 2
	}
	sub := args[0]
	fs := flag.NewFlagSet("choir texture "+sub, flag.ContinueOnError)
	fs.SetOutput(stderr)
	var revisionID, sourceRefID, sourceRefVersionID string
	var title, initialContent, clientRequestID, expectedHeadRevisionID string
	var after int64
	var limit, reconnectAttempts int
	var once bool
	var pollInterval time.Duration
	switch sub {
	case "create":
		fs.StringVar(&title, "title", "", "Texture document title")
		fs.StringVar(&initialContent, "content", "", "initial owner-authored v0 content and objective")
		fs.StringVar(&clientRequestID, "request-id", "", "stable client occurrence id for exact retries")
	case "show":
		fs.StringVar(&revisionID, "revision", "", "exact historical revision id; defaults to current head")
	case "watch":
		fs.Int64Var(&after, "after", 0, "resume after this durable cursor")
		fs.IntVar(&limit, "limit", 100, "events per durable page")
		fs.BoolVar(&once, "once", false, "fetch one durable page and exit")
		fs.DurationVar(&pollInterval, "poll-interval", time.Second, "delay between caught-up durable pages")
		fs.IntVar(&reconnectAttempts, "reconnect-attempts", 3, "consecutive request failures tolerated before exit")
	case "tell", "correct":
		fs.StringVar(&clientRequestID, "request-id", "", "stable client occurrence id for exact retries")
		fs.StringVar(&expectedHeadRevisionID, "expected-head", "", "expected current revision; defaults to a fresh document read")
	case "open-source":
		fs.StringVar(&revisionID, "revision", "", "exact Texture revision id")
		fs.StringVar(&sourceRefID, "source-ref", "", "canonical source_ref id")
		fs.StringVar(&sourceRefVersionID, "source-ref-version", "", "exact source_ref version id")
	}
	c, err := newClient(fs, args[1:], stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir texture: %v\n", err)
		return 2
	}
	rest := fs.Args()
	if sub == "create" {
		if strings.TrimSpace(title) == "" || strings.TrimSpace(initialContent) == "" || strings.TrimSpace(clientRequestID) == "" {
			fmt.Fprintln(stderr, "choir texture create: --title, --content, and --request-id are required")
			return 2
		}
		var created json.RawMessage
		body := map[string]string{"title": strings.TrimSpace(title), "initial_content": strings.TrimSpace(initialContent), "client_request_id": strings.TrimSpace(clientRequestID)}
		if err := c.do(http.MethodPost, "/api/texture/lifecycle-documents", body, &created); err != nil {
			fmt.Fprintf(stderr, "choir texture create: %v\n", err)
			return 1
		}
		var envelope struct {
			Schema string `json:"schema"`
		}
		if json.Unmarshal(created, &envelope) != nil || envelope.Schema != "choir.texture_create.v1" {
			fmt.Fprintf(stderr, "choir texture create: unsupported create schema %q\n", envelope.Schema)
			return 1
		}
		return writeJSON(stdout, created)
	}
	if len(rest) == 0 || strings.TrimSpace(rest[0]) == "" {
		fmt.Fprintf(stderr, "choir texture %s: document id required\n", sub)
		return 2
	}
	docID := strings.TrimSpace(rest[0])
	escapedDocID := url.PathEscape(docID)
	switch sub {
	case "read":
		var resp json.RawMessage
		if err := c.do(http.MethodGet, "/api/texture/documents/"+escapedDocID, nil, &resp); err != nil {
			fmt.Fprintf(stderr, "choir texture read %s: %v\n", docID, err)
			return 1
		}
		return writeJSON(stdout, resp)
	case "history":
		var resp json.RawMessage
		if err := c.do(http.MethodGet, "/api/texture/documents/"+escapedDocID+"/history", nil, &resp); err != nil {
			fmt.Fprintf(stderr, "choir texture history %s: %v\n", docID, err)
			return 1
		}
		return writeJSON(stdout, resp)
	case "revisions":
		var resp json.RawMessage
		if err := c.do(http.MethodGet, "/api/texture/documents/"+escapedDocID+"/revisions", nil, &resp); err != nil {
			fmt.Fprintf(stderr, "choir texture revisions %s: %v\n", docID, err)
			return 1
		}
		return writeJSON(stdout, resp)
	case "show":
		var document json.RawMessage
		if err := c.do(http.MethodGet, "/api/texture/documents/"+escapedDocID, nil, &document); err != nil {
			fmt.Fprintf(stderr, "choir texture show %s: %v\n", docID, err)
			return 1
		}
		var head struct {
			CurrentRevisionID string `json:"current_revision_id"`
		}
		if err := json.Unmarshal(document, &head); err != nil {
			fmt.Fprintf(stderr, "choir texture show %s: decode document: %v\n", docID, err)
			return 1
		}
		requestedRevisionID := strings.TrimSpace(revisionID)
		if requestedRevisionID == "" {
			requestedRevisionID = strings.TrimSpace(head.CurrentRevisionID)
		}
		if requestedRevisionID == "" {
			fmt.Fprintf(stderr, "choir texture show %s: document has no current revision\n", docID)
			return 1
		}
		var revision json.RawMessage
		if err := c.do(http.MethodGet, "/api/texture/revisions/"+url.PathEscape(requestedRevisionID), nil, &revision); err != nil {
			fmt.Fprintf(stderr, "choir texture show %s: %v\n", docID, err)
			return 1
		}
		var exactRevision struct {
			DocID string `json:"doc_id"`
		}
		if err := json.Unmarshal(revision, &exactRevision); err != nil || strings.TrimSpace(exactRevision.DocID) != docID {
			fmt.Fprintf(stderr, "choir texture show %s: revision %s is not an exact version of this document\n", docID, requestedRevisionID)
			return 1
		}
		return writeJSON(stdout, textureShowCLIResponse{
			Schema: "choir.texture_show.v1", Document: document, Revision: revision,
			Current: requestedRevisionID == strings.TrimSpace(head.CurrentRevisionID),
		})
	case "watch":
		if after < 0 || limit <= 0 || limit > 1000 || reconnectAttempts < 0 || pollInterval < 0 {
			fmt.Fprintln(stderr, "choir texture watch: after, limit, reconnect-attempts, or poll-interval is invalid")
			return 2
		}
		cursor, failures := after, 0
		encoder := json.NewEncoder(stdout)
		for {
			path := fmt.Sprintf("/api/texture/documents/%s/events?after=%d&limit=%d", escapedDocID, cursor, limit)
			var page textureWatchCLIPage
			if err := c.do(http.MethodGet, path, nil, &page); err != nil {
				var responseErr *apiErrorResp
				if errors.As(err, &responseErr) && responseErr.Status == http.StatusConflict {
					var expired textureWatchCLIPage
					if json.Unmarshal([]byte(responseErr.Body), &expired) == nil && expired.CursorExpired && expired.ReplayRequired {
						fmt.Fprintf(stderr, "choir texture watch %s: cursor %d expired; replay required at watermark %d\n", docID, cursor, expired.Watermark)
						return 1
					}
				}
				failures++
				retryable := !errors.As(err, &responseErr) || responseErr.Status >= http.StatusInternalServerError
				if !retryable || failures > reconnectAttempts {
					fmt.Fprintf(stderr, "choir texture watch %s after %d: %v\n", docID, cursor, err)
					return 1
				}
				if pollInterval > 0 {
					time.Sleep(pollInterval)
				}
				continue
			}
			failures = 0
			if page.Schema != "choir.texture_observation.v1" {
				fmt.Fprintf(stderr, "choir texture watch %s: unsupported observation schema %q\n", docID, page.Schema)
				return 1
			}
			if page.CursorExpired || page.ReplayRequired {
				fmt.Fprintf(stderr, "choir texture watch %s: cursor %d expired; replay required at watermark %d\n", docID, cursor, page.Watermark)
				return 1
			}
			terminal := false
			for _, event := range page.Events {
				if err := encoder.Encode(event); err != nil {
					fmt.Fprintf(stderr, "choir texture watch %s: encode event: %v\n", docID, err)
					return 1
				}
				terminal = terminal || textureWatchEventTerminal(event)
			}
			if page.NextCursor < cursor {
				fmt.Fprintf(stderr, "choir texture watch %s: durable cursor regressed from %d to %d\n", docID, cursor, page.NextCursor)
				return 1
			}
			cursor = page.NextCursor
			if once || terminal {
				return 0
			}
			if len(page.Events) == 0 && pollInterval > 0 {
				time.Sleep(pollInterval)
			}
		}
	case "tell", "correct":
		if strings.TrimSpace(clientRequestID) == "" || len(rest) < 2 || strings.TrimSpace(strings.Join(rest[1:], " ")) == "" {
			fmt.Fprintf(stderr, "choir texture %s: --request-id and instruction text are required\n", sub)
			return 2
		}
		headID := strings.TrimSpace(expectedHeadRevisionID)
		if headID == "" {
			var document struct {
				CurrentRevisionID string `json:"current_revision_id"`
			}
			if err := c.do(http.MethodGet, "/api/texture/documents/"+escapedDocID, nil, &document); err != nil {
				fmt.Fprintf(stderr, "choir texture %s %s: %v\n", sub, docID, err)
				return 1
			}
			headID = strings.TrimSpace(document.CurrentRevisionID)
		}
		body := map[string]string{"client_request_id": strings.TrimSpace(clientRequestID), "content": strings.TrimSpace(strings.Join(rest[1:], " ")), "expected_head_revision_id": headID}
		var response json.RawMessage
		if err := c.do(http.MethodPost, "/api/texture/documents/"+escapedDocID+"/"+sub, body, &response); err != nil {
			fmt.Fprintf(stderr, "choir texture %s %s: %v\n", sub, docID, err)
			return 1
		}
		var envelope struct {
			Schema string `json:"schema"`
		}
		if json.Unmarshal(response, &envelope) != nil || envelope.Schema != "choir.texture_owner_instruction.v1" {
			fmt.Fprintf(stderr, "choir texture %s %s: unsupported owner-instruction schema %q\n", sub, docID, envelope.Schema)
			return 1
		}
		return writeJSON(stdout, response)
	case "open-source":
		if strings.TrimSpace(revisionID) == "" || strings.TrimSpace(sourceRefID) == "" || strings.TrimSpace(sourceRefVersionID) == "" {
			fmt.Fprintln(stderr, "choir texture open-source: --revision, --source-ref, and --source-ref-version are required")
			return 2
		}
		query := url.Values{}
		query.Set("revision_id", revisionID)
		query.Set("source_ref_id", sourceRefID)
		query.Set("source_ref_version_id", sourceRefVersionID)
		var resp json.RawMessage
		if err := c.do(http.MethodGet, "/api/texture/documents/"+escapedDocID+"/source-open?"+query.Encode(), nil, &resp); err != nil {
			fmt.Fprintf(stderr, "choir texture open-source %s: %v\n", docID, err)
			return 1
		}
		var sourceOpen struct {
			Schema string `json:"schema"`
		}
		if json.Unmarshal(resp, &sourceOpen) != nil || sourceOpen.Schema != "choir.texture_source_open.v1" {
			fmt.Fprintf(stderr, "choir texture open-source %s: unsupported source-open schema %q\n", docID, sourceOpen.Schema)
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
	if len(args) < 2 || args[0] != "mode" {
		fmt.Fprintln(stderr, "choir self-dev: effects are disabled; only mode get|set is available")
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
	pendingRef := fs.String("expected-pending-ref", "", "Expected pending transition reference (empty when absent)")
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
		body["expected_pending_transition_ref"] = strings.TrimSpace(*pendingRef)
		body["expected_desired_state_commitment"] = *desiredCommitment
		body["expected_effective_state_commitment"] = *effectiveCommitment
		body["bundle_digest"] = *bundle
	}
	var response json.RawMessage
	path := "/api/computers/" + url.PathEscape(strings.TrimSpace(*computerID)) + "/self-development/mode"
	if err := c.do(http.MethodPut, path, body, &response); err != nil {
		fmt.Fprintf(stderr, "choir self-dev mode set: %v\n", err)
		return 1
	}
	return writeJSON(stdout, response)
}

// ---- computer ----

func runComputer(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "choir computer: subcommand required (status|replay-completeness|replace-workspace|rematerialize-from-tape|restore|bootstrap-chain|stop|start|restart)")
		return 2
	}
	switch args[0] {
	case "status":
		return runComputerStatus(args[1:], stdout, stderr)
	case "replay-completeness":
		return runComputerReplayCompleteness(args[1:], stdout, stderr)
	case "replace-workspace":
		return runComputerReplaceWorkspace(args[1:], stdout, stderr)
	case "rematerialize-from-tape":
		return runComputerRematerializeFromTape(args[1:], stdout, stderr)
	case "restore":
		return runComputerRestore(args[1:], stdout, stderr)
	case "bootstrap-chain":
		return runComputerBootstrapChain(args[1:], stdout, stderr)
	case "stop", "start", "restart":
		return runComputerAction(args[1:], args[0], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "choir computer: unknown subcommand %q\n", args[0])
		return 2
	}
}

func runComputerStatus(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir computer status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	computerID := fs.String("computer", "", "Stable ComputerID")
	c, err := newClient(fs, args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir computer status: %v\n", err)
		return 2
	}
	if strings.TrimSpace(*computerID) == "" || len(fs.Args()) != 0 {
		fmt.Fprintln(stderr, "choir computer status: --computer is required")
		return 2
	}
	var response json.RawMessage
	path := "/api/computers/" + url.PathEscape(strings.TrimSpace(*computerID)) + "/lifecycle/status"
	if err := c.do(http.MethodGet, path, nil, &response); err != nil {
		fmt.Fprintf(stderr, "choir computer status: %v\n", err)
		return 1
	}
	return writeJSON(stdout, response)
}

func runComputerReplayCompleteness(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir computer replay-completeness", flag.ContinueOnError)
	fs.SetOutput(stderr)
	computerID := fs.String("computer", "", "Stable ComputerID")
	c, err := newClient(fs, args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir computer replay-completeness: %v\n", err)
		return 2
	}
	if strings.TrimSpace(*computerID) == "" || len(fs.Args()) != 0 {
		fmt.Fprintln(stderr, "choir computer replay-completeness: --computer is required")
		return 2
	}
	var response json.RawMessage
	path := "/api/computers/" + url.PathEscape(strings.TrimSpace(*computerID)) + "/self-development/replay-completeness"
	if err := c.do(http.MethodGet, path, nil, &response); err != nil {
		fmt.Fprintf(stderr, "choir computer replay-completeness: %v\n", err)
		return 1
	}
	return writeJSON(stdout, response)
}

func runComputerRestore(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir computer restore", flag.ContinueOnError)
	fs.SetOutput(stderr)
	computerID := fs.String("computer", "", "Stable ComputerID")
	checkpointFile := fs.String("checkpoint-file", "", "JSON checkpoint artifact")
	c, err := newClient(fs, args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir computer restore: %v\n", err)
		return 2
	}
	if strings.TrimSpace(*computerID) == "" || strings.TrimSpace(*checkpointFile) == "" || len(fs.Args()) != 0 {
		fmt.Fprintln(stderr, "choir computer restore: --computer and --checkpoint-file are required")
		return 2
	}
	raw, err := os.ReadFile(*checkpointFile)
	if err != nil {
		fmt.Fprintf(stderr, "choir computer restore: %v\n", err)
		return 2
	}
	var checkpoint json.RawMessage
	if err := json.Unmarshal(raw, &checkpoint); err != nil {
		fmt.Fprintf(stderr, "choir computer restore: invalid checkpoint: %v\n", err)
		return 2
	}
	var response json.RawMessage
	path := "/api/computers/" + url.PathEscape(strings.TrimSpace(*computerID)) + "/lifecycle/restore"
	body := map[string]any{
		"checkpoint":     checkpoint,
		"operand_scopes": []string{"vm_local", "computer_surface_frontend"},
	}
	if err := c.do(http.MethodPost, path, body, &response); err != nil {
		fmt.Fprintf(stderr, "choir computer restore: %v\n", err)
		return 1
	}
	return writeJSON(stdout, response)
}

func runComputerRematerializeFromTape(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir computer rematerialize-from-tape", flag.ContinueOnError)
	fs.SetOutput(stderr)
	computerID := fs.String("computer", "", "Stable ComputerID")
	checkpointFile := fs.String("checkpoint-file", "", "JSON checkpoint artifact")
	c, err := newClient(fs, args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir computer rematerialize-from-tape: %v\n", err)
		return 2
	}
	if strings.TrimSpace(*computerID) == "" || strings.TrimSpace(*checkpointFile) == "" || len(fs.Args()) != 0 {
		fmt.Fprintln(stderr, "choir computer rematerialize-from-tape: --computer and --checkpoint-file are required")
		return 2
	}
	raw, err := os.ReadFile(*checkpointFile)
	if err != nil {
		fmt.Fprintf(stderr, "choir computer rematerialize-from-tape: %v\n", err)
		return 2
	}
	var checkpoint json.RawMessage
	if err := json.Unmarshal(raw, &checkpoint); err != nil {
		fmt.Fprintf(stderr, "choir computer rematerialize-from-tape: invalid checkpoint: %v\n", err)
		return 2
	}
	var response json.RawMessage
	path := "/api/computers/" + url.PathEscape(strings.TrimSpace(*computerID)) + "/lifecycle/rematerialize-from-tape"
	if err := c.do(http.MethodPost, path, map[string]json.RawMessage{"checkpoint": checkpoint}, &response); err != nil {
		fmt.Fprintf(stderr, "choir computer rematerialize-from-tape: %v\n", err)
		return 1
	}
	return writeJSON(stdout, response)
}

func runComputerReplaceWorkspace(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir computer replace-workspace", flag.ContinueOnError)
	fs.SetOutput(stderr)
	computerID := fs.String("computer", "", "Stable ComputerID")
	c, err := newClient(fs, args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir computer replace-workspace: %v\n", err)
		return 2
	}
	if strings.TrimSpace(*computerID) == "" || len(fs.Args()) != 0 {
		fmt.Fprintln(stderr, "choir computer replace-workspace: --computer is required")
		return 2
	}
	var response json.RawMessage
	path := "/api/computers/" + url.PathEscape(strings.TrimSpace(*computerID)) + "/lifecycle/replace-workspace"
	if err := c.do(http.MethodPost, path, nil, &response); err != nil {
		fmt.Fprintf(stderr, "choir computer replace-workspace: %v\n", err)
		return 1
	}
	return writeJSON(stdout, response)
}

func runComputerBootstrapChain(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir computer bootstrap-chain", flag.ContinueOnError)
	fs.SetOutput(stderr)
	computerID := fs.String("computer", "", "Stable ComputerID")
	c, err := newClient(fs, args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir computer bootstrap-chain: %v\n", err)
		return 2
	}
	if strings.TrimSpace(*computerID) == "" || len(fs.Args()) != 0 {
		fmt.Fprintln(stderr, "choir computer bootstrap-chain: --computer is required")
		return 2
	}
	var response json.RawMessage
	path := "/api/computers/" + url.PathEscape(strings.TrimSpace(*computerID)) + "/lifecycle/bootstrap-chain"
	if err := c.do(http.MethodPost, path, nil, &response); err != nil {
		fmt.Fprintf(stderr, "choir computer bootstrap-chain: %v\n", err)
		return 1
	}
	return writeJSON(stdout, response)
}

func runComputerAction(args []string, action string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir computer "+action, flag.ContinueOnError)
	fs.SetOutput(stderr)
	computerID := fs.String("computer", "", "Stable ComputerID")
	idempotencyKey := fs.String("idempotency-key", "", "Unique idempotency key")
	c, err := newClient(fs, args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir computer %s: %v\n", action, err)
		return 2
	}
	if strings.TrimSpace(*computerID) == "" || strings.TrimSpace(*idempotencyKey) == "" || len(fs.Args()) != 0 {
		fmt.Fprintf(stderr, "choir computer %s: --computer and --idempotency-key are required\n", action)
		return 2
	}
	var response json.RawMessage
	path := "/api/computers/" + url.PathEscape(strings.TrimSpace(*computerID)) + "/lifecycle/" + action
	if err := c.do(http.MethodPost, path, map[string]string{"idempotency_key": strings.TrimSpace(*idempotencyKey)}, &response); err != nil {
		fmt.Fprintf(stderr, "choir computer %s: %v\n", action, err)
		return 1
	}
	return writeJSON(stdout, response)
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
	idempotencyKey := fs.String("idempotency-key", "", "Stable caller-supplied lifecycle command key for replay/conflict detection")
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
	if strings.TrimSpace(*idempotencyKey) == "" {
		fmt.Fprintln(stderr, "choir run start: --idempotency-key is required")
		return 2
	}
	text := strings.TrimSpace(strings.Join(rest, " "))
	var resp promptBarSubmitResponse
	if err := c.do(http.MethodPost, "/api/prompt-bar", map[string]string{"text": text, "command_id": strings.TrimSpace(*idempotencyKey)}, &resp); err != nil {
		fmt.Fprintf(stderr, "choir run start: %v\n", err)
		return 1
	}
	if resp.Schema != "choir.durable_work.v1" || resp.CommandID == "" || resp.StartRequestDigest == "" ||
		resp.TrajectoryID == "" || resp.DocID == "" || resp.RevisionID == "" ||
		resp.SubjectID == "" || len(resp.ObligationIDs) == 0 || resp.ReducerSeq <= 0 || resp.SnapshotCursor <= 0 {
		fmt.Fprintln(stderr, "choir run start: incomplete durable-work response")
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
	Schema             string   `json:"schema"`
	CommandID          string   `json:"command_id"`
	StartRequestDigest string   `json:"start_request_digest"`
	TrajectoryID       string   `json:"trajectory_id"`
	DocID              string   `json:"doc_id"`
	RevisionID         string   `json:"revision_id"`
	SubjectID          string   `json:"subject_id"`
	ObligationIDs      []string `json:"obligation_ids"`
	ReducerSeq         int64    `json:"reducer_seq"`
	SnapshotCursor     int64    `json:"snapshot_cursor"`
	SubmissionID       string   `json:"submission_id"`
	State              string   `json:"state"`
	CreatedAt          string   `json:"created_at"`
	StatusURL          string   `json:"status_url"`
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
	computerFlag := fs.String("computer", os.Getenv("CHOIR_COMPUTER_ID"), "Optional stable ComputerID to attenuate the key to one computer; omit for an owner-wide key that controls every computer you own")
	expiresFlag := fs.String("expires-at", "", "Optional RFC 3339 expiry (for example 2026-08-11T12:00:00Z); must be in the future")
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
		"label":       strings.TrimSpace(*labelFlag),
		"scopes":      scopes,
		"computer_id": strings.TrimSpace(*computerFlag),
	}
	if strings.TrimSpace(*expiresFlag) != "" {
		exp, err := time.Parse(time.RFC3339, strings.TrimSpace(*expiresFlag))
		if err != nil {
			fmt.Fprintf(stderr, "choir api-key create: --expires-at must be RFC 3339: %v\n", err)
			return 2
		}
		body["expires_at"] = exp.UTC().Format(time.RFC3339)
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
