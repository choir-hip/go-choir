package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

func TestMain(m *testing.M) {
	_ = os.Setenv(apiKeyEnvVar, "choir_sk_test")
	os.Exit(m.Run())
}

func TestClientTimeoutPrecedence(t *testing.T) {
	tests := []struct {
		name string
		env  string
		args []string
		want time.Duration
	}{
		{name: "default", want: 75 * time.Second},
		{name: "environment", env: "90s", want: 90 * time.Second},
		{name: "flag overrides environment", env: "90s", args: []string{"--timeout=2m"}, want: 2 * time.Minute},
		{name: "valid flag ignores invalid environment", env: "eventually", args: []string{"--timeout=2m"}, want: 2 * time.Minute},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(timeoutEnvVar, tt.env)
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			c, err := newClient(fs, tt.args, io.Discard, io.Discard)
			if err != nil {
				t.Fatalf("newClient() error = %v", err)
			}
			if c.http.Timeout != tt.want {
				t.Fatalf("timeout = %s, want %s", c.http.Timeout, tt.want)
			}
		})
	}
}

func TestClientRejectsInvalidTimeout(t *testing.T) {
	tests := []struct {
		name    string
		env     string
		args    []string
		wantErr string
	}{
		{name: "invalid environment", env: "eventually", wantErr: "$CHOIR_TIMEOUT must be a valid duration"},
		{name: "zero environment", env: "0s", wantErr: "$CHOIR_TIMEOUT must be greater than zero"},
		{name: "negative flag", env: "90s", args: []string{"--timeout=-1s"}, wantErr: "--timeout must be greater than zero"},
		{name: "invalid flag overrides environment", env: "90s", args: []string{"--timeout=soon"}, wantErr: "--timeout must be a valid duration"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(timeoutEnvVar, tt.env)
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			_, err := newClient(fs, tt.args, io.Discard, io.Discard)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("newClient() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestRequestTimeoutCancelsDelayedServer(t *testing.T) {
	t.Setenv(timeoutEnvVar, "")
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer stub.Close()

	started := time.Now()
	var out, errOut bytes.Buffer
	code := run([]string{"wire", "stories", "--host=" + stub.URL, "--timeout=30ms"}, &out, &errOut)
	elapsed := time.Since(started)
	if code != 1 {
		t.Fatalf("code = %d, want 1; stderr=%s", code, errOut.String())
	}
	if !strings.Contains(errOut.String(), "context deadline exceeded") {
		t.Fatalf("stderr = %q, want context deadline exceeded", errOut.String())
	}
	if elapsed > time.Second {
		t.Fatalf("request elapsed = %s, want cancellation within 1s", elapsed)
	}
}

// TestRunRequiresAPIKey asserts the CLI fails fast with a clear error when
// no API key is supplied, without making a network request.
func TestRunRequiresAPIKey(t *testing.T) {
	t.Setenv(apiKeyEnvVar, "") // isolate from a developer's real key
	var out, errOut bytes.Buffer
	code := run([]string{"wire", "stories"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("code = %d, want 2; stderr=%s", code, errOut.String())
	}
	if !strings.Contains(errOut.String(), "api key required") {
		t.Fatalf("stderr = %q, want it to mention api key required", errOut.String())
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty on auth failure", out.String())
	}
}

// TestRunRejectsBadKeyPrefix asserts the CLI rejects a key without the
// choir_sk_ prefix before any network call.
func TestRunRejectsBadKeyPrefix(t *testing.T) {
	t.Setenv(apiKeyEnvVar, "not-a-choir-key")
	var out, errOut bytes.Buffer
	code := run([]string{"wire", "stories"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("code = %d, want 2; stderr=%s", code, errOut.String())
	}
	if !strings.Contains(errOut.String(), "must start with") {
		t.Fatalf("stderr = %q, want it to mention prefix requirement", errOut.String())
	}
}

func TestCLIAPIKeyFileRequiresMode0600AndRetiredPlaintextFlagRefuses(t *testing.T) {
	keyPath := filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(keyPath, []byte("choir_sk_from_file\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readCLISecretFile(keyPath, nil); err == nil {
		t.Fatal("group-readable API key file was accepted")
	}
	if err := os.Chmod(keyPath, 0o600); err != nil {
		t.Fatal(err)
	}
	key, err := readCLISecretFile(keyPath, nil)
	if err != nil || key != "choir_sk_from_file" {
		t.Fatalf("mode-0600 key = %q, %v", key, err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"wire", "stories", "--api-key=choir_sk_plaintext"}, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "flag provided but not defined") {
		t.Fatalf("retired plaintext flag code=%d stderr=%q", code, stderr.String())
	}
}

func TestCLIAPIKeyFileDashReadsStdin(t *testing.T) {
	key, err := readCLISecretFile("-", strings.NewReader("choir_sk_stdin\n"))
	if err != nil || key != "choir_sk_stdin" {
		t.Fatalf("stdin key = %q, %v", key, err)
	}
}

// TestWireStoriesHitsAPI starts a stub server that asserts the CLI sends the
// Bearer API key header, and returns a canned wire stories response that the
// CLI must decode and print as JSON.
func TestWireStoriesHitsAPI(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/universal-wire/stories" {
			t.Errorf("path = %q, want /api/universal-wire/stories", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer choir_sk_test" {
			t.Errorf("Authorization = %q, want Bearer choir_sk_test", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"stories":[{"id":"s1","headline":"Test headline","dek":"test dek","story_texture_doc_id":"doc-1","source_state":"fresh"}],"source":"universal-wire-edition-texture","diagnostics":{"texture_edition":"present"}}`)
	}))
	defer stub.Close()

	var out, errOut bytes.Buffer
	code := run([]string{"wire", "stories", "--host=" + stub.URL}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%s", code, errOut.String())
	}
	var resp wireStoriesResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("decode stdout: %v; stdout=%s", err, out.String())
	}
	if len(resp.Stories) != 1 || resp.Stories[0].Headline != "Test headline" {
		t.Fatalf("stories = %+v, want one story with headline Test headline", resp.Stories)
	}
	if resp.Source != "universal-wire-edition-texture" {
		t.Fatalf("source = %q, want universal-wire-edition-texture", resp.Source)
	}
}

// TestWireDiagnosticsPrintsOnlyDiagnostics asserts the diagnostics subcommand
// decodes and prints the diagnostics field.
func TestWireDiagnosticsPrintsOnlyDiagnostics(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"stories":[],"source":"universal-wire-edition-texture","diagnostics":{"texture_edition":"missing","reason":"alias not bootstrapped"}}`)
	}))
	defer stub.Close()

	var out, errOut bytes.Buffer
	code := run([]string{"wire", "diagnostics", "--host=" + stub.URL}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%s", code, errOut.String())
	}
	var diag map[string]string
	if err := json.Unmarshal(out.Bytes(), &diag); err != nil {
		t.Fatalf("decode diagnostics: %v; stdout=%s", err, out.String())
	}
	if diag["texture_edition"] != "missing" {
		t.Fatalf("texture_edition = %q, want missing", diag["texture_edition"])
	}
}

// TestTrajectoriesHitsAPI asserts the trajectories command hits the right
// path and decodes the list.
func TestTrajectoriesHitsAPI(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/trajectories" {
			t.Errorf("path = %q, want /api/trajectories", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"trajectories":[{"trajectory_id":"traj-1","kind":"ingestion"}]}`)
	}))
	defer stub.Close()

	var out, errOut bytes.Buffer
	code := run([]string{"trajectories", "--host=" + stub.URL}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%s", code, errOut.String())
	}
	var resp trajectoriesListResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v; stdout=%s", err, out.String())
	}
	if len(resp.Trajectories) != 1 || resp.Trajectories[0].TrajectoryID != "traj-1" {
		t.Fatalf("trajectories = %+v, want one traj-1", resp.Trajectories)
	}
}

func TestIdentityVerifiesJoinedGuestAndPlatformAttestations(t *testing.T) {
	guestPublic, guestPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	platformPublic, platformPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	previousTrustDigest := executionIdentityPlatformTrustDigest
	executionIdentityPlatformTrustDigest = func() (string, error) {
		return "sha256:" + computerevent.DigestBytes(platformPublic), nil
	}
	t.Cleanup(func() { executionIdentityPlatformTrustDigest = previousTrustDigest })
	guestSigner := computerevent.SigningKey{
		SignerRef: computerevent.SignerRef{SignerDomain: "guest-core", KeyID: "guest-test"}, PrivateKey: guestPrivate,
	}
	platformKeyID := computerevent.DigestBytes(platformPublic)[:16]
	platformSigner := computerevent.SigningKey{
		SignerRef: computerevent.SignerRef{SignerDomain: "platform-control", KeyID: platformKeyID}, PrivateKey: platformPrivate,
	}
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		issuedAt := time.Now().UTC()
		identity := map[string]any{
			"schema": "choir.execution_identity.v1", "nonce": r.URL.Query().Get("nonce"),
			"audience":    "choir.news/acceptance/execution-identity",
			"computer_id": "computer-test", "realization_id": "vm-test-epoch-1", "vm_epoch": "1",
			"build":     map[string]any{"commit": "1234567890abcdef1234567890abcdef12345678", "deployed_commit": "1234567890abcdef1234567890abcdef12345678"},
			"issued_at": issuedAt.Format(time.RFC3339Nano), "expires_at": issuedAt.Add(2 * time.Minute).Format(time.RFC3339Nano),
		}
		guestFields := make(map[string]any, len(identity)-1)
		for key, value := range identity {
			if key != "issued_at" {
				guestFields[key] = value
			}
		}
		guestReceipt, signErr := computerevent.NewSignedReceipt("ExecutionIdentity", "choir-autoputer", guestFields, []computerevent.SigningKey{guestSigner}, issuedAt)
		if signErr != nil {
			t.Fatal(signErr)
		}
		vmctlIdentity := map[string]any{"computer_id": "computer-test", "vm_id": "vm-test", "epoch": 1, "state": "active"}
		route := json.RawMessage(`{"code_commit":"1234567890abcdef1234567890abcdef12345678"}`)
		hostBuild := json.RawMessage(`{"service":"proxy","commit":"1234567890abcdef1234567890abcdef12345678","deployed_commit":"1234567890abcdef1234567890abcdef12345678"}`)
		deployment := json.RawMessage(`{"target_commit":"1234567890abcdef1234567890abcdef12345678","artifacts":{"proxy":{"commit":"1234567890abcdef1234567890abcdef12345678","status":"active"}},"host_identity":{"canonical_ref":"refs/heads/main@1234567890abcdef1234567890abcdef12345678","nixos_closure_digest":"sha256:nixos","services":{"proxy":{"role":"proxy","package_digest":"sha256:proxy","embedded_commit":"1234567890abcdef1234567890abcdef12345678"}}}}`)
		guestDigest, _ := executionIdentityCLIDigest(guestReceipt)
		routeDigest, _ := executionIdentityCLIDigest(route)
		hostDigest, _ := executionIdentityCLIDigest(hostBuild)
		deployDigest, _ := executionIdentityCLIDigest(deployment)
		guestSignerDigest := "sha256:" + computerevent.DigestBytes(guestPublic)
		platformFields := map[string]any{
			"schema": "choir.execution_identity.v1", "nonce": identity["nonce"],
			"audience": "choir.news/acceptance/execution-identity", "deployed_commit": "1234567890abcdef1234567890abcdef12345678",
			"computer_id": identity["computer_id"], "realization_id": identity["realization_id"], "vm_epoch": identity["vm_epoch"],
			"guest_receipt_digest": guestDigest, "guest_signer_key_digest": guestSignerDigest,
			"vmctl": vmctlIdentity, "route_digest": routeDigest,
			"host_build_digest": hostDigest, "deployment_receipt_digest": deployDigest,
		}
		platformReceipt, signErr := computerevent.NewSignedReceipt("ExecutionIdentityJoin", "corpusd", platformFields, []computerevent.SigningKey{platformSigner}, issuedAt)
		if signErr != nil {
			t.Fatal(signErr)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"schema": "choir.execution_identity.v1", "joined": true,
			"guest": map[string]any{"schema": "choir.execution_identity.v1", "identity": identity, "receipt": guestReceipt, "signer_public_key": base64.RawStdEncoding.EncodeToString(guestPublic)},
			"vmctl": vmctlIdentity, "route_digest": routeDigest, "host_build": hostBuild, "deployment_receipt": deployment,
			"platform_attestation": map[string]any{"receipt": platformReceipt, "signer_public_key": base64.RawStdEncoding.EncodeToString(platformPublic)},
		})
	}))
	defer stub.Close()

	var out, errOut bytes.Buffer
	code := run([]string{"identity", "--host=" + stub.URL}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), `"joined": true`) {
		t.Fatalf("stdout = %s, want joined identity", out.String())
	}
	platformSigner.SignerRef.KeyID = "untrusted-key-id"
	out.Reset()
	errOut.Reset()
	if code := run([]string{"identity", "--host=" + stub.URL}, &out, &errOut); code != 1 ||
		!strings.Contains(errOut.String(), "platform identity join verification failed") {
		t.Fatalf("code=%d stderr=%q, want platform signer key-id refusal", code, errOut.String())
	}
}

func TestIdentityRefusesUnjoinedGuestEnvelope(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"schema": "choir.execution_identity.v1", "joined": false,
		})
	}))
	defer stub.Close()
	var out, errOut bytes.Buffer
	code := run([]string{"identity", "--host=" + stub.URL}, &out, &errOut)
	if code != 1 || !strings.Contains(errOut.String(), "platform identity join refused") {
		t.Fatalf("code=%d stderr=%q, want joined-identity refusal", code, errOut.String())
	}
}

func TestExecutionIdentityCLICommonCommitAllowsBoundMixedGeneration(t *testing.T) {
	const targetCommit = "1234567890abcdef1234567890abcdef12345678"
	const hostCommit = "abcdef1234567890abcdef1234567890abcdef12"
	const guestCommit = "fedcba0987654321fedcba0987654321fedcba09"
	signed := &executionIdentityCLIEnvelope{Identity: map[string]any{
		"build": map[string]any{"commit": guestCommit, "deployed_commit": guestCommit},
	}}
	host := json.RawMessage(`{"service":"proxy","commit":"` + hostCommit + `"}`)
	deployment := json.RawMessage(`{
		"target_commit":"` + targetCommit + `",
		"artifacts":{"gateway":{"commit":"` + targetCommit + `","status":"active"}},
		"host_identity":{
			"canonical_ref":"refs/heads/main@` + targetCommit + `",
			"nixos_closure_digest":"sha256:nixos",
			"services":{
				"proxy":{"role":"proxy","package_digest":"sha256:proxy","embedded_commit":"` + hostCommit + `"},
				"auth":{"role":"auth","package_digest":"sha256:auth","embedded_commit":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
			}
		}
	}`)
	if got, ok := executionIdentityCLICommonCommit(signed, host, deployment); !ok || got != targetCommit {
		t.Fatalf("mixed-generation identity = (%q, %v), want target and accepted", got, ok)
	}

	staleMetadata := json.RawMessage(`{"service":"proxy","commit":"` + hostCommit + `","deployed_commit":"` + targetCommit + `"}`)
	if _, ok := executionIdentityCLICommonCommit(signed, staleMetadata, deployment); ok {
		t.Fatal("unselected proxy with deployment metadata was accepted")
	}

	selectedStalePair := json.RawMessage(`{
		"target_commit":"` + targetCommit + `",
		"artifacts":{"proxy":{"commit":"` + targetCommit + `","status":"active"}},
		"host_identity":{
			"canonical_ref":"refs/heads/main@` + targetCommit + `",
			"nixos_closure_digest":"sha256:nixos",
			"services":{"proxy":{"role":"proxy","package_digest":"sha256:proxy","embedded_commit":"` + hostCommit + `"}}
		}
	}`)
	if _, ok := executionIdentityCLICommonCommit(signed, staleMetadata, selectedStalePair); ok {
		t.Fatal("selected proxy with stale runtime/package pair was accepted")
	}
}

func TestTextureCreateStableJSON(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/texture/lifecycle-documents" {
			t.Errorf("request=%s %s", r.Method, r.URL.Path)
		}
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["title"] != "Durable note" || body["initial_content"] != "Initial body" || body["client_request_id"] != "create-one" {
			t.Errorf("body=%v", body)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"schema":"choir.texture_create.v1","doc_id":"doc-created","revision_id":"rev-zero","trajectory_id":"trajectory-created"}`)
	}))
	defer stub.Close()
	var out, errOut bytes.Buffer
	if code := run([]string{"texture", "create", "--host=" + stub.URL, "--title=Durable note", "--content=Initial body", "--request-id=create-one"}, &out, &errOut); code != 0 ||
		!strings.Contains(out.String(), `"schema": "choir.texture_create.v1"`) || !strings.Contains(out.String(), `"doc_id": "doc-created"`) {
		t.Fatalf("code/out/err=%d %s %s", code, out.String(), errOut.String())
	}
}

