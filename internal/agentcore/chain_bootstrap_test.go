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

	"github.com/yusefmosiah/go-choir/internal/events"
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

func TestCreateRunAdmissionRefusesPreGenesis(t *testing.T) {
	computerID := "computer-pre-genesis-admission"
	rt := chainBootstrapRuntime(t, computerID)
	rt.bus = events.NewEventBus()
	if err := rt.store.BindProjectionTape(computerID, rt.eventAppender); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	// Empty chain -> admission refused before any store mutation.
	rec, err := rt.createRunWithMetadata(ctx, "hello", "owner-admission", map[string]any{})
	if err == nil || rec != nil || !strings.Contains(err.Error(), "pre-genesis") {
		t.Fatalf("pre-genesis admission = rec=%v err=%v, want pre-genesis refusal", rec, err)
	}
	// The gate must NOT fire once the chain has a genesis: the bootstrapped
	// computer's admission proceeds normally (the gate is a no-op when the
	// projection head exists; the full run path is exercised by the runtime
	// suites). Verify the head now exists so the gate would pass.
	report, err := rt.BootstrapChain(ctx, "owner-bootstrap", computerID)
	if err != nil || !report.AppendedEvent {
		t.Fatalf("bootstrap = %#v %v", report, err)
	}
	head, err := rt.store.Head(ctx, computerID)
	if err != nil || head == nil || head.Sequence != 1 {
		t.Fatalf("post-bootstrap head = %#v %v", head, err)
	}
}

func TestMintProvisionedGenesisMintsOnce(t *testing.T) {
	computerID := "computer-provisioned-genesis"
	rt := chainBootstrapRuntime(t, computerID)
	if err := rt.store.BindProjectionTape(computerID, rt.eventAppender); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	// First boot on an empty, unchained store mints genesis_imported seq=1.
	appended, err := MintProvisionedGenesis(ctx, rt.store, rt.eventAppender, computerID, "owner-provisioned")
	if err != nil || !appended {
		t.Fatalf("provisioned mint = appended=%v err=%v", appended, err)
	}
	head, err := rt.store.Head(ctx, computerID)
	if err != nil || head == nil || head.Sequence != 1 {
		t.Fatalf("post-mint head = %#v err=%v", head, err)
	}

	// The mint is idempotent and shares the converged-head contract with
	// BootstrapChain: a second call is a no-op, and a later owner-scoped
	// bootstrap-chain call reports already_bootstrapped instead of a
	// duplicate genesis.
	appended, err = MintProvisionedGenesis(ctx, rt.store, rt.eventAppender, computerID, "owner-provisioned")
	if err != nil || appended {
		t.Fatalf("second provisioned mint = appended=%v err=%v", appended, err)
	}
	report, err := rt.BootstrapChain(ctx, "owner-bootstrap", computerID)
	if err != nil || !report.AlreadyBootstrapped || report.AppendedEvent {
		t.Fatalf("post-mint bootstrap-chain = %#v %v", report, err)
	}
}

func TestCompletePromptBarDecisionRefusesPreGenesis(t *testing.T) {
	computerID := "computer-pre-genesis-promptbar"
	rt := chainBootstrapRuntime(t, computerID)
	rt.bus = events.NewEventBus()
	if err := rt.store.BindProjectionTape(computerID, rt.eventAppender); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	// The prompt-bar decision path persists the agent before StartRun
	// admission; without its own gate a pre-genesis submit fails deep in
	// the projection batch with invalid genesis (the observed staging 500).
	// It must refuse with the typed sentinel instead.
	rec, err := rt.CompletePromptBarDecision(ctx, "hello", "owner-pre-genesis", map[string]any{}, PromptBarDecisionSpec{Action: "open_app", App: "texture", Title: "hello"})
	if !errors.Is(err, ErrPreGenesis) || rec != nil {
		t.Fatalf("pre-genesis prompt-bar = rec=%v err=%v, want ErrPreGenesis", rec, err)
	}

	// After genesis the sentinel is resolved — the refusal was about the
	// missing head, not the call shape.
	if _, err := MintProvisionedGenesis(ctx, rt.store, rt.eventAppender, computerID, "owner-provisioned"); err != nil {
		t.Fatalf("provisioned mint: %v", err)
	}
	if _, err := rt.CompletePromptBarDecision(ctx, "hello", "owner-pre-genesis", map[string]any{}, PromptBarDecisionSpec{Action: "open_app", App: "texture", Title: "hello"}); errors.Is(err, ErrPreGenesis) {
		t.Fatalf("post-genesis prompt-bar still refused pre-genesis: %v", err)
	}
}
