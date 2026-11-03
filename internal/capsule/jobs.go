//go:build linux

package capsule

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/yusefmosiah/go-choir/internal/capsulejob"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Capsule jobs, executor side (docs/design/capsule-long-exec-2026-10-15.md).
// The executor starts a job through the capsule broker, watches it, and
// when it ends mints its receipt from the broker's status and its own
// worktree digests, stores its log by content address, and hands the
// runtime a JobCompletion. Jobs live in their capsule: a capsule that ends
// ends its job, and a guest restart closes every open job as interrupted.

// jobPollInterval is how often the watcher reads a running job. A var so a
// test can shorten it.
var jobPollInterval = 2 * time.Second

// A build-class job waits while the host is short of memory (S4,
// docs/design/capsule-heavy-work-scheduling-2026-11-03.md, point 3): it is
// recorded waiting, with the reason in its status, and starts when the host
// facts clear. hostMemoryWaitPoll is how often a waiting job reads them; a
// job that waited hostMemoryMaxWait ends stopped with the reason, so a stall
// is visible and never silent. Vars so a test can shorten them.
var (
	hostMemoryWaitPoll = 5 * time.Second
	hostMemoryMaxWait  = 30 * time.Minute
)

type capsuleJob struct {
	// boosted says the job's memory need raised its capsule, returned by
	// endJobMemory.
	boosted bool
	// waiting is true while the job waits for host memory; whoever clears
	// it (its start, its stop, its capsule's end) owns the job's next step.
	// Guarded by the executor's mu.
	waiting     bool
	agentRunID  string
	handle      string
	capsuleID   string
	spec        capsulejob.Spec
	notify      map[string]string
	startDigest string
	// memoryNote says the executor defaulted spec.MemoryMiB (job_shot_default.go).
	memoryNote string
	status     capsulejob.Status
	receiptRef string
	logRef     string
	done       chan struct{}
}

// jobRecord is a job's durable record under the artifact root: what the
// restart rule closes at the next boot.
type jobRecord struct {
	ID         string   `json:"id"`
	AgentRunID string   `json:"agent_run_id"`
	CapsuleID  string   `json:"capsule_id"`
	State      string   `json:"state"`
	Argv       []string `json:"argv"`
	Cwd        string   `json:"cwd"`
	StartedAt  string   `json:"started_at"`
	FinishedAt string   `json:"finished_at,omitempty"`
	ReceiptRef string   `json:"receipt_ref,omitempty"`
	Reason     string   `json:"reason,omitempty"`
}

// SetJobCompletionHandler installs the runtime's handler for ended jobs.
func (e *Executor) SetJobCompletionHandler(handler func(JobCompletion)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.jobDone = handler
}