func TestTextureTellAndCorrectUseClientOccurrenceAndExactHead(t *testing.T) {
	posts := 0
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			if r.URL.Path != "/api/texture/documents/doc-owner" {
				t.Errorf("GET path=%s", r.URL.Path)
			}
			_, _ = io.WriteString(w, `{"doc_id":"doc-owner","current_revision_id":"rev-current"}`)
			return
		}
		posts++
		if r.Method != http.MethodPost || (r.URL.Path != "/api/texture/documents/doc-owner/tell" && r.URL.Path != "/api/texture/documents/doc-owner/correct") {
			t.Errorf("POST=%s %s", r.Method, r.URL.Path)
		}
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["expected_head_revision_id"] != "rev-current" || body["client_request_id"] == "" || body["content"] != "same prose" {
			t.Errorf("body=%v", body)
		}
		_, _ = io.WriteString(w, `{"schema":"choir.texture_owner_instruction.v1","instruction_id":"instruction-`+body["client_request_id"]+`","request_id":"runtime-`+body["client_request_id"]+`","status":"pending"}`)
	}))
	defer stub.Close()
	for _, args := range [][]string{
		{"texture", "tell", "--host=" + stub.URL, "--request-id=occurrence-one", "doc-owner", "same", "prose"},
		{"texture", "tell", "--host=" + stub.URL, "--request-id=occurrence-two", "doc-owner", "same", "prose"},
		{"texture", "correct", "--host=" + stub.URL, "--request-id=occurrence-three", "--expected-head=rev-current", "doc-owner", "same", "prose"},
	} {
		var out, errOut bytes.Buffer
		if code := run(args, &out, &errOut); code != 0 || !strings.Contains(out.String(), "choir.texture_owner_instruction.v1") {
			t.Fatalf("args=%v code=%d out=%s err=%s", args, code, out.String(), errOut.String())
		}
	}
	if posts != 3 {
		t.Fatalf("posts=%d", posts)
	}
}

