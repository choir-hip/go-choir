package proxy

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/dbx/dbxtest"

	"github.com/yusefmosiah/go-choir/internal/decisionpolicy"
	"github.com/yusefmosiah/go-choir/internal/owneract"
	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// S1: the proxy signs an owner-act receipt over every owner mode write it
// forwards, and corpusd (enforce) accepts the owner's write only with it
// (docs/problems/effect-accepted-admitted-without-authority-2026-10-15.md).

func testOwnerActSigner(t *testing.T) *owneract.Signer {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := owneract.NewSigner(private)
	if err != nil {
		t.Fatal(err)
	}
	return signer
}

// openTestCorpusdModes runs corpusd's real mode route on an embedded Dolt
// store with the given owner-act policy.
func openTestCorpusdModes(t *testing.T, authority *platform.OwnerActAuthority) *httptest.Server {
	t.Helper()
	root := t.TempDir()
	control, err := dbx.OpenPostgres(dbxtest.PostgresDSN(t), dbx.PGOptions{})
	if err != nil {
		t.Fatal(err)
	}
	graph, err := dbx.OpenPostgres(dbxtest.PostgresDSN(t), dbx.PGOptions{})
	if err != nil {
		t.Fatal(err)
	}
	store := platform.NewStore(control, graph)
	if err := store.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	service := platform.NewService(store, filepath.Join(root, "artifacts"), filepath.Join(root, "platform-signing.key"))
	modes, err := service.SelfDevelopmentModeRuntime()
	if err != nil {
		t.Fatal(err)
	}
	modes.ConfigureOwnerActs(authority)
	handler := platform.NewHandler(service)
	if err := handler.ConfigureSelfDevelopmentModes(modes); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(handler.HandleSelfDevelopmentMode))
	t.Cleanup(server.Close)
	return server
}

func armBody(t *testing.T, generation uint64, key string) (string, platform.SetSelfDevelopmentModeRequest) {
	t.Helper()
	digest := strings.Repeat("a", 64)
	pending := ""
	request := platform.SetSelfDevelopmentModeRequest{
		Mode: platform.SelfDevelopmentModeAcceptOnce, ExpectedGeneration: generation, IdempotencyKey: key,
		OperationID: "op-1", BundleDigest: digest,
		ExpectedDesiredEventHead: digest, ExpectedEffectiveEventHead: digest, ExpectedPendingTransitionRef: &pending,
		ExpectedDesiredStateCommitment: digest, ExpectedEffectiveStateCommitment: digest,
		ExpiresAt: time.Now().UTC().Add(5 * time.Minute).Truncate(time.Microsecond).Format(time.RFC3339Nano),
	}
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return string(body), request
}

func TestOwnerSessionModeWriteCarriesAValidOwnerActReceipt(t *testing.T) {
	signer := testOwnerActSigner(t)
	verifier, err := owneract.NewVerifier(signer.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	var seen []string
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Header.Get(owneract.Header))
		_ = json.NewEncoder(w).Encode(map[string]any{"computer_id": "computer-a", "mode": "accept_once", "generation": 1})
	}))
	defer corpusd.Close()
	env := newOwnerActEnv(t)
	env.handler.cfg.CorpusdURL = corpusd.URL
	env.handler.ownerActSigner = signer
	body, request := armBody(t, 0, "arm-session")

	// The owner's session.
	if rec := env.send(t, http.MethodPut, "/api/computers/computer-a/self-development/mode", body, ""); rec.Code != http.StatusOK {
		t.Fatalf("owner session arm: %d %s", rec.Code, rec.Body.String())
	}
	// An owner:acts key.
	keySecret := env.key(t, "computer:self_development:mode", OwnerActsScope)
	if rec := env.send(t, http.MethodPut, "/api/computers/computer-a/self-development/mode", body, keySecret); rec.Code != http.StatusOK {
		t.Fatalf("owner:acts key arm: %d %s", rec.Code, rec.Body.String())
	}
	if len(seen) != 2 {
		t.Fatalf("corpusd saw %d writes", len(seen))
	}
	commitment, err := platform.SelfDevelopmentModeRequestCommitment("computer-a", request)
	if err != nil {
		t.Fatal(err)
	}
	for i, wantMethod := range []string{owneract.AuthMethodSession, owneract.AuthMethodAPIKey} {
		receipt, err := owneract.Decode(seen[i])
		if err != nil {
			t.Fatalf("write %d carried no receipt: %v", i, err)
		}
		verified, err := verifier.Verify(receipt, "computer-a", owneract.ActSelfDevApprove, time.Now())
		if err != nil {
			t.Fatalf("write %d receipt does not verify: %v", i, err)
		}
		if verified.OwnerUserID != env.user.ID || verified.AuthMethod != wantMethod ||
			verified.SubjectString("request_commitment") != commitment || verified.SubjectString("operation_id") != "op-1" ||
			verified.SubjectString("bundle_digest") != request.BundleDigest || verified.SubjectString("mode") != "accept_once" {
			t.Fatalf("write %d receipt binds the wrong act: %+v", i, verified)
		}
		prefix := "session-jwt-sha256:"
		if wantMethod == owneract.AuthMethodAPIKey {
			prefix = "api-key:"
		}
		if !strings.HasPrefix(verified.CredentialRef, prefix) || strings.Contains(verified.CredentialRef, keySecret) {
			t.Fatalf("write %d credential_ref = %q", i, verified.CredentialRef)
		}
	}
	if seen[0] == seen[1] {
		t.Fatal("two writes carried the same receipt")
	}
}

