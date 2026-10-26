package agentcore

// P4 end to end, in one process
// (docs/problems/p4-standing-consensus-autonomy-2026-10-15.md, "Tests").
//
// The guest runtime is wired as production M7 is (derivable fixture: store,
// appender, real updater engine on its unix socket, verifier, route ledger).
// corpusd is real where every decision of P4 is made: the mode authority
// with S1 owner acts in enforce, the event CAS with its admission gate (every
// guest append passes through it), the decision poster, and P5's host build
// service behind its corpusd routes (job records, admission, the
// platform-update blob store, platform-control signing). Only the nix build
// is a stand-in: it checks the patch applies at the base with real git and
// emits a layered closure. The owner arms the standing mode once; two
// candidates are frozen, verified by management's real Verify path, built by
// the host, decided by corpusd on that build and applied as it, with no
// further owner act.

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	embedded "github.com/dolthub/driver/v2"
	"github.com/yusefmosiah/go-choir/internal/builder"
	"github.com/yusefmosiah/go-choir/internal/capsule"
	transaction "github.com/yusefmosiah/go-choir/internal/capsule/transaction"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/decisionpolicy"
	"github.com/yusefmosiah/go-choir/internal/hostbuild"
	"github.com/yusefmosiah/go-choir/internal/owneract"
	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/receiptsigner"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/updater"
)

type allowAllPins struct{}

func (allowAllPins) ValidateEventPins(context.Context, computerevent.CASRequest) error { return nil }

// guestCapability accepts the fixture guest's bearer for its own computer
// only, as corpusd's signed capability verifier does for a real guest.
type guestCapability struct{ computerID string }

func (g guestCapability) Authorize(r *http.Request, computerID, _ string) error {
	if computerID != g.computerID || r.Header.Get("Authorization") != "Bearer capability" {
		return fmt.Errorf("capability refused")
	}
	return nil
}
func (g guestCapability) AuthorizeRenewal(r *http.Request, computerID string) error {
	return g.Authorize(r, computerID, "")
}

type staticKey struct{ key ed25519.PublicKey }

func (s staticKey) ResolveReceiptKey(string, string, string, uint64, time.Time) (ed25519.PublicKey, error) {
	return s.key, nil
}

func openE2ECorpusdStore(t *testing.T) (*platform.Store, string) {
	t.Helper()
	root := t.TempDir()
	parse := func(dsn string) embedded.Config {
		cfg, err := embedded.ParseDSN(dsn)
		if err != nil {
			t.Fatal(err)
		}
		return cfg
	}
	rootConnector, err := embedded.NewConnector(parse(fmt.Sprintf("file://%s?commitname=Choir&commitemail=system@choir.local&multistatements=true", root)))
	if err != nil {
		t.Fatal(err)
	}
	rootDB := sql.OpenDB(rootConnector)
	if _, err := rootDB.Exec("CREATE DATABASE IF NOT EXISTS platform"); err != nil {
		t.Fatal(err)
	}
	_ = rootDB.Close()
	_ = rootConnector.Close()
	connector, err := embedded.NewConnector(parse(fmt.Sprintf("file://%s?commitname=Choir&commitemail=system@choir.local&database=platform&multistatements=true&clientfoundrows=true", root)))
	if err != nil {
		t.Fatal(err)
	}
	store := platform.NewStore(sql.OpenDB(connector))
	if err := store.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close(); _ = connector.Close() })
	return store, root
}

func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// platformHistory is the host's copy of the platform: one base commit.
type platformHistory struct{ dir, base string }

