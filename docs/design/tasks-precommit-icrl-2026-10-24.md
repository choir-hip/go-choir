# Background Tasks, Mandatory Precommits and In-Context Reinforcement Learning (2026-10-24)

Status: **design, green.** Nothing here is built. Written on lane
`lane/fp0-icrl-design` from `origin/main` 5ca8d8f3b, with the owner's framing
in `docs/precommitment-records.md` §1.0 (fe56626d8, on main after the branch
point) read as well.

Authority: the owner's rulings of 2026-10-24 in the launch first pass ledger
(`docs/definitions/choir-launch-first-pass-metamission-2026-10-21.ledger.md`,
entries 17:5x and 18:2x), and the coordinator's addendum of the same day
(research searches are a first-class case, not a later fold-in). The design
inherits [Choir Doctrine](../choir-doctrine.md) and
[precommitment-records.md](../precommitment-records.md). It is not doctrine;
§12 proposes the doctrine text change the rulings require.

Starting point:
[psa-precommitment-icrl-status-2026-10-24.md](psa-precommitment-icrl-status-2026-10-24.md)
(what is built). Every claim about code is marked **[V]** (verified by
reading the code at the branch point, with file:line) or **[H]**
(hypothesis, to be checked by the slice that depends on it).

---

## 0. Answer in brief

1. **One primitive, one record shape.** A background task is a precommit
   record on the commitment ledger (`task:<id>:start`), carrying what the
   task runs and what the desk expects of it, followed by an end record and
   at most two settlement records (one by rule, one by System1). Engineering's
   capsule jobs (`choir.Start`) are the first kind; research's searches
   (`choir.SearchStart`) are the second, with their own slice. Every new
   record is a `choir.commitment_record` object on the object graph and tape.
   There is no new table and no new store, so replay eligibility does not
   change.
2. **Precommit is mandatory and in one act.** `choir.Start(argv, opts,
   expect)` and `choir.SearchStart(query, expect)` take the claims as an
   argument. The reducer (for Start) or the host (for SearchStart) writes
   the start record first; if that write fails, the task does not start.
   There is no way to start a task without claims.
3. **Two kinds of claim.** Rule claims come from a small closed vocabulary
   and settle from the task's receipt when it ends (exit code and state,
   duration, whether the tree changed, and later the changed paths; for
   searches, the number of distinct results and whether a named domain
   appears). Free-form claims carry a statement and a criterion, and a host
   step acting for management scores them at the task's end with the System1
   model (Jev today), blind to the claimant's probability, identity and
   model.
4. **Texture sees it at once.** At start, the runtime commits an observation
   record ("engineering expects X with p") to the supervising document's
   Texture inbox and wakes Texture. At the end it commits a second one with
   the receipt, the rule settlements and the System1 score. Texture writes
   the revisions, so it stays the sole writer of document text. That makes
   two Texture wakes per engineering task.
5. **Engineering learns from aggregates.** The capsule cell frame gets the
   acting pack it has always lacked. The digest reuses `internal/calibration`
   and `calibration_feedback.go` and adds two things: engineering
   observations and a grouping by settler. The digest shows n, the share
   that held, mean p and Brier per claim kind and per settler. It is shown
   only at n ≥ 20 (bins at n ≥ 5), never includes the 10% held-out set,
   leaves out the assignment being worked on, and never carries a System1
   verdict on a single item. It goes in a user turn after the history, so
   the prompt cache holds. Arms are assigned by hashing the assignment.
6. **Several tasks at once.** The one-job rule (executor and broker) is
   replaced by memory admission, which already exists per job: at most 4
   running tasks per run, and at most one task with no declared memory.
   Freeze still refuses while any task runs.
7. **Long messages go by reference.** A record carries `Refs` to exact
   immutable versions: a Texture revision id, an `artifact:sha256:` blob, or
   a store record id. Each desk gets one read verb, `choir.ReadRef`, for
   references addressed to it. Engineering also sees them read-only at
   `/input/<ref>`. An act body over 4 KiB is moved automatically into a body
   record and replaced by a summary plus a reference. The 16 KiB cut and its
   truncation marker in `engineering_cast_request.go` are deleted.
8. **The capsule reads the computer's files.** `files/` is bind-mounted
   read-only at `/computer/files`, on b@c first, behind a per-computer
   setting. Signing keys, credentials, the gateway token and raw store files
   are siblings of `files/`, so they are outside the mount by construction.
   Two things are named in §8: Landlock cannot make a subtree of an allowed
   path read-only, and reading files plus egress together is an exfiltration
   path. Texture documents are not files. They are reached through
   `choir.ReadRef`, not through the mount.
9. **The doctrine change** (§12): "scores stay off the acting desk's
   context" becomes "a desk may see aggregates of its own settled record,
   never a model scorer's verdict on an item, never a verdict on work in
   progress, with a held-out tenth". The change touches four places: I26 in
   doctrine, the product doctrine, and §1.2, §4.7 and §5.4 of
   precommitment-records.

The build is ten slices (§10), each landable alone. The first slice that
lets engineering learn is S1 (orange, behind a setting). The first that
makes precommits mandatory is S2 (red, tier 2).

---

## 1. What is built, and what is missing

### 1.1 Tasks today (capsule jobs)

- `choir.Start(argv, opts)` stages a job intent in the tray and returns an
  id. The executor starts it after the cell returns. Start, Stop and Poll
  belong to engineering only. [V] `internal/yaegikernel/jobs.go:166-230`.
- **One job runs at a time per capsule**, refused twice: in the executor
  [V] `internal/capsule/jobs.go:113` and in the broker's manager [V]
  `internal/capsulejob/manager.go:91`.
- **Memory is already per job.** A declared `memory_mib` is admitted against
  the live budget and raises the capsule cgroup through a boost keyed by job
  id. [V] `internal/capsule/job_memory_linux.go` (`boostJobMemory`,
  `caps.memoryBoosts[jobID]`); the admission hook is at [V]
  `jobs.go:68-80`.
- **A job's end is already a ledger record.** `handleCapsuleJobDone`
  commits a system report `job:<id>:ended` with a `CommitmentJob` body
  (state, exit code, argv, receipt ref, log ref, tail) into
  `engineering:<assignment>` and wakes the run. [V]
  `internal/agentcore/capsule_jobs.go:130-187`; [V]
  `internal/types/commitment_job.go:11-34`.
- **A job's start is not on the ledger.** The durable start record is a JSON
  file under the capsule artifact root (`jobs/<id>.json`), which the boot
  closes as "interrupted by a restart". [V] `jobs.go:54-66, 398-454`. That
  gives job state two authorities: the file and the inbox end record.
- **The receipt** binds argv, cwd, the environment digest, exit code, output
  digest, times, job state, and the worktree digests at start and at end.
  It holds **no changed-path list**. [V] `internal/capsule/types.go:216-241`;
  `internal/capsule/job_receipt.go:60-70`; `digestCapsuleWorktree` hashes
  `/workspace/platform` only, [V] `internal/capsule/executor.go:1274-1276`.
- **Tasks are invisible to supervision.** No record reaches Texture at
  start or end. [V] by reading `capsule_jobs.go` (the only inbox it writes
  is the engineering inbox, :173). The Texture supervision document stops at
  the decision boundary.
  [V] `docs/problems/texture-supervision-doc-stops-at-the-decision-boundary-2026-10-24.md`.

### 1.2 Precommits today

- Engineering's rule precommits: six closed kinds settled from the Freeze,
  from management's Verify and at the assignment's end, by
  `system:rule`, with one resolve id per precommit (first one wins). [V]
  `internal/types/commitment_rules.go:72-104, 478-481`; settlement runtime at
  [V] `internal/agentcore/engineering_precommit_rules.go:166-246`. Failure
  modes R1 to R12 are at `commitment_rules.go:26-70` [V].
- A rule precommit is per assignment, not per task. The design note for
  long exec listed "a rule-settled precommitment on job start" as residual
  R4. [V] `docs/design/capsule-long-exec-2026-10-15.md` §5.
- `RulePrecommitSayDo` computes Brier per rule kind. It has no non-test
  caller. [V] `commitment_rules.go:616`; status doc §2.3.

### 1.3 System1 today

