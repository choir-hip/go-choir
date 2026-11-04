package platform

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/hostbuild"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	"github.com/yusefmosiah/go-choir/internal/tapestore"
)

// Ways the clean rebase gate could fail, written before the code: a guest
// forges a receipt (not on the tape, or bytes that do not hash to the
// authority); the receipt names another computer, operation or bundle; the
// branch runs on a computer outside ring R1 (an owner computer, or the
// paper); the prior decision is missing, accepted another bundle, or was
// itself admitted without authority (an observe-mode "would refuse"); the
// next bundle changes a field other than its base identity, or carries
// another patch, or the base never moved; the host build is missing,
// unfinished, of another bundle, or another result; observe mode admits a
// forged acceptance. Round 2 (Devin F1): the receipt rebases an older
// approved change, a rolled-back change or one that never applied, so a
// re-application silently reverts the current change or re-lands a removed
// one; the receipt's booted base is not the image the host deployed; the host
// build is of another guest image; the computer's mode is off or absent.

const (
	crComputer   = "computer-0123456789abcdef0123456789abcdef"
	crStackBase  = "1111111111111111111111111111111111111111"
	crBootedBase = "2222222222222222222222222222222222222222"
)

type fakeTape map[string][]byte

func (f fakeTape) put(ns tapestore.NS, raw []byte) string {
	digest := computerevent.DigestBytes(raw)
	f[fmt.Sprintf("%d/%s", ns, digest)] = raw
	return digest
}

func (f fakeTape) read(_ string, ns tapestore.NS, digest string) ([]byte, error) {
	raw, ok := f[fmt.Sprintf("%d/%s", ns, digest)]
	if !ok {
		return nil, errors.New("not on the tape")
	}
	return raw, nil
}

type fakeJobs map[string]hostbuild.Job

// fakeHost is the host build service with a deployed guest image.
type fakeHost struct {
	fakeJobs
	digest, commit string
}

func (f fakeHost) DeployedBase() (string, string, error) {
	if f.commit == "" {
		return "", "", errors.New("no manifest")
	}
	return f.digest, f.commit, nil
}

const crDeployedDigest = "9999999999999999999999999999999999999999999999999999999999999999"

func (f fakeJobs) Get(computerID, jobID string) (hostbuild.Job, bool) {
	job, ok := f[jobID]
	if !ok || job.ComputerID != computerID {
		return hostbuild.Job{}, false
	}
	return job, true
}

// cleanRebaseCase is one attempt; mutators change it before it is written.
type cleanRebaseCase struct {
	priorBundle, nextBundle map[string]any
	priorAuthority          string
	priorMarked             bool
	priorParent             string
	priorEffect             string // "" = the prior bundle's digest
	recordPrior             bool
	job                     *hostbuild.Job
	result                  hostbuild.Result
	receipt                 selfdevprotocol.CleanRebaseReceipt
	receiptBytes            []byte // nil = canonical receipt
	citeReceipt             bool
	test, platform          bool
	tierErr                 error
	jobs                    bool
	applied                 bool   // record the prior's materialization_applied
	deployedCommit          string // "" = crBootedBase; "-" = unreadable manifest
	after                   func(t *testing.T, f *cleanRebaseFixture, priorDecision, priorBundle string)
	mutateReceipt           func(*selfdevprotocol.CleanRebaseReceipt)
	mutateJob               func(*hostbuild.Job)
}

func hexOf(c byte) string { return strings.Repeat(string(c), 64) }

func newCleanRebaseCase() *cleanRebaseCase {
	bundle := func(base, head string) map[string]any {
		return map[string]any{
			"bundle_version": 1, "computer_id": crComputer,
			"source_patch_sha256": hexOf('a'), "source_patch_base_commit": base,
			"base_event_head": head, "content_digest": head, "changed_paths": []any{"internal/x.go"},
		}
	}
	return &cleanRebaseCase{
		priorBundle: bundle(crStackBase, hexOf('3')), nextBundle: bundle(crBootedBase, hexOf('4')),
		priorAuthority: AuthorityExternalOwner + "owner-1", priorMarked: true, priorParent: "op-1",
		recordPrior: true, citeReceipt: true, test: true, jobs: true, applied: true,
	}
}