func TestTextureShowCurrentAndExactHistoricalVersion(t *testing.T) {
	var revisionPath string
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/texture/documents/doc-one":
			_, _ = io.WriteString(w, `{"doc_id":"doc-one","current_revision_id":"rev-current"}`)
		case "/api/texture/revisions/rev-current", "/api/texture/revisions/rev-history", "/api/texture/revisions/rev-foreign":
			revisionPath = r.URL.Path
			revisionDocID := "doc-one"
			if strings.HasSuffix(r.URL.Path, "rev-foreign") {
				revisionDocID = "doc-other"
			}
			_, _ = io.WriteString(w, `{"revision_id":"`+strings.TrimPrefix(r.URL.Path, "/api/texture/revisions/")+`","doc_id":"`+revisionDocID+`","content":"body"}`)
		default:
			t.Errorf("unexpected request %s", r.URL.String())
			http.NotFound(w, r)
		}
	}))
	defer stub.Close()

	for name, args := range map[string][]string{
		"current":    {"texture", "show", "--host=" + stub.URL, "doc-one"},
		"historical": {"texture", "show", "--host=" + stub.URL, "--revision=rev-history", "doc-one"},
	} {
		t.Run(name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			if code := run(args, &out, &errOut); code != 0 {
				t.Fatalf("code=%d stderr=%s", code, errOut.String())
			}
			var response textureShowCLIResponse
			if err := json.Unmarshal(out.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if response.Schema != "choir.texture_show.v1" || len(response.Document) == 0 || len(response.Revision) == 0 {
				t.Fatalf("show response=%s", out.String())
			}
			if name == "current" && (!response.Current || revisionPath != "/api/texture/revisions/rev-current") {
				t.Fatalf("current response=%s path=%q", out.String(), revisionPath)
			}
			if name == "historical" && (response.Current || revisionPath != "/api/texture/revisions/rev-history") {
				t.Fatalf("historical response=%s path=%q", out.String(), revisionPath)
			}
		})
	}
	var out, errOut bytes.Buffer
	if code := run([]string{"texture", "show", "--host=" + stub.URL, "--revision=rev-foreign", "doc-one"}, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "not an exact version of this document") {
		t.Fatalf("cross-document show code=%d stdout=%s stderr=%s", code, out.String(), errOut.String())
	}
}

func TestTextureWatchJSONLResumeCursorAndDeterministicPage(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/texture/documents/doc-watch/events" || r.URL.Query().Get("after") != "7" || r.URL.Query().Get("limit") != "2" {
			t.Errorf("watch request=%s", r.URL.String())
		}
		_, _ = io.WriteString(w, `{"schema":"choir.texture_observation.v1","events":[{"schema":"choir.texture_observation.v1","cursor":8,"kind":"update_applied","work_state":"working"},{"schema":"choir.texture_observation.v1","cursor":9,"kind":"trajectory_settled","work_state":"terminal"}],"next_cursor":9,"watermark":9}`)
	}))
	defer stub.Close()

	var out, errOut bytes.Buffer
	code := run([]string{"texture", "watch", "--host=" + stub.URL, "--after=7", "--limit=2", "--once", "doc-watch"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, errOut.String())
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], `"cursor":8`) || !strings.Contains(lines[1], `"cursor":9`) {
		t.Fatalf("watch JSONL=%q", out.String())
	}
}