// StartJob starts a job in the run's capsule. One job runs at a time per
// capsule; the id must be new.
func (e *Executor) StartJob(ctx context.Context, agentRunID, handle string, req JobRequest) (yaegikernel.JobView, error) {
	capability, caps, err := e.resolveOne(agentRunID, handle, "job_start")
	if err != nil {
		return yaegikernel.JobView{}, err
	}
	if capability.AgentRole != RoleEngineering {
		return yaegikernel.JobView{}, fmt.Errorf("capsule jobs are engineering's")
	}
	memoryNote := ""
	spec, err := capsulejob.Spec{ID: req.ID, Owner: agentRunID, Argv: req.Argv, Cwd: req.Cwd, WallClockMS: req.WallClockMS, MemoryMiB: req.MemoryMiB}.Normalize()
	if err != nil {
		return yaegikernel.JobView{}, err
	}
	if mib, note := defaultShotMemory(caps, spec.Argv, spec.Cwd, spec.MemoryMiB); note != "" {
		spec.MemoryMiB = mib
		memoryNote = note
	}
	job := &capsuleJob{memoryNote: memoryNote, agentRunID: agentRunID, handle: handle, capsuleID: caps.ID, spec: spec, notify: req.Notify, done: make(chan struct{})}
	e.mu.Lock()
	if e.jobs == nil {
		e.jobs = map[string]*capsuleJob{}
	}
	if _, taken := e.jobs[spec.ID]; taken {
		e.mu.Unlock()
		return yaegikernel.JobView{}, fmt.Errorf("job %s already exists", spec.ID)
	}
	for _, other := range e.jobs {
		if other.capsuleID != caps.ID {
			continue
		}
		select {
		case <-other.done:
		default:
			e.mu.Unlock()
			return yaegikernel.JobView{}, fmt.Errorf("job %s is still running; one job runs at a time in a capsule: wait for it or choir.Stop it", other.spec.ID)
		}
	}
	e.jobs[spec.ID] = job
	hm := e.hostMemory
	e.mu.Unlock()
	// A build-class job reads the host facts before it starts (S4): while
	// the host is under memory pressure or has less than its own need (a
	// declared memory_mib, else one build's peak) over its floor, it waits, visibly, and starts when that clears.
	if hm != nil && caps.Spec.SizeClass.HostReservationBytes() > 0 {
		facts, fresh := hm.Facts()
		if reason := jobWaitReason(job, facts, fresh, e.capsuleReservationCredit(caps, facts)); reason != "" {
			e.mu.Lock()
			job.waiting = true
			job.status = capsulejob.Status{ID: spec.ID, State: capsulejob.StateWaiting, Argv: append([]string(nil), spec.Argv...),
				Cwd: spec.Cwd, WallClockMS: spec.WallClockMS, Reason: reason + jobHoldersNote(e.jobs, job, time.Now())}
			view := job.view(job.status, "", "")
			e.mu.Unlock()
			e.persistJobRecord(job)
			log.Printf("capsule: job %s in capsule %s %s", spec.ID, caps.ID, reason)
			go e.waitForHostMemory(job, hm, time.Now(), hostMemoryWaitPoll, hostMemoryMaxWait)
			return view, nil
		}
	}
	view, err := e.launchJob(ctx, job, capability, caps)
	if err != nil {
		e.mu.Lock()
		delete(e.jobs, spec.ID)
		e.mu.Unlock()
		return yaegikernel.JobView{}, err
	}
	return view, nil
}

// launchJob starts a registered job in its capsule: its memory need, the
// workspace digest, the broker's start, then its watcher. On error nothing
// it took is kept; the caller decides what the job becomes.
func (e *Executor) launchJob(ctx context.Context, job *capsuleJob, capability *Capability, caps *Capsule) (yaegikernel.JobView, error) {
	spec := job.spec
	abandon := func(err error) (yaegikernel.JobView, error) {
		e.returnJobMemory(job)
		return yaegikernel.JobView{}, err
	}
	// A memory need raises the capsule against the guest's live budget
	// (docs/archive/reports/memory-hotplug-controller-design-2026-10-14.md section 9).
	if spec.MemoryMiB > 0 {
		if err := e.boostJobMemory(caps.ID, spec.ID, int64(spec.MemoryMiB)<<20); err != nil {
			return abandon(fmt.Errorf("job %s not admitted: %w; start it without memory_mib to run within the capsule's own size", spec.ID, err))
		}
		job.boosted = true
	}
	var err error
	if job.startDigest, err = digestCapsuleWorktree(ctx, caps); err != nil {
		return abandon(fmt.Errorf("job %s: digest the workspace: %w", spec.ID, err))
	}
	st, err := caps.Job(ctx, capability, "job_start", JobParams{ID: spec.ID, Argv: spec.Argv, Cwd: spec.Cwd, WallClockMS: spec.WallClockMS})
	if err != nil {
		return abandon(fmt.Errorf("job %s: %w", spec.ID, err))
	}
	e.mu.Lock()
	job.status = st
	e.mu.Unlock()
	e.persistJobRecord(job)
	go e.watchJob(job)
	return job.view(st, "", ""), nil
}