type cleanRebaseRun struct {
	authority *OwnerActAuthority
	event     computerevent.Event
	f         *cleanRebaseFixture
}

type cleanRebaseFixture struct {
	store *Store
	tape  fakeTape
	seq   int
}

func newCleanRebaseFixture(t *testing.T) *cleanRebaseFixture {
	t.Helper()
	store, _ := openTestPlatformStore(t)
	f := &cleanRebaseFixture{store: store, tape: fakeTape{}}
	f.setMode(t, SelfDevelopmentModeStandingConsensus)
	return f
}

// setMode writes the computer's mode row ("" removes it).
func (f *cleanRebaseFixture) setMode(t *testing.T, mode string) {
	t.Helper()
	if _, err := f.store.db.Exec(`DELETE FROM computer_self_development_modes WHERE computer_id=?`, crComputer); err != nil {
		t.Fatal(err)
	}
	if mode == "" {
		return
	}
	_, err := f.store.db.Exec(`INSERT INTO computer_self_development_modes (computer_id, mode, generation, last_idempotency_key, last_request_commitment, mode_receipt_json, mode_receipt_digest, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		crComputer, mode, 1, "mode-key", hexOf('0'), "{}", hexOf('0'), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
}

// recordApplied puts a materialization_applied (or a rollback event) naming
// decision and bundle on the tape and in the DB.
func (f *cleanRebaseFixture) recordApplied(t *testing.T, kind computerevent.EventKind, decision, bundle string) {
	t.Helper()
	event := computerevent.Event{
		ComputerID: crComputer, EventKind: kind, DecisionRef: decision, ProposedEffectRef: bundle,
		IdempotencyKey: fmt.Sprintf("applied-%d", f.seq+1),
	}
	f.recordEvent(t, kind, f.tape.put(tapestore.NSEvent, crJSON(t, event)))
}

func (f *cleanRebaseFixture) recordEvent(t *testing.T, kind computerevent.EventKind, digest string) {
	t.Helper()
	f.seq++
	_, err := f.store.db.Exec(`INSERT INTO computer_event_append_receipts (computer_id, idempotency_key, request_commitment, sequence, event_kind, event_digest, event_pin_receipt_digest, event_head_receipt_id, event_head_receipt_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		crComputer, fmt.Sprintf("key-%d", f.seq), hexOf('0'), f.seq, string(kind), digest, hexOf('0'), fmt.Sprintf("receipt-%d", f.seq), "{}", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
}

func (f *cleanRebaseFixture) markOwner(t *testing.T, eventDigest string) {
	t.Helper()
	f.seq++
	_, err := f.store.db.Exec(`INSERT INTO self_development_mode_receipts (receipt_digest, computer_id, generation, old_generation, old_mode, new_mode, mode_writer, receipt_json, admitted_event_digest, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		computerevent.DigestBytes([]byte(fmt.Sprintf("mode-%d", f.seq))), crComputer, f.seq, f.seq-1, SelfDevelopmentModeAcceptOnce, SelfDevelopmentModeProposeOnly, ModeWriterGuestConsumption, "{}", eventDigest, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
}

func crJSON(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := computerevent.CanonicalJSON(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// write puts c's objects on the tape and in the DB and returns the decision
// event. operation is the new operation id.
func (f *cleanRebaseFixture) write(t *testing.T, c *cleanRebaseCase, operation string, jobs fakeJobs) (computerevent.Event, string) {
	t.Helper()
	priorDigest := f.tape.put(tapestore.NSPayload, crJSON(t, c.priorBundle))
	nextDigest := f.tape.put(tapestore.NSPayload, crJSON(t, c.nextBundle))
	effect := c.priorEffect
	if effect == "" {
		effect = priorDigest
	}
	prior := computerevent.Event{
		ComputerID: crComputer, EventKind: computerevent.EventEffectAccepted,
		ParentEventID: c.priorParent, ProposedEffectRef: effect, AuthorityRef: c.priorAuthority,
	}
	priorEventDigest := f.tape.put(tapestore.NSEvent, crJSON(t, prior))
	if c.recordPrior {
		f.recordEvent(t, computerevent.EventEffectAccepted, priorEventDigest)
	}
	if c.priorMarked {
		f.markOwner(t, priorEventDigest)
	}
	if c.applied {
		f.recordApplied(t, computerevent.EventMaterializationApplied, priorEventDigest, priorDigest)
	}
	if c.after != nil {
		c.after(t, f, priorEventDigest, priorDigest)
	}
	c.result = hostbuild.Result{
		JobID: hexOf('7'), ComputerID: crComputer, OperationID: operation, BundleDigest: nextDigest,
		BaseCommit: crBootedBase, SourcePatchSHA256: hexOf('a'), BuiltAt: "2026-11-04T00:00:00Z",
	}
	resultDigest, err := c.result.Digest()
	if err != nil {
		t.Fatal(err)
	}
	job := hostbuild.Job{
		JobID: c.result.JobID, ComputerID: crComputer, OperationID: operation, BundleDigest: nextDigest,
		BaseCommit: crBootedBase, SourcePatchSHA256: hexOf('a'), BaseImageManifestDigest: crDeployedDigest,
		State: hostbuild.StateDone, Result: &c.result,
	}
	if c.mutateJob != nil {
		c.mutateJob(&job)
	}
	if jobs != nil {
		jobs[job.JobID] = job
	}
	c.receipt = selfdevprotocol.CleanRebaseReceipt{
		Schema: selfdevprotocol.CleanRebaseReceiptSchema, ComputerID: crComputer,
		OperationID: operation, BundleDigest: nextDigest,
		RebaseOf: c.priorParent, PriorBundleDigest: priorDigest, PriorDecisionEvent: priorEventDigest,
		SourcePatchSHA256: hexOf('a'), StackBase: crStackBase, BootedBase: crBootedBase,
		HostBuildJobID: job.JobID, HostBuildResultDigest: resultDigest,
		Ruling: selfdevprotocol.CleanRebaseRuling, Statement: selfdevprotocol.CleanRebaseStatement(c.priorParent, crBootedBase),
	}
	if c.mutateReceipt != nil {
		c.mutateReceipt(&c.receipt)
	}
	raw := c.receiptBytes
	if raw == nil {
		raw, err = computerevent.CanonicalJSON(c.receipt)
		if err != nil {
			t.Fatal(err)
		}
	}
	receiptDigest := f.tape.put(tapestore.NSPayload, raw)
	inputs := []string{"artifact:sha256:" + hexOf('5')}
	if c.citeReceipt {
		inputs = append(inputs, "artifact:sha256:"+receiptDigest)
	}
	return computerevent.Event{
		ComputerID: crComputer, EventKind: computerevent.EventEffectAccepted,
		ParentEventID: operation, ProposedEffectRef: nextDigest,
		AuthorityRef: selfdevprotocol.CleanRebaseAuthority(receiptDigest), InputArtifactRefs: inputs,
	}, receiptDigest
}

func (f *cleanRebaseFixture) authority(c *cleanRebaseCase, mode OwnerActMode, jobs fakeJobs) *OwnerActAuthority {
	a := NewOwnerActAuthority(mode, nil)
	if c.jobs {
		host := fakeHost{fakeJobs: jobs, digest: crDeployedDigest, commit: crBootedBase}
		switch c.deployedCommit {
		case "":
		case "-":
			host.commit = ""
		default:
			host.commit = c.deployedCommit
		}
		a.ConfigureCleanRebase(f.tape.read, host)
	} else {
		a.ConfigureCleanRebase(f.tape.read, nil)
	}
	a.cleanRebaseTier = func(string) (bool, bool, error) { return c.test, c.platform, c.tierErr }
	return a
}

func (f *cleanRebaseFixture) judge(t *testing.T, a *OwnerActAuthority, event computerevent.Event) (string, error) {
	t.Helper()
	tx, err := f.store.db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	reason, mark, err := a.effectAcceptedRefusal(context.Background(), tx, event, hexOf('e'))
	if mark != nil {
		t.Fatal("the clean rebase branch returned a mark")
	}
	return reason, err
}

func TestCleanRebaseGate(t *testing.T) {
	f := newCleanRebaseFixture(t)
	n := 0
	run := func(mutate func(*cleanRebaseCase)) (string, error) {
		n++
		c := newCleanRebaseCase()
		// Each case is its own operation and its own prior decision.
		c.priorParent = fmt.Sprintf("op-prior-%d", n)
		if mutate != nil {
			mutate(c)
		}
		jobs := fakeJobs{}
		event, _ := f.write(t, c, fmt.Sprintf("op-next-%d", n), jobs)
		return f.judge(t, f.authority(c, OwnerActEnforce, jobs), event)
	}
	admitted := func(name string, mutate func(*cleanRebaseCase)) {
		t.Helper()
		if reason, err := run(mutate); reason != "" || err != nil {
			t.Fatalf("%s: refused: %q %v", name, reason, err)
		}
	}
	refusedWith := func(name, want string, mutate func(*cleanRebaseCase)) {
		t.Helper()
		reason, err := run(mutate)
		got := reason
		if err != nil {
			got = err.Error()
		}
		if got == "" || !strings.Contains(got, want) {
			t.Fatalf("%s: got %q, want a refusal containing %q", name, got, want)
		}
	}

	admitted("happy path", nil)
	refusedWith("receipt not cited", "not an input artifact", func(c *cleanRebaseCase) { c.citeReceipt = false })
	refusedWith("receipt bytes are not canonical receipt", "does not decode", func(c *cleanRebaseCase) { c.receiptBytes = []byte(`{"schema":"x"}`) })
	refusedWith("receipt for another bundle", "another bundle", func(c *cleanRebaseCase) {
		c.mutateReceipt = func(r *selfdevprotocol.CleanRebaseReceipt) { r.BundleDigest = hexOf('9') }
	})
	refusedWith("receipt for another computer", "another computer", func(c *cleanRebaseCase) {
		c.mutateReceipt = func(r *selfdevprotocol.CleanRebaseReceipt) {
			r.ComputerID = "computer-ffffffffffffffffffffffffffffffff"
		}
	})
	refusedWith("not a test computer", "ring R1", func(c *cleanRebaseCase) { c.test = false })
	refusedWith("platform computer", "platform computer", func(c *cleanRebaseCase) { c.platform = true })
	refusedWith("tier file unreadable", "computer tiers", func(c *cleanRebaseCase) { c.tierErr = errors.New("bad tier file") })
	refusedWith("prior decision missing", "not on this computer's tape", func(c *cleanRebaseCase) { c.recordPrior = false })
	refusedWith("prior decision names another bundle", "another bundle than the receipt's prior bundle", func(c *cleanRebaseCase) { c.priorEffect = hexOf('8') })
	refusedWith("prior decision is a platform update", "not an owner, consensus or clean rebase", func(c *cleanRebaseCase) { c.priorAuthority = AuthorityPlatformUpdate })
	refusedWith("prior owner decision was never admitted", "never admitted", func(c *cleanRebaseCase) { c.priorMarked = false })
	refusedWith("bundles differ in a non-changeable field", `field "changed_paths" differs`, func(c *cleanRebaseCase) {
		c.nextBundle["changed_paths"] = []any{"internal/y.go"}
	})
	refusedWith("next bundle adds a field", `adds field "extra"`, func(c *cleanRebaseCase) { c.nextBundle["extra"] = "x" })
	refusedWith("patch digest differs", "same source patch", func(c *cleanRebaseCase) { c.nextBundle["source_patch_sha256"] = hexOf('b') })
	refusedWith("receipt names another patch", "another patch", func(c *cleanRebaseCase) {
		c.mutateReceipt = func(r *selfdevprotocol.CleanRebaseReceipt) { r.SourcePatchSHA256 = hexOf('b') }
	})
	refusedWith("base did not move", "did not move", func(c *cleanRebaseCase) { c.nextBundle["source_patch_base_commit"] = crStackBase })
	refusedWith("receipt names another booted base", "booted base", func(c *cleanRebaseCase) {
		c.mutateReceipt = func(r *selfdevprotocol.CleanRebaseReceipt) {
			r.BootedBase = "3333333333333333333333333333333333333333"
			r.Statement = selfdevprotocol.CleanRebaseStatement(r.RebaseOf, r.BootedBase)
		}
	})
	refusedWith("no host build service", "no host build service", func(c *cleanRebaseCase) { c.jobs = false })
	refusedWith("host build missing", "not known", func(c *cleanRebaseCase) {
		c.mutateReceipt = func(r *selfdevprotocol.CleanRebaseReceipt) { r.HostBuildJobID = hexOf('6') }
	})
	refusedWith("host build not done", "not done", func(c *cleanRebaseCase) {
		c.mutateJob = func(j *hostbuild.Job) { j.State = hostbuild.StateRunning }
	})
	refusedWith("host build failed", "not done", func(c *cleanRebaseCase) {
		c.mutateJob = func(j *hostbuild.Job) { j.State = hostbuild.StateFailed }
	})
	refusedWith("host build for another bundle", "another operation or bundle", func(c *cleanRebaseCase) {
		c.mutateJob = func(j *hostbuild.Job) { j.BundleDigest = hexOf('9') }
	})
	refusedWith("host build of another base", "another base or patch", func(c *cleanRebaseCase) {
		c.mutateJob = func(j *hostbuild.Job) { j.BaseCommit = crStackBase }
	})
	refusedWith("host build result digest mismatch", "not the one the receipt names", func(c *cleanRebaseCase) {
		c.mutateReceipt = func(r *selfdevprotocol.CleanRebaseReceipt) { r.HostBuildResultDigest = hexOf('c') }
	})

	// Devin F1: the rebased change must be the one the computer has applied
	// now, and the booted base the image the host deployed.
	refusedWith("an older approved change (a newer one applied since)", "not the one rebased", func(c *cleanRebaseCase) {
		c.after = func(t *testing.T, f *cleanRebaseFixture, _, _ string) {
			f.recordApplied(t, computerevent.EventMaterializationApplied, hexOf('d'), hexOf('b'))
		}
	})
	refusedWith("a never-applied change", "not the one rebased", func(c *cleanRebaseCase) { c.applied = false })
	refusedWith("a rolled-back change", "latest self-development event is a rollback_applied", func(c *cleanRebaseCase) {
		c.after = func(t *testing.T, f *cleanRebaseFixture, decision, bundle string) {
			f.recordApplied(t, computerevent.EventRollbackRequested, decision, bundle)
			f.recordApplied(t, computerevent.EventRollbackApplied, decision, bundle)
		}
	})
	refusedWith("a change with a rollback pending", "latest self-development event is a rollback_requested", func(c *cleanRebaseCase) {
		c.after = func(t *testing.T, f *cleanRebaseFixture, decision, bundle string) {
			f.recordApplied(t, computerevent.EventRollbackRequested, decision, bundle)
		}
	})
	refusedWith("a target base that is not the deployed image", "is not the host's deployed guest image", func(c *cleanRebaseCase) {
		c.deployedCommit = "5555555555555555555555555555555555555555"
	})
	refusedWith("an unreadable deployed manifest", "deployed guest image is unreadable", func(c *cleanRebaseCase) { c.deployedCommit = "-" })
	refusedWith("host build of another guest image", "another guest image", func(c *cleanRebaseCase) {
		c.mutateJob = func(j *hostbuild.Job) { j.BaseImageManifestDigest = hexOf('8') }
	})
}

func TestCleanRebaseGateNeverAppliedOnAFreshComputer(t *testing.T) {
	f := newCleanRebaseFixture(t)
	c := newCleanRebaseCase()
	c.applied = false
	jobs := fakeJobs{}
	event, _ := f.write(t, c, "op-2", jobs)
	if reason, err := f.judge(t, f.authority(c, OwnerActEnforce, jobs), event); err != nil || !strings.Contains(reason, "applied no self-development change") {
		t.Fatalf("got %q %v", reason, err)
	}
}

func TestCleanRebaseGateAdmitsEveryDevelopingMode(t *testing.T) {
	// propose_only is the resting mode after a consumed grant; the owner's
	// ruling admits clean rebases under it.
	for _, mode := range []string{SelfDevelopmentModeProposeOnly, SelfDevelopmentModeAcceptOnce, SelfDevelopmentModeQualifiedConsensus, SelfDevelopmentModeStandingConsensus} {
		f := newCleanRebaseFixture(t)
		f.setMode(t, mode)
		c := newCleanRebaseCase()
		jobs := fakeJobs{}
		event, _ := f.write(t, c, "op-2", jobs)
		if reason, err := f.judge(t, f.authority(c, OwnerActEnforce, jobs), event); reason != "" || err != nil {
			t.Fatalf("mode %q refused: %q %v", mode, reason, err)
		}
	}
}

func TestCleanRebaseGateModeCheck(t *testing.T) {
	for _, mode := range []string{"", SelfDevelopmentModeOff, SelfDevelopmentModeAuditOnly, "a-future-mode"} {
		f := newCleanRebaseFixture(t)
		f.setMode(t, mode)
		c := newCleanRebaseCase()
		jobs := fakeJobs{}
		event, _ := f.write(t, c, "op-2", jobs)
		reason, err := f.judge(t, f.authority(c, OwnerActEnforce, jobs), event)
		if err != nil || !strings.Contains(reason, "mode") {
			t.Fatalf("mode %q: got %q %v", mode, reason, err)
		}
	}
}

func TestCleanRebaseGateReceiptDigestMismatch(t *testing.T) {
	f := newCleanRebaseFixture(t)
	c := newCleanRebaseCase()
	jobs := fakeJobs{}
	event, receiptDigest := f.write(t, c, "op-2", jobs)
	// The tape answers the digest with other bytes.
	f.tape[fmt.Sprintf("%d/%s", tapestore.NSPayload, receiptDigest)] = append(f.tape[fmt.Sprintf("%d/%s", tapestore.NSPayload, receiptDigest)], ' ')
	reason, err := f.judge(t, f.authority(c, OwnerActEnforce, jobs), event)
	if err != nil || !strings.Contains(reason, "do not hash") {
		t.Fatalf("got %q %v", reason, err)
	}
	// An authority whose receipt is not on the tape at all is a read error.
	event.AuthorityRef = selfdevprotocol.CleanRebaseAuthority(hexOf('d'))
	event.InputArtifactRefs = append(event.InputArtifactRefs, "artifact:sha256:"+hexOf('d'))
	if reason, err := f.judge(t, f.authority(c, OwnerActEnforce, jobs), event); err == nil && reason == "" {
		t.Fatal("a receipt not on the tape was admitted")
	}
	event.AuthorityRef = selfdevprotocol.CleanRebaseAuthorityPrefix + "not-a-digest"
	if reason, _ := f.judge(t, f.authority(c, OwnerActEnforce, jobs), event); !strings.Contains(reason, "names no receipt digest") {
		t.Fatalf("malformed authority: %q", reason)
	}
}

func TestCleanRebaseGateRefusesEvenInObserve(t *testing.T) {
	f := newCleanRebaseFixture(t)
	c := newCleanRebaseCase()
	c.priorMarked = false // a forged chain
	jobs := fakeJobs{}
	event, _ := f.write(t, c, "op-2", jobs)
	a := f.authority(c, OwnerActObserve, jobs)
	tx, err := f.store.db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err := a.admitEffectAccepted(context.Background(), tx, event, hexOf('e')); !errors.Is(err, ErrEffectAdmissionRefused) {
		t.Fatalf("observe admitted a refused clean rebase: %v", err)
	}
	// A read error refuses in observe too.
	event.AuthorityRef = selfdevprotocol.CleanRebaseAuthority(hexOf('d'))
	event.InputArtifactRefs = append(event.InputArtifactRefs, "artifact:sha256:"+hexOf('d'))
	if err := a.admitEffectAccepted(context.Background(), tx, event, hexOf('e')); !errors.Is(err, ErrEffectAdmissionRefused) {
		t.Fatalf("observe admitted a clean rebase whose receipt is unreadable: %v", err)
	}
}

func TestCleanRebaseGateAdmitsAChainOfCleanRebases(t *testing.T) {
	f := newCleanRebaseFixture(t)
	// Link 1: an owner-approved op-1 re-applied as op-2 (host build retired
	// since; a prior link's host build is not re-read).
	first := newCleanRebaseCase()
	firstEvent, firstReceipt := f.write(t, first, "op-2", nil)
	firstEventDigest := f.tape.put(tapestore.NSEvent, crJSON(t, firstEvent))
	f.recordEvent(t, computerevent.EventEffectAccepted, firstEventDigest)
	f.recordApplied(t, computerevent.EventMaterializationApplied, firstEventDigest, firstEvent.ProposedEffectRef)

	// Link 2: op-2 re-applied as op-3 onto another booted base.
	const nextBase = "4444444444444444444444444444444444444444"
	second := map[string]any{}
	for k, v := range first.nextBundle {
		second[k] = v
	}
	second["source_patch_base_commit"] = nextBase
	second["base_event_head"] = hexOf('6')
	priorDigest := firstEvent.ProposedEffectRef
	nextDigest := f.tape.put(tapestore.NSPayload, crJSON(t, second))
	result := hostbuild.Result{JobID: hexOf('2'), ComputerID: crComputer, OperationID: "op-3", BundleDigest: nextDigest, BaseCommit: nextBase, SourcePatchSHA256: hexOf('a')}
	resultDigest, _ := result.Digest()
	jobs := fakeJobs{hexOf('2'): {JobID: hexOf('2'), ComputerID: crComputer, OperationID: "op-3", BundleDigest: nextDigest, BaseCommit: nextBase, SourcePatchSHA256: hexOf('a'), BaseImageManifestDigest: crDeployedDigest, State: hostbuild.StateDone, Result: &result}}
	receipt := selfdevprotocol.CleanRebaseReceipt{
		Schema: selfdevprotocol.CleanRebaseReceiptSchema, ComputerID: crComputer, OperationID: "op-3", BundleDigest: nextDigest,
		RebaseOf: "op-2", PriorBundleDigest: priorDigest, PriorDecisionEvent: firstEventDigest,
		SourcePatchSHA256: hexOf('a'), StackBase: crBootedBase, BootedBase: nextBase,
		HostBuildJobID: hexOf('2'), HostBuildResultDigest: resultDigest,
		Ruling: selfdevprotocol.CleanRebaseRuling, Statement: selfdevprotocol.CleanRebaseStatement("op-2", nextBase),
	}
	raw, digest, err := receipt.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	f.tape.put(tapestore.NSPayload, raw)
	event := computerevent.Event{
		ComputerID: crComputer, EventKind: computerevent.EventEffectAccepted, ParentEventID: "op-3", ProposedEffectRef: nextDigest,
		AuthorityRef: selfdevprotocol.CleanRebaseAuthority(digest), InputArtifactRefs: []string{"artifact:sha256:" + hexOf('5'), "artifact:sha256:" + digest},
	}
	first.deployedCommit = nextBase
	a := f.authority(first, OwnerActEnforce, jobs)
	if reason, err := f.judge(t, a, event); reason != "" || err != nil {
		t.Fatalf("chain refused: %q %v", reason, err)
	}
	// Break the first link: its owner decision was never admitted.
	if _, err := f.store.db.Exec(`UPDATE self_development_mode_receipts SET admitted_event_digest=NULL`); err != nil {
		t.Fatal(err)
	}
	if reason, _ := f.judge(t, a, event); !strings.Contains(reason, "prior clean rebase does not hold") {
		t.Fatalf("broken chain: %q", reason)
	}
	_ = firstReceipt
}
