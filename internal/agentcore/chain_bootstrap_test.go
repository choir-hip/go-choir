package agentcore

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/buildinfo"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

const chainBootstrapTestCommit = "0123456789abcdef0123456789abcdef01234567"

func chainBootstrapRuntime(t *testing.T, computerID string) *Runtime {
	t.Helper()
	dir := t.TempDir()
	manifest := filepath.Join(dir, "guest-manifest")
	if err := os.WriteFile(manifest, []byte("guest-closure"), 0o600); err != nil {
		t.Fatal(err)
	}
	deployReceipt := filepath.Join(dir, "deploy-receipt.json")
	receiptBody := map[string]any{
		"schema_version": 1, "target_commit": chainBootstrapTestCommit, "activated_at": time.Now().UTC().Format(time.RFC3339),
		"artifacts": map[string]any{"autoputer": map[string]any{"commit": chainBootstrapTestCommit, "status": "active"}},
	}
	raw, _ := json.Marshal(receiptBody)
	if err := os.WriteFile(deployReceipt, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CHOIR_GUEST_IMAGE_MANIFEST", manifest)
	t.Setenv("CHOIR_DEPLOY_RECEIPT_PATH", deployReceipt)
	previousCommit := buildinfo.Commit
	buildinfo.Commit = chainBootstrapTestCommit
	t.Cleanup(func() { buildinfo.Commit = previousCommit })

	storePath := filepath.Join(dir, "runtime.db")
	store, err := choirstore.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signingKey := computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "test"}, PrivateKey: privateKey}
	appender, err := computerevent.NewComputerEventAppender(computerID, rollbackTestPinner{signingKey}, store, rollbackTestCAS{key: signingKey, projection: store}, rollbackTestReceiptVerifier{})
	if err != nil {
		t.Fatal(err)
	}
	return &Runtime{
		cfg:           provideriface.Config{ComputerID: computerID, StorePath: storePath},
		store:         store,
		eventAppender: appender,
	}
}

func TestBootstrapChainAppendsSingleGenesis(t *testing.T) {
	computerID := "computer-bootstrap"
	rt := chainBootstrapRuntime(t, computerID)
	report, err := rt.BootstrapChain(context.Background(), "owner-bootstrap", computerID)
	if err != nil {
		t.Fatal(err)
	}
	if report.AlreadyBootstrapped || !report.AppendedEvent {
		t.Fatalf("bootstrap report flags = already=%v appended=%v", report.AlreadyBootstrapped, report.AppendedEvent)
	}
	if report.Head == nil || report.Head.Sequence != 1 {
		t.Fatalf("bootstrap head = %#v", report.Head)
	}
	if report.Head.CanonicalEventHead == "" || report.Head.CanonicalEventHead != report.Head.DesiredEventHead || report.Head.DesiredEventHead != report.Head.EffectiveEventHead {
		t.Fatalf("bootstrap heads not equal: %#v", report.Head)
	}
	if report.CodeRef != "git:"+chainBootstrapTestCommit {
		t.Fatalf("code ref = %q", report.CodeRef)
	}
	if !strings.HasPrefix(report.ArtifactProgramRef, "guest-image:sha256:") {
		t.Fatalf("artifact program ref = %q", report.ArtifactProgramRef)
	}
	if !computerevent.IsSHA256(report.TargetStateCommitment) || report.TargetStateCommitment != report.Head.EffectiveStateCommitment {
		t.Fatalf("state commitment = %q, head effective = %q", report.TargetStateCommitment, report.Head.EffectiveStateCommitment)
	}
	if report.PublishedCheckpoint || report.WroteSelfDevOperation {
		t.Fatalf("bootstrap produced checkpoint=%v operation=%v", report.PublishedCheckpoint, report.WroteSelfDevOperation)
	}
	head, err := rt.store.Head(context.Background(), computerID)
	if err != nil || head == nil || head.Sequence != 1 {
		t.Fatalf("stored head = %#v err=%v", head, err)
	}
}

func TestBootstrapChainIdempotentSecondCall(t *testing.T) {
	computerID := "computer-bootstrap-idem"
	rt := chainBootstrapRuntime(t, computerID)
	first, err := rt.BootstrapChain(context.Background(), "owner-bootstrap", computerID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := rt.BootstrapChain(context.Background(), "owner-bootstrap", computerID)
	if err != nil {
		t.Fatal(err)
	}
	if !second.AlreadyBootstrapped || second.AppendedEvent {
		t.Fatalf("second bootstrap flags = already=%v appended=%v", second.AlreadyBootstrapped, second.AppendedEvent)
	}
	if first.Head == nil || second.Head == nil || *first.Head != *second.Head {
		t.Fatalf("idempotent heads differ: first=%#v second=%#v", first.Head, second.Head)
	}
}

func TestBootstrapChainRefusesIncompleteIdentity(t *testing.T) {
	computerID := "computer-bootstrap-noidentity"
	rt := chainBootstrapRuntime(t, computerID)
	previousCommit := buildinfo.Commit
	buildinfo.Commit = "local"
	t.Cleanup(func() { buildinfo.Commit = previousCommit })
	_, err := rt.BootstrapChain(context.Background(), "owner-bootstrap", computerID)
	if !errors.Is(err, ErrChainBootstrapUnavailable) {
		t.Fatalf("incomplete identity error = %v, want ErrChainBootstrapUnavailable", err)
	}
	head, headErr := rt.store.Head(context.Background(), computerID)
	if headErr != nil || head != nil {
		t.Fatalf("refused bootstrap still produced head = %#v err=%v", head, headErr)
	}
}

func TestBootstrapChainRejectsNonPost(t *testing.T) {
	computerID := "computer-bootstrap-method"
	rt := chainBootstrapRuntime(t, computerID)
	handler := &APIHandler{rt: rt}
	request := httptest.NewRequest(http.MethodGet, "/api/computers/"+computerID+"/lifecycle/bootstrap-chain", nil)
	request.Header.Set("X-Authenticated-User", "owner-bootstrap")
	request.Header.Set("X-Authenticated-Computer", computerID)
	response := httptest.NewRecorder()
	handler.HandleComputersRouter(response, request)
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET bootstrap-chain status=%d body=%s", response.Code, response.Body.String())
	}
}
