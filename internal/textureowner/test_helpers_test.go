package textureowner

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	contentowner "github.com/yusefmosiah/go-choir/internal/content"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type Runtime = agentcore.Runtime
type conductorDecision = ConductorDecision

type textureTestEventAuthority struct {
	store  *store.Store
	signer computerevent.SigningKey
	cipher *computerevent.PrivateArtifactCipher
}

func (a textureTestEventAuthority) Head(ctx context.Context, computerID string) (*computerevent.Head, error) {
	return a.store.Head(ctx, computerID)
}

func (a textureTestEventAuthority) CompareAndSwap(_ context.Context, request computerevent.CASRequest) (computerevent.Receipt, error) {
	return computerevent.NewSignedReceipt("EventHeadReceipt", "corpusd", map[string]any{
		"computer_id": request.Event.ComputerID, "previous_head": request.Event.PreviousHead,
		"event_digest": request.EventDigest, "sequence": request.Event.Sequence,
		"event_kind": request.Event.EventKind, "request_commitment": request.Event.RequestCommitment,
		"pin_receipt_digests": append([]string{request.EventPinReceiptDigest}, request.PayloadPinReceiptDigests...),
		"desired_event_head":  request.Next.DesiredEventHead, "effective_event_head": request.Next.EffectiveEventHead,
		"pending_transition_ref": request.Next.PendingTransitionRef, "desired_state_commitment": request.Next.DesiredStateCommitment,
		"effective_state_commitment": request.Next.EffectiveStateCommitment,
	}, []computerevent.SigningKey{a.signer}, time.Now().UTC())
}

func (a textureTestEventAuthority) PinEvent(_ context.Context, computerID string, canonical []byte, requestCommitment string) (computerevent.PinResult, error) {
	digest := computerevent.DigestBytes(canonical)
	receipt, err := computerevent.NewSignedReceipt("PinReceipt", "corpusd", map[string]any{
		"computer_id": computerID, "artifact_digest": digest, "request_commitment": requestCommitment,
	}, []computerevent.SigningKey{a.signer}, time.Now().UTC())
	return computerevent.PinResult{ArtifactDigest: digest, Receipt: receipt}, err
}

func (a textureTestEventAuthority) PreparePrivatePayload(ctx context.Context, cipher *computerevent.PrivateArtifactCipher, computerID, eventID, mediaType string, plaintext []byte) ([]byte, computerevent.PrivateArtifactMetadata, error) {
	return cipher.Encrypt(ctx, computerID, eventID, mediaType, "private", plaintext)
}

func (a textureTestEventAuthority) PinPrivatePayload(ctx context.Context, cipher *computerevent.PrivateArtifactCipher, computerID, eventID string, envelope []byte, pinIntentCommitment string) (computerevent.PinResult, error) {
	_, metadata, err := cipher.Decrypt(ctx, envelope, computerID, eventID)
	if err != nil {
		return computerevent.PinResult{}, err
	}
	digest := computerevent.DigestBytes(envelope)
	receipt, err := computerevent.NewSignedReceipt("PinReceipt", "corpusd", map[string]any{
		"computer_id": computerID, "artifact_digest": digest, "media_type": metadata.MediaType,
		"privacy_class": "private", "pin_intent_commitment": pinIntentCommitment,
	}, []computerevent.SigningKey{a.signer}, time.Now().UTC())
	return computerevent.PinResult{ArtifactDigest: digest, Receipt: receipt}, err
}

type textureTestKeyResolver struct{ key ed25519.PublicKey }

func (r textureTestKeyResolver) ResolveReceiptKey(string, string, string, uint64, time.Time) (ed25519.PublicKey, error) {
	return r.key, nil
}

func authenticatedRequest(method, path, body, user string) *http.Request {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req.Header.Set("X-Authenticated-User", user)
	return req
}

func runtimeHandlerRequest(t *testing.T, handler http.HandlerFunc, method, path, body, user string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, authenticatedRequest(method, path, body, user))
	return w
}

func newTextureTestEventAppender(t *testing.T, s *store.Store, dir string) (*computerevent.ComputerEventAppender, *computerevent.PrivateArtifactCipher) {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer := computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "texture-test"}, PrivateKey: privateKey}
	cipher, err := computerevent.LoadGuestPrivateArtifactCipher(filepath.Join(dir, "privacy-key.json"), "sandbox-test", true)
	if err != nil {
		t.Fatal(err)
	}
	authority := textureTestEventAuthority{store: s, signer: signer, cipher: cipher}
	appender, err := computerevent.NewComputerEventAppender("sandbox-test", authority, s, authority, computerevent.EventHeadReceiptVerifier{Keys: textureTestKeyResolver{key: publicKey}})
	if err != nil {
		t.Fatal(err)
	}
	eventID, err := computerevent.NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	genesisCommitment := computerevent.DigestBytes([]byte("texture-test-genesis"))
	genesis := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: "sandbox-test",
		EventKind: computerevent.EventGenesisImported, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
		IdempotencyKey: "texture-test-genesis", ActorProfile: "trusted-core", AuthorityRef: "authority:test",
		PrivacyClass: "private", ReducerVersion: computerevent.ReducerVersionV1,
		PayloadCommitment: genesisCommitment, ResultingEffectiveCommitment: genesisCommitment,
	}
	if _, err := appender.AppendNew(context.Background(), genesis, computerevent.TransitionInput{TargetStateCommitment: genesisCommitment}, nil); err != nil {
		t.Fatal(err)
	}
	return appender, cipher
}

