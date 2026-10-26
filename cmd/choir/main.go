// Command choir is the headless control surface for Choir. It wraps the
// /api/ HTTP surface with API key (Bearer choir_sk_...) auth so agents and
// scripts can read Texture documents, observe trajectories, search,
// and start runs without a browser.
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

// run dispatches through the command registry (registry.go, commands.go),
// which generates the CLI and the MCP tools from one declaration.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 2
	}
	switch args[0] {
	case "-h", "--help":
		printUsage(stdout)
		return 0
	}
	return dispatch(args, stdout, stderr)
}

// client holds shared CLI state.
type client struct {
	host       string
	apiKey     string
	computerID string
	http       *http.Client
	stdout     io.Writer
	stderr     io.Writer
}

// newClient defines the shared client flags on flags, parses args and
// builds the client. Verbs with their own --computer flag (computer-scoped
// routes) register it before newClient; the client shares it.
func newClient(flags *flag.FlagSet, args []string, stdout, stderr io.Writer) (*client, error) {
	build := clientFlags(flags)
	if err := flags.Parse(args); err != nil {
		return nil, err
	}
	c, err := build()
	if err != nil {
		return nil, err
	}
	c.stdout, c.stderr = stdout, stderr
	return c, nil
}

func newHTTPClient(timeout time.Duration) *http.Client { return &http.Client{Timeout: timeout} }

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
	if c.computerID != "" {
		req.Header.Set("X-Choir-Computer", c.computerID)
	}
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

// doRawBytes sends raw bytes and returns raw response bytes for
// non-JSON surfaces such as the files API.
func (c *client) doRawBytes(method, path string, body []byte) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, c.host+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if c.computerID != "" {
		req.Header.Set("X-Choir-Computer", c.computerID)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/octet-stream")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &apiErrorResp{Status: resp.StatusCode, Body: string(respBody)}
	}
	return respBody, nil
}