func TestTextureWatchReconnectsFromLastDurableCursorWithoutSleep(t *testing.T) {
	requests := 0
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		switch requests {
		case 1:
			if r.URL.Query().Get("after") != "11" {
				t.Errorf("initial cursor=%q, want 11", r.URL.Query().Get("after"))
			}
			_, _ = io.WriteString(w, `{"schema":"choir.texture_observation.v1","events":[{"cursor":12,"work_state":"working"}],"next_cursor":12,"watermark":12}`)
		case 2:
			if r.URL.Query().Get("after") != "12" {
				t.Errorf("cursor after delivered event=%q, want 12", r.URL.Query().Get("after"))
			}
			http.Error(w, "temporary", http.StatusServiceUnavailable)
		case 3:
			if r.URL.Query().Get("after") != "12" {
				t.Errorf("reconnect cursor=%q, want acknowledged 12", r.URL.Query().Get("after"))
			}
			_, _ = io.WriteString(w, `{"schema":"choir.texture_observation.v1","events":[{"cursor":13,"work_state":"terminal"}],"next_cursor":13,"watermark":13}`)
		default:
			t.Errorf("unexpected extra request %d", requests)
		}
	}))
	defer stub.Close()
	var out, errOut bytes.Buffer
	code := run([]string{"texture", "watch", "--host=" + stub.URL, "--after=11", "--poll-interval=0", "--reconnect-attempts=1", "doc-watch"}, &out, &errOut)
	if code != 0 || requests != 3 || strings.Count(out.String(), `"cursor":12`) != 1 || strings.Count(out.String(), `"cursor":13`) != 1 {
		t.Fatalf("code=%d requests=%d stdout=%s stderr=%s", code, requests, out.String(), errOut.String())
	}
}

func TestTextureWatchReportsCursorExpiry(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = io.WriteString(w, `{"schema":"choir.texture_observation.v1","cursor_expired":true,"replay_required":true,"next_cursor":999,"watermark":12}`)
	}))
	defer stub.Close()
	var out, errOut bytes.Buffer
	code := run([]string{"texture", "watch", "--host=" + stub.URL, "--after=999", "--once", "--reconnect-attempts=0", "doc-watch"}, &out, &errOut)
	if code != 1 || !strings.Contains(errOut.String(), "cursor 999 expired; replay required at watermark 12") || out.Len() != 0 {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, out.String(), errOut.String())
	}
}

func TestTextureOpenSourceUsesExactPinnedIdentity(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/texture/documents/doc-source/source-open" || r.URL.Query().Get("revision_id") != "rev-2" ||
			r.URL.Query().Get("source_ref_id") != "obj:choir.source_ref:one" || r.URL.Query().Get("source_ref_version_id") != "ver-ref-2" {
			t.Errorf("source open request=%s", r.URL.String())
		}
		_, _ = io.WriteString(w, `{"schema":"choir.texture_source_open.v1","revision_id":"rev-2","source_identity":{"source_entity_hash":"sha256:source","selectors":[{"kind":"text_quote"}],"open_path":"/exact"}}`)
	}))
	defer stub.Close()
	var out, errOut bytes.Buffer
	code := run([]string{"texture", "open-source", "--host=" + stub.URL, "--revision=rev-2", "--source-ref=obj:choir.source_ref:one", "--source-ref-version=ver-ref-2", "doc-source"}, &out, &errOut)
	if code != 0 || !strings.Contains(out.String(), `"source_entity_hash": "sha256:source"`) {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, out.String(), errOut.String())
	}
}

func TestLifecycleEventsUsesDurableCursor(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/trajectories/trajectory-one/events" {
			t.Errorf("request = %s %s, want lifecycle event page", r.Method, r.URL.EscapedPath())
		}
		if r.URL.Query().Get("after") != "7" || r.URL.Query().Get("limit") != "25" {
			t.Errorf("query = %q", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"schema":"choir.durable_work.v1","events":[],"next_cursor":7,"watermark":9}`)
	}))
	defer stub.Close()

	var out, errOut bytes.Buffer
	code := run([]string{"lifecycle", "events", "--host=" + stub.URL, "--after=7", "--limit=25", "trajectory-one"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), `"watermark": 9`) {
		t.Fatalf("stdout = %s", out.String())
	}
}

// TestTrajectoriesDecodesObjectSettlementRule asserts the trajectories
// command handles settlement_rule as the JSON object the API actually
// returns (e.g. {"require_no_open_work_items":true}), not a string.
func TestTrajectoriesDecodesObjectSettlementRule(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"trajectories":[{"trajectory_id":"traj-1","kind":"document","status":"live","subject_refs":{"channel_id":"ch-1"},"settlement_rule":{"require_no_open_work_items":true}}]}`)
	}))
	defer stub.Close()

	var out, errOut bytes.Buffer
	code := run([]string{"trajectories", "--host=" + stub.URL}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%s", code, errOut.String())
	}
	var resp trajectoriesListResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v; stdout=%s", err, out.String())
	}
	if len(resp.Trajectories) != 1 || resp.Trajectories[0].Status != "live" {
		t.Fatalf("trajectories = %+v, want one live trajectory", resp.Trajectories)
	}
	if !strings.Contains(string(resp.Trajectories[0].SettlementRule), "require_no_open_work_items") {
		t.Fatalf("settlement_rule = %s, want the rule object passed through", resp.Trajectories[0].SettlementRule)
	}
}
func TestTrajectoryCancelPostsCommandBoundEscapedRequest(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if got := r.URL.EscapedPath(); got != "/api/trajectories/traj%2Fwith%20space/cancel" {
			t.Errorf("escaped path = %q, want /api/trajectories/traj%%2Fwith%%20space/cancel", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer choir_sk_test" {
			t.Errorf("Authorization = %q, want Bearer choir_sk_test", got)
		}
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if request["idempotency_key"] != "cancel-test-1" || request["reason"] != "operator request" ||
			request["expected_lifecycle_version"] != float64(7) || request["expected_head_revision_id"] != "revision-7" {
			t.Errorf("request = %#v, want command key, CAS preconditions, and reason", request)
		}
		_, _ = io.WriteString(w, `{"trajectory_id":"traj/with space","status":"cancelled","cancelled_run_ids":[]}`)
	}))
	defer stub.Close()

	var out, errOut bytes.Buffer
	code := run([]string{"trajectory", "cancel", "--host=" + stub.URL, "--idempotency-key=cancel-test-1", "--expected-lifecycle-version=7", "--expected-head-revision-id=revision-7", "--reason=operator request", "traj/with space"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%s", code, errOut.String())
	}
}

