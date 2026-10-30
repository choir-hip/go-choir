package selfdev

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/platform"
)

// S3 re-credential on resume, guest install
// (docs/archive/reports/s3-snapshot-hibernate-design-2026-10-14.md, items 29-33, 35).
//
// Ways this could fail, written first:
//   - install sends the frozen (expired) capability anywhere, or needs it, so a
//     guest hibernated past the renewal grace cannot resume;
//   - install accepts an envelope for another computer or realization, or one
//     signed by a different platform key than the guest's receipt resolver;
//   - a refused exchange leaves the guest half-installed (new nonce, old token)
//     instead of unchanged;
//   - install skips consumption, leaving the envelope re-exchangeable and no
//     key_revoked receipt to append;
//   - install runs over an unfinished exchange and drops its pending receipt;
//   - the recovery handoff keeps the frozen token after install;
//   - the installed capability cannot renew.

const resumeTestKeyID = "platform-test"

type fakeResumePlatform struct {
	t          *testing.T
	privateKey ed25519.PrivateKey
	mu         sync.Mutex
	bearers    []string
	minted     map[string]bool
	refuse     bool
	exchanges  int
	consumes   int
	renewals   int
	envelope   platform.ComputerCredentialEnvelope
}

func (p *fakeResumePlatform) mint(epoch uint64, nonce string, expiresAt time.Time) string {
	p.t.Helper()
	token, err := platform.MintComputerCapability(platform.ComputerCapability{
		Version: 1, ComputerID: p.envelope.ComputerID, Scopes: []string{"event:read", "event:pin", "event:append"},
		ExpiresAt: expiresAt.UTC().Truncate(time.Microsecond).Format(time.RFC3339Nano), RevocationEpoch: epoch, Nonce: nonce,
	}, p.privateKey)
	if err != nil {
		p.t.Fatal(err)
	}
	p.minted[token] = true
	return token
}

