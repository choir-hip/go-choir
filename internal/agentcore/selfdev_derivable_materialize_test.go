package agentcore

// M7 derivable-continuation proofs: an Accepted self-development operation must
// reach Applied through the canonical post-commit observer alone (no API
// caller), and an operation interrupted mid-materialize must recover through
// the same reconciler. The fixture binds the operation store to the
// projection tape exactly as production does, so every operation mutation is a
// canonical EventProjectionBatchRecorded event that the observer fires on —
// that is the derivable-wake mechanism under test, not a test-side drive loop.

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/capsule"
	transaction "github.com/yusefmosiah/go-choir/internal/capsule/transaction"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/receiptsigner"
	"github.com/yusefmosiah/go-choir/internal/routeledger"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/updater"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// derivableChainCAS mints EventHeadReceipts like rollbackTestCAS but retains
// the committed durable chain for replay sources. The mutex serializes
// concurrent appender commits with test-side enumeration.
type derivableChainCAS struct {
	key        computerevent.SigningKey
	projection computerevent.ProjectionStore
	mu         sync.Mutex
	events     []computerevent.DurableEvent
	// gate, when set, is corpusd's own CAS (with its admission gate): an
	// append it refuses is refused here (P4 end-to-end test).
	gate func(context.Context, computerevent.CASRequest) error
}

// installGate replays the chain so far into gate's store, then routes every
// later append through it first.
func (c *derivableChainCAS) installGate(ctx context.Context, gate func(context.Context, computerevent.CASRequest) error) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, record := range c.events {
		if err := gate(ctx, record.Request); err != nil {
			return err
		}
	}
	c.gate = gate
	return nil
}

func (c *derivableChainCAS) Head(ctx context.Context, computerID string) (*computerevent.Head, error) {
	return c.projection.Head(ctx, computerID)
}

func (c *derivableChainCAS) CompareAndSwap(ctx context.Context, request computerevent.CASRequest) (computerevent.Receipt, error) {
	receipt, err := computerevent.NewSignedReceipt(
		"EventHeadReceipt", "corpusd",
		map[string]any{"event_digest": request.EventDigest},
		[]computerevent.SigningKey{c.key}, time.Now().UTC(),
	)
	if err != nil {
		return computerevent.Receipt{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.gate != nil {
		if err := c.gate(ctx, request); err != nil {
			return computerevent.Receipt{}, err
		}
	}
	c.events = append(c.events, computerevent.DurableEvent{Request: request, Receipt: receipt})
	return receipt, nil
}

func (c *derivableChainCAS) Events(_ context.Context, _ string, afterSequence uint64) ([]computerevent.DurableEvent, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	records := make([]computerevent.DurableEvent, 0, len(c.events))
	for _, record := range c.events {
		if record.Request.Event.Sequence > afterSequence {
			records = append(records, record)
		}
	}
	return records, nil
}

func (c *derivableChainCAS) snapshot() []computerevent.DurableEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]computerevent.DurableEvent(nil), c.events...)
}

// derivableTestPinner signs pin receipts like rollbackTestPinner and also
// implements NonPrivatePayloadPinner so projection-batch and materialization
// payloads pin — recording the bytes so replay sources can fetch them.
type derivableTestPinner struct {
	key      computerevent.SigningKey
	mu       sync.Mutex
	payloads map[string][]byte
}

func (p *derivableTestPinner) PinEvent(_ context.Context, computerID string, canonical []byte, requestCommitment string) (computerevent.PinResult, error) {
	digest := computerevent.DigestBytes(canonical)
	receipt, err := computerevent.NewSignedReceipt("PinReceipt", "corpusd",
		map[string]any{"computer_id": computerID, "artifact_digest": digest, "request_commitment": requestCommitment},
		[]computerevent.SigningKey{p.key}, time.Now().UTC())
	return computerevent.PinResult{ArtifactDigest: digest, Receipt: receipt}, err
}