// capsuleReservationCredit is what caps's admitted host reservation still
// holds for its job: the reservation less the capsule cgroup's usage now,
// at most what the host still holds for this computer's builds. The
// cgroup is read here, in the guest, because only it splits the
// computer's builds by capsule; the host's figure caps it, so a capsule
// never spends a reservation the host no longer holds.
func (e *Executor) capsuleReservationCredit(caps *Capsule, facts HostMemoryFacts) int64 {
	reservation := caps.Spec.SizeClass.HostReservationBytes()
	if reservation <= 0 || facts.OwnReservedBytes <= 0 {
		return 0
	}
	cg := e.cgroupMemoryOf(caps)
	if cg == nil {
		return 0
	}
	usage, err := cg.Current()
	return UnrealisedReservation(reservation, usage, err == nil, facts.OwnReservedBytes)
}

// claimWaitingJob ends a job's wait: true for the one caller that clears it.
func (e *Executor) claimWaitingJob(job *capsuleJob) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !job.waiting {
		return false
	}
	job.waiting = false
	return true
}

// waitForHostMemory starts a waiting job once the host facts clear, or ends
// it: stopped when its capsule or its authority ends first, or when it has
// waited hostMemoryMaxWait; failed when its start fails.
func (e *Executor) waitForHostMemory(job *capsuleJob, hm HostMemory, since time.Time, poll, maxWait time.Duration) {
	ticker := time.NewTicker(poll)
	defer ticker.Stop()
	for range ticker.C {
		e.mu.RLock()
		caps := e.capsules[job.capsuleID]
		waiting := job.waiting
		e.mu.RUnlock()
		if !waiting {
			return // stopped meanwhile
		}
		state := StateDestroyed
		if caps != nil {
			caps.mu.RLock()
			state = caps.State
			caps.mu.RUnlock()
		}
		switch state {
		case StateActive:
		case StateDestroying, StateDestroyed:
			if e.claimWaitingJob(job) {
				e.finishJob(job, nil, capsulejob.StateStopped, "the capsule ended before the job did")
			}
			return
		default:
			continue // frozen for a Freeze, or quiescing: never start now
		}
		capability, err := e.ResolveCapability(job.agentRunID, job.handle)
		if err != nil {
			if e.claimWaitingJob(job) {
				e.finishJob(job, nil, capsulejob.StateStopped, "the assignment's capsule authority ended before the job did")
			}
			return
		}
		facts, fresh := hm.Facts()
		reason := jobWaitReason(job, facts, fresh, e.capsuleReservationCredit(caps, facts))
		if reason != "" && time.Since(since) < maxWait {
			e.mu.Lock()
			job.status.Reason = reason + jobHoldersNote(e.jobs, job, time.Now())
			e.mu.Unlock()
			continue
		}
		if !e.claimWaitingJob(job) {
			return
		}
		if reason != "" {
			e.finishJob(job, nil, capsulejob.StateStopped, fmt.Sprintf("waited %s for host memory and the host stayed short (%s); start it again later", maxWait, reason))
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		_, err = e.launchJob(ctx, job, capability, caps)
		cancel()
		if err != nil {
			e.finishJob(job, nil, capsulejob.StateFailed, "could not start after waiting for host memory: "+err.Error())
			return
		}
		log.Printf("capsule: job %s in capsule %s started after waiting %s for host memory", job.spec.ID, job.capsuleID, time.Since(since).Round(time.Second))
		return
	}
}

// jobHoldersNote names the jobs on this computer that run now, so a wait for
// host memory says what its own computer may be holding: id, command head,
// declared MiB, age. Waiting and ended jobs hold nothing. The caller holds
// the executor's mu.
func jobHoldersNote(jobs map[string]*capsuleJob, self *capsuleJob, now time.Time) string {
	var held []string
	for _, j := range jobs {
		if j == self || j.waiting {
			continue
		}
		select {
		case <-j.done:
			continue
		default:
		}
		head := strings.Join(j.spec.Argv, " ")
		if len(head) > 60 {
			head = head[:60] + "..."
		}
		entry := fmt.Sprintf("%s (%s", j.spec.ID, head)
		if j.spec.MemoryMiB > 0 {
			entry += fmt.Sprintf(", %d MiB", j.spec.MemoryMiB)
		}
		if !j.status.StartedAt.IsZero() {
			entry += ", running " + now.Sub(j.status.StartedAt).Round(time.Minute).String()
		}
		held = append(held, entry+")")
	}
	if len(held) == 0 {
		return ""
	}
	sort.Strings(held)
	return "; jobs running on this computer now: " + strings.Join(held, ", ") + "; if one is yours and no longer needed, choir.Stop its id"
}

// StopJob asks the run's job id to stop.
func (e *Executor) StopJob(ctx context.Context, agentRunID, handle, id string) (yaegikernel.JobView, error) {
	capability, caps, err := e.resolveOne(agentRunID, handle, "job_stop")
	if err != nil {
		return yaegikernel.JobView{}, err
	}
	e.mu.RLock()
	job := e.jobs[id]
	e.mu.RUnlock()
	if job == nil || job.agentRunID != agentRunID || job.capsuleID != caps.ID {
		return yaegikernel.JobView{}, fmt.Errorf("no job %q in this capsule", id)
	}
	// A job still waiting for host memory never reached the broker: it
	// ends here.
	if e.claimWaitingJob(job) {
		e.finishJob(job, nil, capsulejob.StateStopped, "stopped while waiting for host memory")
		e.mu.RLock()
		view := job.view(job.status, "", "")
		e.mu.RUnlock()
		return view, nil
	}
	st, err := caps.Job(ctx, capability, "job_stop", JobParams{ID: id})
	if err != nil {
		return yaegikernel.JobView{}, fmt.Errorf("job %s: %w", id, err)
	}
	return job.view(st, "", ""), nil
}

// RunningJob reports the run's running job, if it has one, and when its
// wall clock ends.
func (e *Executor) RunningJob(agentRunID string) (yaegikernel.JobView, time.Time, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, job := range e.jobs {
		if job.agentRunID != agentRunID {
			continue
		}
		select {
		case <-job.done:
			continue
		default:
		}
		started := job.status.StartedAt
		if started.IsZero() {
			started = time.Now()
		}
		return job.view(job.status, "", ""), started.Add(job.spec.WallClock()), true
	}
	return yaegikernel.JobView{}, time.Time{}, false
}

// RunningJobInCapsule reports a running job of any run on the capsule: what
// writes the capsule's tree, whoever started it. RunningJob answers for one
// run only.
func (e *Executor) RunningJobInCapsule(capsuleID string) (yaegikernel.JobView, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, job := range e.jobs {
		if job.capsuleID != capsuleID {
			continue
		}
		select {
		case <-job.done:
			continue
		default:
		}
		return job.view(job.status, "", ""), true
	}
	return yaegikernel.JobView{}, false
}

// JobViews lists the run's jobs, oldest first, as a cell sees them.
func (e *Executor) JobViews(agentRunID string) []yaegikernel.JobView {
	e.mu.RLock()
	var jobs []*capsuleJob
	for _, job := range e.jobs {
		if job.agentRunID == agentRunID {
			jobs = append(jobs, job)
		}
	}
	views := make([]yaegikernel.JobView, 0, len(jobs))
	sort.Slice(jobs, func(i, k int) bool { return jobs[i].status.StartedAt.Before(jobs[k].status.StartedAt) })
	for _, job := range jobs {
		views = append(views, job.view(job.status, job.receiptRef, job.logRef))
	}
	e.mu.RUnlock()
	return views
}

// watchJob reads a running job until it ends or its capsule does.
func (e *Executor) watchJob(job *capsuleJob) {
	ticker := time.NewTicker(jobPollInterval)
	defer ticker.Stop()
	for {
		if e.pollJob(job) {
			return
		}
		<-ticker.C
	}
}

// pollJob reads the job once; it reports whether the job is finished.
func (e *Executor) pollJob(job *capsuleJob) bool {
	e.mu.RLock()
	caps := e.capsules[job.capsuleID]
	e.mu.RUnlock()
	if caps == nil {
		e.finishJob(job, nil, capsulejob.StateStopped, "the capsule ended before the job did")
		return true
	}
	caps.mu.RLock()
	state := caps.State
	caps.mu.RUnlock()
	switch state {
	case StateDestroying, StateDestroyed:
		e.finishJob(job, nil, capsulejob.StateStopped, "the capsule ended before the job did")
		return true
	case StateActive:
	default:
		return false // frozen for a Freeze, or quiescing: read it later
	}
	capability, err := e.ResolveCapability(job.agentRunID, job.handle)
	if err != nil {
		e.finishJob(job, nil, capsulejob.StateStopped, "the assignment's capsule authority ended before the job did")
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	st, err := caps.Job(ctx, capability, "job_status", JobParams{ID: job.spec.ID})
	if err != nil {
		return false
	}
	e.mu.Lock()
	job.status = st
	e.mu.Unlock()
	if !st.Terminal() {
		return false
	}
	e.finishJob(job, caps, st.State, "")
	return true
}

// finishJob closes a job and hands the runtime the completion: with its
// capsule, it stores the log and mints the receipt first; without one, the
// job closes with state and reason and no receipt.
func (e *Executor) finishJob(job *capsuleJob, caps *Capsule, state, reason string) {
	now := time.Now().UTC()
	var receipt *ExecutionReceipt
	e.mu.Lock()
	st := job.status
	e.mu.Unlock()
	if caps == nil {
		st.State, st.Reason = state, reason
		if st.FinishedAt.IsZero() {
			st.FinishedAt = now
		}
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		endDigest, err := digestCapsuleWorktree(ctx, caps)
		logRef := e.storeJobLog(caps, st)
		cancel()
		if err != nil {
			log.Printf("capsule: job %s: digest the workspace at its end: %v", st.ID, err)
		}
		minted, canonical, mintErr := MintJobReceipt(JobReceiptInput{
			AgentRunID: job.agentRunID, Handle: job.handle, CapsuleID: caps.ID, SourceTreeDigest: caps.SourceSnapshotDigest,
			StartWorktreeDigest: job.startDigest, EndWorktreeDigest: endDigest, LogRef: logRef, Status: st, ObservedAt: now,
		})
		if mintErr == nil {
			mintErr = e.persistReceiptArtifact("execution", minted.ReceiptRef, canonical)
		}
		if mintErr != nil {
			log.Printf("capsule: job %s: mint its receipt: %v", st.ID, mintErr)
		} else {
			receipt = &minted
			e.mu.Lock()
			e.executionReceipts[minted.ReceiptRef] = minted
			job.logRef = logRef
			e.mu.Unlock()
		}
	}
	e.mu.Lock()
	job.status = st
	if receipt != nil {
		job.receiptRef = receipt.ReceiptRef
	}
	handler := e.jobDone
	view := job.view(st, job.receiptRef, job.logRef)
	e.mu.Unlock()
	// Memory first, while the job still reads as running: no next job in
	// the capsule can raise the cgroup while this one is lowered.
	e.returnJobMemory(job)
	close(job.done)
	e.persistJobRecord(job)
	// A job closed with its capsule has no receipt; the runtime still learns
	// it ended, so a turn waiting on it is released.
	if handler != nil {
		handler(JobCompletion{AgentRunID: job.agentRunID, CapsuleID: job.capsuleID, Notify: job.notify, View: view, Status: st, Receipt: receipt})
	}
}

// returnJobMemory returns a job's capsule boost, if it took one.
// endJobMemory is idempotent.
func (e *Executor) returnJobMemory(job *capsuleJob) {
	if job.boosted {
		e.endJobMemory(job.capsuleID, job.spec.ID)
	}
}

// storeJobLog copies the job's log out of the capsule into the artifact
// store under its sha256, and returns its ref. The file is opened beneath
// the capsule root without following any symlink (a cell can write there),
// and kept only when its digest is the one the broker computed from the
// stream; otherwise the receipt still binds the broker's digest and no log
// is kept.
func (e *Executor) storeJobLog(caps *Capsule, st capsulejob.Status) string {
	if st.OutputSHA256 == "" {
		return ""
	}
	rootFD, err := unix.Open(caps.MergedDir, unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return ""
	}
	defer unix.Close(rootFD)
	rel := strings.TrimPrefix(filepath.Join(capsulejob.Root, st.ID, capsulejob.LogFile), "/")
	in, err := openBeneath(rootFD, rel)
	if err != nil {
		return ""
	}
	defer in.Close()
	if info, err := in.Stat(); err != nil || !info.Mode().IsRegular() || info.Size() > capsulejob.MaxLogBytes {
		return ""
	}
	dir := filepath.Join(e.artifactRoot(), "job-logs")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return ""
	}
	tmp, err := os.CreateTemp(dir, ".log-*")
	if err != nil {
		return ""
	}
	defer os.Remove(tmp.Name())
	hash := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, hash), io.LimitReader(in, capsulejob.MaxLogBytes+1)); err != nil {
		_ = tmp.Close()
		return ""
	}
	if err := tmp.Close(); err != nil {
		return ""
	}
	sum := hex.EncodeToString(hash.Sum(nil))
	if sum != st.OutputSHA256 {
		log.Printf("capsule: job %s: its log changed after the broker wrote it; not stored", st.ID)
		return ""
	}
	_ = os.Chmod(tmp.Name(), 0o400)
	if err := os.Rename(tmp.Name(), filepath.Join(dir, sum+".log")); err != nil {
		return ""
	}
	return "capsule-job-log:sha256:" + sum
}

