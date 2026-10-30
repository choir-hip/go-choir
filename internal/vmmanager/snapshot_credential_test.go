package vmmanager

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// S3 re-credential on resume, host side
// (docs/archive/reports/s3-snapshot-hibernate-design-2026-10-14.md, items 26-28,
// 33, 35-38). The host mints a fresh envelope for the snapshot's realization
// and delivers it in the resumed call, so a snapshot outlives the frozen
// capability's renewal window.
//
// Ways this could fail, written first:
//   - the resume_deadline still refuses a snapshot whose guest accepts
//     delivered credentials, so long hibernations still cold boot;
//   - the host mints an envelope for a snapshot that then fails validation
//     (an issuance per wake, for nothing);
//   - the envelope is not sent, or sent to a guest that never advertised it;
//   - the envelope lands in metadata, the timeline, or the instance config;
//   - issuance failure past the window loads anyway (the guest wakes unable
//     to write its tape), or within the window cold boots needlessly;
//   - a guest that refuses the envelope is kept, or its snapshot reloaded;
//   - max age stops applying once the deadline is relaxed.

const testEnvelope = "eyJ2ZXJzaW9uIjoxLCJiZWFyZXIiOiJzZWNyZXQifQ"

type countingIssuer struct {
	calls    int
	envelope string
	err      error
}

func (c *countingIssuer) issue() (string, error) {
	c.calls++
	if c.err != nil {
		return "", c.err
	}
	return c.envelope, nil
}

// hibernateLongAgo hibernates the fixture VM with a guest whose capability
// expired two hours ago, so the old resume_deadline has long passed.
func hibernateLongAgo(t *testing.T, credentialResume bool) *snapshotFixture {
	t.Helper()
	f := newSnapshotFixture(t)
	f.guest.credentialResume = credentialResume
	f.guest.capExpiresAt = time.Now().Add(-2 * time.Hour)
	f.serveLaunchAPI(t)
	if err := f.mgr.HibernateVM(f.vmID); err != nil {
		t.Fatalf("HibernateVM: %v", err)
	}
	meta, err := readSnapshotMeta(f.mgr.snapshotDir(f.vmID))
	if err != nil {
		t.Fatalf("committed snapshot: %v", err)
	}
	if meta.CredentialResume != credentialResume {
		t.Fatalf("meta credential_resume = %v, want %v", meta.CredentialResume, credentialResume)
	}
	if !time.Now().After(meta.ResumeDeadline) {
		t.Fatalf("fixture deadline %s has not passed", meta.ResumeDeadline)
	}
	return f
}

func assertNoEnvelopeOnDisk(t *testing.T, root string) {
	t.Helper()
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.Mode().IsRegular() || info.Size() > 1<<20 {
			return nil
		}
		data, _ := os.ReadFile(path)
		if strings.Contains(string(data), testEnvelope) {
			t.Fatalf("credential envelope written to %s", path)
		}
		return nil
	})
}

func TestResumeDeliversFreshCredentialPastRenewalWindow(t *testing.T) {
	f := hibernateLongAgo(t, true)
	restoreAPI := &fakeFirecrackerAPI{}
	var restoreArgs []string
	f.restoreStarter(t, restoreAPI, &restoreArgs)
	issuer := &countingIssuer{envelope: testEnvelope}
	inst, err := f.mgr.ResumeVMFromSnapshot(f.vmID, f.expect, issuer.issue)
	if err != nil {
		t.Fatalf("resume past the renewal window with a delivered credential: %v", err)
	}
	if inst.State != StateRunning || inst.Config.Epoch != 7 {
		t.Fatalf("resumed instance state=%s epoch=%d, want running epoch 7 (same realization)", inst.State, inst.Config.Epoch)
	}
	if issuer.calls != 1 || len(f.guest.envelopes) != 1 || f.guest.envelopes[0] != testEnvelope {
		t.Fatalf("issuer calls=%d guest envelopes=%v, want one envelope delivered", issuer.calls, f.guest.envelopes)
	}
	if inst.Config.ComputerCredentialEnvelope != "" {
		t.Fatal("the delivered envelope was kept in the instance config")
	}
	tl, err := f.mgr.BootTimelineForVM(f.vmID)
	if err != nil {
		t.Fatal(err)
	}
	if tl.BootKind != "snapshot_resume" || tl.Outcome != "healthy" || tl.Snapshot.Credential != snapshotCredentialDelivered {
		t.Fatalf("resume timeline = %s %s credential=%q", tl.BootKind, tl.Outcome, tl.Snapshot.Credential)
	}
	assertNoEnvelopeOnDisk(t, f.mgr.cfg.StateDir)
}