func (p *derivableTestPinner) PinNonPrivatePayload(_ context.Context, computerID string, payload []byte, mediaType, privacyClass, pinIntentCommitment string) (computerevent.PinResult, error) {
	digest := computerevent.DigestBytes(payload)
	receipt, err := computerevent.NewSignedReceipt("PinReceipt", "corpusd",
		map[string]any{"computer_id": computerID, "artifact_digest": digest, "media_type": mediaType, "privacy_class": privacyClass, "pin_intent_commitment": pinIntentCommitment},
		[]computerevent.SigningKey{p.key}, time.Now().UTC())
	if err != nil {
		return computerevent.PinResult{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.payloads == nil {
		p.payloads = map[string][]byte{}
	}
	p.payloads[digest] = append([]byte(nil), payload...)
	return computerevent.PinResult{ArtifactDigest: digest, Receipt: receipt}, nil
}

func (p *derivableTestPinner) fetch(digest string) ([]byte, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	payload, ok := p.payloads[digest]
	return payload, ok
}

// payloadChainSource serves the rebuild's payload fetches from the recorded
// pins; everything else is inherited verbatim from memChainSource.
type payloadChainSource struct {
	*memChainSource
	pinner *derivableTestPinner
}

func (s *payloadChainSource) FetchPayload(_ context.Context, _ string, artifactDigest string) ([]byte, error) {
	if payload, ok := s.pinner.fetch(artifactDigest); ok {
		return payload, nil
	}
	return nil, fmt.Errorf("test payload source: artifact %s not pinned", artifactDigest)
}

// collaborators; the receipt signer signs guest-core receipts for the updater
// exactly like cmd/choir-updater's signer proxy.
type derivableServiceManager struct{ restarts int }

func (m *derivableServiceManager) Restart(context.Context) error                   { m.restarts++; return nil }
func (m *derivableServiceManager) RecoveryRestart(context.Context) error           { m.restarts++; return nil }
func (m *derivableServiceManager) CleanupRecoveryCredential(context.Context) error { return nil }

type derivableHealthProber struct{}

func (derivableHealthProber) Probe(_ context.Context, digest string, _ updater.ReleaseManifest) ([]string, error) {
	return []string{strings.Repeat("f", 64)}, nil
}

type derivableReceiptSigner struct{ key computerevent.SigningKey }

func (s derivableReceiptSigner) PublicKey(context.Context) (computerevent.SignerRef, ed25519.PublicKey, error) {
	return s.key.SignerRef, s.key.PrivateKey.Public().(ed25519.PublicKey), nil
}

func (s derivableReceiptSigner) SignReceipt(_ context.Context, kind, issuer string, fields map[string]any, issuedAt time.Time) (computerevent.Receipt, error) {
	return computerevent.NewSignedReceipt(kind, issuer, fields, []computerevent.SigningKey{s.key}, issuedAt)
}

// derivableSelfDevFixture wires the production M7 seam end-to-end with real
// durable state and fake edge services only: updater + verifier on unix
// sockets (same contract as cmd/choir-updater and the signer daemon), an HTTP
// platform-control authority, and a vmctl route endpoint backed by a real
// route ledger.
type derivableSelfDevFixture struct {
	computerID  string
	store       *choirstore.Store
	cas         *derivableChainCAS
	pinner      *derivableTestPinner
	appender    *computerevent.ComputerEventAppender
	rt          *Runtime
	operations  *selfdev.Store
	ledger      *routeledger.MemoryLedger
	closures    map[string]computerversion.CodeClosure
	programs    map[string]computerversion.ArtifactProgram
	controlFail *atomic.Bool
	updaterRoot string
	engine      *updater.Updater
	applyCalls  atomic.Int32
	service     *derivableServiceManager
	platformKey ed25519.PrivateKey
	baseSource  *restoreTapeFake
	checkpoints map[string]selfdevprotocol.CheckpointResponse
	// goPatch, when set, makes writeFrozenBundle freeze a Go change: a
	// workspace/platform Go effect, source.patch, and its sha and base
	// commit (P5 host build tests).
	goPatch []byte
	// sourceBase, when set and goPatch is not, makes writeFrozenBundle freeze
	// a frontend-only candidate that still records its source patch and the
	// base commit it was authored against (the plain release's join input).
	sourceBase string
	// bootedManifest is the booted base image manifest path when the fixture
	// was built with newDerivableSelfDevFixtureBooted.
	bootedManifest string
	// platformMux and platformURL are the fake platform-control server, so
	// a test can mount real corpusd routes beside the fakes.
	platformMux *http.ServeMux
	platformURL string
	sockDir     string
	// trajectoryID, when set, is the lifecycle trajectory new operations
	// name (owner-act wake tests bind a Texture document to it).
	trajectoryID string
}

// shortSockDir makes a short-named temp dir because unix socket paths cap at
// ~104 bytes and t.TempDir() alone overflows that on macOS.
func shortSockDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "m7-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func newDerivableSelfDevFixture(t *testing.T, computerID string) *derivableSelfDevFixture {
	return newDerivableSelfDevFixtureBooted(t, computerID, "")
}

// newDerivableSelfDevFixtureBooted is the fixture on a guest whose booted base
// image manifest carries buildCommit as its build_commit line, wired into both
// the updater and the runtime exactly as production wires them
// (CHOIR_GUEST_IMAGE_MANIFEST). An empty buildCommit leaves the base unwired,
// as every older fixture is.
func newDerivableSelfDevFixtureBooted(t *testing.T, computerID, buildCommit string) *derivableSelfDevFixture {
	fx := &derivableSelfDevFixture{computerID: computerID, controlFail: &atomic.Bool{}, checkpoints: map[string]selfdevprotocol.CheckpointResponse{}}
	sockDir := shortSockDir(t)

	productStore, err := choirstore.Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = productStore.Close() })
	fx.store = productStore

	_, appKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signingKey := computerevent.SigningKey{
		SignerRef:  computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "derivable-test"},
		PrivateKey: appKey,
	}
	fx.pinner = &derivableTestPinner{key: signingKey}
	fx.cas = &derivableChainCAS{key: signingKey, projection: productStore}
	appender, err := computerevent.NewComputerEventAppender(computerID, fx.pinner, productStore, fx.cas, rollbackTestReceiptVerifier{})
	if err != nil {
		t.Fatal(err)
	}
	fx.appender = appender
	if err := productStore.BindProjectionTape(computerID, appender); err != nil {
		t.Fatal(err)
	}

	// Updater: real engine behind the same unix-socket HTTP contract as
	// cmd/choir-updater.
	updaterRoot, err := os.MkdirTemp("", "m7-updater-")
	if err != nil {
		t.Fatal(err)
	}
	// Frozen release dirs are mode 0555/0444 — the tree must be chmodded back
	// before RemoveAll or unlink fails on macOS.
	t.Cleanup(func() {
		_ = filepath.WalkDir(updaterRoot, func(p string, _ fs.DirEntry, walkErr error) error {
			if walkErr == nil {
				_ = os.Chmod(p, 0o700)
			}
			return nil
		})
		_ = os.RemoveAll(updaterRoot)
	})
	fx.updaterRoot = updaterRoot
	updaterSock := filepath.Join(sockDir, "updater.sock")
	_, updaterKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	fx.service = &derivableServiceManager{}
	guestSigner := derivableReceiptSigner{key: computerevent.SigningKey{
		SignerRef:  computerevent.SignerRef{SignerDomain: "guest-core", KeyID: "updater-test"},
		PrivateKey: updaterKey,
	}}
	var engine *updater.Updater
	if buildCommit == "" {
		engine, err = updater.New(updaterRoot, computerID, "realization-derivable", fx.service, derivableHealthProber{}, guestSigner)
	} else {
		fx.bootedManifest = filepath.Join(t.TempDir(), "guest-image-manifest")
		fx.setBootedBuildCommit(t, buildCommit)
		engine, err = updater.NewWithBase(updaterRoot, computerID, "realization-derivable", fx.service, derivableHealthProber{}, guestSigner, fx.bootedManifest)
	}
	if err != nil {
		t.Fatal(err)
	}
	fx.engine = engine
	updaterMux := http.NewServeMux()
	updaterMux.HandleFunc("/v1/public-key", func(w http.ResponseWriter, r *http.Request) {
		ref, publicKey, keyErr := guestSigner.PublicKey(r.Context())
		if keyErr != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"signer_domain": ref.SignerDomain, "key_id": ref.KeyID,
			"public_key": base64.RawStdEncoding.EncodeToString(publicKey),
		})
	})
	updaterMux.HandleFunc("/v1/apply", func(w http.ResponseWriter, r *http.Request) {
		fx.applyCalls.Add(1)
		var request updater.ApplyRequest
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20)).Decode(&request) != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		result, applyErr := engine.Apply(r.Context(), request)
		if applyErr != nil {
			if result.RecoveryReceipt != nil {
				w.WriteHeader(http.StatusConflict)
				_ = json.NewEncoder(w).Encode(struct {
					Result updater.ApplyResult `json:"result"`
					Error  string              `json:"error"`
				}{result, "materialization failed and prior release was restored"})
				return
			}
			// As cmd/choir-updater answers a pre-mutation refusal: the
			// stable discriminator and the computed gate message.
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "updater refused request", "reason": applyErr.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(result)
	})
	fx.serveUnixSocket(t, updaterSock, updaterMux)
	updaterClient, err := updater.NewClient(updaterSock)
	if err != nil {
		t.Fatal(err)
	}

	// Verifier: the real receipt-signer handler in verifier mode.
	verifierSock := filepath.Join(sockDir, "verifier.sock")
	_, verifierKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	verifierHandler, err := receiptsigner.NewHandler(receiptsigner.ModeVerifier, computerID, t.TempDir(), computerevent.SigningKey{
		SignerRef:  computerevent.SignerRef{SignerDomain: receiptsigner.ModeVerifier, KeyID: "verifier-test"},
		PrivateKey: verifierKey,
	})
	if err != nil {
		t.Fatal(err)
	}
	fx.serveUnixSocket(t, verifierSock, verifierHandler)
	verifierClient, err := receiptsigner.NewClient(verifierSock, receiptsigner.ModeVerifier)
	if err != nil {
		t.Fatal(err)
	}

	// Platform control: checkpoint + route-projection authority signed by one
	// platform-control key the guest credential verifies against.
	_, platformKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	fx.platformKey = platformKey
	authorityKey := computerevent.SigningKey{
		SignerRef:  computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "authority-test"},
		PrivateKey: platformKey,
	}
	platformMux := http.NewServeMux()
	platformMux.HandleFunc("/internal/computers/checkpoints", func(w http.ResponseWriter, r *http.Request) {
		if fx.controlFail.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if r.Method == http.MethodGet {
			key := r.URL.Query().Get("idempotency_key")
			response, ok := fx.checkpoints[key]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(response)
			return
		}
		raw, ok := readBody(r.Body)
		var request selfdevprotocol.CheckpointRequest
		if !ok || selfdevprotocol.DecodeStrict(raw, &request) != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if existing, ok := fx.checkpoints[request.IdempotencyKey]; ok {
			commitment, _ := selfdevprotocol.Digest(request)
			if commitment == existing.Receipt.RequestCommitment {
				w.WriteHeader(http.StatusCreated)
				_ = json.NewEncoder(w).Encode(existing)
				return
			}
			w.WriteHeader(http.StatusConflict)
			return
		}
		checkpoint, _, checkpointErr := selfdevprotocol.CheckpointFromRequest(request)
		if checkpointErr != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if joinErr := fx.verifierDecisionJoin(r.Context(), request); joinErr != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		requestCommitment, _ := selfdevprotocol.Digest(request)
		receipt, receiptErr := selfdevprotocol.NewAuthorityReceipt(selfdevprotocol.ReceiptKindCheckpoint,
			request.ComputerID, requestCommitment, checkpoint.Digest, "corpusd", authorityKey, time.Now().UTC())
		if receiptErr != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		response := selfdevprotocol.CheckpointResponse{Checkpoint: checkpoint, Receipt: receipt}
		fx.checkpoints[request.IdempotencyKey] = response
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(response)
	})
	platformMux.HandleFunc("/internal/computers/route-projection-certificates", func(w http.ResponseWriter, r *http.Request) {
		if fx.controlFail.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		raw, ok := readBody(r.Body)
		var request selfdevprotocol.RouteProjectionRequest
		if !ok || selfdevprotocol.DecodeStrict(raw, &request) != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		issuedAt := time.Now().UTC()
		certificate, artifact, certErr := selfdevprotocol.RouteProjectionFromRequest(request, issuedAt)
		if certErr != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		requestCommitment, _ := selfdevprotocol.Digest(request)
		receipt, receiptErr := selfdevprotocol.NewAuthorityReceipt(selfdevprotocol.ReceiptKindRouteProjection,
			request.ComputerID, requestCommitment, computerevent.DigestBytes(artifact), "corpusd", authorityKey, issuedAt)
		if receiptErr != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(selfdevprotocol.RouteProjectionResponse{Certificate: certificate, Receipt: receipt})
	})
	platformServer := httptest.NewServer(platformMux)
	t.Cleanup(platformServer.Close)
	fx.platformMux, fx.platformURL, fx.sockDir = platformMux, platformServer.URL, sockDir

	// vmctl: a real route ledger behind the same two endpoints.
	fx.ledger = routeledger.NewMemoryLedger()
	fx.closures = map[string]computerversion.CodeClosure{}
	fx.programs = map[string]computerversion.ArtifactProgram{}
	vmctlMux := http.NewServeMux()
	vmctlMux.HandleFunc("/internal/vmctl/computer-version-routes/resolve", func(w http.ResponseWriter, r *http.Request) {
		slotID := r.URL.Query().Get("route_slot_id")
		slot, receipt, err := fx.ledger.Resolve(r.Context(), slotID)
		if err != nil {
			_ = json.NewEncoder(w).Encode(vmctl.RouteResolution{RouteAbsent: true})
			return
		}
		_ = json.NewEncoder(w).Encode(vmctl.RouteResolution{
			Slot: slot, LatestReceipt: receipt,
			CodeClosure: fx.closures[string(slot.Current.CodeRef)], ArtifactProgram: fx.programs[string(slot.Current.ArtifactProgramRef)],
		})
	})
	// The immutable inputs of a version the route ledger has promoted: a
	// rollback resolves its target's closure and program from here.
	vmctlMux.HandleFunc("/internal/vmctl/computer-version-inputs/resolve", func(w http.ResponseWriter, r *http.Request) {
		var version computerversion.ComputerVersion
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&version) != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		closure, closureOK := fx.closures[string(version.CodeRef)]
		program, programOK := fx.programs[string(version.ArtifactProgramRef)]
		if !closureOK || !programOK {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(vmctl.ComputerVersionInputs{CodeClosure: closure, ArtifactProgram: program})
	})
	vmctlMux.HandleFunc("/internal/vmctl/computer-version-routes/apply-self-development", func(w http.ResponseWriter, r *http.Request) {
		raw, ok := readBody(r.Body)
		var request selfdevprotocol.ApplyRouteProjectionRequest
		if !ok || json.Unmarshal(raw, &request) != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		command := request.Projection.Command
		slot, receipt, err := fx.ledger.TransitionWithEvidence(r.Context(), command,
			[]routeledger.AuthorizationEvidence{request.Projection.ApprovalEvidence, request.Projection.PromotionEvidence})
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		fx.closures[string(slot.Current.CodeRef)] = request.Projection.CodeClosure
		fx.programs[string(slot.Current.ArtifactProgramRef)] = request.Projection.ArtifactProgram
		_ = json.NewEncoder(w).Encode(vmctl.RouteResolution{
			Slot: slot, LatestReceipt: receipt, TransitionReceipt: &receipt,
			CodeClosure: request.Projection.CodeClosure, ArtifactProgram: request.Projection.ArtifactProgram,
		})
	})
	vmctlMux.HandleFunc("/internal/vmctl/computer-version-routes/apply-platform-follow", func(w http.ResponseWriter, r *http.Request) {
		raw, ok := readBody(r.Body)
		var request selfdevprotocol.ApplyRouteProjectionRequest
		if !ok || json.Unmarshal(raw, &request) != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		// Mirror production scope gating: the platform-follow endpoint only
		// accepts the platform-follow actor/scope on promote commands.
		if request.Projection.DecisionScope != selfdevprotocol.PlatformUpdateFollowScope ||
			request.Projection.DecisionActor != selfdevprotocol.PlatformUpdateFollowActor ||
			(request.Projection.Command.Kind != routeledger.TransitionPromote &&
				request.Projection.Command.Kind != routeledger.TransitionBootstrap) {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "platform-follow promote scope is required"})
			return
		}
		command := request.Projection.Command
		slot, receipt, err := fx.ledger.TransitionWithEvidence(r.Context(), command,
			[]routeledger.AuthorizationEvidence{request.Projection.ApprovalEvidence, request.Projection.PromotionEvidence})
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		fx.closures[string(slot.Current.CodeRef)] = request.Projection.CodeClosure
		fx.programs[string(slot.Current.ArtifactProgramRef)] = request.Projection.ArtifactProgram
		_ = json.NewEncoder(w).Encode(vmctl.RouteResolution{
			Slot: slot, LatestReceipt: receipt, TransitionReceipt: &receipt,
			CodeClosure: request.Projection.CodeClosure, ArtifactProgram: request.Projection.ArtifactProgram,
		})
	})
	vmctlServer := httptest.NewServer(vmctlMux)
	t.Cleanup(vmctlServer.Close)

	// Runtime wired exactly as production M7: store + appender + selfdev
	// services + the post-commit observer driving the coalesced drain.
	rt := &Runtime{
		cfg:           provideriface.Config{ComputerID: computerID},
		store:         productStore,
		eventAppender: appender,
	}
	operations, err := selfdev.NewStore(productStore, productStore)
	if err != nil {
		t.Fatal(err)
	}
	operations.BindProjectionSink(productStore)
	rt.selfdevOperations = operations
	// The recovery tape is the live chain with no snapshot: every probe and
	// restore replays from genesis, so any historical head is reachable while
	// the chain continues forward.
	fx.baseSource = newRestoreTapeFake(fx.cas.snapshot)
	rt.restoreTape = fx.baseSource
	WithSelfDevelopmentUpdater(updaterClient, updaterRoot, computerID, "realization-derivable")(rt)
	rt.selfdevGuestImageManifest = fx.bootedManifest
	WithSelfDevelopmentVerifier(verifierClient)(rt)
	WithSelfDevelopmentControl(selfdev.GuestCredentialsWithCapability(platformServer.URL, computerID, "capability", time.Now().UTC().Add(time.Hour), platformKey.Public().(ed25519.PublicKey)))(rt)
	WithSelfDevelopmentRoute(vmctl.NewClient(vmctlServer.URL), "owner", "primary")(rt)
	WithEventPayloadReader(&payloadChainSource{pinner: fx.pinner})(rt)
	appender.SetPostCommitObserver(func(computerevent.EventKind) { rt.triggerSelfDevelopmentReconcile() })
	fx.rt = rt
	fx.operations = operations

	// Genesis + baseline route bootstrap: the slot exists at generation 1 on a
	// trivial baseline version before self-development promotes.
	fx.appendGenesis(t)
	fx.bootstrapRoute(t)
	return fx
}