- `choir.System1` belongs to research only. The host refuses every other
  session role. [V] `internal/agentcore/system1_verb.go:134-137`.
- It sends closed choice questions with context passed **by reference**
  (capture ids, story revisions) to the one `CandidateJudge`, which is Jev
  through the gateway. Each call is recorded on the tape as
  `system1.judgment` with the phases requested, answered and failed.
  Replays of the same run and digest are free. Calls are capped per run and
  per day, and a cap is off until it is configured. [V]
  `system1_verb.go:26-52, 140-300`.
- The model id recorded is the judge's reported model, falling back to
  `~typesafe/jev-latest`. [V] `internal/agentcore/support_judge.go:37`.
  "latest" is not a pinned version (§6.4).
- Context per reference is bounded to 1,000 runes. [V] `system1_verb.go:63`.

### 1.4 The calibration digest today

- Desks: management, Texture, research. Engineering maps to "", so it has no
  digest. [V] `internal/agentcore/calibration_feedback.go:78-87`.
- Arithmetic: Brier, ECE over five bins, mean p against observed, a window
  of 400, nothing under n = 20, no bin under n = 5, a 10% held-out set by
  hash, and arms by hash of the thread. [V]
  `internal/calibration/calibration.go:64, 93-107, 159-235`.
- Grouping is by claim type and by model. **There is no grouping by
  settler.** [V] `calibration.go:200-230` (`ClaimTypes`, `Model`).
- Placement: a user turn headed `Calibration digest:` after the history,
  re-sent only when the text changes. [V]
  `calibration_feedback.go:360-407`.
- Snapshot: up to 20,000 commitment records listed per computer every
  30 minutes. [V] `calibration_feedback.go:41, 293-322`.
- Switch: `calibration.feedback`, default false. [V]
  `internal/store/computer_setting.go:36, 77`. The owner lifted "stays off"
  on 2026-10-24 (ledger 18:2x).

### 1.5 The acting pack in the capsule

- `choir.Pack()` returns the frame's pack, or an empty one. [V]
  `internal/yaegikernel/choir.go:1296`.
- The capsule eval request has no pack field. [V]
  `internal/capsule/types.go:155-172` (Source, Cwd, AllowedPackages,
  TimeoutMS, Inbox, Updates, Records, Jobs).
- The broker builds the session frame without a pack. [V]
  `cmd/capsule-broker/session_worker.go:227`.
- So engineering's pack is always empty, although its prompt says it shows
  how each claim settled. The status doc records this (§2.3). It still
  needs its own problem record before any code (S0).

### 1.6 Messages and references today

- A desk act body is capped at 16 KiB in the tray and 256 KiB per cell.
  [V] `internal/yaegikernel/intent.go:160-167`; the reducer check is at [V]
  `internal/agentcore/rlm_reduce.go:137`.
- Engineering's view of a Texture Cast cuts the objective and the body at
  16 KiB each and appends a marker that names the parent record. [V]
  `internal/agentcore/engineering_cast_request.go:29-36, 74-92, 135-138`.
  The full Cast record is already durable on the ledger. The cut is only in
  the view.
- Engineering has no read-by-reference verb. Its verbs are WriteFile, Exec,
  Start, Poll, Stop, Complete, Freeze, Precommit, Report, Ask, Reply and
  Note. [V] `internal/deskmanifest/manifest.go:258`.