func TestResumeIssuesCredentialOnlyForAValidSnapshot(t *testing.T) {
	f := hibernateLongAgo(t, true)
	// The data disk moved: the snapshot is invalid.
	later := time.Now().Add(time.Minute)
	if err := os.Chtimes(filepath.Join(f.mgr.vmStateDir(f.vmID), "data.img"), later, later); err != nil {
		t.Fatal(err)
	}
	issuer := &countingIssuer{envelope: testEnvelope}
	_, err := f.mgr.ResumeVMFromSnapshot(f.vmID, f.expect, issuer.issue)
	var fb *SnapshotFallbackError
	if !errors.As(err, &fb) || fb.Reason != snapshotReasonDisk {
		t.Fatalf("resume = %v, want disk fallback", err)
	}
	if issuer.calls != 0 {
		t.Fatal("an envelope was minted for a snapshot that failed validation")
	}
}

func TestResumeOfOldGuestSnapshotKeepsTheDeadline(t *testing.T) {
	f := hibernateLongAgo(t, false)
	issuer := &countingIssuer{envelope: testEnvelope}
	started := false
	f.mgr.firecrackerStarter = func(string, []string) error { started = true; return nil }
	_, err := f.mgr.ResumeVMFromSnapshot(f.vmID, f.expect, issuer.issue)
	var fb *SnapshotFallbackError
	if !errors.As(err, &fb) || fb.Reason != snapshotReasonCredentialWindow {
		t.Fatalf("resume = %v, want credential_window_passed for a guest without credential_resume", err)
	}
	if issuer.calls != 0 || started {
		t.Fatalf("issuer calls=%d started=%v; an old guest's expired snapshot must not load", issuer.calls, started)
	}
}

func TestResumeWithoutIssuerKeepsTheDeadline(t *testing.T) {
	f := hibernateLongAgo(t, true)
	_, err := f.mgr.ResumeVMFromSnapshot(f.vmID, f.expect, nil)
	var fb *SnapshotFallbackError
	if !errors.As(err, &fb) || fb.Reason != snapshotReasonCredentialWindow {
		t.Fatalf("resume = %v, want credential_window_passed when the host cannot issue", err)
	}
}

func TestResumeIssuanceFailurePastWindowColdBoots(t *testing.T) {
	f := hibernateLongAgo(t, true)
	issuer := &countingIssuer{err: errors.New("corpusd unavailable")}
	started := false
	f.mgr.firecrackerStarter = func(string, []string) error { started = true; return nil }
	_, err := f.mgr.ResumeVMFromSnapshot(f.vmID, f.expect, issuer.issue)
	var fb *SnapshotFallbackError
	if !errors.As(err, &fb) || fb.Reason != snapshotReasonCredentialIssue {
		t.Fatalf("resume = %v, want credential_issue_failed", err)
	}
	if started {
		t.Fatal("a snapshot past its renewal window loaded without a credential")
	}
	if got := f.mgr.takeSnapshotFallback(f.vmID); got != snapshotReasonCredentialIssue {
		t.Fatalf("cold boot fallback reason = %q", got)
	}
}

