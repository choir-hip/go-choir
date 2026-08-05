package agentcore

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	contentowner "github.com/yusefmosiah/go-choir/internal/content"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/modelpolicy"
	"github.com/yusefmosiah/go-choir/internal/promptstore"
	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/texturedoc"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func seedDurableTextureSubject(t *testing.T, s *store.Store, ownerID, docID string) string {
	t.Helper()
	agentID := currentTextureAgentID(docID)
	computerID := "sandbox-test"
	trajectoryID := "test-trajectory:" + ownerID + ":" + docID
	commandID := "test-start:" + ownerID + ":" + docID
	workID := "test-work:" + ownerID + ":" + docID
	revisionID := "test-revision:" + ownerID + ":" + docID
	objective := "process durable updates"
	content := "Initial durable test content"

	mutation := func(kind string, body map[string]any) computerevent.SupervisionMutation {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("encode %s mutation: %v", kind, err)
		}
		return computerevent.SupervisionMutation{Kind: kind, Body: raw}
	}
	transaction := computerevent.SupervisionTransaction{
		Schema: computerevent.SupervisionSchemaV1, Reducer: computerevent.SupervisionReducerV1,
		DigestRecipe: computerevent.SupervisionDigestRecipeV1, TransactionID: commandID,
		TransactionClass: "open_trajectory", OwnerID: ownerID, ComputerID: computerID,
		TrajectoryID: trajectoryID, CommandID: commandID, CommandDigest: computerevent.ZeroHead,
		Actor: computerevent.SupervisionActor{ActorID: agentID, Role: "texture", AuthorityRef: "texture:trajectory:" + trajectoryID},
		Mutations: []computerevent.SupervisionMutation{
			mutation("trajectory_started", map[string]any{
				"trajectory_kind": string(types.TrajectoryKindDocument), "subject_refs": map[string]string{"artifact": "texture://documents/" + docID, "doc_id": docID},
				"intent_revision_id": revisionID, "artifact_id": docID, "artifact_revision_id": revisionID,
				"texture_actor_id": agentID, "initial_assignment_ids": []string{workID}, "objective": objective,
			}),
			mutation("intent_revised", map[string]any{
				"intent_revision_id": revisionID, "parent_intent_revision_id": nil, "intent": objective,
				"material": false, "affected_targets": []string{},
			}),
			mutation("texture_revision", map[string]any{
				"artifact_id": docID, "revision_id": revisionID, "title": "Durable test subject",
				"parent_revision_id": nil, "content": content, "source_graph": map[string]any{}, "metadata": map[string]any{},
				"metadata_digest": computerevent.DigestBytes([]byte(content)), "narrative_kind": "texture_synthesis",
				"fulfills_intent_revision_id": revisionID,
			}),
		},
	}
	digest, err := transaction.ComputeCommandDigest()
	if err != nil {
		t.Fatalf("digest supervision fixture: %v", err)
	}
	transaction.CommandDigest = digest

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("create supervision fixture signer: %v", err)
	}
	signer := computerevent.SigningKey{
		SignerRef:  computerevent.SignerRef{SignerDomain: "corpusd", KeyID: "fixture"},
		PrivateKey: privateKey,
	}
	canonicalHead, err := s.Head(context.Background(), computerID)
	if err != nil {
		t.Fatalf("read supervision fixture head: %v", err)
	}
	cas := &testSupervisionCAS{head: canonicalHead, signer: signer}
	pinner := &testSupervisionPinner{signer: signer, private: make(map[string]testSupervisionPrivateArtifact)}
	verifier := computerevent.EventHeadReceiptVerifier{Keys: testSupervisionKeyResolver{key: publicKey}}
	appender, err := computerevent.NewComputerEventAppender(computerID, pinner, s, cas, verifier)
	if err != nil {
		t.Fatalf("create supervision fixture appender: %v", err)
	}
	if canonicalHead == nil {
		eventID, err := computerevent.NewEventID()
		if err != nil {
			t.Fatalf("create supervision fixture genesis identity: %v", err)
		}
		stateCommitment := computerevent.DigestBytes([]byte("fixture-state:" + ownerID + ":" + docID))
		genesis := computerevent.Event{
			SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: computerID,
			EventKind: computerevent.EventGenesisImported, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
			IdempotencyKey: "fixture-genesis:" + ownerID + ":" + docID, ActorProfile: "runtime",
			AuthorityRef: "authority:test", PayloadCommitment: stateCommitment, PrivacyClass: "private",
			ReducerVersion: computerevent.ReducerVersionV1, ResultingEffectiveCommitment: stateCommitment,
		}
		if _, err := appender.AppendNew(context.Background(), genesis, computerevent.TransitionInput{TargetStateCommitment: stateCommitment}, nil); err != nil {
			t.Fatalf("seed supervision fixture genesis: %v", err)
		}
	}
	cipher, err := computerevent.LoadGuestPrivateArtifactCipher(filepath.Join(t.TempDir(), "privacy-key.json"), computerID, true)
	if err != nil {
		t.Fatalf("create supervision fixture cipher: %v", err)
	}
	event := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, ComputerID: computerID,
		EventKind: computerevent.EventSupervisionTransaction, PrivacyClass: "private",
		ReducerVersion: computerevent.ReducerVersionV1,
	}
	if _, _, err := appender.AppendNewSupervisionTransaction(context.Background(), event, computerevent.TransitionInput{}, transaction, cipher); err != nil {
		t.Fatalf("seed durable Texture subject: %v", err)
	}
	return trajectoryID
}