// Through corpusd's real mode route in enforce: the proxy with its key arms
// for the owner; without it, the same owner is refused, and so is a
// loopback caller that skips the proxy.
func TestOwnerActEndToEndThroughCorpusdEnforce(t *testing.T) {
	signer := testOwnerActSigner(t)
	verifier, err := owneract.NewVerifier(signer.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	corpusd := openTestCorpusdModes(t, platform.NewOwnerActAuthority(platform.OwnerActEnforce, verifier))
	env := newOwnerActEnv(t)
	env.handler.cfg.CorpusdURL = corpusd.URL
	body, _ := armBody(t, 0, "arm-e2e")

	// No key loaded: corpusd refuses the owner's write (enforce).
	if rec := env.send(t, http.MethodPut, "/api/computers/computer-a/self-development/mode", body, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("keyless proxy arm under enforce: %d %s", rec.Code, rec.Body.String())
	}
	// A loopback caller with the host-internal marker and no receipt.
	direct, err := http.NewRequest(http.MethodPost, corpusd.URL+"/internal/computers/self-development/mode?computer_id=computer-a", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	direct.Header.Set("X-Internal-Caller", "true")
	direct.Header.Set("X-Authenticated-User", env.user.ID)
	response, err := http.DefaultClient.Do(direct)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("loopback mode write without a receipt: %d", response.StatusCode)
	}

	// The tracer's path: an owner:acts key through the proxy with its key.
	env.handler.ownerActSigner = signer
	rec := env.send(t, http.MethodPut, "/api/computers/computer-a/self-development/mode", body, env.key(t, "computer:self_development:mode", OwnerActsScope))
	if rec.Code != http.StatusOK {
		t.Fatalf("owner:acts key arm through the proxy: %d %s", rec.Code, rec.Body.String())
	}
	var mode platform.SelfDevelopmentMode
	if err := json.Unmarshal(rec.Body.Bytes(), &mode); err != nil {
		t.Fatal(err)
	}
	if mode.Mode != platform.SelfDevelopmentModeAcceptOnce || mode.Receipt == nil ||
		mode.Receipt.KindFields["owner_act_receipt_digest"] == nil || mode.Receipt.KindFields["owner_user_id"] != env.user.ID {
		t.Fatalf("armed mode does not name the owner act: %+v", mode)
	}
	// The owner's session disarms, also signed.
	off := `{"mode":"off","expected_generation":1,"idempotency_key":"off-e2e"}`
	if rec := env.send(t, http.MethodPut, "/api/computers/computer-a/self-development/mode", off, ""); rec.Code != http.StatusOK {
		t.Fatalf("owner session off through the proxy: %d %s", rec.Code, rec.Body.String())
	}
}

// P4 failure mode 15: the proxy arms standing_consensus only on a computer
// vmctl classes as test, and signs that tier into the owner act; through
// corpusd's real mode route the test-tier arm lands with the tier recorded.
func TestStandingArmThroughTheProxyIsTestTierOnly(t *testing.T) {
	signer := testOwnerActSigner(t)
	verifier, err := owneract.NewVerifier(signer.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	corpusd := openTestCorpusdModes(t, platform.NewOwnerActAuthority(platform.OwnerActEnforce, verifier))
	env := newOwnerActEnv(t)
	env.handler.cfg.CorpusdURL = corpusd.URL
	env.handler.ownerActSigner = signer
	// The class is read by the fake vmctl on the request path and by the
	// API-key activity touch's own goroutine, so it is guarded.
	var classMu sync.Mutex
	class := "primary"
	setClass := func(c string) { classMu.Lock(); class = c; classMu.Unlock() }
	ownership := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		classMu.Lock()
		current := class
		classMu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"computer_id": "computer-a", "desktop_id": "primary", "user_id": env.user.ID,
			"state": "active", "computer_url": "http://127.0.0.1:1", "vm_id": "vm-a", "warmness_class": current,
		})
	}))
	defer ownership.Close()
	env.handler.vmctlClient = vmctl.NewClient(ownership.URL)
	expires := time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond).Format(time.RFC3339Nano)
	body := fmt.Sprintf(`{"mode":"standing_consensus","expected_generation":0,"idempotency_key":"standing-proxy","policy_digest":%q,"expires_at":%q}`,
		decisionpolicy.PolicyDigestReversibleSelfDevV1, expires)
	key := env.key(t, "computer:self_development:mode", OwnerActsScope)

	for _, refusedClass := range []string{"primary", "premium_always_on", "critical_protected", ""} {
		setClass(refusedClass)
		if rec := env.send(t, http.MethodPut, "/api/computers/computer-a/self-development/mode", body, key); rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "test-tier") {
			t.Fatalf("class %q: %d %s", refusedClass, rec.Code, rec.Body.String())
		}
	}
	setClass("test")
	rec := env.send(t, http.MethodPut, "/api/computers/computer-a/self-development/mode", body, key)
	if rec.Code != http.StatusOK {
		t.Fatalf("test-tier arm: %d %s", rec.Code, rec.Body.String())
	}
	var mode platform.SelfDevelopmentMode
	if err := json.Unmarshal(rec.Body.Bytes(), &mode); err != nil {
		t.Fatal(err)
	}
	if mode.Mode != platform.SelfDevelopmentModeStandingConsensus || mode.Receipt.KindFields["computer_tier"] != platform.ComputerTierTest ||
		mode.Receipt.KindFields["owner_act_receipt_digest"] == nil {
		t.Fatalf("standing arm: %+v", mode)
	}
}