func TestResumeIssuanceFailureWithinWindowRenews(t *testing.T) {
	f := newSnapshotFixture(t)
	f.guest.credentialResume = true
	f.serveLaunchAPI(t)
	if err := f.mgr.HibernateVM(f.vmID); err != nil {
		t.Fatal(err)
	}
	restoreAPI := &fakeFirecrackerAPI{}
	var restoreArgs []string
	f.restoreStarter(t, restoreAPI, &restoreArgs)
	issuer := &countingIssuer{err: errors.New("corpusd unavailable")}
	if _, err := f.mgr.ResumeVMFromSnapshot(f.vmID, f.expect, issuer.issue); err != nil {
		t.Fatalf("resume within the renewal window after an issuance failure: %v", err)
	}
	if len(f.guest.envelopes) != 1 || f.guest.envelopes[0] != "" {
		t.Fatalf("guest envelopes = %v, want one resumed call without an envelope", f.guest.envelopes)
	}
	tl, err := f.mgr.BootTimelineForVM(f.vmID)
	if err != nil {
		t.Fatal(err)
	}
	if tl.Snapshot.Credential != snapshotCredentialRenewOnly {
		t.Fatalf("timeline credential = %q, want %q", tl.Snapshot.Credential, snapshotCredentialRenewOnly)
	}
}

func TestResumeRefusedCredentialFallsBackAndNeverReloads(t *testing.T) {
	f := hibernateLongAgo(t, true)
	f.guest.resumedStatus = http.StatusServiceUnavailable
	restoreAPI := &fakeFirecrackerAPI{}
	var restoreArgs []string
	f.restoreStarter(t, restoreAPI, &restoreArgs)
	issuer := &countingIssuer{envelope: testEnvelope}
	_, err := f.mgr.ResumeVMFromSnapshot(f.vmID, f.expect, issuer.issue)
	var fb *SnapshotFallbackError
	if !errors.As(err, &fb) || fb.Reason != snapshotReasonGuestResumed {
		t.Fatalf("resume = %v, want guest_resumed_failed", err)
	}
	if strings.Contains(err.Error(), testEnvelope) {
		t.Fatal("the fallback error carries the envelope")
	}
	if f.mgr.GetVM(f.vmID) != nil && f.mgr.GetVM(f.vmID).State == StateRunning {
		t.Fatal("a guest that refused its credential was kept running")
	}
	_, err = f.mgr.ResumeVMFromSnapshot(f.vmID, f.expect, issuer.issue)
	if !errors.As(err, &fb) || fb.Reason != snapshotReasonNoSnapshot {
		t.Fatalf("second resume = %v, want no_snapshot (one-shot)", err)
	}
	if issuer.calls != 1 {
		t.Fatalf("issuer calls = %d, want 1", issuer.calls)
	}
	assertNoEnvelopeOnDisk(t, f.mgr.cfg.StateDir)
}

func TestResumeMaxAgeStillAppliesWithDeliveredCredentials(t *testing.T) {
	f := hibernateLongAgo(t, true)
	dir := f.mgr.snapshotDir(f.vmID)
	meta, err := readSnapshotMeta(dir)
	if err != nil {
		t.Fatal(err)
	}
	meta.CreatedAt = time.Now().Add(-f.mgr.snapshotMaxAge() - time.Minute)
	if err := writeSnapshotMeta(dir, meta); err != nil {
		t.Fatal(err)
	}
	issuer := &countingIssuer{envelope: testEnvelope}
	_, err = f.mgr.ResumeVMFromSnapshot(f.vmID, f.expect, issuer.issue)
	var fb *SnapshotFallbackError
	if !errors.As(err, &fb) || fb.Reason != snapshotReasonMaxAge {
		t.Fatalf("resume = %v, want max_age_passed", err)
	}
	if issuer.calls != 0 {
		t.Fatal("an envelope was minted for a snapshot past its max age")
	}
}