func installTestSupervisionAppender(t *testing.T, rt *Runtime, s *store.Store) *testSupervisionPinner {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer := computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "corpusd", KeyID: "fixture-runtime"}, PrivateKey: privateKey}
	head, err := s.Head(context.Background(), rt.TextureSandboxID())
	if err != nil {
		t.Fatal(err)
	}
	pinner := &testSupervisionPinner{signer: signer, private: make(map[string]testSupervisionPrivateArtifact)}
	cas := &testSupervisionCAS{head: head, signer: signer}
	verifier := computerevent.EventHeadReceiptVerifier{Keys: testSupervisionKeyResolver{key: publicKey}}
	appender, err := computerevent.NewComputerEventAppender(rt.TextureSandboxID(), pinner, s, cas, verifier)
	if err != nil {
		t.Fatal(err)
	}
	cipher, err := computerevent.LoadGuestPrivateArtifactCipher(filepath.Join(t.TempDir(), "privacy-key.json"), rt.TextureSandboxID(), true)
	if err != nil {
		t.Fatal(err)
	}
	rt.eventAppender = appender
	rt.privateArtifactCipher = cipher
	return pinner
}

type testSupervisionKeyResolver struct{ key ed25519.PublicKey }

func (r testSupervisionKeyResolver) ResolveReceiptKey(string, string, string, uint64, time.Time) (ed25519.PublicKey, error) {
	return r.key, nil
}

type testSupervisionPrivateArtifact struct {
	envelope []byte
	pin      computerevent.PinResult
}

type testSupervisionPinner struct {
	mu      sync.Mutex
	signer  computerevent.SigningKey
	private map[string]testSupervisionPrivateArtifact
}

func (p *testSupervisionPinner) PinEvent(_ context.Context, computerID string, canonicalEvent []byte, requestCommitment string) (computerevent.PinResult, error) {
	digest := computerevent.DigestBytes(canonicalEvent)
	receipt, err := computerevent.NewSignedReceipt("PinReceipt", "corpusd", map[string]any{
		"computer_id": computerID, "artifact_digest": digest, "request_commitment": requestCommitment,
	}, []computerevent.SigningKey{p.signer}, time.Now().UTC())
	return computerevent.PinResult{ArtifactDigest: digest, Receipt: receipt}, err
}