func TestTrajectoryCancelPrintsServerJSON(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"trajectory_id":"traj-1","status":"cancelled","cancelled_run_ids":["run-1","run-2"]}`)
	}))
	defer stub.Close()

	var out, errOut bytes.Buffer
	code := run([]string{"trajectory", "cancel", "--host=" + stub.URL, "--idempotency-key=cancel-test-2", "--expected-lifecycle-version=3", "--expected-head-revision-id=revision-3", "traj-1"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%s", code, errOut.String())
	}
	var resp struct {
		TrajectoryID    string   `json:"trajectory_id"`
		Status          string   `json:"status"`
		CancelledRunIDs []string `json:"cancelled_run_ids"`
	}
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("decode stdout: %v; stdout=%s", err, out.String())
	}
	if resp.TrajectoryID != "traj-1" || resp.Status != "cancelled" || !reflect.DeepEqual(resp.CancelledRunIDs, []string{"run-1", "run-2"}) {
		t.Fatalf("response = %+v, want cancelled traj-1 with run-1 and run-2", resp)
	}
}

func TestTrajectoryCancelRequiresID(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"trajectory", "cancel"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("code = %d, want 2; stderr=%s", code, errOut.String())
	}
	if !strings.Contains(errOut.String(), "choir trajectory cancel: trajectory id required") {
		t.Fatalf("stderr = %q, want trajectory id usage error", errOut.String())
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", out.String())
	}
}

func TestTrajectoryCancelReportsServerError(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "cannot cancel trajectory", http.StatusConflict)
	}))
	defer stub.Close()

	var out, errOut bytes.Buffer
	code := run([]string{"trajectory", "cancel", "--host=" + stub.URL, "--idempotency-key=cancel-test-3", "--expected-lifecycle-version=4", "--expected-head-revision-id=revision-4", "traj-1"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("code = %d, want 1; stderr=%s", code, errOut.String())
	}
	if !strings.Contains(errOut.String(), "choir trajectory cancel traj-1: http 409: cannot cancel trajectory") {
		t.Fatalf("stderr = %q, want server error", errOut.String())
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", out.String())
	}
}

func TestTrajectoryGetCompatibility(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/api/trajectories/traj-1" {
			t.Errorf("path = %q, want /api/trajectories/traj-1", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"trajectory_id":"traj-1","status":"live"}`)
	}))
	defer stub.Close()

	var out, errOut bytes.Buffer
	code := run([]string{"trajectory", "--host=" + stub.URL, "traj-1"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%s", code, errOut.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("decode stdout: %v; stdout=%s", err, out.String())
	}
	if resp["trajectory_id"] != "traj-1" || resp["status"] != "live" {
		t.Fatalf("response = %v, want live traj-1", resp)
	}
}

// TestTextureRevisionsHitsAPI asserts the texture revisions command GETs the
// revisions endpoint, which returns full content bodies.
func TestTextureRevisionsHitsAPI(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/texture/documents/doc-1/revisions" {
			t.Errorf("path = %q, want /api/texture/documents/doc-1/revisions", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"revisions":[{"revision_id":"rev-1","doc_id":"doc-1","content":"hello"}]}`)
	}))
	defer stub.Close()

	var out, errOut bytes.Buffer
	code := run([]string{"texture", "revisions", "--host=" + stub.URL, "doc-1"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), `"content": "hello"`) {
		t.Fatalf("stdout = %q, want revision content", out.String())
	}
}

func TestAPIKeyEnvironmentSecretNeverAppearsInHelp(t *testing.T) {
	const secret = "choir_sk_verifier_known_secret_must_not_render"
	t.Setenv(apiKeyEnvVar, secret)

	fs := flag.NewFlagSet("secret-redaction", flag.ContinueOnError)
	var flagHelp bytes.Buffer
	fs.SetOutput(&flagHelp)
	if _, err := newClient(fs, []string{"--help"}, io.Discard, io.Discard); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("newClient help error = %v, want flag.ErrHelp", err)
	}
	if strings.Contains(flagHelp.String(), secret) {
		t.Fatalf("shared client flag help leaked environment secret: %s", flagHelp.String())
	}
	if !strings.Contains(flagHelp.String(), "$CHOIR_API_KEY") {
		t.Fatalf("shared client flag help omitted safe environment hint: %s", flagHelp.String())
	}

	for _, args := range [][]string{
		{"wire", "stories", "--help"},
		{"texture", "read", "--help"},
		{"run", "list", "--help"},
		{"computer", "status", "--help"},
		{"api-key", "list", "--help"},
		{"api-key", "create", "--help"},
		{"api-key", "revoke", "--help"},
	} {
		var stdout, stderr bytes.Buffer
		run(args, &stdout, &stderr)
		if strings.Contains(stdout.String(), secret) || strings.Contains(stderr.String(), secret) {
			t.Fatalf("%v help leaked environment secret: stdout=%q stderr=%q", args, stdout.String(), stderr.String())
		}
	}
}

// TestAPIKeyFromEnv asserts the CLI reads the API key from CHOIR_API_KEY.
func TestAPIKeyFromEnv(t *testing.T) {
	t.Setenv(apiKeyEnvVar, "choir_sk_fromenv")
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer choir_sk_fromenv" {
			t.Errorf("Authorization = %q, want Bearer choir_sk_fromenv", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"stories":[],"source":"universal-wire-edition-texture"}`)
	}))
	defer stub.Close()

	var out, errOut bytes.Buffer
	code := run([]string{"wire", "stories", "--host=" + stub.URL}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%s", code, errOut.String())
	}
}

// TestNon2xxReturnsError asserts the CLI surfaces a non-2xx response as an
// error with the status code and body.
func TestNon2xxReturnsError(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":"authentication required"}`)
	}))
	defer stub.Close()

	var out, errOut bytes.Buffer
	code := run([]string{"wire", "stories", "--host=" + stub.URL}, &out, &errOut)
	if code != 1 {
		t.Fatalf("code = %d, want 1; stderr=%s", code, errOut.String())
	}
	if !strings.Contains(errOut.String(), "http 401") {
		t.Fatalf("stderr = %q, want it to mention http 401", errOut.String())
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty on error", out.String())
	}
}

// TestUnknownCommand asserts unknown commands print usage and exit 2.
func TestUnknownCommand(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"frobnicate"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if !strings.Contains(errOut.String(), "unknown command") {
		t.Fatalf("stderr = %q, want unknown command", errOut.String())
	}
}

// TestRunStartPostsToPromptBar asserts the run start command POSTs to
// /api/prompt-bar with the prompt text and decodes the submission response.
func TestRunStartPostsToPromptBar(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/prompt-bar" {
			t.Errorf("path = %q, want /api/prompt-bar", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["text"] != "hello world" {
			t.Fatalf("text = %q, want hello world", body["text"])
		}
		if body["command_id"] != "cli-start-1" {
			t.Fatalf("command_id = %q, want cli-start-1", body["command_id"])
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(w, `{"schema":"choir.durable_work.v1","submission_id":"sub-123","state":"pending","created_at":"2026-07-06T00:00:00.000Z","status_url":"/api/prompt-bar/submissions/sub-123","command_id":"command-123","start_request_digest":"digest-123","trajectory_id":"trajectory-123","doc_id":"document-123","revision_id":"revision-123","subject_id":"texture:document-123","obligation_ids":["work-123"],"reducer_seq":1,"snapshot_cursor":1}`)
	}))
	defer stub.Close()

	var out, errOut bytes.Buffer
	code := run([]string{"run", "start", "--host=" + stub.URL, "--idempotency-key=cli-start-1", "hello", "world"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%s", code, errOut.String())
	}
	var resp promptBarSubmitResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v; stdout=%s", err, out.String())
	}
	if resp.SubmissionID != "sub-123" {
		t.Fatalf("submission_id = %q, want sub-123", resp.SubmissionID)
	}
}

// TestRunStartRequiresText asserts the run start command fails when no prompt
// text is provided.
func TestRunStartRequiresText(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"run", "start"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("code = %d, want 2; stderr=%s", code, errOut.String())
	}
	if !strings.Contains(errOut.String(), "prompt text required") {
		t.Fatalf("stderr = %q, want prompt text required", errOut.String())
	}
}