- Read verbs by id exist elsewhere: `ReadCapture`, `ReadEvidence`,
  `RunMemoryEntry` and `ReadDoc` (Texture's own document). [V]
  `choir.go:616, 667, 681, 1122`.
- Content-addressed refs exist: `artifact:sha256:` (imageblob) [V]
  `internal/imageblob/imageblob.go:1-27`, and `capsule-job-log:sha256:` [V]
  `jobs.go:396`.

### 1.7 The capsule filesystem today

- The overlay's lowest layer is the guest root,
  `CHOIR_CAPSULE_LOWER_ROOT = "/"`, at [V] `nix/autoputer-vm.nix:1061`. The
  executor puts the source snapshot on top of it, [V] `executor.go:612`.
- `/mnt/persistent` is a separate virtio-blk mount [V]
  `nix/autoputer-vm.nix:315-323`, which an overlay of `/` does not cross.
  It is masked in the capsule anyway: `prepareCapsuleRoot` mounts a tmpfs
  over `run`, `mnt`, `proc` and `sys` [V] `executor.go:2357-2372`.
- The read-only input pattern already exists. The verifier bundle, the
  adoption input and the carried patch are bind-mounted and then remounted
  with `MS_RDONLY|MS_NOSUID|MS_NODEV` over paths in the merged root. [V]
  `executor.go:629-680`.
- **Landlock grants the job the whole chroot.** The job-exec stage applies
  `capsule.NewWorkloadLandlock("/")` inside the chroot [V]
  `cmd/capsule-broker/jobs.go:74`, and that grants full directory access
  (read, write, make, remove, execute) to every listed path [V]
  `internal/capsule/landlock.go:49-62, 80-92`. Landlock rights only add up
  down a hierarchy, so no rule can make a subtree of `/` read-only. The
  read-only guarantee for a mount inside the capsule therefore comes from
  the read-only bind and the capability drop, not from Landlock. This
  corrects the brief's "Landlock read-only" (§8.2).
- `files/` is `/mnt/persistent/files` (`AUTOPUTER_FILES_ROOT`) [V]
  `nix/autoputer-vm.nix:1022`. It holds the Files app tree, including
  `Source/platform` [V] `:1060` and `System/model-policy.toml` [V]
  `internal/provideriface/config.go:88-93`.
- The secrets are siblings of `files/`, not inside it:
  `/mnt/persistent/choir-signers/*`, `/mnt/persistent/choir-credentials` [V]
  `nix/autoputer-vm.nix:598-605`, and `/run/choir-bootstrap/gateway-token`
  [V] `:1043`.
- **Texture documents are not files.** Revisions live in the object graph
  (`GetTextureRevisionOG`) [V] `internal/store/graph_store.go:2526`. A files
  mount does not give the capsule Texture documents.

### 1.8 What the tape already guarantees

- Commitment records are `choir.commitment_record` objects written under a
  not-exists condition, so a replay mints nothing. [V]
  `internal/store/commitment.go:16-52`.
- `og_objects` is an event projection in the replay manifest. [V]
  `internal/agentcore/replay_eligibility.go:93`.
- `store/commitment.go` does not decode with `DisallowUnknownFields` [V]
  (by grep). An older binary therefore reads a record that carries a new
  body field [H: holds for every reader; S2's review checks the readers].

---

## 2. Data model

Rule: **every new fact is a commitment record.** It uses existing kinds
(precommit, report, resolve) and adds body fields only. There are no new
kinds, no new tables and no new op kinds. This is the opposite of D39,
whose new op kinds made a revert unsafe. Here an older binary ignores the
new fields, so a revert is a clean rollback [H, checked in S2 review].

### 2.1 Identifiers

| Record | Id | Kind | Addressed to | Writer |
|---|---|---|---|---|
| Task start, with claims | `task:<id>:start` | precommit | nobody (ledger only) | the reducer (Start) or the host (SearchStart) |
| Capsule task end | `job:<id>:ended` (exists) | report, system | `engineering:<assignment>` | `system:jobs` |
| Search task end | `task:<id>:ended` | report, system | nobody | `system:tasks` |
| Rule settlement | `task:<id>:settle:rule` | resolve | nobody | `system:rule` |
| System1 score | `task:<id>:score:system1` | resolve | nobody | `management:system1` |
| Texture observation at start | `task:<id>:texture:start` | report, system | `texture:<doc>` | `system:tasks` |
| Texture observation at end | `task:<id>:texture:end` | report, system | `texture:<doc>` | `system:tasks` |
| Body of a long act | `<record_id>:body` | report, system | nobody | the reducer |

The task id is the job id for capsule tasks (`capsulejob.NewID()`, which
exists) and the search id for searches. Every id is deterministic, so a
retried write replays. A second settler writing the same id loses (R4's
rule, reused).

**Why one start record holds all the claims, not one record per claim.**
Engineering's assignment-level rule precommits use one record per claim
(`commitment_rules.go`). Tasks are more numerous: one engineering
assignment runs tens of jobs, and research runs many searches per story
[H: the paper's search rate is not measured; §9.4]. With one record per
claim and one resolve per claim, a task with four claims costs nine
records. The compact shape costs four: start, end, rule settlement and
System1 score. That compact shape uses fields that already exist:
`CommitmentPrediction.Questions` holds several typed questions, and
`CommitmentScore.Answers` holds an answer per question [V]
`internal/types/commitment.go:79-160`. Assignment-level rule precommits
stay as they are.

### 2.2 The task start record

```go
// internal/types/commitment_task.go (new file, body types only)

// CommitmentTask is a background task's start body: what runs, for whom,
// and what the desk expects. The record's Kind is precommit; it is the
// task's precommitment, written before the task starts (one act).
type CommitmentTask struct {
	TaskID       string      `json:"task_id"`
	TaskKind     string      `json:"task_kind"` // "capsule_job" | "search"
	Desk         string      `json:"desk"`      // "engineering" | "research"
	RunID        string      `json:"run_id"`
	AssignmentID string      `json:"assignment_id,omitempty"`
	Argv         []string    `json:"argv,omitempty"`
	Cwd          string      `json:"cwd,omitempty"`
	WallClockMS  int64       `json:"wall_clock_ms,omitempty"`
	MemoryMiB    int         `json:"memory_mib,omitempty"`
	Query        string      `json:"query,omitempty"`
	// Supervisor is the Texture inbox that renders this task
	// (texture:<doc>), resolved by the runtime, never by the model.
	Supervisor string      `json:"supervisor,omitempty"`
	Claims     []TaskClaim `json:"claims"`
	StartedAt  string      `json:"started_at"`
}

// TaskClaim is one expectation, yes/no, with p = p(yes).
type TaskClaim struct {
	ID        string   `json:"id"`   // c1..c8, stamped by the runtime
	Kind      string   `json:"kind"` // a TaskRuleKind, or "free"
	P         float64  `json:"p"`    // [0.02, 0.98], as R3
	Minutes   int      `json:"minutes,omitempty"`
	Paths     []string `json:"paths,omitempty"`
	N         int      `json:"n,omitempty"`
	Domain    string   `json:"domain,omitempty"`
	Statement string   `json:"statement,omitempty"` // free: the claim
	Criterion string   `json:"criterion,omitempty"` // free: what settles it from this task's evidence
}
```

The record's `Provenance.AgentID` is the desk thread
(`engineering:<assignment>`, `research:<doc>`). `ModelID` is the run's
model. `SubjectRefs` is `{task_id, assignment | doc}`.
`Prediction.Hypothesis` is a one-line summary for generic views. For
generic tooling, `Prediction.Questions` mirrors the claims, one question per
claim, with `{yes, no}` and p.

**Validation, fail closed** (reducer or host, before anything runs):

- 1 to 8 claims;
- p is in [0.02, 0.98];
- rule kinds come from the closed list and their parameters are bounded;
- a free claim has both a statement and a criterion (§5.4 of
  precommitment-records: "judged questions need a criterion");
- at most 4 free claims per task (the cost bound, §6.4).

### 2.3 Rule kinds for tasks (closed vocabulary; adding one is a reviewed change)

| Kind | Settles yes when | Evidence | Slice |
|---|---|---|---|
| `exits_zero` | the job ended `done` with exit 0 | receipt exit code and state | S2 |
| `within` | the job ended (any state but interrupted) within `minutes` | receipt start and end times | S2 |
| `tree_unchanged` | the start and end worktree digests are equal | receipt `StartWorktreeDigest`, `WorktreeDigest` | S2 |
| `changes_within` | every changed file lies under `paths` | per-job changed-path list (not built) | S2b |
| `hits_at_least` | the search returned at least `n` distinct URLs | the search's arrivals at stream end | S8 |
| `domain_present` | some hit's host is `domain` or ends in `.domain` | the arrivals | S8 |

`changes_within` needs the executor to list the files that differ between
the start tree and the end tree. Today it computes only a digest [V]
`executor.go:1274`. Whether `digestCanonicalSubjectTree` can return the
list it walks is [H]. Path matching reuses `WithinPathPrefix` and
`NormalizeRulePath` (whole segments, R8) [V] `commitment_rules.go:225-249`.

An interrupted task (a restart, the capsule or session ended) settles every
claim `interrupted`. Calibration excludes those, as §5.4 of
precommitment-records requires ("questions on work a crash closed are
interrupted and excluded").

### 2.4 Settlement and score records

```go
// TaskSettlement is the body of a task's rule settlement or System1 score.
type TaskSettlement struct {
	TaskID  string            `json:"task_id"`
	Settler string            `json:"settler"` // "rule" | "system1"
	Answers []TaskClaimAnswer `json:"answers"`
}

type TaskClaimAnswer struct {
	ClaimID string `json:"claim_id"`
	Answer  string `json:"answer"` // yes | no | unresolved | interrupted
	Excerpt string `json:"excerpt"` // the observation it rests on, bounded
	// System1 only: the judge's distribution as given, never normalized.
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
}
```

Each record is a resolve with `ParentID = task:<id>:start`. `Scores[0]`
carries the scorer id (`rule:task` or the System1 model as reported) and
`Answers` keyed by claim id. `Discrepancy` is set per record to
`unresolved`. The verdict lives per claim in the body, never on the record,
so no generic pack reads a single discrepancy as the task's verdict (the
pattern of the calibration score record, [V]
`calibration_feedback.go:110-113`).

### 2.5 Reference records

```go
// MessageRef names one exact immutable version.
type MessageRef struct {
	Ref       string `json:"ref"`  // texture-rev:<revision_id> | artifact:sha256:<hex> | record:<record_id>
	Kind      string `json:"kind"` // texture_revision | artifact | record
	SHA256    string `json:"sha256"` // digest of the bytes ReadRef returns
	Bytes     int64  `json:"bytes"`
	MediaType string `json:"media_type,omitempty"`
	Summary   string `json:"summary,omitempty"` // <= 280 runes, the sender's
}
```

`CommitmentRecord` gains `Refs []MessageRef`. The existing string
`EvidenceRefs` stays for receipts and sources, and `Refs` is what a desk
may read.

**Body records.** When an act body is over the inline limit, the reducer
keeps the full text as a body record, `<record_id>:body`, with the text in
`Observation.Excerpt`. That text is bounded by the tray's 256 KiB, against
the store's 8 MiB preflight for a projection op (ledger 12:5x). The act
then carries the first 1 KiB, the marker "[continued in <ref>]" and one
`MessageRef{Kind: record}`. The content stays on the tape, so replay,
checkpoint and restore cover it. Nothing goes to a blob store whose
checkpoint coverage is unproven [H: whether `state.image-blobs` travels
with a checkpoint].

### 2.6 How this rides the ledger and tape

- All the records above go through `AppendCommitmentRecord` (ledger only)
  or `CommitInboxRecords` (addressed). Both exist and are idempotent [V]
  `store/commitment.go:24`, `store/inbox.go:638`.
- **Task state is a fold:** a start with no end is running, and an end
  closes it. The fold is the single authority. The executor's in-memory
  table is a cache. After S2, the `jobs/<id>.json` file record is read only
  as a one-release fallback at boot, then **deleted** (it is replaced, not
  extended).
- **Boot closure.** At executor construction, today's
  `closeInterruptedJobs` closes file records [V] `executor.go:272`,
  `jobs.go:425`. After S2, the boot lists open task starts on this computer.
  For each one it writes `job:<id>:ended` (state interrupted, "interrupted
  by a restart") and a rule settlement with every claim `interrupted`.
  Nothing is resumed or re-dispatched, as the CLAUDE.md restart rule
  requires. A planned restart cannot resume a task either, because the
  capsule does not survive any guest restart.

---

## 3. Desk verbs

The style follows the in-cell `choir` module: a method on `*ChoirScope`,
typed bodies passed as `any` and decoded as strict JSON (the
`choir.Precommit(any)` pattern [V] `choir.go:1391`), refusals as errors that
say what to do.

### 3.1 Engineering (capsule)

```go
// Start stages a capsule task running argv with the claims in expect, and
// returns its id. The reducer writes the task's precommitment
// (task:<id>:start) before the executor starts it; a task whose
// precommitment cannot be written never runs. expect is
// {"claims": [{"kind": "exits_zero", "p": 0.8}, {"kind": "within",
// "minutes": 10, "p": 0.7}, {"kind": "free", "statement": "...",
// "criterion": "...", "p": 0.6}]}. opts as today: cwd, minutes,
// memory_mib. Up to 4 tasks run at once in this run; at most one without
// memory_mib.
func (s *ChoirScope) Start(argv []string, opts map[string]any, expect any) (string, error)

// Tasks returns every task of this run as the cell-start snapshot shows
// it, oldest first: state, receipt, tail, and the claims as committed
// (never a settlement's verdict on a free claim).
func (s *ChoirScope) Tasks() []JobView

// Poll and Stop are unchanged.
func (s *ChoirScope) Poll(id string) (JobView, error)
func (s *ChoirScope) Stop(id string) error

// ReadRef returns up to max bytes of a reference addressed to this desk,
// from byte from. A ref is addressed to this desk when a record in its
// inbox, or its own Cast's parent record, names it in Refs. Anything else
// is refused.
func (s *ChoirScope) ReadRef(ref string, from, max int) (RefText, error)

// Pack is unchanged in signature; it is now filled in the capsule (S1).
func (s *ChoirScope) Pack() types.ActingPack
```

```go
// RefText is one page of a reference.
type RefText struct {
	Ref    string `json:"ref"`
	Kind   string `json:"kind"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
	From   int    `json:"from"`
	Text   string `json:"text"`
	More   bool   `json:"more"`
	// Path is where the same bytes are, read-only, in the capsule
	// (engineering only): /input/<file name of ref>.
	Path string `json:"path,omitempty"`
}
```

`JobView` gains `Claims []TaskClaimView{ID, Kind, P, Statement}`, the claims
as committed, and nothing else.

**The breaking change is deliberate.** A cell that calls `choir.Start(argv,
opts)` with two arguments fails to compile in Yaegi, and the error names
the three-argument signature. The engineering overlay
(`internal/runtimeprompts/overlays/rlm_engineering_runtime.yaml`) and the
`choir.Help("Start")` text change in the same slice. The first-cell
assignment precommit stays. Task precommits are additional to it.

### 3.2 Research

```go
// SearchStart starts a search with the claims in expect and returns as
// soon as the host has named the providers. The host writes the search's
// precommitment (task:<id>:start) before it opens the stream; no record,
// no search. expect is {"claims": [{"kind": "hits_at_least", "n": 5,
// "p": 0.9}, {"kind": "domain_present", "domain": "who.int", "p": 0.5},
// {"kind": "free", "statement": "a primary source dated after March",
// "criterion": "a result's title or snippet names ...", "p": 0.4}]}.
func (s *ChoirScope) SearchStart(query string, expect any) (SearchStub, error)

func (s *ChoirScope) ReadRef(ref string, from, max int) (RefText, error)
```

`SearchStart` is a host call (`s.call(ActionSearchStart, ...)`) [V]
`internal/yaegikernel/search_store.go:329-340`, so the host writes the
precommit before it starts the stream [V the start path is host-side,
`internal/agentcore/desk_search.go:160-170`].

### 3.3 Management and Texture

```go
// management
func (s *ChoirScope) ReadRef(ref string, from, max int) (RefText, error)
```

Texture keeps `ReadDoc` and gets no new verb. A reference to a capture is
read through `ReadCapture` as today. Management gets no scoring verb: the
host scores for it (§6). Management may still dispute a score with
`choir.Disagreement`, which exists [V] `choir.go:1446`.

### 3.4 Owner and agent routes (no SSH, standing question 9)

- `GET /api/tasks?desk=&assignment=&state=` lists the fold: start, end,
  settlements, and scores. It is the supervision view and carries scores.
  It is gated like `/api/calibration`.
- `GET /api/calibration` gains the engineering and research digests and
  the per-settler lines [V the route exists, `internal/apihandler/routes.go:34`].
- CLI: `choir task list|show <id>` over that route.

---

## 4. Texture rendering

**Which document.** The runtime resolves the supervising document of an
engineering assignment from its binding (`TrajectoryID`) [V the field
exists, `internal/types/engineering_assignment.go:85`] to the trajectory's
Texture inbox `texture:<doc>`. That the trajectory names its Texture
document is [H]; the problem record suggests addressing "the trajectory's
Texture agent". S3 verifies it first. A task with no resolvable supervisor
still writes all its ledger records and is listed in `/api/tasks`. It is
never silently dropped.

**What arrives.** There are two system records per engineering task,
addressed to `texture:<doc>`. Each wakes Texture through
`WakeRecordsInbox` [V `internal/agentcore/inbox_records.go:870`].

1. At start (`task:<id>:texture:start`): "Engineering started task 3f2a:
   `go test ./internal/agentcore -run TestX`. It expects: exits 0 (p 0.80);
   ends within 10 min (p 0.70); 'the new test fails before the fix'
   (p 0.60)."
2. At end (`task:<id>:texture:end`), written once the rule settlement and
   the System1 score exist, or after 60 s if the score has not come: "Task
   3f2a ended done in 6m12s, exit 0. Held: exits 0, within 10 min. Scored
   by System1: 'the new test fails before the fix': cannot tell from the
   output (unresolved)."

Texture turns these into revisions of the supervision document. It stays
the sole writer of document text (I1). I2 also holds: the records are
deliveries, and how to render them is Texture's choice. The Texture overlay
gets one line: render every task record as a line in the document's
progress section, and keep earlier lines. The materiality projection
already renders records addressed to Texture as evidence entities, with
falsified ones first [V]
`internal/textureowner/texture_evidence_sources.go:690-830`.

**Cost.** Two Texture turns per engineering task. Texture reads every
record up to the head in one turn (the inbox cursor), so several task
records that arrive together cost one turn [V the cursor model,
precommitment-records §4.3; that a burst of records costs one turn is H].

**Research.** Searches do not wake Texture one at a time. A research round
can run tens of searches, and each would cost a Texture turn. Research's
claims reach Texture in aggregate, in the final Report: research adds a
line "searches: n, rule claims held k of m". Open decision 3 asks whether
that is enough.

**Leak guard.** The supervision document carries System1's verdict on the
current task, and engineering must not read it while it works (owner
ruling). Engineering's `ReadRef` reads only exact references addressed to
it. A revision written after the assignment opened is never addressed to
it. Failure mode T15.

---

## 5. Settlement by rule

- **When.** A capsule task settles in `handleCapsuleJobDone`, after the end
  record is committed [V the hook point, `capsule_jobs.go:169-187`]. A
  search settles when its stream ends in the host search table [V the table
  is host authority, `desk_search.go:29-33, 97-110`].
- **How.** The settlement is a pure function,
  `types.SettleTaskRules(task CommitmentTask, obs TaskObservation)
  TaskSettlement`, beside `SettleRuleOnFreeze` and in its style [V
  `commitment_rules.go:300-372`]. It writes one resolve record with a
  not-exists condition.
- **Never in the way.** Settlement is best effort and never fails or delays
  the act it observes (R6, reused).

```go
type TaskObservation struct {
	State           string // done | failed | timeout | stopped | interrupted
	ExitCode        *int
	StartedAt       time.Time
	FinishedAt      time.Time
	StartTree       string
	EndTree         string
	ChangedPaths    []string // nil until S2b
	ReceiptRef      string
	Hits            []string // search: distinct URLs, in arrival order
	ProvidersAnswered int
}
```

---

## 6. Scoring free-form claims with System1

### 6.1 Who and how

- The score is management's act, executed by a host step. No management
  model turn is spent. Provenance is `management:system1`, a scorer
  identity that belongs to management and is distinct from the desk's model
  thread. This follows the owner's "scored by management's System1 model"
  without waking management for every task. Open decision 4 offers the
  alternative.
- The host step is `rt.scoreTaskClaims(ctx, task, obs)`. It reuses the body
  of `system1Egress` [V `system1_verb.go:134-300`]: the same
  `CandidateJudge`, `system1.judgment` tape events, replay by digest,
  per-day caps and failure records. Two changes: it is called by the
  runtime for management, not by a research cell, and it has its own cap
  scope, `task_scores`.
- **One call per task**, with each free claim as one question with the
  choices `yes`, `no` and `cannot_tell`.

### 6.2 What System1 sees

| Sees | Bound |
|---|---|
| Task kind, argv or query, cwd | as recorded |
| State, exit code, duration, the wall-clock bound | numbers |
| Whether the tree changed (and, after S2b, the changed paths) | 32 paths |
| The output tail, and the first 2 KiB of the stored log read by its content address | 4 KiB + 2 KiB |
| For a search: titles, URLs and snippets of distinct hits | 40 hits × 300 runes |
| Each claim's statement and criterion | 1 KiB each |

System1's per-reference context bound rises from 1,000 runes to 4,000 for
these kinds only (`system1ContextRunes` [V `system1_verb.go:63`]).

### 6.3 What System1 may not see

- the claimant's probability (blind, as §5.2 of precommitment-records
  requires for Jev);
- the desk, the assignment, the model or the run id;
- the Cast's objective or the assignment's other claims;
- other scores, including the rule settlement's verdicts (it sees the raw
  exit code instead);
- any document text.

The questions say only "given this task's evidence".

**From distribution to outcome.** Following §5.3 of precommitment-records
("treat scorer probabilities as a monotone score until calibrated"):

- yes when p(yes) ≥ 0.8;
- no when p(no) ≥ 0.8;
- otherwise unresolved, and so is `cannot_tell` ≥ 0.5.

The full distribution is kept. The cut-off is a view parameter, and the
records keep the evidence, so it can be changed later without rewriting
anything.

### 6.4 Cost bound

- At most 4 free claims per task, scored in one call.
- The `task_scores` cap is 500 calls a day per computer by default. It is
  off until it is set or the owner turns on `tasks.system1_scoring`, like
  `research.system1` [V `system1_verb.go:91-110`].
- About 4k input tokens per call. At Jev's 2026-10-11 price of about $0.042
  per million input tokens (precommitment-records §5.3, "needs
  rechecking"), 500 calls cost about $0.08 a day. [H: the price.]
- The rate limit is 60 calls a minute per computer (same source).
- **Model pinning.** The score record stores the model id as the judge
  reported it [V `system1_verb.go:249`]. If the reported id is the alias
  `~typesafe/jev-latest`, scores from different versions cannot be told
  apart, and §5.3 requires calibration "per type and pinned version". S4
  refuses to write a score whose reported model is the alias, and records
  it as `failed`. [H: what Jev actually reports.]

### 6.5 Scoring the scorer

- The held-out tenth (§7.2) is also queued for an independent off-computer
  audit, through `POST /api/calibration/settlements` [V route,
  `routes.go:35`], by a different model family. The owner samples
  disagreements.
- Disagreement between System1 and the audit, per claim kind, is a
  supervision number shown to the owner. It is never shown to a desk.

---

## 7. The calibration digest for engineering (ICRL)

### 7.1 What engineering reads

The digest is reused from `calibration.Compute` and `Render` [V
`calibration.go:159-299`], with three changes:

1. **Observations.** `collectCalibration` [V
   `calibration_feedback.go:152-284`] gains engineering observations from
   the same snapshot, so no extra listing is made:
   - task claims, one observation per claim with a rule or System1 answer
     of yes or no, keyed `task:<id>:<claim>`, ClaimType = the rule kind or
     `free`, Scorers = [`rule`] or [`system1`];
   - assignment rule precommits, the existing six kinds, from the same
     records `RulePrecommitSayDo` reads (it is then used, and no longer
     dead).
   `calibrationDeskOfThread` maps `engineering:` to `engineering`.
2. **Grouping by settler.** `CalibrationDigest` gains `Settlers
   []CalibrationStat` (rule, system1), computed like `ClaimTypes` [V
   `calibration.go:200-216`]. `Render` adds one line per settler at
   n ≥ MinBin: "Settled by rule: you said 0.86, 0.61 held (n=44). Scored by
   System1: you said 0.70, 0.52 held (n=21)."
3. **Exclusion of the work in progress.** `Compute` gains
   `Options.ExcludeThread`. The engineering reader sets it to its own
   `engineering:<assignment>`, so no observation from the open assignment
   enters the digest it reads, even after a snapshot refresh.

Kept as they are:

- Window 400.
- MinN 20: no text under it.
- MinBin 5.
- The held-out tenth by `HeldOut(key)`. Held-out items are counted, never
  shown [V `calibration.go:93-97, 176-186`].
- Text with no times, stable when nothing changed.
- The line for the run's own model at n ≥ 20.
- Brier beside strict Brier (unresolved counted as not held, R11). The
  digest shows both for engineering, because "unresolvable costs as much as
  wrong" for agents that write their own questions (precommitment-records
  §5.4).

### 7.2 What engineering never sees

- A System1 verdict on a single item, in any context: not in `Pack()`, not
  in `Records()`, not in `Poll` or `Tasks()`, not in a turn.
  `BuildActingPack` [V `commitment_views.go:486`] drops resolutions by
  `management:system1` from the join.
- Per item, the acting pack shows only the desk's own acts with **rule**
  settlements and their observation excerpts. Those are observations of
  receipts the desk already holds (R12's reading, [V
  `commitment_rules.go:67-70`]).
- The held-out tenth, in any aggregate.
- Any score of another desk.

### 7.3 Where it goes

- **The pack.** `GoEvalRequest` gains `Pack *types.ActingPack`, filled in
  `tools_capsule.go` next to `req.Records` [V `tools_capsule.go:631-632`]
  from the same builder the host desk cell uses [V `tools_desk.go:934-956`].
  The broker copies it into `SessionFrame.Pack` at
  `session_worker.go:227` [V the frame is built there]. `choir.Pack()` then
  works in the capsule as the overlay already claims.
- **The turn.** A digest turn at activation start, through
  `calibrationDigestInjector` [V `calibration_feedback.go:394-407`], after
  the history, under the `Calibration digest:` header, and re-sent only when
  the text changes. That engineering's capsule run uses the same tool loop
  injection hook is [H]; S1 checks it.
- **Prompt caching.** The system prompt and the tool catalog do not change.
  The digest is the last user turn, so the cached prefix stays
  byte-identical (experiment P1 [V the design,
  `docs/reports/icrl-calibration-experiment-2026-10-14.md` §2]).

### 7.4 Arms and the switch

- **Arms** come from `calibration.Arm("engineering:<assignment>")` [V
  `calibration.go:101-107`], 50/50, so H-ICRL can be read for engineering.
  Open decision 1 asks whether to show it to every assignment instead.
- **Setting** `engineering.track_record`, per computer, default off. It is
  turned on for b@c (computer-6c50d6e5) in S1's proof. It is kept separate
  from `calibration.feedback`, so the other desks' experiment stays clean
  until the owner flips that one too (the ruling lifts the ban; it does not
  flip the switch).
- Rollback is the setting.

### 7.5 What it does not prove

S1 shows that the loop closes. It does not show that engineering learns.
That needs the arms over many assignments: at about 20 settled claims per
arm, the 2026-10-14 pre-registration's analysis applies.

---

## 8. Read-only access to the computer's files

### 8.1 Shape (S7, b@c first)

- **The mount.** The executor bind-mounts `/mnt/persistent/files` at
  `<merged>/computer/files` and remounts it `MS_RDONLY|MS_NOSUID|MS_NODEV`.
  This is exactly the pattern of `/selfdev/bundle` [V `executor.go:629-645`].
  It happens before the broker starts, behind a per-computer setting,
  `engineering.computer_files`, default off.
- **What is outside it.** The signing keys (`choir-signers/*`), the
  credentials (`choir-credentials`), the gateway token
  (`/run/choir-bootstrap`) and the raw store files (`state*`) are not under
  `files/`, so they are outside the mount by construction. Inside the
  chroot, an absolute symlink in `files/` resolves against the capsule root,
  where `/mnt` is a tmpfs mask [V `executor.go:2363-2372`], so a symlink
  cannot reach them. A test asserts that a planted symlink to
  `/mnt/persistent/choir-credentials` resolves to nothing (T19).
- **Store state** is read only through read verbs (`ReadRef` on a record id,
  `/api/*` through management's `ProductAPI`), never as files.
- **Texture documents** are read by reference (`texture-rev:<id>`), not
  through the mount (§1.7).

### 8.2 Landlock: a correction to the brief

Landlock rights add up down a hierarchy. The job's domain grants full
access to `/` of the chroot [V `cmd/capsule-broker/jobs.go:74`,
`landlock.go:49-62`], so no added rule can make `/computer` read-only. The
read-only bind is the control, together with the capability drop and
seccomp that keep the workload from remounting.

For defense in depth, S7b replaces the `"/"` entry in
`NewWorkloadLandlock` with an enumerated list of the capsule root's
top-level directories with full access, and `/computer` with read access
only (ReadFile, ReadDir, Execute). This is a broker TCB change (red, tier
2) and its own slice. S7 does not need it to be safe.

### 8.3 Exfiltration

Read access plus egress is an exfiltration path. A prompt injection in a
mail message, a document or a fetched page can tell engineering to read
`/computer/files/...` and send it out.

The guard, recommended as the default (open decision 2): a capsule with
`/computer/files` mounted gets the compiled-in L2 ecosystem egress only
(package mirrors) [V that L2 is the default, egress-grant design §1]. It
gets no open-egress grant (L1, L3, L4). A grant request in such a capsule
is refused with the reason, and the owner can still mount files or grant
egress, but not both, in one assignment.

Whatever L2 allows (mirror fetches with chosen URLs) is a residual covert
channel, named and not closed here.

### 8.4 Reproducibility

Builds must not depend on `/computer/files`. The host is the build
authority and builds from source only [V doctrine product path; that the
host build sees no files mount is H]. A candidate whose build reads
`/computer/files` therefore fails the host build. That failure is the
check, not a new detector. The overlay says the same in one sentence.

### 8.5 Later: grants as records (forward-looking, not built)

The owner's direction: this becomes a user setting, then an organizational
policy. The owner's example is a school, where teachers see some student
files, parents some of their children's, and the principal others, and
where students and parents grant access. Outside databases come with their
own access rules.

The fit with what exists:

- **A read grant** is a sibling of the egress grant record
  (`internal/egressgrant`, schema `choir.egress_grant/v1` [V the design,
  `docs/design/egress-grant-object-2026-10-21.md` §2]): holder-typed,
  decided by an owner act, with an id that is the digest of the
  declaration and a disposition (`granted`, `revoked`, `expired`). The
  recommendation is to generalize that one package to `grant.Grant` with a
  resource kind (`egress`, `read`), not to build a second.
- **Fields:** grantee (a desk, an app desk, another computer, a person);
  scope (a path prefix under `files/`, a document id, an app object kind,
  or a reference pattern); rights (read); granted by (an owner-act receipt;
  later, a delegation chain); conditions (expiry, purpose); revoked by.
- **With references.** `ReadRef` and the mount ask one question: "is this
  reference addressed to me, or covered by a live grant that names me?"
  The S7 mount is the first grant, written as a setting until grants exist:
  {grantee engineering, scope `files/`, rights read, granted by the owner's
  setting act}.
- **Organizations.** Each person's computer issues grants over its own
  objects. An organization is a set of computers whose grants name each
  other's desks. A parent's computer can issue a grant to a teacher's
  computer for a child's folder, and revoke it. Nothing here is an
  organization product. The point is that grants are records on the
  issuer's tape, revocable, and read by the holder's `ReadRef` through
  publication and adoption, never by direct merge (CLAUDE.md, forked
  computers).
- **Outside databases.** A reference kind such as `external:<system>:<id>`
  would be resolved by a connector that asks the outside system with the
  holder's credentials. The outside system's own rules decide, and the
  grant only allows the desk to ask.

---

## 9. Replay, restore and checkpoint

### 9.1 Tables

None are added. Every record in §2 is a `choir.commitment_record` in
`og_objects`, an event projection [V `replay_eligibility.go:93`]. The
manifest version stays 4 [V `:20`]. The executor's in-memory task table,
the transitional `jobs/*.json` files and `/input` copies are not store
state, and they are rebuilt or closed at boot.

### 9.2 Replay and restore

- A replay re-mints nothing, because every id is deterministic and every
  write has a not-exists condition [V `store/commitment.go:21-52`].
- On a restore to an earlier head, task records after the head go with the
  tail. A task open at the head is closed at the next boot by the crash
  rule (§2.6), so the fold is never left with a start and no end.
- A revert of the S2 binary on a tape that holds task records is safe,
  because the kinds are old and the bodies are new fields [H, S2 review].
  This is unlike D39 (ledger 12:5x).

### 9.3 Checkpoint and release

- A checkpoint covers the records. Job logs (`capsule-job-log:sha256:`) live
  under the capsule artifact root, as they do today. Whether a checkpoint
  carries them is [H], and the records quote the bounded excerpt they rest
  on, so a missing log does not unsettle a claim.
- The release record carries the assignment's settled precommits [V
  `internal/release/record.go:54`]. Adding the task claims' say-do summary
  to it is optional (S9).

### 9.4 Write volume (the resource rule)

- **Engineering:** about 4 records per task plus 2 Texture observations,
  tens of tasks per assignment, so on the order of a few hundred records a
  day on b@c [H].
- **Research:** about 3 records per search (start, end, rule settlement),
  plus a score when a free claim is present. The paper's searches per day
  are not measured [H].
- **The snapshot listing.** The digest snapshot already lists up to 20,000
  records every 30 minutes [V `calibration_feedback.go:41`]. S8 must first
  read the paper's search rate. If research's task records could crowd the
  20,000 listing, S8 replaces the full listing with targeted listings (by
  `schema_id` and agent prefix) before it lands. S8 lands on b@c only until
  that is measured.

---

## 10. Failure modes, written before code, each with its test

Tests are written first, as CLAUDE.md requires. The S column names the
slice that carries the test.

| # | Failure | Guard | Test | S |
|---|---|---|---|---|
| T1 | A task starts without a precommit | Start requires `expect`; the reducer refuses an intent whose start record failed to write, and the executor is never called | reducer test: append fails, so `StartJob` is not called and `job_results` says why | S2 |
| T2 | A precommit is written after the evidence | the start record is committed before `StartJob`; the reducer stamps `StartedAt`, the model cannot | test: the record's commit time precedes the job's start in the receipt | S2 |
| T3 | Claims are moved later | the record is immutable; there is no edit verb | store test: a second write of the same id is a no-op | S2 |
| T4 | Dogmatic or empty claims | p in [0.02, 0.98], 1 to 8 claims, a criterion on every free claim | validation table test | S2 |
| T5 | A task settles twice, or by two passes | one id per settler, with not-exists | double-settle test | S2 |
| T6 | Settlement blocks or fails the end path | best effort, logged (R6) | the store fails and the end record still commits | S2 |
| T7 | A crash leaves a started task open | boot closure from the ledger fold writes end and `interrupted` | restart test: open start, boot, end and settlement present, and nothing re-dispatched | S2 |
| T8 | Interrupted claims count as wrong | `interrupted` is excluded from observations | calibration test | S2 |
| T9 | Two heavy tasks OOM the capsule | at most one task without `memory_mib`; others admitted by boost against the live budget; at most 4 | executor test: a second undeclared start refused, a declared one admitted, a fifth refused | S5 |
| T10 | A job's receipt claims a tree another job changed | the existing start and end digest binding (F12); Freeze refuses a build or test receipt whose digests differ | concurrent test: job A edits while B tests, so B's receipt is refused in the test slot | S5 |
| T11 | Freeze while a task runs | the existing refusal, generalized to any task | test with two running | S5 |
| T12 | The park waits on the wrong task | the park waits for the awaited id; a stale wake does not release it (existing) | two jobs, end the other first, the park stays | S5 |
| T13 | System1 sees the claimant's p or identity | the request builder takes only the §6.2 fields; a test inspects the request bytes for p, desk, model and assignment | request-bytes test | S4 |
| T14 | A System1 verdict reaches engineering per item | `BuildActingPack` drops `management:system1` resolutions; score records are ledger-only (not addressed) | pack test; inbox test | S1, S4 |
| T15 | Engineering reads its own task's score through the supervision document | `ReadRef` reads only refs addressed to the desk; a revision after the assignment opened is never addressed | refusal test | S6 |
| T16 | The digest shows the open assignment's outcomes | `ExcludeThread` | calibration test with an open-thread observation | S1 |
| T17 | A small n reads as a finding | MinN 20, MinBin 5 (existing) | existing tests, plus per-settler lines | S1 |
| T18 | Held-out items leak | `HeldOut` exclusion (existing), applied to the settler lines too | test | S1 |
| T19 | The files mount reaches secrets | bind only `files/`; `/mnt` masked; symlink test | in-capsule test: a planted symlink to `choir-credentials`, `choir-signers`, `state` and `/run/choir-bootstrap` resolves to nothing | S7 |
| T20 | The workload writes through the files mount | `MS_RDONLY` remount; no `CAP_SYS_ADMIN`; seccomp | in-capsule write and remount attempts fail | S7 |
| T21 | Read plus egress exfiltrates | the files mount and open-egress grants are exclusive | a grant request in a files capsule is refused | S7 |
| T22 | An auto-converted body loses text | the body record keeps the full text, and the act keeps the first KiB and the ref; `ReadRef` pages reassemble to the same sha256 | round-trip test at 4 KiB, 16 KiB and 200 KiB | S6 |
| T23 | A reference to a mutable thing | ref kinds are immutable only (revision id, sha256, record id); `ReadRef` re-hashes the bytes against the ref's sha256 | tamper test | S6 |
| T24 | Texture is woken per search and drowns | research does not address Texture per search | test: 10 searches, 0 Texture records | S8 |
| T25 | A search precommit costs the paper a write storm | targeted listing before S8 leaves b@c; a measured rate | proof reads the paper's rate first | S8 |
| T26 | System1 is scored under an unpinned alias | refuse the alias as a scorer id | test with `~typesafe/jev-latest` reported | S4 |
| T27 | The digest breaks the prompt cache | the turn goes after the history, never the system prompt (existing P1) | the existing injector test, run for engineering | S1 |

---

## 11. Build slices

Each slice lands alone, through the Landing Loop. Staging acceptance is on
b@c (computer-6c50d6e5, `b@c.com`), read through product routes and the
CLI. Review tiers follow CLAUDE.md.

| S | What | Class | Protected surfaces | Review | Proof on staging (b@c) | Rollback |
|---|---|---|---|---|---|---|
| S0 | Problem record "engineering's acting pack is empty in the capsule"; this design; the doctrine text (§12) as its own commit once the owner ratifies | green | doctrine | tier 0 (doctrine change reviewed like the change it designs: one seat) | docs truth checker | revert |
| S1 | Engineering's pack in the capsule; engineering observations; per-settler grouping; `ExcludeThread`; `engineering.track_record` | orange (runtime context) | none red | tier 1 (Devin) | seed at least 20 settled assignment rule claims (drills); run one assignment in the digest arm; its run events show the digest turn and a non-empty `choir.Pack()` in the first cell; `GET /api/calibration` shows the engineering digest | the setting off; revert |
| S2 | Mandatory task precommit at `choir.Start`; task start records; rule kinds `exits_zero`, `within`, `tree_unchanged`; rule settlement at job end; boot closure from the fold; `/api/tasks`; overlay | red (tape and evidence, a new boot path) | Trace/evidence, boot path | tier 2 (Devin, DeepSeek) | the Go-change drill (`scripts/reeng_tracer.mjs --scenario engineering --change go`); `choir task list` shows every job with start, end and rule settlement; a planned restart mid-job shows `interrupted`; nothing resumes | revert (old kinds; new fields are ignored [H]) |
| S2b | Changed-path list per job; rule kind `changes_within` | orange | none red | tier 1 | a drill job that edits one file settles `changes_within` | revert |
| S3 | Texture observations at task start and end; supervisor resolution; Texture overlay line | orange (Texture desk wakes) | Texture canonical writes are not touched (Texture writes) | tier 1 | a drill: the supervision document's revisions show each task's start with p and its end, read with `choir texture history <doc>` | revert |
| S4 | System1 scoring of free claims at task end, as management; blind request; `task_scores` cap; alias refusal; audit queue for the held-out tenth | red (provider calls) | gateway and provider calls | tier 1 (additive, behind `tasks.system1_scoring`, one-act rollback) | a drill with one free claim: a `system1.judgment` answered event, a `task:<id>:score:system1` record, and the request bytes in the event digest test; the Texture end observation quotes the score | setting off |
| S5 | Concurrency: drop the one-job rule in the executor and the broker; memory admission; at most 4; parks on any awaited task | red (capsule broker, guest TCB; new concurrency) | capsule effects, broker | tier 2 | a drill running build and test as two tasks at once: both receipts, the memory boost logs, and Freeze refused until both end | revert |
| S6 | References: `Refs`; body records and auto-convert above 4 KiB; `ReadRef` for engineering, management and research; `/input/<ref>` read-only bind; **delete** the 16 KiB cut and marker in `engineering_cast_request.go` | red (tape and evidence, capsule mounts) | Trace/evidence, capsule | tier 2 | cast a 40 KiB objective through the prompt bar; engineering's first cell `choir.ReadRef` returns it whole (sha256 equal) and `/input/` holds it | revert (body records stay readable as reports) |
| S7 | `/computer/files` read-only on b@c behind `engineering.computer_files`; egress exclusivity; the overlay's reproducibility sentence | red (capsule mounts) | capsule effects | tier 2 | on b@c: a cell reads a Files app document at `/computer/files/...`; write, remount and symlink-escape attempts fail (T19, T20); a grant request is refused (T21) | setting off |
| S7b | Landlock: enumerated roots; `/computer` read only | red (broker TCB) | capsule | tier 2 | the same in-capsule tests, plus Landlock denials on write | revert |
| S8 | Research: mandatory precommit at `choir.SearchStart`; `hits_at_least`, `domain_present`, free claims scored by System1 from the hits; settled at stream end; a research digest per kind and settler (behind `calibration.feedback` for research, or its own setting); a Report line for Texture; targeted listing | red (tape and evidence, provider calls) | Trace/evidence, gateway | tier 2 | on b@c: one story's research round; every search has start, end and rule settlement records; `GET /api/calibration` shows research's task lines; the paper's search rate is read before the paper takes it | setting off; revert |
| S9 | Later: management's `Delegate` as a third task kind (precommit at Delegate, settled by the task's closing Report); task claims in the release record; other desks' digests; read grants as records | each its own | | | | |

**Replace, not extend.** The following existing mechanisms are replaced,
not extended:

- the `jobs/*.json` file authority, replaced by the ledger fold (S2);
- the 16 KiB truncation marker, replaced by a summary plus a reference
  (S6);
- the one-job rule in two places, replaced by memory admission (S5).

`RulePrecommitSayDo` stops being dead code in S1.

---

## 12. Proposed doctrine text

These are proposals only. They land as their own green commit after the
owner ratifies (S0). Each item gives the current text and the replacement.

**A. `docs/choir-doctrine.md`, I26, last sentence.**

Current:

> The commitment ledger is the management supervision surface: its objects
> live on the tape, reports resolve commitments, and scores stay off the
> acting desk's context.

Replace with:

> The commitment ledger is the management supervision surface: its objects
> live on the tape, and reports, settlements and scores resolve
> commitments. Every background task starts with a precommitment, in one
> act. Feedback to an acting desk is aggregate: its context may carry the
> calibration of its own settled predictions by claim kind and by settler
> (n, share held, mean p, Brier), shown only above a minimum sample and
> never including a fixed held-out tenth. Per item, a desk sees its own
> receipts and rule settlements, never a model scorer's verdict, and never
> a verdict on work it is still doing; no desk can query or address a
> scorer (owner, 2026-10-24, refining 2026-09-29).

**B. `docs/agent-product-doctrine.md`, the commitment ledger bullet.**

Current: "reports resolve commitments; scores stay outside the acting
desk's context."

Replace with: "reports and settlements resolve commitments; an acting desk
sees aggregates of its own calibration and never a model scorer's verdict
on an item (choir-doctrine I26)."

**C. `docs/precommitment-records.md` §1.2, the context-design sentence.**

Current:

> **Context design is therefore a safety decision**: feed an agent
> observations and discrepancies; keep independent scores out of its
> context, or sample and delay them, so it cannot learn a scorer's quirks
> inside one session (section 4.7).

Replace with:

> **Context design is therefore a safety decision.** Feed an agent
> observations, discrepancies and the aggregate calibration of its own
> settled predictions, by claim kind and by settler. Never show it a model
> scorer's verdict on a single item, nor any verdict on work in progress,
> so it cannot learn one scorer's quirks inside a session, and hold a fixed
> tenth of settled claims out of every aggregate so gaming shows as a gap
> between what it is shown and what it is not (owner, 2026-10-24; section
> 4.7).

**D. `docs/precommitment-records.md` §4.7.**

In the "Decided" paragraph, replace "Not built: record query verbs in the
REPL as such, and automatic pack injection, which stays off until held-out
evidence supports it (H2)." with: "The calibration digest is on by owner
ruling (2026-10-24): aggregates of the desk's own settled record, per claim
kind and per settler, with the held-out tenth excluded. Automatic injection
of individual records by corrective value stays off until held-out evidence
supports it (H2)."

Replace the paragraph that begins "The tension (8.3)" and ends "Where
exactly the line falls is open." with:

> **The line (owner, 2026-10-24).** A desk sees aggregates of its own
> calibration: n, share held, mean p and Brier per claim kind and per
> settler, shown above a minimum sample, never including a fixed held-out
> tenth, and never including the work it is doing now. Per item it sees its
> own acts, its receipts and rule settlements, which are observations, not
> judgments. It never sees a model scorer's verdict on an item, never any
> verdict on work in progress, and never reaches a scorer. This refines the
> 2026-09-29 boundary (the acting pack never carries a scorer's judgment of
> an item) and implements the 2026-10-14 intent (a desk sees its settled
> record). Engineering is first; other desks follow.

**E. `docs/precommitment-records.md` §5.4, the "Scorer gaming" row.**

Replace "independent scores out of the acting context, or sampled and
delayed" with "aggregates only in the acting context, never a per-item
model verdict, never on work in progress, a held-out tenth".

**F. `docs/precommitment-records.md` §8.3, item 2.**

Mark it settled by the 2026-10-24 ruling, with a pointer to D.

**G. `docs/precommitment-records.md` §8.1.**

Refresh what is built (the status doc's drift note). This is green and
separate.

---

## 13. Standing questions

1. **Who settled this?** The owner, 2026-10-24, ledger 17:5x and 18:2x
   (mandatory precommit, both claim kinds, System1, aggregates, the
   held-out tenth, feedback lifted, references, read-only files), plus the
   coordinator's addendum (research as a first-class case). Choices this
   design makes are proposals: the compact record shape, the 0.8 cut-off,
   4 KiB inline, at most 4 tasks, the egress exclusivity, and host-side
   scoring for management. The open decisions in §14 name the ones the
   owner may want to set.
2. **Does the topology conform to settled decisions?** Records stay on the
   tape, never in a third store (two-store directive). Texture stays the
   sole writer of document text (I1, I2). Engineering never addresses
   Texture: the runtime does, with system records (precommitment-records
   §4.2). Management holds no mutation authority, and its System1 scoring
   writes ledger records only. Crash closes and planned restart may resume
   (tasks cannot resume, because the capsule dies with any restart).
3. **What references the deleted things?** The 16 KiB cut is cited by
   `docs/problems/engineering-cast-objective-cut-at-4kib-2026-10-24.md` and
   the ledger (D40). The one-job rule is cited by
   `docs/design/capsule-long-exec-2026-10-15.md` F4 and the overlay. The
   file job record is cited by the same design §3 F19. Each deletion slice
   repairs or redirects these in the same commit.
4. **Who consumes this in production?**
   - `RunningJob` is called by `rlm_reduce.go:848` and
     `tools_capsule.go:168` [V].
   - `JobViews` is called by the park and the guard [V
     `capsule_jobs.go:213-370`].
   - `RulePrecommitSayDo` and `BuildSupervisionPack` have no caller [V
     status doc]. S1 gives the first a caller.
   - The calibration digest has callers but is switched off.
5. **What is the single authority?** Task state is the ledger fold (start,
   end). The executor's map and the `jobs/*.json` files are caches until S2
   deletes the files. A claim's outcome is its settlement record per
   settler; disagreement is kept, never merged; the governing settler for a
   view is a query parameter (rule over owner over independent over self
   [V `internal/calibration/math.go:31-65`]).
6. **What artifact proves success?** For each slice, the records fetched
   through `/api/tasks`, `/api/calibration` and `choir texture history`,
   and the run events of a drill on b@c. A prompt that "says" engineering
   sees its record proves nothing (that is how the empty pack hid).
7. **What does the read path fate-share with?** The ledger and
   `/api/tasks` share only the guest runtime and its store. The Texture
   rendering also needs the Texture desk and its provider. When Texture is
   down, tasks still record and settle, and the route still serves them.
   The System1 score needs the gateway and Jev. When they fail, the score is
   `failed` on the tape, the rule settlement stands, and the Texture end
   observation goes out after 60 s without the score.
8. **What happens on restart?** A crash or a planned restart ends the
   capsule and so every task. The boot closes open starts from the fold
   with "interrupted by a restart". Claims settle `interrupted` and are
   excluded from calibration. The digest snapshot is process-local and is
   rebuilt within 30 minutes. `/input` copies are rebuilt per capsule.
9. **Could an agent do this without SSH?** Yes, through `/api/tasks`,
   `/api/calibration`, the `choir task` CLI and `choir texture history`,
   under a scoped key.
10. **Registry hygiene.** This is a design, not a Definition.
    `docs/ACTIVE.md`, `docs/mission-graph.yaml` and the authority manifest
    change when a slice becomes a mission.

---

## 14. Open decisions for the owner

Each has a recommended default. Work proceeds on the default unless the
owner rules otherwise ("No Blocking Asks").

1. **Arms for engineering.** Half of the assignments (by hash) see the
   digest and half do not, so the learning claim can be tested.
   *Default:* arms on until 200 settled observations per arm, then the owner
   reads H-ICRL for engineering and decides whether to show it to all.
2. **Files and egress together.** *Default:* a capsule with
   `/computer/files` gets L2 mirrors only and no open-egress grant. The
   alternative is to allow both with an owner act per assignment.
3. **Research in Texture.** *Default:* searches reach Texture only in
   aggregate, in the final Report ("searches n; rule claims held k of m").
   The alternative is a running list of searches in the document, which
   costs a Texture turn per batch.
4. **Who frames the System1 question.** *Default:* a host step for
   management, with no management model turn per task (cheap, blind,
   uniform). The alternative is a management turn that writes the question,
   which is smarter, costs a turn per task, and lets management see the
   claimant's p unless that is hidden.
5. **Per-item verdicts on finished work.** The ruling forbids the verdict on
   the task in progress. *Default:* aggregates only, always (the stricter
   reading). The alternative is to let a later assignment's pack show
   per-item System1 verdicts on finished assignments, which is richer
   feedback with more risk of learning the scorer.

---

## 15. Ceremony

- **Mutation class:** green (this document). The slices are classed in
  §11.
- **Conjecture delta:** C7 and the PSA claim move from "records exist" to
  "every background task carries a precommitment that settles". The ICRL
  conjecture for engineering becomes testable (H-ICRL arms) once S1 and S2
  land. The blind edge: whether aggregate feedback improves calibration, or
  only makes the desk vaguer. The digest shows Brier and strict Brier
  beside calibration so vagueness shows (precommitment-records §5.3).
- **Heresy delta.**
  - *Discovered:*
    1. job state has two authorities, a file record and the inbox end
       record, and a job's start never reaches the ledger;
    2. the job Landlock domain is the whole chroot (`"/"`), so read-only
       inside the capsule rests on the bind alone;
    3. Texture documents are not files, so a files mount alone would not
       give engineering the documents the owner named first;
    4. the System1 scorer id is an unpinned alias (`jev-latest`), against
       §5.3's "pinned version" [H on what is reported];
    5. the digest has no grouping by settler;
    6. mandatory per-search precommits could crowd the 20,000-record
       snapshot on the paper [H on rate].
  - *Introduced:* none (design only).
  - *Repaired:* none.
- **Residuals:**
  - a covert channel through L2 mirror fetches;
  - checkpoint coverage of job logs and image blobs [H];
  - the paper's search rate [H];
  - Jev's price and its reported model id [H].