func (p *testSupervisionPinner) PreparePrivatePayload(ctx context.Context, cipher *computerevent.PrivateArtifactCipher, computerID, eventID, mediaType string, plaintext []byte) ([]byte, computerevent.PrivateArtifactMetadata, error) {
	return cipher.Encrypt(ctx, computerID, eventID, mediaType, "private", plaintext)
}

func (p *testSupervisionPinner) PinPrivatePayload(ctx context.Context, cipher *computerevent.PrivateArtifactCipher, computerID, eventID string, envelope []byte, pinIntentCommitment string) (computerevent.PinResult, error) {
	if _, metadata, err := cipher.Decrypt(ctx, envelope, computerID, eventID); err != nil {
		return computerevent.PinResult{}, err
	} else {
		digest := computerevent.DigestBytes(envelope)
		receipt, err := computerevent.NewSignedReceipt("PinReceipt", "corpusd", map[string]any{
			"computer_id": computerID, "artifact_digest": digest, "media_type": metadata.MediaType,
			"privacy_class": "private", "pin_intent_commitment": pinIntentCommitment,
		}, []computerevent.SigningKey{p.signer}, time.Now().UTC())
		pin := computerevent.PinResult{ArtifactDigest: digest, Receipt: receipt}
		if err == nil {
			p.mu.Lock()
			p.private[digest] = testSupervisionPrivateArtifact{envelope: append([]byte(nil), envelope...), pin: pin}
			p.mu.Unlock()
		}
		return pin, err
	}
}

func (p *testSupervisionPinner) Events(context.Context, string, uint64) ([]computerevent.DurableEvent, error) {
	return nil, nil
}

func (p *testSupervisionPinner) PrivateArtifact(_ context.Context, _ string, digest string) ([]byte, computerevent.PinResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	artifact, ok := p.private[digest]
	if !ok {
		return nil, computerevent.PinResult{}, store.ErrNotFound
	}
	return append([]byte(nil), artifact.envelope...), artifact.pin, nil
}

type testSupervisionCAS struct {
	head   *computerevent.Head
	signer computerevent.SigningKey
}

func (c *testSupervisionCAS) Head(context.Context, string) (*computerevent.Head, error) {
	if c.head == nil {
		return nil, nil
	}
	head := *c.head
	return &head, nil
}