func TestRunStartRequiresIdempotencyKey(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"run", "start", "hello"}, &out, &errOut)
	if code != 2 || !strings.Contains(errOut.String(), "--idempotency-key is required") {
		t.Fatalf("code=%d stderr=%q", code, errOut.String())
	}
}

// TestRunStatusHitsRunResource asserts the run status command GETs the
// canonical run resource.
func TestRunStatusHitsRunResource(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/runs/sub-123" {
			t.Errorf("path = %q, want /api/runs/sub-123", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"run_id":"sub-123","state":"completed","created_at":"2026-07-06T00:00:00.000Z","updated_at":"2026-07-06T00:01:00.000Z"}`)
	}))
	defer stub.Close()

	var out, errOut bytes.Buffer
	code := run([]string{"run", "status", "--host=" + stub.URL, "sub-123"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%s", code, errOut.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v; stdout=%s", err, out.String())
	}
	if resp["state"] != "completed" {
		t.Fatalf("state = %v, want completed", resp["state"])
	}
}

func TestRunListHitsRunsEndpoint(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/runs" {
			t.Errorf("path = %q, want /api/runs", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if got := r.URL.Query().Get("limit"); got != "7" {
			t.Errorf("limit = %q, want 7", got)
		}
		_, _ = io.WriteString(w, `{"runs":[{"run_id":"run-123","state":"running"}]}`)
	}))
	defer stub.Close()

	var out, errOut bytes.Buffer
	code := run([]string{"run", "list", "--host=" + stub.URL, "--limit=7"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%s", code, errOut.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v; stdout=%s", err, out.String())
	}
	if len(resp["runs"].([]any)) != 1 {
		t.Fatalf("runs = %v, want one run", resp["runs"])
	}
}

func TestRunCancelPostsRunResource(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/runs/run-123/cancel" {
			t.Errorf("path = %q, want /api/runs/run-123/cancel", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		_, _ = io.WriteString(w, `{"run_id":"run-123","state":"cancelled"}`)
	}))
	defer stub.Close()

	var out, errOut bytes.Buffer
	code := run([]string{"run", "cancel", "--host=" + stub.URL, "run-123"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%s", code, errOut.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v; stdout=%s", err, out.String())
	}
	if resp["state"] != "cancelled" {
		t.Fatalf("state = %v, want cancelled", resp["state"])
	}
}

func TestComputerLifecycleCommandsUseTargetedProductAPI(t *testing.T) {
	var requests []struct {
		method         string
		path           string
		idempotencyKey string
	}
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idempotencyKey := ""
		if r.Method == http.MethodPost {
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode lifecycle body: %v", err)
			}
			idempotencyKey = body["idempotency_key"]
		}
		requests = append(requests, struct {
			method         string
			path           string
			idempotencyKey string
		}{method: r.Method, path: r.URL.Path, idempotencyKey: idempotencyKey})
		_, _ = io.WriteString(w, `{"computer_id":"computer-1","state":"active"}`)
	}))
	defer stub.Close()

	for _, command := range []string{"status", "stop", "start", "restart", "refresh"} {
		args := []string{"computer", command, "--host=" + stub.URL, "--computer=computer-1"}
		if command != "status" {
			args = append(args, "--idempotency-key="+command+"-1")
		}
		var out, errOut bytes.Buffer
		if code := run(args, &out, &errOut); code != 0 {
			t.Fatalf("computer %s code = %d, stderr=%s", command, code, errOut.String())
		}
	}
	want := []struct {
		method         string
		path           string
		idempotencyKey string
	}{
		{method: http.MethodGet, path: "/api/computers/computer-1/lifecycle/status"},
		{method: http.MethodPost, path: "/api/computers/computer-1/lifecycle/stop", idempotencyKey: "stop-1"},
		{method: http.MethodPost, path: "/api/computers/computer-1/lifecycle/start", idempotencyKey: "start-1"},
		{method: http.MethodPost, path: "/api/computers/computer-1/lifecycle/restart", idempotencyKey: "restart-1"},
		{method: http.MethodPost, path: "/api/computers/computer-1/lifecycle/refresh", idempotencyKey: "refresh-1"},
	}
	if !reflect.DeepEqual(requests, want) {
		t.Fatalf("lifecycle requests = %+v, want %+v", requests, want)
	}
}

func TestComputerReplaceWorkspaceUsesProductPath(t *testing.T) {
	var method, path string
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		if r.Body != nil {
			defer r.Body.Close()
			body, _ := io.ReadAll(r.Body)
			if len(body) != 0 {
				t.Fatalf("replace-workspace sent body %s", body)
			}
		}
		_, _ = io.WriteString(w, `{"computer_id":"computer-1","appended_event":false,"published_checkpoint":false,"store_closed":true}`)
	}))
	defer stub.Close()

	var out, errOut bytes.Buffer
	if code := run([]string{"computer", "replace-workspace", "--host=" + stub.URL, "--computer=computer-1"}, &out, &errOut); code != 0 {
		t.Fatalf("computer replace-workspace code = %d, stderr=%s", code, errOut.String())
	}
	if method != http.MethodPost || path != "/api/computers/computer-1/lifecycle/replace-workspace" {
		t.Fatalf("replace-workspace request method=%s path=%s", method, path)
	}
	if strings.Contains(path, "self-development/genesis") {
		t.Fatal("replace-workspace used genesis")
	}
}

func TestComputerRematerializeFromTapeUsesProductPath(t *testing.T) {
	var method, path, body string
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		if r.Body != nil {
			defer r.Body.Close()
			raw, _ := io.ReadAll(r.Body)
			body = string(raw)
		}
		_, _ = io.WriteString(w, `{"computer_id":"computer-1","witness_matched":true,"original_denied":true,"store_closed":true}`)
	}))
	defer stub.Close()
	checkpointPath := filepath.Join(t.TempDir(), "checkpoint.json")
	if err := os.WriteFile(checkpointPath, []byte(`{"digest":"abc"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := run([]string{"computer", "rematerialize-from-tape", "--host=" + stub.URL, "--computer=computer-1", "--checkpoint-file=" + checkpointPath}, &out, &errOut); code != 0 {
		t.Fatalf("computer rematerialize-from-tape code = %d, stderr=%s", code, errOut.String())
	}
	if method != http.MethodPost || path != "/api/computers/computer-1/lifecycle/rematerialize-from-tape" {
		t.Fatalf("rematerialize request method=%s path=%s", method, path)
	}
	if !strings.Contains(body, `"checkpoint"`) {
		t.Fatalf("rematerialize omitted checkpoint body: %s", body)
	}
	if strings.Contains(path, "self-development/genesis") {
		t.Fatal("rematerialize used genesis")
	}
}

func TestComputerImportResidueSnapshotUsesProductPath(t *testing.T) {
	var method, path string
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		if r.Body != nil {
			defer r.Body.Close()
			_, _ = io.ReadAll(r.Body)
		}
		_, _ = io.WriteString(w, `{"computer_id":"computer-1","appended":false}`)
	}))
	defer stub.Close()
	var out, errOut bytes.Buffer
	if code := run([]string{"computer", "import-residue-snapshot", "--host=" + stub.URL, "--computer=computer-1"}, &out, &errOut); code != 0 {
		t.Fatalf("computer import-residue-snapshot code = %d, stderr=%s", code, errOut.String())
	}
	if method != http.MethodPost || path != "/api/computers/computer-1/lifecycle/import-residue-snapshot" {
		t.Fatalf("import-residue-snapshot request = %s %s", method, path)
	}
}