func (fx *derivableSelfDevFixture) serveUnixSocket(t *testing.T, socketPath string, handler http.Handler) {
	t.Helper()
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: handler}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		_ = server.Close()
		_ = listener.Close()
		_ = os.Remove(socketPath)
	})
}

func readBody(body io.Reader) ([]byte, bool) {
	raw, err := io.ReadAll(body)
	if err != nil {
		return nil, false
	}
	return raw, true
}

func (fx *derivableSelfDevFixture) appendGenesis(t *testing.T) {
	t.Helper()
	genesisID, err := computerevent.NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	commitment := strings.Repeat("a", 64)
	genesis := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: genesisID, ComputerID: fx.computerID,
		EventKind: computerevent.EventGenesisImported, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
		IdempotencyKey: "genesis", ActorProfile: "management", AuthorityRef: "owner", PrivacyClass: "owner",
		PayloadCommitment: commitment, ProposedEffectRef: strings.Repeat("b", 64),
		ResultingEffectiveCommitment: commitment, ReducerVersion: computerevent.ReducerVersionV1,
	}
	if _, err := fx.appender.AppendNew(context.Background(), genesis, computerevent.TransitionInput{TargetStateCommitment: commitment}, nil); err != nil {
		t.Fatal(err)
	}
}

func (fx *derivableSelfDevFixture) bootstrapRoute(t *testing.T) {
	t.Helper()
	now := time.Now().UTC()
	slotID, err := routeledger.RouteSlotID("owner", "primary")
	if err != nil {
		t.Fatal(err)
	}
	digest := strings.Repeat("1", 64)
	closure, err := computerversion.NewCodeClosure(strings.Repeat("1", 40), []computerversion.CodeArtifact{{Name: "baseline", SHA256: digest, URI: "artifact+sha256://" + digest + "/baseline"}}, now)
	if err != nil {
		t.Fatal(err)
	}
	program, err := computerversion.NewArtifactProgram([]computerversion.ArtifactProgramEntry{{Kind: "baseline", ContentSHA256: digest, ArtifactURI: "artifact+sha256://" + digest + "/baseline"}}, now)
	if err != nil {
		t.Fatal(err)
	}
	baseline := computerversion.ComputerVersion{CodeRef: closure.Ref, ArtifactProgramRef: program.Ref}
	approval, err := routeledger.NewAuthorizationEvidence(routeledger.AuthorizationEvidenceApproval, slotID, baseline, json.RawMessage(`{"approval":true}`), now)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := routeledger.NewAuthorizationEvidence(routeledger.AuthorizationEvidencePromotionCertificate, slotID, baseline, json.RawMessage(`{"certificate":true}`), now)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = fx.ledger.TransitionWithEvidence(context.Background(), routeledger.TransitionCommand{
		RouteSlotID: slotID, Kind: routeledger.TransitionBootstrap, New: baseline,
		ExpectedGeneration: 0, ApprovalRef: routeledger.ApprovalRef(approval.Ref),
		PromotionCertificateRef: routeledger.PromotionCertificateRef(certificate.Ref),
		IdempotencyKey:          routeledger.IdempotencyKey("idempotency:baseline-bootstrap"),
	}, []routeledger.AuthorizationEvidence{approval, certificate})
	if err != nil {
		t.Fatal(err)
	}
	fx.closures[string(closure.Ref)] = closure
	fx.programs[string(program.Ref)] = program
}

