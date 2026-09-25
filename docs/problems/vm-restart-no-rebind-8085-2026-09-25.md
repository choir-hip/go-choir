# Guest VM does not rebind :8085 after SIGKILL + vmctl resolve-restart

**Status:** ROOT-CAUSED 2026-09-25, diagnosis narrowed. Guest autoputer fatals on
restart: canonical chain exists but no ProjectionBase watermark is advertised
(`watermarkSeq==0`), and `PlanRecovery` refuses `chainExists && watermarkSeq==0`
at `internal/projectionbase/recovery_plan.go:98` — **before** checking whether a
retained store can resume from its own head. A non-empty retained store reaches
`RecoveryResume` from `localSeq` alone (lines 107-110,127-129) and never needs a
base; the base is only required for `empty→Install` / `local<W→Rebase`. The
watermark==0 refuse fires on a path where no base is needed. Separately, nothing
auto-publishes a base post-genesis (`BootstrapChain` publishes genesis only,
`chain_bootstrap.go:22-26`; `choir-rebuild-base` is a manual operator tool). Net
effect: a restarted computer that never had a snapshot cannot resume — a real
restart-durability gap, not a test artifact. Blocks mission K's live proof.
**Surface:** `red` — vmctl VM lifecycle + guest autoputer + projection base.
**Mutation class of any fix:** red (vmctl/guest/projectionbase).

## Root cause (confirmed via guest serial console in vmctl journal)

Not a bind/tap/route failure. On the recovered VM, systemd *does* start the
autoputer (`[OK] Started go-choir Autoputer Runtime`) — it is
`wantedBy=multi-user.target` + `Restart=on-failure`, `SERVER_HOST=0.0.0.0`,
no first-boot condition (`nix/autoputer-vm.nix:660-765`). The guest reaches its
new host gateway, opens the retained store (`fresh=false`), then fatals:

```
autoputer: required projection base refused; refusing genesis fallback:
  required base is missing for an existing chain
```

(`internal/autoputer/run.go:278-280`, before `s.Start()` at :550 — so :8085
never binds). Causal chain:

1. First boot has no canonical chain → genesis allowed.
2. `BootstrapChain` publishes only the genesis event, **no checkpoint**
   (`internal/agentcore/chain_bootstrap.go:22-26`).
3. On next boot `materializeProjectionBaseIfNeeded` queries the watermark →
   `PlanRecovery` (`internal/projectionbase/recovery_plan.go:98-100`)
   **deliberately refuses any existing chain with `watermarkSeq==0`** → fatal.

Fail-closed is correct; the missing piece is that nothing publishes a
verified ProjectionBase at seq 1 after genesis. `cmd/choir-rebuild-base`
(:20-31,82-125) already does the offline verified rebuild + advertise, but
needs the platform-artifact root + computer privacy key + advertise cap — a
host/platform worker must run it post-genesis, then fence run admission until
W=1. Stale-tap delete warning (`vmmanager/manager.go:2516-2528` ignores delete
errors) is a separate host-cleanup defect, not causal here.

## Symptom

A disposable diagnostic guest VM boots healthy, accepts a real runtime run,
then — after its Firecracker process is SIGKILLed and the VM is re-created via
`vmctl internal resolve` — the guest autoputer never re-binds TCP :8085.
The restarted VM registers `state=active` with a fresh epoch and route, but:

- `curl http://<new-ip>:8085/health` → `HTTP 000` / `curl: (7) Failed to connect … after 16 ms`
- `/internal/runtime/runs` run-status → `HTTP 000`
- vmctl `resolve` readiness check stays blocked and is eventually cancelled.

## Reproduction (both identical)

| | Attempt 1 | Attempt 2 |
|---|---|---|
| owner | `diagnostic-kernel-restart-proof-20260925` | `diagnostic-kernel-restart-proof2-20260925` |
| computer | `computer-ba9da4a7d5a96d9bfa6ca59520be59d7` | `computer-4333c73f967dbccc94638f7dbcac7342` |
| vm | `vm-9eab5b2ba58c563a3ee7d7f1c4a54f5d` | `vm-a6ccc8e7846f732e305745a5adddd7ca` |
| bootstrap | `POST /lifecycle/bootstrap-chain` → 201 | same → 201 |
| run | `POST /internal/runtime/runs` → 202 (`61d4f020…`) | → 202 (`b0d31112…`) |
| kill | SIGKILL the VM's Firecracker PID | same |
| restart | `vmctl resolve` → active, epoch 12677, `10.200.3.2:8085` | → active, epoch 12679, `10.200.5.2:8085` |
| :8085 | never binds | never binds |

Both guests ran deployed build `91c8fd9f`. Wire platform
(`computer-4c20ff4a…`/`d03dacaa`, held) untouched. Both proof guests cleaned
up (ownership removed, state dir removed, orphan Firecracker terminated).

## Why it matters

Mission K deleted all process-local restart-resumption sweeps on the ruling
"boot causes no work" — recovery is the dispatcher's durable pending
projection (`internal/actor/dispatcher.go` `Run` re-reads
`SQLiteLog.PendingAgents`/`NextDue` every poll). That mechanism is correct by
source trace, but cannot be *demonstrated* on staging because a restarted
guest autoputer never comes back on the wire. Either:

- (a) the guest autoputer fails to (re)bind :8085 on a re-created VM — a guest
  runtime/bind defect; or
- (b) the vmctl resolve-restart path registers `active`/`route` before the
  guest is actually serving — a readiness/route projection defect.

Either way, restart-resume is unprovable on disposable guests and — more
importantly — a restarted disposable guest is not actually serving, which is a
product defect independent of K.

## Open questions for the fix

- Does the guest autoputer bind to the VM's *old* IP/interface identity? A
  re-created VM gets a fresh IP (10.200.3.2 → 10.200.5.2) — if the autoputer
  or firewall binds the stale source address, it will never accept :8085.
- Is there a guest-side supervisor that restarts autoputer on boot, and does
  it run on the resolve-restart path (vs. first-boot only)?
- Does the resolve readiness check poll the guest `/health`, or only the
  ownership route? (It reported `active` while nothing listened — suggests
  route is projected before the guest serves.)

## Evidence refs

- Subagent proof log: RestartProof task (2 attempts), transcripts under
  `history://RestartProof`.
- Goal file: `docs/definitions/choir-ontology-kernel-2026-09-24.md` receipts
  `k-restart-resume-proof-attempt-2026-09-25`.
