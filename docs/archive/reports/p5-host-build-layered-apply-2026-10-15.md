# P5: Host Build, Layered Apply (Station A slice)

Date: 2026-10-15. Branch `feat/p5-host-build-layered-apply` from `origin/main`
133de45fa. Mutation class: **red** (deployment routing: what binary a computer
runs; the guest update path; a new host build surface on the trust root).
This commit is docs only: the design, the failure modes the code must hold
(written before the tests), and the ceremony. Code follows in later commits
on the same branch, each naming the failure modes it closes.

Sources: `docs/reports/selfdev-update-architecture-2026-10-15.md` (P5, §1.2
step 3 and 5, §1.4, Station A) and its owner rulings at the top (source only;
the host is the sole builder; builds are reproducible and host-signed; adoption
is source onto the adopter's own tree). The problem this closes is residual R4
of `docs/problems/selfdev-go-drill-jobs-0101-2026-10-15.md`: after an applied
Go change the guest still reported build `e83735f2…`, because the materializer
mints a plain release and the boot wrapper only execs a release binary through
a `layering-entrypoint` pointer.

## What runs today, read in code

- **Freeze** (`internal/capsule/executor.go`, `emitSourcePatch`) writes
  `source.patch` (git-applicable unified diff, `a/` `b/` paths relative to the
  repo root) into the bundle directory, proves it applies, and records
  `source_patch_sha256` and `source_patch_base_commit` (the capsule source
  pin's commit) on the bundle (`internal/capsule/transaction/builder.go`).
- **Apply** (`internal/agentcore/self_development_materializer.go`,
  `materializeSelfDevelopmentOperation`) mints a **plain** release from
  `bundle.RuntimeFiles` (the capsule's `bin/autoputer` plus `frontend/`), and
  the updater swaps `current` and restarts under the planned-restart marker
  `self_development_apply`.
- **Boot** (`nix/autoputer-vm.nix`, `autoputerRuntimeExec`) execs
  `current/layering-entrypoint` only when it exists, the release's
  `base_image_manifest_digest` equals the booted base, and the boot-loop guard
  has not tripped. A plain release has no entrypoint, so the base binary runs.
  Only the frontend changes. That is R4.
- **The layered format** already exists (`internal/updater`): a release whose
  manifest carries `closure_digest`, `layering_entrypoint`,
  `base_image_manifest_digest`, `base_commit`, `code_commit`,
  `store_schema_version` and `builder_receipt_digest`, and whose files include
  `closure.nar` (a `nix-store --export` stream). Apply replays the closure into
  the private store, writes `layering-entrypoint` inside the release dir, and
  verifies the entrypoint's `share/go-choir/build.json` commit equals
  `code_commit` before the pointer swap. The S2 station proved this for
  platform pushes.
- **The host builder library** (`internal/builder`, CLI `cmd/choir-builder
  --source-patch`) builds a flake installable from `base_commit + patch` and
  exports the base-absent closure. It has no service, no route and no caller.
  (`cmd/choir-rebuild-base` is the projection-base rebuilder, not a code
  builder; it is not on this path.)
- **The host blob store**: corpusd's `sha256/platform-update/<sha>` namespace
  (`internal/platform/platform_update.go`), guest-readable through
  `GET /internal/computers/platform-updates/blob/<sha>` with the computer's
  `event:read` capability (`computerevent.HTTPClient.FetchBlobRaw`).
- **The patch read** lives on branch `selfdev-patch-export` (c8db35df5, not on
  `main`): `readBoundSelfDevelopmentBundle`, `readBoundSourcePatch` and the
  added-line secret scan. This branch cherry-picks it and reuses those
  internals.

## The design

```text
guest (accepted op, bundle touches Go input)
  -> POST corpusd /internal/computers/selfdev-builds      (patch, sha, base, booted base digest)
host corpusd: hostbuild.Service
  -> job record (durable, id = sha256 of the request)     queued
  -> admission: one build at a time, bounded queue,
     memory floor + PSI memory some avg60                  running
  -> builder.Build(.#autoputer, base_commit + patch)       nix sandbox, timeout
  -> closure.nar + builder-receipt.json into
     sha256/platform-update/<sha>                          done
  -> platform-control signed receipt over the result
guest (retry wake every 60 s while queued/running)
  -> GET  /internal/computers/selfdev-builds/<job>        done | failed
  -> verify receipt (platform-control key), fetch blobs, re-hash
  -> mint LAYERED release: closure.nar, builder-receipt.json, host-build.json
  -> updater.Apply (marker self_development_apply)        planned restart
boot wrapper execs current/layering-entrypoint            host-built binary
health probe: release_digest AND build.commit == code_commit
```

### Host side: `internal/hostbuild`

- **Request**: `computer_id`, `operation_id`, `bundle_digest`, `base_commit`,
  `source_patch_sha256`, `source_patch_base64`, `base_image_manifest_digest`
  (the guest's booted base). The job id is the sha256 of the canonical request
  without the patch bytes (the patch sha stands for them), so a retry names the
  same job and a different patch never does.
- **Refusals before any build** (typed reason codes, the job ends `failed`
  with the code and a sentence):
  - `patch_digest_mismatch`: the bytes do not hash to the declared sha;
  - `patch_too_large`: over 2 MiB (the export cap);
  - `recipe_change_refused`: the patch touches the build recipe or the
    dependency set (`flake.nix`, `flake.lock`, `nix/`, `go.mod`, `go.sum`,
    `go.work*`, `vendor/`, `frontend/package.json`,
    `frontend/package-lock.json`). The toolchain is the base commit's pinned
    `flake.lock`; a guest-authored patch never changes what the host evaluates
    or fetches. This also keeps flake evaluation from fetching anything a guest
    named;
  - `store_epoch_change_refused`: the patch touches `internal/storeschema/`
    (until LU2, an epoch-bumping release is refused, architecture §1.4);
  - `base_moved`: the guest's booted base digest is not the host's deployed
    guest image (a deploy landed since the computer booted);
  - `base_commit_mismatch`: the patch's base commit is not the deployed
    image's `build_commit` (the source seed lags the image, P13);
  - `queue_full`: more than `MaxQueue` jobs waiting;
  - `not_admitted`: the queue deadline passed while the host was under memory
    pressure;
  - `build_failed`, `build_timeout`: the build itself;
  - `interrupted_by_host_restart`: corpusd restarted while the job was queued
    or running (crash rule: a restart ends work, never resumes it).
- **Admission**: one build at a time. Before a queued job starts, the worker
  samples `/proc/meminfo` and `/proc/pressure/memory`; it starts only when
  `MemAvailable >= floor` (default 6 GiB) and `memory some avg60 <= max`
  (default 10). Otherwise it waits and resamples every 15 s until the job's
  queue deadline (default 20 min), then fails `not_admitted`.
- **Build**: `builder.Build` with installable `.#autoputer`, the host's
  deployed source checkout (`/opt/go-choir`, configurable) as the object store
  only, and the deployed guest image's manifest and store disk as the base.
  Bounded by a context timeout (default 40 min).
- **Reproducible inputs**: the builder's synthetic commit is now
  deterministic: fixed author and committer (`choir-builder`), and author and
  committer dates equal to the base commit's committer date. The commit, and so
  the flake's `self.rev` and `self.lastModifiedDate`, depend only on the base
  commit and the patch. The result records `code_commit`, the `.drv` path
  (input-addressed: it names every input), the base `flake.lock` sha256 and the
  base identity.
- **Signed**: corpusd signs a platform-control `AuthorityReceipt` of the new
  kind `host_build` with `request_commitment = job id` and
  `artifact_digest = sha256(canonical result)`. The guest already pins the
  platform-control key (`selfdevControl.PublicKey()`).
- **Durable state, single authority**: the job record file
  (`<state>/jobs/<id>.json`) is the only authority for a job's state. Outputs go
  to the platform-update blob namespace by content digest; the work directory
  (and its nix GC-root link) is removed after ingest.
- **Routes** (corpusd, guest-reachable like the blob route):
  `POST /internal/computers/selfdev-builds` (scope `event:append`, computer
  bound) and `GET /internal/computers/selfdev-builds/<job>?computer_id=`
  (scope `event:read`). Unconfigured host builder: `503` with code
  `host_builder_unconfigured`.

### Guest side

- **Which candidates**: a bundle any of whose `ordered_file_effects` is a
  binary input under `workspace/platform/` (`capsule.BinaryInputPath`, the
  same classifier Freeze uses to decide binary reuse). A frontend-only bundle
  keeps the plain path unchanged.
- **When**: in `materializeSelfDevelopmentOperation`, while the operation is
  `accepted`, before `materialization_started`. Desks keep working while the
  host builds (the actor hold covers only `materializing`). Queued or running
  returns a pending error; the reconciler's existing 60 s retry wake polls.
- **Failure path**: a failed host job (any reason code above, or
  `host_builder_unconfigured`) appends `materialization_failed` with the
  reason, restores the prior effective state on the tape
  (`RestoredPriorEffective`), and moves the operation to `failed` with
  `terminal_error = "host build failed (<code>): <detail>"`. The updater is
  never called: `current` and the running release do not change.
- **Mint**: an incoming dir holding `closure.nar`, `builder-receipt.json` and
  `host-build.json` (the signed result), and a layered manifest carrying the
  result's closure digest, entrypoint (`<basename(runtime_path)>/bin/autoputer`),
  base digest, base commit, code commit, store schema version and builder
  receipt digest. The layer's own `frontend/` (built from the same patched
  tree) is staged by the updater, so binary and frontend move in one swap.
  The capsule-built `bin/autoputer` is not a release input on this path.
- **Health**: the updater's probe additionally requires `build.commit ==
  code_commit` for a layered release with a declared code commit, so
  `applied` means the host-built binary is serving. `/health` gains
  `effective_release` (release digest, layered, code commit, base commit,
  closure digest, builder receipt digest) read from `current`.
- **Restore removes the layer**: a tape restore restages the checkpoint's
  pinned release. When that changes the exec identity (the layered entrypoint
  differs between the release that was serving and the restored one), the
  guest writes the planned-restart marker `self_development_restore` and asks
  the updater (new `POST /v1/restart`) to restart the runtime into `current`.
  Without the restart the layered binary keeps running after a restore.

## Failure modes (written before the tests)

Host build service:

- H1 a patch whose bytes do not hash to the declared sha is built;
- H2 a patch touching the recipe or dependency set is evaluated;
- H3 a patch touching the store schema is built;
- H4 a build runs for a base the guest did not boot (`base_moved`), or whose
  patch base is not the image's commit;
- H5 two jobs build at once, or a job starts while memory is under the floor or
  PSI is over the bound;
- H6 a queued job waits forever under pressure (no queue deadline);
- H7 a build runs past its timeout;
- H8 a retry of the same request starts a second build; a different patch
  reuses another job's result;
- H9 a host restart leaves a job `running` forever, or resumes it;
- H10 a job reported `done` whose closure is not in the blob store under its
  digest;
- H11 the result is unsigned, or the signature does not bind the job id and
  the result digest;
- H12 the synthetic commit (and so `code_commit`, `self.rev`) differs between
  two builds of the same base and patch;
- H13 a guest reads or creates a job for another computer;
- H14 an unbounded queue (a guest floods requests).

Guest apply path:

- G1 a Go-touching bundle applies as a plain release (the base binary keeps
  running; R4);
- G2 a frontend-only bundle is sent to the host builder;
- G3 a host-build failure leaves the operation `accepted` forever, or swaps
  `current`;
- G4 a forged or mis-bound host receipt is accepted (wrong key, wrong job id,
  result digest mismatch);
- G5 a fetched closure whose bytes do not hash to the signed digest is
  staged;
- G6 the layered manifest differs between a first pass and a resumed pass
  (the updater journal's request commitment would not match);
- G7 `applied` is recorded while the base binary is serving (probe checks only
  the release digest);
- G8 a pending host build holds desk work (`materializing` during the build);
- G9 crash-boot settlement calls the host builder.

Restore:

- R1 a restore to a pre-apply checkpoint leaves the layered binary running;
- R2 a restore that does not change the exec identity restarts the computer;
- R3 a restore restart without the planned marker reads as a crash.

### Review findings (2026-10-15, written before the fixes)

An independent review of the branch found that the recipe refusal (H2) was
decided on a text parse of the patch that git does not share: `git apply -p1`
strips any first path segment and unquotes C-style quoted names, while the
host's parser stripped only a literal `a/` or `b/` and read quotes as bytes.
Three patches the parser passed write a recipe file:
`diff --git x/flake.nix y/flake.nix`; `--- foo/flake.nix` / `+++ bar/flake.nix`
with no git header; `diff --git "a/nix/a.nix" "b/nix/a.nix"`. corpusd
evaluates the tree as a nix trusted user, so a guest-controlled `flake.nix` or
`nix/` file is guest control of host evaluation (a fixed-output derivation has
network access on the host; eval-time fetchers run in corpusd). The failure
modes the fixes answer:

- H15 the admission check reads a different path than git writes (any first
  segment, no git header, quoted names): the recipe refusal is bypassed.
  Authority: the tree the builder actually commits, read with
  `git diff --raw -z --no-renames <base> HEAD` in the builder's clone before
  nix runs. The submit-time check uses git's own parser
  (`git apply --numstat -z`), never a text parse.
- H16 a patch creates or changes a symlink (mode 120000) or a gitlink
  (160000), or a `.gitattributes` or `.gitmodules` file at any depth: the tree
  means something to nix or git beyond its file bytes.
- H17 the patched working tree holds files the commit does not (ignored or
  untracked files `git apply` wrote): the tree nix reads is not the committed
  one.
- H18 every nix call is capped at the builder's 10-minute per-call bound, so
  the 40-minute build bound is really 10 minutes and a slow build reports
  `build_failed`, not `build_timeout`.
- H19 a deploy lands while a job is queued or running: the job builds against
  (and the result records) a base the computer did not boot.
- H20 a transient refusal (`queue_full`, `computer_busy`) is recorded as a
  terminal job under the deterministic job id, so the retry of the same
  request fails permanently.
- H21 one computer submits builds without bound (each a new operation id):
  closures (100-200 MB each) fill the blob store.
- H22 job records (refused ones especially) and the in-memory job map grow
  without bound.
- H23 a host build misconfiguration (an unwritable state dir) stops corpusd
  instead of leaving the build service unconfigured.
- H24 an interrupted job's stored patch stays on disk.
- R4 the restore restart spends an apply-attempt count (limit 3), so the
  fourth restore to the same checkpoint skips the restart while the restore
  answer already said `restart_scheduled: true`.
- R5 the restore restart cuts running runs and self-development operations
  mid-transaction (no busy gate).
- R6 `restart_scheduled` is reported before the marker is durable or the gate
  has passed.
- H25 a derivation from a guest-authored tree builds on a remote builder.
  Node B's nix config (read 2026-10-15) has `sandbox = true`,
  `sandbox-fallback = false`, `max-jobs = 2` and
  `builders = @/etc/nix/machines`, which names
  `ssh-ng://root@node-a-builder` (x86_64-linux, big-parallel, kvm). node-a is
  shared with another project and must never build guest-authored code. Fix:
  every nix call the builder makes (build, eval, path-info) carries
  `--option builders ''` (no remote builders), `--option max-jobs 2` (local
  building stays on; with no remote builders, a future `max-jobs = 0` would
  fail every build) and `--option sandbox true` (restating Node B's setting
  so a config change cannot run a guest derivation unsandboxed). corpusd runs
  as root, a trusted user, so the daemon honors the overrides; for an
  untrusted client they could not weaken the daemon anyway. `nix-store
  --export` builds nothing and is unchanged.

## Ceremony (red)

- **Conjecture delta.** Before: "applied" for a Go change means source and
  frontend, the base binary keeps running (R4, a hypothesis in the drill doc).
  After: the guest runs only host-built binaries from published source, and
  `applied` is gated on the serving binary's commit equalling the host build's
  derived commit. A failed host build fails the operation visibly and changes
  nothing.
- **Protected surfaces touched.** The guest update path (materializer mint,
  updater probe, a new updater restart route), deployment routing (what binary
  a computer runs), a new host build surface in corpusd (trust root), a new
  platform-control receipt kind. Not touched: canonical acceptance (the owner
  or consensus decision is unchanged), checkpoint/route projection, auth.
- **Admissible evidence.** Unit tests for the host service (fake builder, fake
  pressure sampler), the builder's deterministic commit (real git, a tiny
  repo), the guest mint and failure paths (fake host client), the probe's
  commit check. The deployed proof is Station A's tracer probe (below) on a
  freshly minted test computer on `choir.news`, with `--expect-sha`. Local
  tests prove no vmctl, no live guest restart, no host nix build.
- **Rollback.** `git revert` of the branch's commits. Runtime rollback without
  a revert: unset the corpusd host-builder configuration; Go-touching
  operations then fail with `host_builder_unconfigured` and change nothing. A
  computer already running a layered release is restored with
  `choir computer restore` to its pre-apply checkpoint.
- **Heresy delta.** Discovered: the layered checkpoint identity and restore
  never restarted into the restored exec (R1 above); the builder's synthetic
  commit was timestamped with wall time, so not reproducible (the report's
  "unverified" claim is refuted in code). Introduced: the host evaluates a
  guest-authored tree (bounded by the recipe refusal, the nix sandbox and the
  timeout; a named residual). Repaired: R4 (on landing and the deployed proof).

## Station A tracer probe

`node scripts/reeng_tracer.mjs --scenario engineering --change go-served …`
asks for a new guest route `GET /api/selfdev-probe` returning a nonce baked
into the Go source. Accepted only when, after `applied`, the route returns the
nonce and `/health` reports a layered `effective_release` whose code commit
equals `build.commit`; and after the restore to the pre-apply checkpoint, the
route no longer returns the nonce and the build commit is the base's again.

## Residuals (named, not fixed here)

- The host does not check the tape for an accepted decision before building.
  The bound is the per-computer quota (`CORPUSD_HOST_BUILD_MAX_PER_COMPUTER_PER_DAY`,
  default 8 jobs in a rolling 24 hours, refused ones included), with
  admission, the queue bound and record pruning (failed records 24 h, done
  7 days, at most 512). A tape check alone would add little: the decision
  event is appended with the same computer capability that submits the
  build, so the host would also have to verify the owner or consensus
  authority behind it (corpusd stores events as artifacts, so this is a
  replay read plus `decisionpolicy` verification on the host). That is the
  next cut, not done here.
- The host evaluates a guest-authored tree. After the review fix the bounds
  are: no change to any `.nix` file, flake file, Go module or npm manifest,
  `vendor/`, the store schema, a link or git metadata, decided on the
  committed tree; the nix sandbox for the Go build itself (whether Node B's
  nix daemon enforces `sandbox = true` is unverified); the build timeout.
- A restore restart deferred by a busy computer waits up to an hour for an
  idle moment, in memory: a crash or another restart in that hour drops it,
  and the previous binary keeps serving until the next restart. An
  always-busy computer may never take it.
- A host restart (every deploy) fails builds in flight; the operation fails
  visibly and a new operation must be requested.
- `goServicesAll` compiles every service for one autoputer change (minutes);
  an autoputer-only derivation is the next cut.
- Nix builds run under the nix daemon, outside corpusd's cgroup; admission
  gates the start, not the peak.
- The CodeRef still names only the bundle; the closure is named by the release
  manifest (in the checkpoint's release digest), not by the CodeClosure.
- corpusd unit wiring (`nix/node-b.nix`: `nix` and `git` on PATH, the source
  checkout, the guest image paths, a writable state dir) is unverified on Node
  B until a deploy.
- A transport failure to corpusd keeps the operation `accepted` and retries
  every 60 s with no deadline.
- Found while building: `/api/acceptance/execution-identity` refuses when
  `build.commit != build.deployed_commit`, so it answers 503 on a computer
  running a host-built layer (its commit is the synthetic one). Identity for
  a layered computer is `effective_release` on `/health` and
  `/api/runtime/observability` until P6 joins them.
- Freeze still requires a capsule-built `bin/autoputer` for a Go change
  (about 10 minutes in the capsule); the host build makes it a test artifact
  only. Dropping that requirement is the next cut on the guest side.
- `cmd/choir-updater` (linux-only, cgo) was not compiled locally; CI builds
  it.