// seedOperationToAwaitingApproval seeds one operation up to the owner-decision
// boundary and writes its frozen bundle into the updater incoming store. Every
// mutation rides the projection tape: each row lands as a canonical
// EventProjectionBatchRecorded event, so replay reproduces the seeded state —
// the same path production boots from.
func (fx *derivableSelfDevFixture) seedOperationToAwaitingApproval(t *testing.T, idempotencyKey string) (selfdev.Operation, string, string) {
	t.Helper()
	ctx := context.Background()
	operation, err := fx.operations.Start(ctx, selfdev.StartRequest{
		ComputerID: fx.computerID, IdempotencyKey: idempotencyKey, TrajectoryID: fx.trajectoryID,
		PromptArtifactRef: "artifact:sha256:" + strings.Repeat("c", 64),
	})
	if err != nil {
		t.Fatal(err)
	}
	bundleDigest := fx.writeFrozenBundle(t, operation)
	operation, err = fx.operations.Transition(ctx, fx.computerID, operation.OperationID, selfdev.StateRequested, selfdev.StateExecuting, func(next *selfdev.Operation) error {
		next.CapsuleID = "capsule-" + idempotencyKey
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	operation, err = fx.operations.Transition(ctx, fx.computerID, operation.OperationID, selfdev.StateExecuting, selfdev.StateFrozen, func(next *selfdev.Operation) error {
		next.BundleDigest = bundleDigest
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// The materializer reads the recorded verification payload for the
	// certificate join (wedge-10 fix): the event digest lands in
	// VerifierRefs while the payload carries the draft bundle digest and
	// cited evidence refs.
	verifierRunID := "run:verifier-" + idempotencyKey
	verificationPayload, err := computerevent.CanonicalJSON(map[string]any{
		"schema_version": 1, "operation_id": operation.OperationID, "bundle_digest": bundleDigest,
		"decision": "pass", "verifier_refs": []string{"capsule-go-eval:sha256:" + strings.Repeat("a", 64)},
		"verifier_run_id": verifierRunID,
	})
	if err != nil {
		t.Fatal(err)
	}
	verificationID, err := computerevent.NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	verificationEvent := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: verificationID, ComputerID: fx.computerID,
		EventKind: computerevent.EventVerificationRecorded, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
		IdempotencyKey: "test-verification-" + idempotencyKey,
		TrajectoryID:   operation.TrajectoryID, CapsuleID: operation.CapsuleID,
		ActorProfile: agentprofile.Engineering, AuthorityRef: "guest-core:self-development-verifier",
		PrivacyClass: "public", ReducerVersion: computerevent.ReducerVersionV1,
	}
	if _, _, err := fx.appender.AppendNewPayload(ctx, verificationEvent, computerevent.TransitionInput{}, verificationPayload, "application/vnd.choir.self-development-verification+json", "public"); err != nil {
		t.Fatal(err)
	}
	verificationEvent, found, err := fx.store.EventByIdempotency(ctx, fx.computerID, "test-verification-"+idempotencyKey)
	if err != nil || !found {
		t.Fatalf("verification event projection unavailable: %v", err)
	}
	verificationDigest, err := verificationEvent.Digest()
	if err != nil {
		t.Fatal(err)
	}
	// Production finalization rewrites the bundle to carry the verification
	// event digest in VerifierReceipts — the operation records the final
	// digest while the recorded verifier payload keeps the draft. Mirror the
	// same shape so the materializer's VerifierReceipts gate holds.
	rawFinal, err := os.ReadFile(filepath.Join(fx.updaterRoot, "incoming", bundleDigest, "bundle.json"))
	if err != nil {
		t.Fatal(err)
	}
	var finalBundle transaction.CapsuleEffectBundle
	if err := json.Unmarshal(rawFinal, &finalBundle); err != nil {
		t.Fatal(err)
	}
	finalBundle.VerifierReceipts = []string{verificationDigest}
	if err := finalBundle.Validate(true); err != nil {
		t.Fatal(err)
	}
	finalBytes, err := computerevent.CanonicalJSON(finalBundle)
	if err != nil {
		t.Fatal(err)
	}
	finalDigest := computerevent.DigestBytes(finalBytes)
	finalDir := filepath.Join(fx.updaterRoot, "incoming", finalDigest)
	if err := os.Rename(filepath.Join(fx.updaterRoot, "incoming", bundleDigest), finalDir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(finalDir, "bundle.json"), finalBytes, 0o400); err != nil {
		t.Fatal(err)
	}
	operation, err = fx.operations.Transition(ctx, fx.computerID, operation.OperationID, selfdev.StateFrozen, selfdev.StateVerified, func(next *selfdev.Operation) error {
		next.BundleDigest = finalDigest
		next.VerifierRefs = []string{verificationDigest}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	operation, err = fx.operations.Transition(ctx, fx.computerID, operation.OperationID, selfdev.StateVerified, selfdev.StateAwaitingApproval, nil)
	if err != nil {
		t.Fatal(err)
	}
	return operation, finalDigest, operation.TrajectoryID
}

// writeFrozenBundle stages the frozen capsule bundle the materializer reads.
// Release files must carry the computer-surface SPA under frontend/ —
// FrontendIdentityFromReleaseFiles refuses a release without it.
func (fx *derivableSelfDevFixture) writeFrozenBundle(t *testing.T, operation selfdev.Operation) string {
	t.Helper()
	content := []byte("<html>derivable materialization</html>\n")
	fileSHA := computerevent.DigestBytes(content)
	bundle := transaction.CapsuleEffectBundle{
		BundleVersion:           1,
		ComputerID:              fx.computerID,
		BaseEventHead:           operation.BaseHead,
		TrajectoryRef:           operation.TrajectoryID,
		CapsuleIdentity:         "capsule-" + operation.IdempotencyKey,
		CapabilityPolicyDigest:  strings.Repeat("2", 64),
		SourceTreeRef:           "source-tree:sha256:" + strings.Repeat("3", 64),
		OrderedFileEffects:      []transaction.ChangeRecord{{Path: "frontend/index.html", Kind: "added", Mode: 0o444}},
		GeneratedArtifactRefs:   []string{"artifact:sha256:" + strings.Repeat("4", 64)},
		BuildRecipeRef:          "capsule-exec:sha256:" + strings.Repeat("5", 64),
		RuntimeArtifactRef:      "runtime-artifact:sha256:" + strings.Repeat("6", 64),
		TestReceipts:            []string{"capsule-exec:sha256:" + strings.Repeat("7", 64)},
		VerifierReceipts:        []string{strings.Repeat("e", 64)},
		DependencyToolchainRefs: []string{"capsule-exec:sha256:" + strings.Repeat("8", 64)},
		ResourceReceipts:        []string{"artifact:sha256:" + strings.Repeat("9", 64)},
		ClassifierV:             "derivable-test",
		ClassifierDigest:        strings.Repeat("0", 64),
		Groups:                  map[string][]transaction.ChangeRecord{},
		RuntimeFiles:            []capsule.FrozenReleaseFile{{Path: "frontend/index.html", SHA256: fileSHA, Mode: 0o444}},
	}
	if fx.goPatch != nil {
		bundle.OrderedFileEffects = append(bundle.OrderedFileEffects, transaction.ChangeRecord{Path: "workspace/platform/internal/selfdevprobe/probe.go", Kind: "added", Mode: 0o644})
		bundle.SourcePatchSHA256 = computerevent.DigestBytes(fx.goPatch)
		bundle.SourcePatchBaseCommit = hostBuildTestBaseCommit
	} else if fx.sourceBase != "" {
		bundle.SourcePatchSHA256 = computerevent.DigestBytes([]byte(plainSourcePatch))
		bundle.SourcePatchBaseCommit = fx.sourceBase
	}
	contentDigest, err := bundle.ComputeContentDigest()
	if err != nil {
		t.Fatal(err)
	}
	bundle.ContentDigest = contentDigest
	raw, err := computerevent.CanonicalJSON(bundle)
	if err != nil {
		t.Fatal(err)
	}
	bundleDigest := computerevent.DigestBytes(raw)
	incoming := filepath.Join(fx.updaterRoot, "incoming", bundleDigest)
	if err := os.MkdirAll(filepath.Join(incoming, "frontend"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(incoming, "bundle.json"), raw, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(incoming, "frontend", "index.html"), content, 0o444); err != nil {
		t.Fatal(err)
	}
	if fx.goPatch != nil {
		if err := os.WriteFile(filepath.Join(incoming, "source.patch"), fx.goPatch, 0o444); err != nil {
			t.Fatal(err)
		}
	} else if fx.sourceBase != "" {
		if err := os.WriteFile(filepath.Join(incoming, "source.patch"), []byte(plainSourcePatch), 0o444); err != nil {
			t.Fatal(err)
		}
	}
	return bundleDigest
}

// waitForStableHead blocks until the canonical head stops moving across
// consecutive samples. Under the bound projection tape, drain-minted boundary
// commitment records advance the head asynchronously; a RequireExpectedHead
// decision append must start from the converged head or the CAS refuses it.
func (fx *derivableSelfDevFixture) waitForStableHead(t *testing.T) *computerevent.Head {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	stable := 0
	var last *computerevent.Head
	for {
		head, err := fx.store.Head(context.Background(), fx.computerID)
		if err != nil || head == nil {
			t.Fatalf("head read: %v %#v", err, head)
		}
		if last != nil && *last == *head {
			stable++
			if stable >= 4 {
				return head
			}
		} else {
			stable = 0
			last = head
		}
		if time.Now().After(deadline) {
			t.Fatalf("canonical head never stabilized; last=%+v", last)
		}
		time.Sleep(75 * time.Millisecond)
	}
}

func (fx *derivableSelfDevFixture) appendDecision(t *testing.T, operation selfdev.Operation, bundleDigest, idempotencyKey string) {
	t.Helper()
	ctx := context.Background()
	head := fx.waitForStableHead(t)
	decisionID, err := computerevent.NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	decision := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: decisionID, ComputerID: fx.computerID,
		EventKind: computerevent.EventEffectAccepted, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
		IdempotencyKey: idempotencyKey, RequestCommitment: computerevent.ZeroHead,
		TrajectoryID: operation.TrajectoryID, CapsuleID: operation.CapsuleID, PreviousHead: head.CanonicalEventHead,
		ParentEventID: operation.OperationID,
		ActorProfile:  "management", AuthorityRef: "external-owner:owner", PrivacyClass: "owner",
		ExpectedDesiredEventHead: head.DesiredEventHead, ExpectedEffectiveEventHead: head.EffectiveEventHead,
		ExpectedDesiredStateCommitment: head.DesiredStateCommitment, ExpectedEffectiveStateCommitment: head.EffectiveStateCommitment,
		RequireExpectedHead: true, PayloadCommitment: computerevent.ZeroHead, ProposedEffectRef: bundleDigest,
		DecisionRef: strings.Repeat("d", 64), VerifierRefs: append([]string(nil), operation.VerifierRefs...), ReducerVersion: computerevent.ReducerVersionV1,
		InputArtifactRefs: []string{"artifact:sha256:" + strings.Repeat("f", 64)},
	}
	target, err := computerevent.CanonicalJSON(map[string]string{"base_head": operation.BaseHead, "bundle_digest": bundleDigest})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fx.appender.AppendNew(ctx, decision, computerevent.TransitionInput{TargetStateCommitment: computerevent.DigestBytes(target)}, nil); err != nil {
		t.Fatal(err)
	}
}

func (fx *derivableSelfDevFixture) pollForState(t *testing.T, operationID string, states ...string) (selfdev.Operation, bool) {
	t.Helper()
	deadline := time.Now().Add(90 * time.Second)
	for {
		operation, err := fx.operations.Get(context.Background(), fx.computerID, operationID)
		if err == nil {
			for _, state := range states {
				if operation.State == state {
					return operation, true
				}
			}
		}
		if time.Now().After(deadline) {
			operation, _ := fx.operations.Get(context.Background(), fx.computerID, operationID)
			return operation, false
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func (fx *derivableSelfDevFixture) waitForState(t *testing.T, operationID string, states ...string) selfdev.Operation {
	t.Helper()
	operation, ok := fx.pollForState(t, operationID, states...)
	if !ok {
		// Drain errors are swallowed by design; rerun the step on the parked
		// operation to surface the real failure.
		var stepErr error
		if operation.State == selfdev.StateAccepted || operation.State == selfdev.StateMaterializing {
			stepErr = fx.rt.materializeSelfDevelopmentOperation(context.Background(), operation, false)
		} else if operation.State == selfdev.StateRollbackPending {
			stepErr = fx.rt.rollbackSelfDevelopmentOperation(context.Background(), operation, false)
		} else if operation.State == selfdev.StateAwaitingApproval {
			_, _, stepErr = fx.rt.recoverSelfDevelopmentDecision(context.Background(), operation)
		}
		t.Fatalf("operation %s state=%s, want one of %v (step error: %v)", operationID, operation.State, states, stepErr)
	}
	return operation
}
func (fx *derivableSelfDevFixture) countEventKinds() map[computerevent.EventKind]int {
	got := map[computerevent.EventKind]int{}
	for _, record := range fx.cas.snapshot() {
		got[record.Request.Event.EventKind]++
	}
	return got
}

// The derivable advance: a decision commit fires the observer; the coalesced
// drain recovers the operation to Accepted and materializes it — event
// started/applied, checkpoint, route projection — with the API caller absent
// for the entire advance.
func TestSelfDevReconcileMaterializesDerivably(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-m7-materialize")
	operation, bundleDigest, _ := fx.seedOperationToAwaitingApproval(t, "op-materialize")
	fx.appendDecision(t, operation, bundleDigest, "decision-materialize")

	applied := fx.waitForState(t, operation.OperationID, selfdev.StateApplied)

	storedDecision, found, err := fx.store.EventByIdempotency(context.Background(), fx.computerID, "decision-materialize")
	if err != nil || !found {
		t.Fatalf("stored decision unavailable: found=%v err=%v", found, err)
	}
	decisionDigest, _ := storedDecision.Digest()
	if applied.DecisionEvent != decisionDigest || applied.DecisionActor != "owner" || applied.DecisionReceipt == "" {
		t.Fatalf("applied operation decision binding = %+v", applied)
	}
	if applied.MaterializationReceipt == "" || !strings.HasPrefix(applied.CheckpointRef, "checkpoint:sha256:") ||
		applied.RouteReceipt == "" || applied.RouteGeneration == nil || applied.ReleaseDigest == "" || applied.CodeRef == "" {
		t.Fatalf("applied operation missing materialization receipts: %+v", applied)
	}

	kinds := fx.countEventKinds()
	for _, want := range []computerevent.EventKind{
		computerevent.EventMaterializationStarted, computerevent.EventMaterializationApplied,
		computerevent.EventCheckpointPublished, computerevent.EventRouteProjectionUpdated,
	} {
		if kinds[want] != 1 {
			t.Fatalf("event kind %s count=%d, want exactly 1 (full chain: %v)", want, kinds[want], kinds)
		}
	}

	// The route promoted to the applied version, not the baseline.
	slotID, _ := routeledger.RouteSlotID("owner", "primary")
	slot, latest, err := fx.ledger.Resolve(context.Background(), slotID)
	if err != nil {
		t.Fatal(err)
	}
	if !slot.Current.Valid() || !routeledger.SameVersion(slot.Current, latest.New) || slot.Current.CodeRef != computerversion.CodeRef(applied.CodeRef) {
		t.Fatalf("route slot %+v does not bind applied version %+v", slot, applied)
	}
	if fx.service.restarts != 1 {
		t.Fatalf("updater restarts=%d, want exactly 1", fx.service.restarts)
	}

	// A second pass over the same durable state is a no-op: no duplicate
	// events, no second materialization.
	before := fx.countEventKinds()
	fx.rt.reconcileSelfDevelopmentMaterialization(context.Background())
	after := fx.countEventKinds()
	for kind, count := range before {
		if after[kind] != count {
			t.Fatalf("second reconcile duplicated %s: %d -> %d (all: %v)", kind, count, after[kind], after)
		}
	}
	still, err := fx.operations.Get(context.Background(), fx.computerID, operation.OperationID)
	if err != nil || still.State != selfdev.StateApplied || still.RouteReceipt != applied.RouteReceipt {
		t.Fatalf("operation changed under idempotent reconcile: %+v", still)
	}
}

// Mid-advance crash leg: the operation reaches Materializing and the platform
// control surface dies before the checkpoint publishes. The op stays at its
// durable recovery point; re-running the same reconciler — exactly what the
// boot phase does — completes the apply idempotently and lands Applied.
func TestSelfDevReconcileRecoversMaterializingOperation(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-m7-crash")
	fx.controlFail.Store(true)
	operation, bundleDigest, _ := fx.seedOperationToAwaitingApproval(t, "op-crash")
	fx.appendDecision(t, operation, bundleDigest, "decision-crash")

	parked := fx.waitForState(t, operation.OperationID, selfdev.StateMaterializing)
	if parked.DecisionEvent == "" {
		t.Fatalf("materializing operation lost its decision binding: %+v", parked)
	}

	// Recovery: control restored; the same reconciler path the boot phase
	// drives completes the apply — updater journal replays the identical
	// receipts, events are idempotent, no API call intervenes.
	fx.controlFail.Store(false)
	fx.rt.reconcileSelfDevelopmentMaterialization(context.Background())

	applied := fx.waitForState(t, operation.OperationID, selfdev.StateApplied)
	if applied.DecisionEvent != parked.DecisionEvent || applied.DecisionActor != "owner" {
		t.Fatalf("recovered operation decision binding changed: %+v", applied)
	}
	if applied.MaterializationReceipt == "" || !strings.HasPrefix(applied.CheckpointRef, "checkpoint:sha256:") ||
		applied.RouteReceipt == "" || applied.RouteGeneration == nil {
		t.Fatalf("recovered operation missing receipts: %+v", applied)
	}
	kinds := fx.countEventKinds()
	for _, want := range []computerevent.EventKind{
		computerevent.EventMaterializationStarted, computerevent.EventMaterializationApplied,
		computerevent.EventCheckpointPublished, computerevent.EventRouteProjectionUpdated,
	} {
		if kinds[want] != 1 {
			t.Fatalf("event kind %s count=%d, want exactly 1 after crash+recovery (%v)", want, kinds[want], kinds)
		}
	}
}

// verifierDecisionJoin is the production checkpoint authority's join of the
// verifier certificate to the recorded verification payload
// (internal/platform/checkpoints.go, verifyVerifierEvidence): the payload the
// certificate's verification event points at must name the certificate's
// operation and bundle. The fake authority enforces it so a materialization
// that signs a certificate for the wrong operation is refused here as it is
// there.
func (fx *derivableSelfDevFixture) verifierDecisionJoin(ctx context.Context, request selfdevprotocol.CheckpointRequest) error {
	certificate := request.VerifierCertificate.Request
	event, found, err := fx.store.EventByDigest(ctx, request.ComputerID, certificate.VerificationEventDigest)
	if err != nil || !found || event.EventKind != computerevent.EventVerificationRecorded || len(event.OutputArtifactRefs) != 1 {
		return nil // a genesis baseline or other certificate: not this join
	}
	ref, err := computerevent.ParseArtifactRef(event.OutputArtifactRefs[0])
	if err != nil {
		return err
	}
	raw, err := fx.rt.eventPayloadReader.FetchPayload(ctx, request.ComputerID, ref.Digest().String())
	if err != nil {
		return err
	}
	var payload struct {
		OperationID  string `json:"operation_id"`
		BundleDigest string `json:"bundle_digest"`
	}
	if json.Unmarshal(raw, &payload) != nil || payload.OperationID != certificate.OperationID || payload.BundleDigest != certificate.BundleDigest {
		return fmt.Errorf("checkpoint authority: verifier decision mismatch")
	}
	return nil
}