func TestComputerCheckpointUsesProductPath(t *testing.T) {
	var method, path string
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		if r.Body != nil {
			defer r.Body.Close()
			_, _ = io.ReadAll(r.Body)
		}
		_, _ = io.WriteString(w, `{"computer_id":"computer-1","checkpoint_eligible":false}`)
	}))
	defer stub.Close()
	var out, errOut bytes.Buffer
	if code := run([]string{"computer", "checkpoint", "--host=" + stub.URL, "--computer=computer-1"}, &out, &errOut); code != 0 {
		t.Fatalf("computer checkpoint code = %d, stderr=%s", code, errOut.String())
	}
	if method != http.MethodPost || path != "/api/computers/computer-1/lifecycle/checkpoint" {
		t.Fatalf("checkpoint request method=%s path=%s", method, path)
	}
	if strings.Contains(path, "self-development/genesis") {
		t.Fatal("checkpoint used genesis")
	}
}

func TestComputerRestoreUsesProductPath(t *testing.T) {
	var method, path, body string
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		if r.Body != nil {
			defer r.Body.Close()
			raw, _ := io.ReadAll(r.Body)
			body = string(raw)
		}
		_, _ = io.WriteString(w, `{"computer_id":"computer-1","witness_matched":true,"frontend_restaged":true}`)
	}))
	defer stub.Close()
	checkpointPath := filepath.Join(t.TempDir(), "checkpoint.json")
	if err := os.WriteFile(checkpointPath, []byte(`{"digest":"abc"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := run([]string{"computer", "restore", "--host=" + stub.URL, "--computer=computer-1", "--checkpoint-file=" + checkpointPath}, &out, &errOut); code != 0 {
		t.Fatalf("computer restore code = %d, stderr=%s", code, errOut.String())
	}
	if method != http.MethodPost || path != "/api/computers/computer-1/lifecycle/restore" {
		t.Fatalf("restore request method=%s path=%s", method, path)
	}
	if !strings.Contains(body, `"checkpoint"`) || !strings.Contains(body, `"vm_local"`) || !strings.Contains(body, `"computer_surface_frontend"`) {
		t.Fatalf("restore omitted operand: %s", body)
	}
	if strings.Contains(path, "self-development/genesis") {
		t.Fatal("restore used genesis")
	}
}

func TestComputerRestoreUnwrapsCheckpointBindReport(t *testing.T) {
	var body string
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			defer r.Body.Close()
			raw, _ := io.ReadAll(r.Body)
			body = string(raw)
		}
		_, _ = io.WriteString(w, `{"computer_id":"computer-1","witness_matched":true}`)
	}))
	defer stub.Close()
	checkpointPath := filepath.Join(t.TempDir(), "checkpoint.json")
	raw := `{"computer_id":"computer-1","checkpoint_eligible":true,"published_checkpoint":{"checkpoint":{"checkpoint_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","request":{"computer_id":"computer-1"}},"receipt":{"kind":"checkpoint"}}}`
	if err := os.WriteFile(checkpointPath, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := run([]string{"computer", "restore", "--host=" + stub.URL, "--computer=computer-1", "--checkpoint-file=" + checkpointPath}, &out, &errOut); code != 0 {
		t.Fatalf("restore unwrap code = %d, stderr=%s", code, errOut.String())
	}
	if strings.Contains(body, "checkpoint_eligible") || strings.Contains(body, "published_checkpoint") {
		t.Fatalf("restore sent bind report instead of checkpoint: %s", body)
	}
	if !strings.Contains(body, `"checkpoint_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"`) {
		t.Fatalf("restore lost nested checkpoint: %s", body)
	}
}

func TestComputerBootstrapChainUsesProductPath(t *testing.T) {
	var method, path string
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		if r.Body != nil {
			defer r.Body.Close()
			body, _ := io.ReadAll(r.Body)
			if len(body) != 0 {
				t.Fatalf("bootstrap-chain sent body %s", body)
			}
		}
		_, _ = io.WriteString(w, `{"computer_id":"computer-1","appended_event":true,"published_checkpoint":false}`)
	}))
	defer stub.Close()

	var out, errOut bytes.Buffer
	if code := run([]string{"computer", "bootstrap-chain", "--host=" + stub.URL, "--computer=computer-1"}, &out, &errOut); code != 0 {
		t.Fatalf("computer bootstrap-chain code = %d, stderr=%s", code, errOut.String())
	}
	if method != http.MethodPost || path != "/api/computers/computer-1/lifecycle/bootstrap-chain" {
		t.Fatalf("bootstrap-chain request method=%s path=%s", method, path)
	}
	if strings.Contains(path, "self-development/genesis") {
		t.Fatal("bootstrap-chain used genesis")
	}
}

// TestAPIKeyListHitsAuthEndpoint asserts the api-key list command GETs
// /auth/api-keys with the Bearer token.
func TestAPIKeyListHitsAuthEndpoint(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/api-keys" {
			t.Errorf("path = %q, want /auth/api-keys", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer choir_sk_test" {
			t.Errorf("Authorization = %q, want Bearer choir_sk_test", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"keys":[{"id":"ak_1","label":"CLI key","scopes":["read:texture"],"created_at":"2026-07-06T00:00:00Z"}]}`)
	}))
	defer stub.Close()

	var out, errOut bytes.Buffer
	code := run([]string{"api-key", "list", "--host=" + stub.URL}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%s", code, errOut.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v; stdout=%s", err, out.String())
	}
	keys, ok := resp["keys"].([]any)
	if !ok || len(keys) != 1 {
		t.Fatalf("keys = %v, want one key", resp["keys"])
	}
}