// doRaw sends raw bytes and decodes a JSON response.
func (c *client) doRaw(method, path string, body []byte, out any) error {
	respBody, err := c.doRawBytes(method, path, body)
	if err != nil {
		return err
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
			Commit             string `json:"commit"`
			Status             string `json:"status"`
			CarriedForwardFrom string `json:"carried_forward_from"`
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
	artifact, selected := deployment.Artifacts["proxy"]
	// A merged receipt carries forward the proxy artifact of the deploy that
	// last moved it; off the target, this deploy did not select the proxy.
	if selected && artifact.CarriedForwardFrom != "" && artifact.Commit != target {
		selected = false
	}
	if selected {
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

// verifyExecutionIdentity fetches the nonce-bound execution identity and
// verifies the guest receipt, the platform join attestation and the common
// commit; any refusal is an error.
func verifyExecutionIdentity(x *call) (any, error) {
	c := x.c
	expectedPlatformSignerKeyDigest, trustErr := executionIdentityPlatformTrustDigest()
	if trustErr != nil || !strings.HasPrefix(expectedPlatformSignerKeyDigest, "sha256:") {
		return nil, errors.New("platform trust configuration unavailable")
	}
	nonceBytes := make([]byte, 24)
	if _, err := rand.Read(nonceBytes); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}
	nonce := base64.RawURLEncoding.EncodeToString(nonceBytes)
	var envelope executionIdentityCLIEnvelope
	if err := c.do(http.MethodGet, "/api/acceptance/execution-identity?nonce="+url.QueryEscape(nonce), nil, &envelope); err != nil {
		return nil, err
	}
	if !envelope.Joined || envelope.Guest == nil || envelope.PlatformAttestation == nil {
		return nil, errors.New("platform identity join refused")
	}
	signed := envelope.Guest
	if signed.Schema != "choir.execution_identity.v1" || signed.Identity["schema"] != signed.Schema || signed.Identity["nonce"] != nonce || signed.Identity["audience"] != executionIdentityCLIAudience {
		return nil, errors.New("schema or nonce binding refused")
	}
	publicKey, err := base64.RawStdEncoding.DecodeString(signed.SignerPublicKey)
	if err != nil || len(publicKey) != ed25519.PublicKeySize || len(signed.Receipt.RequiredSigners) != 1 {
		return nil, errors.New("invalid signer public key")
	}
	actualSignerKeyDigest := "sha256:" + computerevent.DigestBytes(publicKey)
	if signed.Receipt.ReceiptKind != "ExecutionIdentity" {
		return nil, errors.New("unexpected receipt kind")
	}
	ref := signed.Receipt.RequiredSigners[0]
	if ref.SignerDomain != "guest-core" || signed.Receipt.Verify(executionIdentityCLIResolver{ref: ref, key: ed25519.PublicKey(publicKey)}) != nil {
		return nil, errors.New("signature verification failed")
	}
	if !sameJSONValue(signed.Receipt.IssuedAt, signed.Identity["issued_at"]) {
		return nil, errors.New("signed field issued_at mismatch")
	}
	for key, value := range signed.Identity {
		if key == "issued_at" {
			continue
		}
		if !sameJSONValue(signed.Receipt.KindFields[key], value) {
			return nil, fmt.Errorf("signed field %s mismatch", key)
		}
	}
	expiresAt, expiresErr := time.Parse(time.RFC3339Nano, fmt.Sprint(signed.Identity["expires_at"]))
	issuedAt, issuedErr := time.Parse(time.RFC3339Nano, fmt.Sprint(signed.Identity["issued_at"]))
	now := time.Now().UTC()
	if expiresErr != nil || issuedErr != nil || !expiresAt.After(now) || issuedAt.After(now.Add(30*time.Second)) || expiresAt.Sub(issuedAt) > 2*time.Minute {
		return nil, errors.New("expired or invalid validity window")
	}
	build, ok := signed.Identity["build"].(map[string]any)
	if !ok || strings.TrimSpace(fmt.Sprint(build["commit"])) == "" || build["commit"] != build["deployed_commit"] {
		return nil, errors.New("build/deploy identity conflict")
	}
	targetCommit, commonCommit := executionIdentityCLICommonCommit(signed, envelope.HostBuild, envelope.DeploymentReceipt)
	if !commonCommit {
		return nil, errors.New("host, guest, route, and deployment commit join refused")
	}
	if envelope.PlatformAttestation != nil {
		platformKey, decodeErr := base64.RawStdEncoding.DecodeString(envelope.PlatformAttestation.SignerPublicKey)
		if decodeErr != nil || len(platformKey) != ed25519.PublicKeySize ||
			!strings.EqualFold(expectedPlatformSignerKeyDigest, "sha256:"+computerevent.DigestBytes(platformKey)) {
			return nil, errors.New("platform signer key does not match the repository trust manifest")
		}
		guestDigest, guestDigestErr := executionIdentityCLIDigest(signed.Receipt)
		routeDigest := strings.TrimSpace(envelope.RouteDigest)
		hostBuildDigest, hostBuildDigestErr := executionIdentityCLIDigest(envelope.HostBuild)
		deploymentDigest, deploymentDigestErr := executionIdentityCLIDigest(envelope.DeploymentReceipt)
		if guestDigestErr != nil || !strings.HasPrefix(routeDigest, "sha256:") || hostBuildDigestErr != nil || deploymentDigestErr != nil {
			return nil, errors.New("platform join digest unavailable")
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
			return nil, errors.New("platform identity join verification failed")
		}
	}
	return envelope, nil
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

// runTextureWatch streams one document's durable version/control events as
// JSONL until a terminal event (CLI only: an MCP call returns once).
func runTextureWatch(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir texture watch", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var after int64
	var limit, reconnectAttempts int
	var once bool
	var pollInterval time.Duration
	fs.Int64Var(&after, "after", 0, "resume after this durable cursor")
	fs.IntVar(&limit, "limit", 100, "events per durable page")
	fs.BoolVar(&once, "once", false, "fetch one durable page and exit")
	fs.DurationVar(&pollInterval, "poll-interval", time.Second, "delay between caught-up durable pages")
	fs.IntVar(&reconnectAttempts, "reconnect-attempts", 3, "consecutive request failures tolerated before exit")
	c, err := newClient(fs, args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir texture: %v\n", err)
		return 2
	}
	rest := fs.Args()
	if len(rest) == 0 || strings.TrimSpace(rest[0]) == "" {
		fmt.Fprintln(stderr, "choir texture watch: document id required")
		return 2
	}
	docID := strings.TrimSpace(rest[0])
	escapedDocID := url.PathEscape(docID)
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
}

// ---- computer ----

// extractCheckpointArtifact accepts the restore operand in any of the product
// envelopes: a Checkpoint, a CheckpointResponse (what choir computer
// checkpoints --digest prints), or an old checkpoint bind report with
// published_checkpoint.checkpoint. restore/rematerialize consume the nested
// Checkpoint without jq.
func extractCheckpointArtifact(raw []byte) (json.RawMessage, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, err
	}
	if published, ok := top["published_checkpoint"]; ok {
		var response struct {
			Checkpoint json.RawMessage `json:"checkpoint"`
		}
		if json.Unmarshal(published, &response) != nil || len(response.Checkpoint) == 0 {
			return nil, fmt.Errorf("bind report did not publish a checkpoint")
		}
		return response.Checkpoint, nil
	}
	if checkpoint, ok := top["checkpoint"]; ok {
		if _, hasDigest := top["checkpoint_digest"]; !hasDigest {
			if _, hasReceipt := top["receipt"]; hasReceipt {
				if len(checkpoint) == 0 {
					return nil, fmt.Errorf("checkpoint response is empty")
				}
				return checkpoint, nil
			}
		}
	}
	return json.RawMessage(raw), nil
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