func (c *testSupervisionCAS) CompareAndSwap(_ context.Context, request computerevent.CASRequest) (computerevent.Receipt, error) {
	if (c.head == nil && request.Event.PreviousHead != computerevent.ZeroHead) ||
		(c.head != nil && request.Event.PreviousHead != c.head.CanonicalEventHead) {
		return computerevent.Receipt{}, computerevent.ErrCASConflict
	}
	receipt, err := computerevent.NewSignedReceipt("EventHeadReceipt", "corpusd", map[string]any{
		"computer_id": request.Event.ComputerID, "previous_head": request.Event.PreviousHead,
		"event_digest": request.EventDigest, "sequence": request.Event.Sequence,
		"event_kind": request.Event.EventKind, "request_commitment": request.Event.RequestCommitment,
		"pin_receipt_digests": append([]string{request.EventPinReceiptDigest}, request.PayloadPinReceiptDigests...),
		"desired_event_head":  request.Next.DesiredEventHead, "effective_event_head": request.Next.EffectiveEventHead,
		"pending_transition_ref": request.Next.PendingTransitionRef, "desired_state_commitment": request.Next.DesiredStateCommitment,
		"effective_state_commitment": request.Next.EffectiveStateCommitment,
	}, []computerevent.SigningKey{c.signer}, time.Now().UTC())
	if err != nil {
		return computerevent.Receipt{}, err
	}
	head := request.Next
	c.head = &head
	return receipt, nil
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// listCoagentRunsByRequester returns the runs owned by ownerID whose
// RequestedByRunID provenance points at requesterRunID. It replaces the
// deleted store helper ListChildRuns: callers used that to count/inspect the
// runs spawned on behalf of a requesting run, which is now expressed through
// requester provenance rather than parent/child control links.
func listCoagentRunsByRequester(t *testing.T, s *store.Store, ownerID, requesterRunID string, limit int) []types.RunRecord {
	t.Helper()
	runs, err := s.ListRunsByOwner(context.Background(), ownerID, limit)
	if err != nil {
		t.Fatalf("list runs by owner: %v", err)
	}
	var matched []types.RunRecord
	for _, run := range runs {
		if strings.TrimSpace(run.RequestedByRunID) == requesterRunID {
			matched = append(matched, run)
		}
	}
	return matched
}

func runGit(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// testAPISetup creates a fresh Runtime and APIHandler for HTTP handler tests.
func testAPISetup(t *testing.T) (*Runtime, *APIHandler) {
	t.Helper()

	dir := filepath.Join(os.TempDir(), "go-choir-m3-api-test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	dbPath := filepath.Join(dir, t.Name()+".db")
	promptRoot := filepath.Join(dir, t.Name()+"-prompts")
	_ = os.Remove(dbPath)
	_ = os.RemoveAll(promptRoot)

	s, err := openTestStore(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	bus := events.NewEventBus()
	rt := New(provideriface.Config{
		SandboxID:           "sandbox-test",
		StorePath:           dbPath,
		PromptRoot:          promptRoot,
		ProviderTimeout:     time.Second,
		SupervisionInterval: time.Hour,
	}, s, bus, provider.NewStubProvider(0), WithContentService(contentowner.NewService(s, bus)))
	setTestDispatch(rt, s)
	handler := NewAPIHandler(rt)

	t.Cleanup(func() {
		rt.Stop()
		_ = s.Close()
		_ = os.Remove(dbPath)
		_ = os.RemoveAll(promptRoot)
	})

	return rt, handler
}

func authenticatedRequest(method, path, body, user string) *http.Request {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	if user != "" {
		req.Header.Set("X-Authenticated-User", user)
	}
	return req
}

func runtimeHandlerRequest(t *testing.T, handler http.HandlerFunc, method, path, body, user string) *httptest.ResponseRecorder {
	t.Helper()
	req := authenticatedRequest(method, path, body, user)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

func textureRequest(t *testing.T, method, path string, body interface{}) *http.Request {
	t.Helper()
	var reqBody *bytes.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reqBody = bytes.NewReader(data)
	} else {
		reqBody = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("X-Authenticated-User", "user-1")
	return req
}

func waitForTaskCompletion(t *testing.T, h *APIHandler, taskID string, timeout time.Duration) types.RunState {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		rec, err := h.rt.GetRun(context.Background(), taskID, "user-1")
		if err != nil {
			t.Fatalf("get task status: %v", err)
		}
		if rec.State.Terminal() {
			return rec.State
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("task %s did not complete within %v", taskID, timeout)
	return ""
}

// waitForEvents polls ListEvents until all expected event kinds are present
// or the deadline expires. This avoids races where the run state becomes
// terminal before the final event is persisted (common under -race).
func waitForEvents(t *testing.T, s *store.Store, runID string, expectedKinds []types.EventKind, timeout time.Duration) []types.EventRecord {
	t.Helper()
	deadline := time.Now().Add(timeout)
	needed := make(map[types.EventKind]bool, len(expectedKinds))
	for _, k := range expectedKinds {
		needed[k] = true
	}
	var evts []types.EventRecord
	for time.Now().Before(deadline) {
		var err error
		evts, err = s.ListEvents(context.Background(), runID, 200)
		if err != nil {
			t.Fatalf("list events: %v", err)
		}
		for _, ev := range evts {
			delete(needed, ev.Kind)
		}
		if len(needed) == 0 {
			return evts
		}
		time.Sleep(20 * time.Millisecond)
	}
	for kind := range needed {
		t.Errorf("missing expected event kind: %s", kind)
	}
	return evts
}

func testRuntime(t *testing.T) (*Runtime, *store.Store) {
	t.Helper()

	dir := filepath.Join(os.TempDir(), "go-choir-m3-runtime-test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	dbPath := filepath.Join(dir, t.Name()+".db")
	promptRoot := filepath.Join(dir, t.Name()+"-prompts")
	_ = os.Remove(dbPath)
	_ = os.RemoveAll(promptRoot)

	s, err := openTestStore(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	bus := events.NewEventBus()
	rt := New(provideriface.Config{
		SandboxID:           "sandbox-test",
		StorePath:           dbPath,
		PromptRoot:          promptRoot,
		ProviderTimeout:     time.Second,
		SupervisionInterval: time.Hour,
	}, s, bus, provider.NewStubProvider(0), WithContentService(contentowner.NewService(s, bus)))

	setTestDispatch(rt, s)

	t.Cleanup(func() {
		rt.Stop()
		_ = s.Close()
		_ = os.Remove(dbPath)
		_ = os.RemoveAll(promptRoot)
	})

	return rt, s
}

func testPeerRuntime(t *testing.T, primary *Runtime, sharedStore *store.Store) *Runtime {
	t.Helper()
	if primary == nil || sharedStore == nil {
		t.Fatal("primary runtime and shared store are required")
	}
	bus := events.NewEventBus()
	peer := New(
		primary.cfg, sharedStore, bus, provider.NewStubProvider(0),
		WithContentService(contentowner.NewService(sharedStore, bus)),
	)
	setTestDispatch(peer, sharedStore)
	t.Cleanup(peer.Stop)
	return peer
}

// setTestDispatch sets a test dispatch function that executes runs
// asynchronously. Production uses the actor runtime (actorruntime.New);
// tests use this minimal dispatch that calls ExecuteActivationSync in a
// goroutine. This is test infrastructure, not production code.
func setTestDispatch(rt *Runtime, s *store.Store) {
	rt.SetDispatchActor(func(ctx context.Context, ownerID, computerID, toAgentID, kind, content, trajectoryID, fromAgentID string) error {
		switch kind {
		case "initial_dispatch":
			runID := strings.TrimSpace(content)
			if runID == "" {
				return nil
			}
			go func() {
				rec, err := s.GetLifecycleRun(ctx, ownerID, computerID, runID)
				if err != nil {
					rec, err = s.GetRunByOwner(ctx, ownerID, runID)
				}
				if err != nil {
					return
				}
				rt.ExecuteActivationSync(ctx, &rec)
			}()
		case "coagent_result":
			// Synchronous: the boot sweep needs the reconcile to
			// complete before the test checks the result.
			agent, err := s.GetAgentByScope(ctx, ownerID, computerID, toAgentID)
			if err != nil {
				return nil // agent not found — nothing to wake
			}
			if _, err := rt.ReconcileCoagentWake(ctx, agent.OwnerID, toAgentID); err != nil {
				log.Printf("test dispatch: reconcile coagent wake for %s: %v", toAgentID, err)
			}
		}
		return nil
	})
}

func testPromptRuntime(t *testing.T) *Runtime {
	t.Helper()
	promptRoot := filepath.Join(t.TempDir(), "prompts")
	return &Runtime{
		cfg: provideriface.Config{
			SandboxID:           "sandbox-prompt-test",
			PromptRoot:          promptRoot,
			SupervisionInterval: time.Hour,
		},
		promptStore: promptstore.New(promptRoot),
		modelPolicy: modelpolicy.NewManager(modelpolicy.ManagerConfig{}),
	}
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal JSON: %v", err)
	}
	return raw
}

func runtimeTestTextureBodyDoc(t *testing.T, docID, revisionID, content string) json.RawMessage {
	t.Helper()
	body, err := json.Marshal(texturedoc.StructuredTextureDoc{
		Schema: texturedoc.SchemaV1,
		Doc: texturedoc.Node{
			Type:  "doc",
			Attrs: map[string]any{"id": "doc-" + docID + "-" + revisionID},
			Content: []texturedoc.Node{{
				Type:    "paragraph",
				Attrs:   map[string]any{"id": "p-" + docID + "-" + revisionID},
				Content: []texturedoc.Node{{Type: "text", Text: content}},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return body
}