// TestAPIKeyCreatePostsToAuthEndpoint asserts the api-key create command POSTs
// to /auth/api-keys with label, scopes, and the required computer binding.
func TestAPIKeyCreatePostsToAuthEndpoint(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/api-keys" {
			t.Errorf("path = %q, want /auth/api-keys", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["label"] != "Devin CLI" {
			t.Fatalf("label = %v, want Devin CLI", body["label"])
		}
		scopes, ok := body["scopes"].([]any)
		if !ok || len(scopes) != 1 || scopes[0] != "read:texture" {
			t.Fatalf("scopes = %v, want [read:texture]", body["scopes"])
		}
		if body["computer_id"] != "computer-abc" {
			t.Fatalf("computer_id = %v, want computer-abc", body["computer_id"])
		}
		if body["expires_at"] != "2026-08-12T00:00:00Z" {
			t.Fatalf("expires_at = %v, want 2026-08-12T00:00:00Z", body["expires_at"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"ak_new","label":"Devin CLI","scopes":["read:texture"],"secret":"choir_sk_newkey"}`)
	}))
	defer stub.Close()

	var out, errOut bytes.Buffer
	code := run([]string{"api-key", "create", "--label=Devin CLI", "--scopes=read:texture", "--computer=computer-abc", "--expires-at=2026-08-12T00:00:00Z", "--host=" + stub.URL}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%s", code, errOut.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v; stdout=%s", err, out.String())
	}
	if resp["secret"] != "choir_sk_newkey" {
		t.Fatalf("secret = %v, want choir_sk_newkey", resp["secret"])
	}
}

// TestAPIKeyCreateOwnerWidePostsEmptyComputer asserts the CLI POSTs an
// owner-wide key (empty computer_id) when --computer is omitted, including
// for computer-selecting scopes.
func TestAPIKeyCreateOwnerWidePostsEmptyComputer(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/api-keys" {
			t.Fatalf("path = %q, want /auth/api-keys", r.URL.Path)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		scopes, ok := body["scopes"].([]any)
		if !ok || len(scopes) != 1 || scopes[0] != "read:texture" {
			t.Fatalf("scopes = %v, want [read:texture]", body["scopes"])
		}
		if body["computer_id"] != "" {
			t.Fatalf("computer_id = %v, want empty for owner-wide key", body["computer_id"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"ak_ow","label":"owner-wide","scopes":["read:texture"],"secret":"choir_sk_ow"}`)
	}))
	defer stub.Close()

	var out, errOut bytes.Buffer
	code := run([]string{"api-key", "create", "--label=owner-wide", "--scopes=read:texture", "--host=" + stub.URL}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%s", code, errOut.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v; stdout=%s", err, out.String())
	}
	if resp["secret"] != "choir_sk_ow" {
		t.Fatalf("secret = %v, want choir_sk_ow", resp["secret"])
	}

	// manage:keys-only keys also POST without a computer binding.
	out.Reset()
	errOut.Reset()
	manageStub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["computer_id"] != "" {
			t.Fatalf("computer_id = %v, want empty for manage:keys-only", body["computer_id"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"ak_mgmt","label":"k","scopes":["manage:keys"],"secret":"choir_sk_mgmt"}`)
	}))
	defer manageStub.Close()
	code = run([]string{"api-key", "create", "--label=k", "--scopes=manage:keys", "--host=" + manageStub.URL}, &out, &errOut)
	if code != 0 {
		t.Fatalf("manage-only code = %d, want 0; stderr=%s", code, errOut.String())
	}
}

// TestAPIKeyCreateRejectsMalformedExpiry asserts --expires-at must parse as RFC 3339.
func TestAPIKeyCreateRejectsMalformedExpiry(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
	}))
	defer stub.Close()

	var out, errOut bytes.Buffer
	code := run([]string{"api-key", "create", "--scopes=read:texture", "--computer=computer-abc", "--expires-at=not-a-date", "--host=" + stub.URL}, &out, &errOut)
	if code != 2 {
		t.Fatalf("code = %d, want 2; stderr=%s", code, errOut.String())
	}
	if !strings.Contains(errOut.String(), "RFC 3339") {
		t.Fatalf("stderr = %q, want RFC 3339 error", errOut.String())
	}
}

// TestAPIKeyRevokeDeletesKey asserts the api-key revoke command DELETEs the
// specified key.
func TestAPIKeyRevokeDeletesKey(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/api-keys/ak_123" {
			t.Errorf("path = %q, want /auth/api-keys/ak_123", r.URL.Path)
		}
		if r.Method != http.MethodDelete {
			t.Errorf("method = %q, want DELETE", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer stub.Close()

	var out, errOut bytes.Buffer
	code := run([]string{"api-key", "revoke", "--host=" + stub.URL, "ak_123"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "ak_123") {
		t.Fatalf("stdout = %q, want it to mention ak_123", out.String())
	}
}

func TestSelfDevelopmentModeCLIQualifiedConsensusCASBody(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/computers/computer-exact/self-development/mode" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["mode"] != "qualified_consensus" || body["idempotency_key"] != "mode-qc-1" || body["expected_generation"] != float64(6) {
			t.Fatalf("mode body = %#v", body)
		}
		if body["operation_id"] != "operation-exact" || body["bundle_digest"] != strings.Repeat("2", 64) {
			t.Fatalf("binding body = %#v", body)
		}
		if body["policy_digest"] != strings.Repeat("c", 64) || body["consensus_receipt_digest"] != strings.Repeat("d", 64) {
			t.Fatalf("consensus body = %#v", body)
		}
		if _, ok := body["expected_pending_transition_ref"]; !ok {
			t.Fatal("qualified_consensus body omitted expected_pending_transition_ref")
		}
		_, _ = io.WriteString(w, `{"computer_id":"computer-exact","mode":"qualified_consensus","generation":7}`)
	}))
	defer stub.Close()
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"self-dev", "mode", "set",
		"--computer=computer-exact",
		"--mode=qualified_consensus",
		"--expected-generation=6",
		"--idempotency-key=mode-qc-1",
		"--expires-at=2026-08-16T06:00:00.000000000Z",
		"--operation=operation-exact",
		"--bundle=" + strings.Repeat("2", 64),
		"--expected-desired-head=" + strings.Repeat("a", 64),
		"--expected-effective-head=" + strings.Repeat("b", 64),
		"--expected-pending-ref=",
		"--expected-desired-commitment=" + strings.Repeat("e", 64),
		"--expected-effective-commitment=" + strings.Repeat("f", 64),
		"--policy-digest=" + strings.Repeat("c", 64),
		"--consensus-receipt-digest=" + strings.Repeat("d", 64),
		"--host=" + stub.URL,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"generation": 7`) {
		t.Fatalf("stdout=%s", stdout.String())
	}
}

func TestSelfDevelopmentModeCLIUsesExplicitComputerAndCASBody(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/computers/computer-exact/self-development/mode" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["mode"] != "propose_only" || body["idempotency_key"] != "mode-1" || body["expected_generation"] != float64(4) {
			t.Fatalf("mode body = %#v", body)
		}
		_, _ = io.WriteString(w, `{"computer_id":"computer-exact","mode":"propose_only","generation":5}`)
	}))
	defer stub.Close()
	var stdout, stderr bytes.Buffer
	code := run([]string{"self-dev", "mode", "set", "--computer=computer-exact", "--mode=propose_only", "--expected-generation=4", "--idempotency-key=mode-1", "--host=" + stub.URL}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"generation": 5`) {
		t.Fatalf("stdout=%s", stdout.String())
	}
}

func TestLifecycleCapsuleEvidenceEscapesRouteAndPreservesJSON(t *testing.T) {
	body := `{"schema":"choir.co_super_capsule_evidence/v1","assignment":{"assignment_id":"assignment one","attempt":3},"reports":[],"candidates":[],"execution_attestations":[],"capsule_fate_history":[],"texture_source_refs":[],"snapshot_cursor":7,"watermark":7,"verifier_contract":{"schema":"choir.co_super_capsule_evidence_verifier/v1","version":"f1-incomplete","physical_isolation_claim":false,"effects_enabled":false},"evidence_complete":false,"deficits":["isolation_probe_missing","run_acceptance_gate_missing","texture_source_missing"]}`
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.EscapedPath() != "/api/trajectories/traj%25%20one/capsule-evidence/assignment%20one" || r.URL.RawQuery != "attempt=3" {
			t.Errorf("request=%s %s?%s", r.Method, r.URL.EscapedPath(), r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, body)
	}))
	defer stub.Close()
	var out, errOut bytes.Buffer
	code := run([]string{"lifecycle", "capsule-evidence", "--host=" + stub.URL, "--attempt=3", "traj% one", "assignment one"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, errOut.String())
	}
	var got, want any
	if json.Unmarshal(out.Bytes(), &got) != nil || json.Unmarshal([]byte(body), &want) != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("json parity got=%s", out.String())
	}
}

func TestLifecycleCapsuleEvidenceRequiresPositiveAttempt(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"lifecycle", "capsule-evidence", "trajectory", "assignment"}, &out, &errOut); code != 2 || !strings.Contains(errOut.String(), "positive --attempt") {
		t.Fatalf("code=%d stderr=%s", code, errOut.String())
	}
}