func (e *Executor) jobRecordDir() string { return filepath.Join(e.artifactRoot(), "jobs") }

// persistJobRecord writes the job's durable record (best effort).
func (e *Executor) persistJobRecord(job *capsuleJob) {
	e.mu.RLock()
	st := job.status
	rec := jobRecord{ID: job.spec.ID, AgentRunID: job.agentRunID, CapsuleID: job.capsuleID, State: st.State,
		Argv: job.spec.Argv, Cwd: job.spec.Cwd, ReceiptRef: job.receiptRef, Reason: st.Reason}
	e.mu.RUnlock()
	if !st.StartedAt.IsZero() {
		rec.StartedAt = st.StartedAt.UTC().Format(time.RFC3339Nano)
	}
	if !st.FinishedAt.IsZero() {
		rec.FinishedAt = st.FinishedAt.UTC().Format(time.RFC3339Nano)
	}
	if err := os.MkdirAll(e.jobRecordDir(), 0o700); err != nil {
		return
	}
	if err := capsulejob.WriteJSONAtomic(e.jobRecordDir(), rec.ID+".json", rec); err != nil {
		log.Printf("capsule: job %s: persist its record: %v", rec.ID, err)
	}
}

// closeInterruptedJobs is the restart rule for jobs (CLAUDE.md): a job a
// previous guest process left open ended with its capsule, so it is closed
// as interrupted by a restart, never resumed. It returns the ids it closed.
func (e *Executor) closeInterruptedJobs() []string {
	dir := e.jobRecordDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var closed []string
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		var rec jobRecord
		if json.Unmarshal(raw, &rec) != nil || rec.ID+".json" != entry.Name() || capsulejob.Terminal(rec.State) {
			continue
		}
		rec.State, rec.Reason, rec.FinishedAt = capsulejob.StateInterrupted, "interrupted by a restart", now
		if err := capsulejob.WriteJSONAtomic(dir, entry.Name(), rec); err == nil {
			closed = append(closed, rec.ID)
		}
	}
	if len(closed) > 0 {
		log.Printf("capsule: closed %d job(s) open before this boot as interrupted by a restart: %s", len(closed), strings.Join(closed, ", "))
	}
	return closed
}

// view projects the job for a cell, with its memory claim when the executor
// defaulted it.
func (j *capsuleJob) view(st capsulejob.Status, receiptRef, logRef string) yaegikernel.JobView {
	v := jobView(st, receiptRef, logRef)
	v.MemoryMiB, v.MemoryNote = j.spec.MemoryMiB, j.memoryNote
	return v
}