func newPlatformHistory(t *testing.T) platformHistory {
	t.Helper()
	dir := t.TempDir()
	gitRun(t, dir, "init", "-q")
	if err := os.MkdirAll(filepath.Join(dir, "frontend"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "frontend", "index.html"), []byte("<html>base</html>\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-q", "-m", "base")
	return platformHistory{dir: dir, base: gitRun(t, dir, "rev-parse", "HEAD")}
}

// patchFor is a source patch against the base, as Freeze's emitSourcePatch
// writes it: one new file under frontend/public.
func (h platformHistory) patchFor(t *testing.T, name, content string) []byte {
	t.Helper()
	path := filepath.Join(h.dir, "frontend", "public", name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, h.dir, "add", "-A")
	patch := gitRun(t, h.dir, "diff", "--cached", "--binary") + "\n"
	gitRun(t, h.dir, "reset", "-q", "--hard", h.base)
	return []byte(patch)
}

// stubNixBuild stands in for builder.Build on the host: the patch must
// apply at the base on the host's platform history (real git), and the
// result is a layered closure with the base the computer booted.
func (h platformHistory) stubNixBuild(t *testing.T) hostbuild.BuildFunc {
	return func(ctx context.Context, req builder.Request) (*builder.ClosureResult, error) {
		check := exec.CommandContext(ctx, "git", "apply", "--check", req.SourcePatch)
		check.Dir = h.dir
		if out, err := check.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("builder: apply source patch: %s", strings.TrimSpace(string(out)))
		}
		patch, err := os.ReadFile(req.SourcePatch)
		if err != nil {
			return nil, err
		}
		base, err := updater.DigestFile(req.BaseManifestPath)
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(req.OutDir, 0o700); err != nil {
			return nil, err
		}
		nar := hostBuildTestClosure()
		exported := filepath.Join(req.OutDir, "closure.nar")
		if err := os.WriteFile(exported, nar, 0o444); err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(req.OutDir, "builder-receipt.json"), []byte(`{"code_commit":"`+hostBuildTestCodeCommit+`"}`), 0o444); err != nil {
			return nil, err
		}
		return &builder.ClosureResult{
			Base:        builder.BaseIdentity{GuestImageManifestDigest: base, StoreDiskSHA256: strings.Repeat("5", 64)},
			RuntimePath: "/nix/store/" + hostBuildTestLayer, ExportedDigest: computerevent.DigestBytes(nar), ExportedPath: exported,
			DerivationPath: "/nix/store/cccccccccccccccccccccccccccccccc-autoputer.drv", CodeCommit: hostBuildTestCodeCommit,
			CodeCommitSource: "derived-patch", SourcePatchSHA256: computerevent.DigestBytes(patch), SourcePatchBaseCommit: req.SourcePatchBaseCommit,
			BuiltAt: "2026-10-15T00:00:00Z",
		}, nil
	}
}

type p4Fixture struct {
	*derivableSelfDevFixture
	handler     *platform.Handler
	modes       *platform.SelfDevelopmentModeCAS
	ownerSigner *owneract.Signer
	history     platformHistory
	modeKeyID   string
	hostBuilds  *hostbuild.Service
}