func testAPISetup(t *testing.T, maildURLs ...string) (*agentcore.Runtime, *Handler) {
	t.Helper()
	maildURL := ""
	if len(maildURLs) > 0 {
		maildURL = maildURLs[0]
	}
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "texture-owner.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	appender, cipher := newTextureTestEventAppender(t, s, dir)
	bus := events.NewEventBus()
	core := agentcore.New(provideriface.Config{
		SandboxID:           "sandbox-test",
		StorePath:           dbPath,
		PromptRoot:          filepath.Join(dir, "prompts"),
		ProviderTimeout:     time.Second,
		SupervisionInterval: time.Hour,
		MaildURL:            maildURL,
	}, s, bus, provider.NewStubProvider(0),
		agentcore.WithContentService(contentowner.NewService(s, bus)),
		agentcore.WithComputerEventAppender(appender),
		agentcore.WithPrivateArtifactCipher(cipher),
	)
	core.SetDispatchActor(func(ctx context.Context, ownerID, computerID, toAgentID, kind, content, trajectoryID, fromAgentID string) error {
		switch kind {
		case "initial_dispatch":
			runID := strings.TrimSpace(content)
			if runID != "" {
				go func() {
					rec, err := s.GetLifecycleRun(ctx, ownerID, computerID, runID)
					if err != nil {
						rec, err = s.GetRunByOwner(ctx, ownerID, runID)
					}
					if err == nil {
						core.ExecuteActivationSync(ctx, &rec)
					}
				}()
			}
		case "coagent_result":
			agent, err := s.GetAgentByScope(ctx, ownerID, computerID, toAgentID)
			if err == nil {
				if _, err := core.ReconcileCoagentWake(ctx, agent.OwnerID, toAgentID); err != nil {
					log.Printf("test dispatch: reconcile coagent wake for %s: %v", toAgentID, err)
				}
			}
		}
		return nil
	})
	if err := core.InstallDefaultAgentTools(t.TempDir()); err != nil {
		t.Fatalf("install generic core tools: %v", err)
	}
	handler := NewHandler(core)
	if err := RegisterTools(core.ToolRegistryForProfile("texture"), handler); err != nil {
		t.Fatalf("register Texture owner tools: %v", err)
	}
	t.Cleanup(func() {
		core.Stop()
		_ = s.Close()
	})
	return core, handler
}

func textureRequest(t *testing.T, method, path string, body any) *http.Request {
	t.Helper()
	var requestBody *bytes.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		requestBody = bytes.NewReader(payload)
	} else {
		requestBody = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, requestBody)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Authenticated-User", "user-1")
	return req
}

func TestStartSupervisionTrajectoryProjectsLifecycleSnapshot(t *testing.T) {
	core, handler := testAPISetup(t)
	now := time.Now().UTC()
	start := types.StartLifecycleRequest{
		OwnerID: "owner-supervision-test", ComputerID: core.TextureSandboxID(), CommandID: "command-supervision-test",
		TrajectoryID: "trajectory-supervision-test", Kind: types.TrajectoryKindTask,
		SubjectRefs:     map[string]string{"artifact": "texture://documents/document-supervision-test"},
		SettlementRule:  types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true},
		InitialWork:     types.WorkItemRecord{WorkItemID: "work-supervision-test", Objective: "Create the supervised document.", AssignedAgentID: "texture:document-supervision-test", AuthorityProfile: "texture"},
		InitialDocument: types.Document{DocID: "document-supervision-test", OwnerID: "owner-supervision-test", ComputerID: core.TextureSandboxID(), TrajectoryID: "trajectory-supervision-test", Title: "Supervised", CreatedAt: now, UpdatedAt: now},
		InitialRevision: types.Revision{RevisionID: "revision-supervision-test", DocID: "document-supervision-test", OwnerID: "owner-supervision-test", ComputerID: core.TextureSandboxID(), TrajectoryID: "trajectory-supervision-test", AuthorKind: types.AuthorUser, AuthorLabel: "owner-supervision-test", Content: "Supervised content.", CreatedAt: now},
		Agent:           types.AgentRecord{AgentID: "texture:document-supervision-test", OwnerID: "owner-supervision-test", ComputerID: core.TextureSandboxID(), SandboxID: core.TextureSandboxID(), Profile: "texture", Role: "texture", ChannelID: "document-supervision-test", CreatedAt: now, UpdatedAt: now},
	}
	digest, err := store.ComputeStartLifecycleRequestDigest(start)
	if err != nil {
		t.Fatal(err)
	}
	start.StartRequestDigest = digest
	result, err := handler.startSupervisionTrajectory(context.Background(), start)
	if err != nil {
		t.Fatal(err)
	}
	if result.Document == nil || result.Revision == nil || result.Agent == nil || result.WorkItem == nil ||
		result.WorkItem.WorkItemID != start.InitialWork.WorkItemID {
		t.Fatalf("Texture open transaction omitted initial assignment: %+v", result)
	}
}