func (p *fakeResumePlatform) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	defer p.mu.Unlock()
	bearer := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	p.bearers = append(p.bearers, bearer)
	body, _ := io.ReadAll(r.Body)
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/internal/computers/credentials/exchange":
		p.exchanges++
		if p.refuse {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		var envelope platform.ComputerCredentialEnvelope
		if json.Unmarshal(body, &envelope) != nil || envelope.Nonce != p.envelope.Nonce {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		expiresAt := time.Now().Add(10 * time.Minute)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(platform.CredentialExchangeResult{
			Capability:               p.mint(3, "exchanged", expiresAt),
			PostRevocationCapability: p.mint(4, "exchanged:revoked", expiresAt),
		})
	case "/internal/computers/credentials/consume":
		p.consumes++
		if !p.minted[bearer] {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		receipt, err := computerevent.NewSignedReceipt("LifecycleReceipt", "corpusd", map[string]any{
			"computer_id": p.envelope.ComputerID, "action": "credential_envelope_consumed",
			"prior_lifecycle_state": "issued", "resulting_lifecycle_state": "consumed",
			"generation": uint64(3), "idempotency_key": "credential-envelope-consume:" + p.envelope.Nonce,
			"request_commitment": p.envelope.RequestCommitment, "completed_at": time.Now().UTC().Format(time.RFC3339Nano),
		}, []computerevent.SigningKey{{
			SignerRef: computerevent.SignerRef{SignerDomain: "platform-control", KeyID: resumeTestKeyID}, PrivateKey: p.privateKey,
		}}, time.Now().UTC())
		if err != nil {
			p.t.Fatal(err)
		}
		_ = json.NewEncoder(w).Encode(platform.CredentialExchangeResult{Receipt: receipt, PendingLifecycleReceipts: []computerevent.Receipt{receipt}})
	case "/internal/computers/credentials/renew":
		p.renewals++
		if !p.minted[bearer] {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		expiresAt := time.Now().Add(30 * time.Minute)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(platform.CredentialExchangeResult{
			Capability:               p.mint(3, "renewed", expiresAt),
			PostRevocationCapability: p.mint(4, "renewed:revoked", expiresAt),
		})
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// signTestEnvelope signs an envelope the way corpusd does
// (credential_envelope.go buildCredentialEnvelope) and returns its canonical
// base64url encoding, the form vmctl delivers.
func signTestEnvelope(t *testing.T, envelope platform.ComputerCredentialEnvelope, privateKey ed25519.PrivateKey) string {
	t.Helper()
	envelope.Signature = ""
	payload, err := computerevent.CanonicalJSON(envelope)
	if err != nil {
		t.Fatal(err)
	}
	preimage := append([]byte("choir-computer-credential-envelope-v1\x00"), payload...)
	envelope.Signature = base64.RawStdEncoding.EncodeToString(ed25519.Sign(privateKey, preimage))
	raw, err := computerevent.CanonicalJSON(envelope)
	if err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

type resumeFixture struct {
	platform    *fakeResumePlatform
	server      *httptest.Server
	credentials *GuestCredentials
	frozen      string
	envelope    platform.ComputerCredentialEnvelope
	privateKey  ed25519.PrivateKey
	handoff     string
}

func newResumeFixture(t *testing.T) *resumeFixture {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	envelope := platform.ComputerCredentialEnvelope{
		Version: 1, ComputerID: "computer-resume", RealizationID: "vm-resume-epoch-7",
		Bearer: "bearer-material", IssuedAt: now.Format(time.RFC3339Nano), ExpiresAt: now.Add(10 * time.Minute).Format(time.RFC3339Nano),
		RevocationEpoch: 3, Nonce: "resume-nonce", IssuanceIdempotencyKey: "guest-credential-resume:vm-resume-epoch-7:7:abcd",
		RequestCommitment: strings.Repeat("c", 64), SigningKeyID: resumeTestKeyID,
		SigningPublicKey: base64.RawStdEncoding.EncodeToString(publicKey),
	}
	fake := &fakeResumePlatform{t: t, privateKey: privateKey, minted: map[string]bool{}, envelope: envelope}
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	// The frozen capability expired two hours ago: past the renewal grace.
	frozen, err := platform.MintComputerCapability(platform.ComputerCapability{
		Version: 1, ComputerID: envelope.ComputerID, Scopes: []string{"event:read", "event:pin", "event:append"},
		ExpiresAt: now.Add(-2 * time.Hour).Format(time.RFC3339Nano), RevocationEpoch: 3, Nonce: "frozen",
	}, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	credentials := &GuestCredentials{
		baseURL: server.URL, computerID: envelope.ComputerID, realizationID: envelope.RealizationID,
		http: server.Client(), token: frozen, expiresAt: now.Add(-2 * time.Hour),
		keyID: resumeTestKeyID, publicKey: publicKey,
	}
	handoff := filepath.Join(t.TempDir(), "revocation-capability")
	credentials.recoveryHandoffPath = handoff
	return &resumeFixture{platform: fake, server: server, credentials: credentials, frozen: frozen, envelope: envelope, privateKey: privateKey, handoff: handoff}
}

func TestInstallResumeEnvelopeReplacesExpiredCapabilityAndRenews(t *testing.T) {
	f := newResumeFixture(t)
	expiresAt, err := f.credentials.InstallResumeEnvelope(context.Background(), signTestEnvelope(t, f.envelope, f.privateKey))
	if err != nil {
		t.Fatalf("InstallResumeEnvelope: %v", err)
	}
	if time.Until(expiresAt) < 9*time.Minute {
		t.Fatalf("installed capability expires %s, want about 10 minutes out", expiresAt)
	}
	if f.platform.exchanges != 1 || f.platform.consumes != 1 {
		t.Fatalf("exchanges=%d consumes=%d, want one each (consumption is part of install)", f.platform.exchanges, f.platform.consumes)
	}
	for _, bearer := range f.platform.bearers {
		if bearer == f.frozen {
			t.Fatal("install sent the frozen capability to the platform")
		}
	}
	if f.credentials.token == f.frozen || !f.platform.minted[f.credentials.token] || !f.credentials.HasPostRevocationCapability() {
		t.Fatal("install did not replace the frozen capability with the exchanged pair")
	}
	pending := f.credentials.PendingLifecycleReceipts()
	if len(pending) != 1 || pending[0].KindFields["action"] != "credential_envelope_consumed" {
		t.Fatalf("pending lifecycle receipts = %d, want the consumption receipt for key_revoked", len(pending))
	}
	if commitment, _, ok := f.credentials.IssuanceProof(); !ok || commitment != f.envelope.RequestCommitment {
		t.Fatal("issuance proof does not name the resume envelope")
	}
	raw, err := os.ReadFile(f.handoff)
	if err != nil {
		t.Fatalf("recovery handoff not rewritten: %v", err)
	}
	if strings.Contains(string(raw), f.frozen) {
		t.Fatal("recovery handoff still carries the frozen capability")
	}
	if _, err := f.credentials.RenewNow(context.Background()); err != nil {
		t.Fatalf("installed capability cannot renew: %v", err)
	}
}

func TestInstallResumeEnvelopeRefusesWithoutChangingState(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(f *resumeFixture) string
	}{
		{"other realization", func(f *resumeFixture) string {
			e := f.envelope
			e.RealizationID = "vm-resume-epoch-8"
			return signTestEnvelope(t, e, f.privateKey)
		}},
		{"other computer", func(f *resumeFixture) string {
			e := f.envelope
			e.ComputerID = "computer-other"
			return signTestEnvelope(t, e, f.privateKey)
		}},
		{"rotated platform key", func(f *resumeFixture) string {
			publicKey, privateKey, _ := ed25519.GenerateKey(rand.Reader)
			e := f.envelope
			e.SigningPublicKey = base64.RawStdEncoding.EncodeToString(publicKey)
			return signTestEnvelope(t, e, privateKey)
		}},
		{"tampered signature", func(f *resumeFixture) string {
			_, otherKey, _ := ed25519.GenerateKey(rand.Reader)
			return signTestEnvelope(t, f.envelope, otherKey)
		}},
		{"exchange refused", func(f *resumeFixture) string {
			f.platform.refuse = true
			return signTestEnvelope(t, f.envelope, f.privateKey)
		}},
		{"unfinished exchange in memory", func(f *resumeFixture) string {
			f.credentials.pendingConsumption = &platform.CredentialConsumptionRequest{ComputerID: f.envelope.ComputerID, Nonce: "boot-nonce", RequestCommitment: strings.Repeat("d", 64)}
			return signTestEnvelope(t, f.envelope, f.privateKey)
		}},
		{"malformed", func(f *resumeFixture) string { return "not-an-envelope" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newResumeFixture(t)
			encoded := tc.mutate(f)
			pendingBefore := f.credentials.pendingConsumption
			if _, err := f.credentials.InstallResumeEnvelope(context.Background(), encoded); err == nil {
				t.Fatal("install accepted the envelope")
			}
			if f.credentials.token != f.frozen || f.credentials.postRevocationToken != "" || f.credentials.pendingConsumption != pendingBefore || len(f.credentials.pendingLifecycle) != 0 {
				t.Fatal("a refused install changed the guest's credential state")
			}
			if f.platform.consumes != 0 {
				t.Fatal("a refused install consumed an envelope")
			}
		})
	}
}