func newP4Fixture(t *testing.T) *p4Fixture {
	t.Helper()
	fx := newDerivableSelfDevFixture(t, "computer-p4-standing")
	store, root := openE2ECorpusdStore(t)
	service := platform.NewService(store, filepath.Join(root, "artifacts"), filepath.Join(root, "platform-signing.key"))
	const keyID = "p4-platform-control"
	platformKey := computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "platform-control", KeyID: keyID}, PrivateKey: fx.platformKey}

	// corpusd: S1 owner acts in enforce, the mode authority, the event CAS
	// with its admission gate, the decision poster and the host builder.
	_, ownerPrivate, _ := ed25519.GenerateKey(rand.Reader)
	ownerSigner, err := owneract.NewSigner(ownerPrivate)
	if err != nil {
		t.Fatal(err)
	}
	ownerVerifier, err := owneract.NewVerifier(ownerSigner.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	authority := platform.NewOwnerActAuthority(platform.OwnerActEnforce, ownerVerifier)
	modes, err := platform.NewSelfDevelopmentModeCAS(store, platformKey)
	if err != nil {
		t.Fatal(err)
	}
	modes.ConfigureOwnerActs(authority)
	cas, err := platform.NewComputerEventCAS(store, "corpusd", platformKey, allowAllPins{})
	if err != nil {
		t.Fatal(err)
	}
	cas.ConfigureEffectAdmission(authority)
	history := newPlatformHistory(t)
	artifacts, err := platform.NewEventArtifactService(service, staticKey{key: fx.platformKey.Public().(ed25519.PublicKey)})
	if err != nil {
		t.Fatal(err)
	}
	handler := platform.NewHandler(service)
	if err := handler.ConfigureComputerEvents(cas, artifacts, guestCapability{computerID: fx.computerID}); err != nil {
		t.Fatal(err)
	}
	if err := handler.ConfigureSelfDevelopmentModes(modes); err != nil {
		t.Fatal(err)
	}
	// P5's host build service, as corpusd configures it, on the computer's
	// booted base (the manifest names the history's base commit).
	manifest := filepath.Join(t.TempDir(), "guest-image-manifest")
	if err := os.WriteFile(manifest, []byte("contract=choir-guest-image-v1\nbuild_commit="+history.base+"\n"), 0o444); err != nil {
		t.Fatal(err)
	}
	storeDisk := filepath.Join(t.TempDir(), "storedisk.erofs")
	if err := os.WriteFile(storeDisk, []byte("store"), 0o444); err != nil {
		t.Fatal(err)
	}
	hostBuilds, err := handler.ConfigureHostBuild(hostbuild.Config{
		StateDir: t.TempDir(), SourceDir: history.dir, BaseManifestPath: manifest, BaseStoreDiskPath: storeDisk,
		AdmitPoll: 20 * time.Millisecond, Build: history.stubNixBuild(t),
		Sample: func() (hostbuild.Pressure, error) { return hostbuild.Pressure{MemAvailableBytes: 1 << 40}, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	workerCtx, stopWorker := context.WithCancel(context.Background())
	hostBuilds.Start(workerCtx)
	t.Cleanup(func() { stopWorker(); hostBuilds.Wait() })
	actuator, err := platform.NewConsensusActuator(store, modes, platformKey, hostBuilds)
	if err != nil {
		t.Fatal(err)
	}
	if err := handler.ConfigureConsensusActuator(actuator); err != nil {
		t.Fatal(err)
	}
	fx.platformMux.HandleFunc("/internal/computers/self-development/mode", handler.HandleSelfDevelopmentMode)
	fx.platformMux.HandleFunc("/internal/computers/self-development/consensus", handler.HandleSelfDevelopmentConsensus)
	fx.platformMux.HandleFunc(hostbuild.Route, handler.HandleHostBuilds)
	fx.platformMux.HandleFunc(hostbuild.Route+"/", handler.HandleHostBuilds)
	fx.platformMux.HandleFunc("/internal/computers/platform-updates/blob/", handler.HandlePlatformUpdateBlob)
	// Every guest append from here on passes corpusd's CAS and its gate.
	if err := fx.cas.installGate(context.Background(), func(ctx context.Context, request computerevent.CASRequest) error {
		_, err := cas.CompareAndSwap(ctx, request)
		return err
	}); err != nil {
		t.Fatalf("replay the chain into corpusd: %v", err)
	}

	// The guest: platform-control verification of signed modes and
	// decisions, and the real guest-core signer for the reviewer ballot.
	fx.rt.selfdevControl = selfdev.GuestCredentialsWithPlatformKey(fx.platformURL, fx.computerID, "capability", time.Now().UTC().Add(time.Hour), keyID, fx.platformKey.Public().(ed25519.PublicKey))
	_, guestCorePrivate, _ := ed25519.GenerateKey(rand.Reader)
	guestCore, err := receiptsigner.NewHandler(receiptsigner.ModeGuestCore, fx.computerID, t.TempDir(), computerevent.SigningKey{
		SignerRef: computerevent.SignerRef{SignerDomain: receiptsigner.ModeGuestCore, KeyID: "guest-core-p4"}, PrivateKey: guestCorePrivate,
	})
	if err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(fx.sockDir, "guest-core.sock")
	fx.serveUnixSocket(t, socket, guestCore)
	ballots, err := receiptsigner.NewClient(socket, receiptsigner.ModeGuestCore)
	if err != nil {
		t.Fatal(err)
	}
	fx.rt.selfdevBallotSigner = ballots
	// The guest reaches the host build routes and the blob store over its
	// own corpusd client, as production derives it.
	client, err := computerevent.NewHTTPClient(fx.platformURL, http.DefaultClient, func(context.Context) (string, error) { return "capability", nil }, true)
	if err != nil {
		t.Fatal(err)
	}
	fx.rt.selfdevHostBuilder = corpusdHostBuilder{client: client}
	fx.rt.selfdevGuestImageManifest = manifest
	fx.rt.selfdevStandingPoll = 20 * time.Millisecond
	return &p4Fixture{derivableSelfDevFixture: fx, handler: handler, modes: modes, ownerSigner: ownerSigner, history: history, modeKeyID: keyID, hostBuilds: hostBuilds}
}

// ownerWrite is the owner's mode write as the proxy forwards it: a
// host-internal call carrying the proxy's owner-act receipt.
func (f *p4Fixture) ownerWrite(t *testing.T, request platform.SetSelfDevelopmentModeRequest, tier string) (int, platform.SelfDevelopmentMode, string) {
	t.Helper()
	subject, err := platform.OwnerActModeSubjectWithTier(f.computerID, request, tier)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := f.ownerSigner.Mint(owneract.Act{
		Kind: platform.OwnerActKindForMode(request.Mode), OwnerUserID: "owner", AuthMethod: owneract.AuthMethodAPIKey,
		CredentialRef: "api-key:p4", ComputerID: f.computerID, Subject: subject,
	}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	header, err := owneract.Encode(receipt)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(request)
	r := httptest.NewRequest(http.MethodPost, "/internal/computers/self-development/mode?computer_id="+f.computerID, bytes.NewReader(body))
	r.RemoteAddr = "127.0.0.1:50000"
	r.Header.Set("X-Internal-Caller", "true")
	r.Header.Set(owneract.Header, header)
	w := httptest.NewRecorder()
	f.handler.HandleSelfDevelopmentMode(w, r)
	var mode platform.SelfDevelopmentMode
	if w.Code == http.StatusOK {
		if err := json.Unmarshal(w.Body.Bytes(), &mode); err != nil {
			t.Fatal(err)
		}
	}
	return w.Code, mode, w.Body.String()
}

func (f *p4Fixture) currentMode(t *testing.T) platform.SelfDevelopmentMode {
	t.Helper()
	mode, err := f.modes.Get(context.Background(), f.computerID)
	if err != nil {
		t.Fatal(err)
	}
	return mode
}

func (f *p4Fixture) armStanding(t *testing.T, expires time.Time) platform.SelfDevelopmentMode {
	t.Helper()
	request := platform.SetSelfDevelopmentModeRequest{
		Mode: platform.SelfDevelopmentModeStandingConsensus, ExpectedGeneration: f.currentMode(t).Generation,
		IdempotencyKey: "p4-standing-" + expires.Format("150405.000000"), PolicyDigest: decisionpolicy.PolicyDigestReversibleSelfDevV1,
		ExpiresAt: expires.UTC().Truncate(time.Microsecond).Format(time.RFC3339Nano),
	}
	code, mode, body := f.ownerWrite(t, request, platform.ComputerTierTest)
	if code != http.StatusOK {
		t.Fatalf("owner standing arm: %d %s", code, body)
	}
	return mode
}

// freeze writes a candidate the way Freeze does (a draft bundle naming its
// source patch, beside the release tree) and moves the operation to frozen.
func (f *p4Fixture) freeze(t *testing.T, key, page string) selfdev.Operation {
	t.Helper()
	return f.freezeWithPatch(t, key, page, f.history.patchFor(t, key+".txt", page))
}

func (f *p4Fixture) freezeWithPatch(t *testing.T, key, page string, patch []byte) selfdev.Operation {
	t.Helper()
	ctx := context.Background()
	operation, err := f.operations.Start(ctx, selfdev.StartRequest{
		ComputerID: f.computerID, IdempotencyKey: key, TrajectoryID: f.trajectoryID, PromptArtifactRef: "artifact:sha256:" + strings.Repeat("c", 64),
	})
	if err != nil {
		t.Fatal(err)
	}
	patchSum := sha256.Sum256(patch)
	content := []byte("<html>" + page + "</html>\n")
	bundle := transaction.CapsuleEffectBundle{
		BundleVersion: 1, ComputerID: f.computerID, BaseEventHead: operation.BaseHead, TrajectoryRef: operation.TrajectoryID,
		CapsuleIdentity: "capsule-" + key, CapabilityPolicyDigest: strings.Repeat("2", 64),
		SourceTreeRef:         "source-tree:sha256:" + strings.Repeat("3", 64),
		OrderedFileEffects:    []transaction.ChangeRecord{{Path: "frontend/index.html", Kind: "added", Mode: 0o444}},
		GeneratedArtifactRefs: []string{"artifact:sha256:" + strings.Repeat("4", 64)}, BuildRecipeRef: "capsule-exec:sha256:" + strings.Repeat("5", 64),
		RuntimeArtifactRef: "runtime-artifact:sha256:" + strings.Repeat("6", 64), TestReceipts: []string{"capsule-exec:sha256:" + strings.Repeat("7", 64)},
		VerifierReceipts: []string{}, DependencyToolchainRefs: []string{"capsule-exec:sha256:" + strings.Repeat("8", 64)},
		ResourceReceipts: []string{"artifact:sha256:" + strings.Repeat("9", 64)}, ClassifierV: "p4-test", ClassifierDigest: strings.Repeat("0", 64),
		Groups:            map[string][]transaction.ChangeRecord{},
		RuntimeFiles:      []capsule.FrozenReleaseFile{{Path: "frontend/index.html", SHA256: computerevent.DigestBytes(content), Mode: 0o444}},
		SourcePatchSHA256: hex.EncodeToString(patchSum[:]), SourcePatchBaseCommit: f.history.base,
	}
	bundle.ContentDigest, err = bundle.ComputeContentDigest()
	if err != nil {
		t.Fatal(err)
	}
	draft, err := computerevent.CanonicalJSON(bundle)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(f.updaterRoot, "incoming", bundle.ContentDigest)
	if err := os.MkdirAll(filepath.Join(root, "frontend"), 0o700); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{"bundle.draft.json": draft, "source.patch": patch, filepath.Join("frontend", "index.html"): content} {
		if err := os.WriteFile(filepath.Join(root, name), data, 0o444); err != nil {
			t.Fatal(err)
		}
	}
	if operation, err = f.operations.Transition(ctx, f.computerID, operation.OperationID, selfdev.StateRequested, selfdev.StateExecuting, func(next *selfdev.Operation) error {
		next.CapsuleID = "capsule-" + key
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if operation, err = f.operations.Transition(ctx, f.computerID, operation.OperationID, selfdev.StateExecuting, selfdev.StateFrozen, func(next *selfdev.Operation) error {
		next.BundleDigest = bundle.ContentDigest
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return operation
}

// verify is management's Verify: the real verdict path, which records the
// verification, finalizes the bundle and reaches awaiting_approval.
func (f *p4Fixture) verify(t *testing.T, operation selfdev.Operation) selfdev.Operation {
	t.Helper()
	toolCtx := &CapsuleToolCtx{AgentRunID: "run:management-p4", ComputerID: f.computerID, OwnerID: "owner",
		EventAppender: f.appender, OperationStore: f.operations, UpdaterRoot: f.updaterRoot, EventProjection: f.store}
	out, err := recordSelfDevelopmentVerdict(context.Background(), toolCtx, operation.TrajectoryID, operation.OperationID, operation.BundleDigest, "pass",
		[]string{"capsule-go-eval:sha256:" + strings.Repeat("a", 64)}, map[string]any{"verifier_desk": "management"})
	if err != nil {
		t.Fatalf("management Verify: %v", err)
	}
	if out["state"] != selfdev.StateAwaitingApproval {
		t.Fatalf("Verify left the operation at %v", out["state"])
	}
	verified, err := f.operations.Get(context.Background(), f.computerID, operation.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	return verified
}

func (f *p4Fixture) decisionEvent(t *testing.T, operationID string) computerevent.Event {
	t.Helper()
	for _, record := range f.cas.snapshot() {
		event := record.Request.Event
		if event.ParentEventID == operationID && (event.EventKind == computerevent.EventEffectAccepted || event.EventKind == computerevent.EventEffectRejected) {
			return event
		}
	}
	t.Fatalf("no decision event for %s", operationID)
	return computerevent.Event{}
}

// hostBuildOf is the done host build of an operation, read from corpusd's
// service by the identity the guest and the poster both derive.
func (f *p4Fixture) hostBuildOf(t *testing.T, operation selfdev.Operation) hostbuild.Job {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(f.updaterRoot, "incoming", operation.BundleDigest, "bundle.json"))
	if err != nil {
		t.Fatal(err)
	}
	var bundle transaction.CapsuleEffectBundle
	if err := json.Unmarshal(raw, &bundle); err != nil {
		t.Fatal(err)
	}
	request, err := f.rt.selfDevelopmentHostBuildRequest(operation, bundle)
	if err != nil {
		t.Fatal(err)
	}
	jobID, _ := request.JobID()
	job, ok := f.hostBuilds.Get(f.computerID, jobID)
	if !ok {
		t.Fatalf("no host build %s for %s", jobID, operation.OperationID)
	}
	return job
}

func TestStandingConsensusEndToEndFreezeVerifyAcceptApplyWithoutPerCandidateOwnerAct(t *testing.T) {
	f := newP4Fixture(t)
	ctx := context.Background()

	// Not autonomous before the owner arms: the driver does nothing.
	first := f.verify(t, f.freeze(t, "p4-first", "first autonomous change"))
	if op, err := f.rt.runStandingConsensus(ctx, first.OperationID, "run:management-p4"); err != nil || op.OperationID != "" {
		t.Fatalf("driver acted without a standing arm: %+v %v", op, err)
	}
	// A non-test computer cannot be armed (the tier the proxy signs).
	refusedArm := platform.SetSelfDevelopmentModeRequest{Mode: platform.SelfDevelopmentModeStandingConsensus, ExpectedGeneration: 0,
		IdempotencyKey: "p4-primary", PolicyDigest: decisionpolicy.PolicyDigestReversibleSelfDevV1,
		ExpiresAt: time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond).Format(time.RFC3339Nano)}
	if code, _, body := f.ownerWrite(t, refusedArm, "primary"); code != http.StatusForbidden {
		t.Fatalf("primary-tier arm: %d %s", code, body)
	}

	// The one owner act.
	f.armStanding(t, time.Now().Add(2*time.Hour))
	if _, err := f.rt.runStandingConsensus(ctx, first.OperationID, "run:management-p4"); err != nil {
		t.Fatalf("standing consensus: %v", err)
	}
	applied := f.waitForState(t, first.OperationID, selfdev.StateApplied)
	decision := f.decisionEvent(t, first.OperationID)
	digest, ok := decisionpolicy.ReceiptDigestFromAuthority(decision.AuthorityRef)
	if decision.EventKind != computerevent.EventEffectAccepted || !ok || applied.DecisionActor != digest {
		t.Fatalf("decision authority %q actor %q", decision.AuthorityRef, applied.DecisionActor)
	}
	if applied.MaterializationReceipt == "" || applied.RouteReceipt == "" || f.service.restarts != 1 {
		t.Fatalf("applied without its receipts or restart: %+v restarts=%d", applied, f.service.restarts)
	}
	// 31: a frontend-only candidate, accepted by consensus, runs as the host
	// build corpusd voted on, not as the capsule's files.
	built := f.hostBuildOf(t, applied)
	current, err := updater.ReadCurrentManifest(f.updaterRoot)
	if err != nil {
		t.Fatal(err)
	}
	if built.State != hostbuild.StateDone || current.ContentDigest != applied.ReleaseDigest || current.ClosureDigest != built.Result.ClosureDigest ||
		current.LayeringEntrypoint != hostBuildTestLayer+"/bin/autoputer" || current.CodeCommit != hostBuildTestCodeCommit {
		t.Fatalf("applied release is not the voted host build: job=%+v current=%+v", built, current)
	}
	served, err := os.ReadFile(filepath.Join(f.updaterRoot, "current", "frontend", "index.html"))
	if err != nil || string(served) != hostBuildTestFrontend {
		t.Fatalf("served frontend = %q %v, want the host build's", served, err)
	}

	// A second candidate in the same window: no further owner act.
	second := f.verify(t, f.freeze(t, "p4-second", "second autonomous change"))
	if _, err := f.rt.runStandingConsensus(ctx, second.OperationID, "run:management-p4"); err != nil {
		t.Fatalf("second standing consensus: %v", err)
	}
	f.waitForState(t, second.OperationID, selfdev.StateApplied)

	// No owner authority anywhere on the chain.
	for _, record := range f.cas.snapshot() {
		if strings.HasPrefix(record.Request.Event.AuthorityRef, "external-owner:") {
			t.Fatalf("an owner authority reached the chain: %+v", record.Request.Event)
		}
	}

	// The guest cannot forge quorum: an effect_accepted citing a digest the
	// poster never minted is refused by corpusd before it is on the chain.
	third := f.verify(t, f.freeze(t, "p4-forged", "forged acceptance"))
	head := f.waitForStableHead(t)
	modeRaw, _ := f.currentMode(t).Receipt.CanonicalBytes()
	forgedDigest := strings.Repeat("e", 64)
	eventID, _ := computerevent.NewEventID()
	forged := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: f.computerID,
		EventKind: computerevent.EventEffectAccepted, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
		IdempotencyKey: "p4-forged-accept", RequestCommitment: computerevent.ZeroHead,
		TrajectoryID: third.TrajectoryID, CapsuleID: third.CapsuleID, PreviousHead: head.CanonicalEventHead, ParentEventID: third.OperationID,
		ActorProfile: "management", AuthorityRef: decisionpolicy.DecisionAuthorityRef(forgedDigest), PrivacyClass: "owner",
		ExpectedDesiredEventHead: head.DesiredEventHead, ExpectedEffectiveEventHead: head.EffectiveEventHead,
		ExpectedDesiredStateCommitment: head.DesiredStateCommitment, ExpectedEffectiveStateCommitment: head.EffectiveStateCommitment,
		RequireExpectedHead: true, PayloadCommitment: computerevent.ZeroHead, ProposedEffectRef: third.BundleDigest,
		DecisionRef: strings.Repeat("d", 64), VerifierRefs: append([]string(nil), third.VerifierRefs...), ReducerVersion: computerevent.ReducerVersionV1,
		InputArtifactRefs: []string{"artifact:sha256:" + computerevent.DigestBytes(modeRaw), "artifact:sha256:" + forgedDigest},
	}
	target, _ := computerevent.CanonicalJSON(map[string]string{"base_head": third.BaseHead, "bundle_digest": third.BundleDigest})
	if _, err := f.appender.AppendNew(ctx, forged, computerevent.TransitionInput{TargetStateCommitment: computerevent.DigestBytes(target)}, nil); err == nil || !strings.Contains(err.Error(), "admission refused") {
		t.Fatalf("guest-forged consensus acceptance: %v", err)
	}
	if op, _ := f.operations.Get(ctx, f.computerID, third.OperationID); op.State != selfdev.StateAwaitingApproval {
		t.Fatalf("forged acceptance moved the operation to %s", op.State)
	}

	// An expired window stops autonomy: re-arm for two seconds, let it lapse.
	f.armStanding(t, time.Now().Add(2*time.Second))
	time.Sleep(2500 * time.Millisecond)
	if op, err := f.rt.runStandingConsensus(ctx, third.OperationID, "run:management-p4"); err != nil || op.OperationID != "" {
		t.Fatalf("driver acted after the window closed: %+v %v", op, err)
	}
	if mode := f.currentMode(t); mode.Mode != platform.SelfDevelopmentModeOff {
		t.Fatalf("expired standing arm reads %s", mode.Mode)
	}
	if op, _ := f.operations.Get(ctx, f.computerID, third.OperationID); op.State != selfdev.StateAwaitingApproval {
		t.Fatalf("operation moved after the window closed: %s", op.State)
	}
}

// A candidate whose source the host cannot build is dissent (28): the host
// build fails, corpusd's capsule-verifier ballot is a signed reject, the
// poster mints nothing and the guest records a rejection under it.
func TestStandingConsensusEndToEndHostDissentRejects(t *testing.T) {
	f := newP4Fixture(t)
	ctx := context.Background()
	f.armStanding(t, time.Now().Add(time.Hour))
	// A patch git reads, against a file whose base content it does not match.
	broken := []byte("diff --git a/frontend/index.html b/frontend/index.html\n--- a/frontend/index.html\n+++ b/frontend/index.html\n@@ -1 +1 @@\n-<html>not the base</html>\n+<html>dissent</html>\n")
	verified := f.verify(t, f.freezeWithPatch(t, "p4-dissent", "a patch the host cannot build", broken))
	if _, err := f.rt.runStandingConsensus(ctx, verified.OperationID, "run:management-p4"); err != nil {
		t.Fatalf("dissent path: %v", err)
	}
	rejected := f.waitForState(t, verified.OperationID, selfdev.StateRejected)
	event := f.decisionEvent(t, verified.OperationID)
	if event.EventKind != computerevent.EventEffectRejected || !strings.HasPrefix(event.AuthorityRef, consensusDissentAuthorityPrefix) || rejected.DecisionActor != event.AuthorityRef {
		t.Fatalf("dissent rejection: %+v %+v", event, rejected)
	}
	if job := f.hostBuildOf(t, verified); job.State != hostbuild.StateFailed || job.Reason != hostbuild.ReasonBuildFailed {
		t.Fatalf("host build = %+v, want failed build_failed", job)
	}
	if f.service.restarts != 0 || f.applyCalls.Load() != 0 {
		t.Fatalf("a rejected candidate applied (%d) or restarted (%d)", f.applyCalls.Load(), f.service.restarts)
	}
}
