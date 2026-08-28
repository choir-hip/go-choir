# Held Computer Boot Crash-Loop and Resolve Race: Owner Primary Bound to a Sealed Computer

<problem_id: held-computer-boot-crash-loop-and-resolve-race-2026-08-28>
<first_observed: 2026-08-27>
<mutation_class: red>
<deployed_commit: c4b7a9a581fbc190dfe291c6b196f9c71b3a7c5a>
<affected_surfaces: [internal/actorruntime/adapter.go, internal/vmctl/ownership.go, internal/proxy/handlers.go, frontend/src/lib/Desktop.svelte, internal/autoputer/run.go]>

## 1. Problem Description

The owner's staging computer still does not boot in the browser. After weeks of
recovery work, the Choir BIOS screen hangs:

```text
CHOIR BIOS
COMPUTER BOOT IS STILL PENDING
00s  Powering user computer
00s  Resolving active computer
15s  Bootstrap probe 1 is still waiting; retrying
```

`https://choir.news/health` (deployed commit `c4b7a9a5`) reports:
`bootstrap.resolve` count 5, errors 3, avg 5410ms, max 15028ms
(`by_status {error:3, ok:2}`); `bootstrap.total` `{http_200:2, resolve_error:3}`;
`surface.auth` errors 7 `{unauthorized:7}`; `surface.total` errors 7
`{platform_shell:7}`.

This is the same user-visible symptom that has been chased as four+ different
root causes over the prior week (`vmctl` resolve global-lock contention;
`vmctl` reserve-epoch deadlock; guest-dependent restore; genesis-surface 503
underivable-SPA; maintenance-serve recovery). It still fails. This receipt
documents the deep cause: **the owner's live desktop is bound to a computer that
the doctrine requires to be a permanently sealed, immutable artifact, and the
maintenance-hold guest fence turns the actorruntime startup reconcile into a
deterministic fatal crash-loop that makes the host resolve never complete
within the browser's 15s probe window.**

## 2. Evidence & Verified Causal Chain (live, 2026-08-28 UTC, Node B)

### A. The guest IS nominally alive — the central contradiction
- `/var/lib/go-choir/vm-state/ownerships.json` for owner
  `5bd6de97-3b58-408c-bf89-c42c81b083de` (yusefnathanson@me.com):
  - computer `computer-03335285269bdba4f94377e56879f9e6`, VM
    `candidate-fleet-e15cb89f25d963c220319b7b`
  - state `active`, epoch `804`, computer_url `http://10.200.2.2:8085`
  - `hold_status`: `{reason:"protect-live-guest-during-hang-diagnosis", held_by:"ox-alpha", held_at:"2026-08-27T23:34:36Z"}`
- Direct guest probe from the host:
  - `GET http://10.200.2.2:8085/health` → `{"status":"ready","service":"autoputer","computer_id":"computer-03335285...","runtime_health":"ready",...}`
  - `GET http://10.200.2.2:8085/api/shell/bootstrap` → `{"computer_id":"computer-03335285...","bootstrap":"placeholder-shell-v1",...}`

So the guest returns 200 with the expected `computer_id` at the sampled instant,
yet the proxy `/api/shell/bootstrap` resolve errors and the browser aborts.

### B. Guest autoputer runtime is in a deterministic crash-loop (THE primary defect)
Node B journal (`go-choir-vmctl`, yusef VM unit
`w3g4gllb97pm6n9xs24g3lbkvh5m0yqn-go-choir-run-autoputer-runtime`): a new PID
every ~18s (2852 → 2890 → 2930 → 2970 → 3012 → … → 3337). Each cycle:
`store: open phase=…` (~14s on the 132k-event tape) → `runtime: maintenance hold
active (RUNTIME_MAINTENANCE_HOLD=1); refusing run admission + agent rewake while
held` → `reconcile Texture owner: reconcile subject …/texture:bda20d6e-…: start
reconciled Texture revision: computer is under maintenance hold: run admission
refused` → process replaced. Last hour: `autoputer: st` ×355, `autoputer: ru` ×69.

**Code path (verified):**
- `internal/autoputer/run.go:363` `rt := actorruntime.New(...)`; `rt` is the
  actorruntime `Adapter`.
- `internal/autoputer/run.go:509-510` (no-replay) and `:581` (replay): `if err :=
  rt.Start(ctx); err != nil { log.Fatalf("autoputer: runtime startup refused: %v", err) }`.
- `internal/actorruntime/adapter.go:583` `a.Runtime.Start(ctx)` → the agentcore
  `Runtime.Start` (`internal/agentcore/runtime.go:580-583`) correctly returns
  `nil` (benign) under the maintenance hold.
- But `internal/actorruntime/adapter.go:592-596` then runs `a.textureOwner.Start(ctx)`;
  the Texture-owner reconcile attempts a run admission, is refused by the hold
  ("run admission refused"), and returns `fmt.Errorf("actorruntime: reconcile
  Texture owner: …")`.
- That error propagates to `log.Fatalf("autoputer: runtime startup refused: …")`
  at `run.go:510`/`:581` → process exits → guest systemd `Restart=on-failure`
  relaunches → 18s loop.

The maintenance-hold fence is supposed to make run-admission refusal a *benign
held state*, but the actorruntime startup reconcile path converts it into a
*fatal startup failure*. The held computer can therefore never stay up; it is
deterministically churned every ~18s.

### C. Host resolve is a race that the browser's 15s abort turns into a failure
- `internal/proxy/handlers.go:609` `HandleBootstrap` calls
  `resolveComputerURLForComputerTarget(r.Context(), …)`. The `r.Context()` is the
  browser request context.
- `frontend/src/lib/Desktop.svelte:119` `BOOTSTRAP_PROBE_TIMEOUT_MS = 15_000`;
  `fetchBootstrapProbe` aborts `/api/shell/bootstrap` after 15s; on `AbortError`
  it logs `Bootstrap probe ${attempt} is still waiting; retrying`.
- `internal/vmctl/ownership.go` `resolveDesktopContext` → `activeOwnershipNeedsReadinessCheck`
  (`ownership.go:809`) returns `true` because `time.Since(LastActiveAt) >=
  activeResolveReadinessCheckInterval` (or `mgr.GetVM` reports not-ready on the
  flapping guest), regardless of `IsHeld()`. This registers
  `r.pendingWaiters[key] = nil` and calls `ensureActiveVMReady`.
- A concurrent browser probe enters `waitForPendingAssignmentLocked`
  (`ownership.go:1591-1604`) and blocks on `select { <-ch; <-ctx.Done() }`.
- Browser aborts at 15s → proxy `r.Context()` canceled → vmctl
  `ResolveDesktopContext` ctx canceled → `ownership.go:1603` returns
  `vm assignment canceled for user …: context canceled`.
- Proxy records `bootstrap.resolve` error / `bootstrap.total` `resolve_error`
  (`handlers.go:612-616`).

Node B `go-choir-vmctl` log:
```
2026/08/28 01:30:47 vmctl: resolve failed for user 5bd6de97-… desktop primary: vm assignment canceled for user 5bd6de97-… desktop primary: context canceled
2026/08/28 01:30:50 vmctl: resolve failed for user 5bd6de97-… desktop primary: vm assignment canceled for user 5bd6de97-… desktop primary: context canceled
```

### D. Doctrine contradiction (the deepest cause)
- `docs/definitions/choir-0333528-stabilize-and-hold-2026-08-24.md` `now.status:
  settled_and_sealed`: "Seal computer-0333528… under permanent hold as an
  immutable historical evidence artifact."
- `docs/definitions/choir-substrate-cleanup-and-cutover-2026-08-25.md` invariant
  `sealed-historical-artifact`: 0333528 "remains sealed under permanent host hold
  (held=true) and guest fence (RUNTIME_MAINTENANCE_HOLD=1) as an immutable
  historical evidence artifact. Linear 133k replay is never a release, test, or
  CI gate."
- Yet the owner's `primary` desktop still routes to `computer-0333528…`
  (`computer_version_route_slots` generation 1), and the `maintenance-serve`
  recovery booted it to `active`/epoch 804 under the hold. So the live primary
  is the sealed artifact. The maintenance hold was intended to keep it
  *stopped/immutable*; re-activating it as a live, booting primary puts it in an
  impossible state: held (must not accept runs) but expected to serve a bootable
  surface. The crash-loop (B) is the guest faithfully refusing to be a live
  computer while under the permanent hold.

## 3. Required Repair Invariants

1. **Guest fence must be benign, not fatal.** Under `RUNTIME_MAINTENANCE_HOLD=1`,
   the actorruntime startup reconcile path (`adapter.go` `textureOwner.Start`)
   must treat "run admission refused (held)" as an expected held state and
   continue serving `:8085` (a held-but-`ready` health), not `log.Fatalf`
   ("runtime startup refused"). This alone stops the 18s crash-loop.
2. **Host resolve must be deterministic for a held computer.**
   `activeOwnershipNeedsReadinessCheck` (`ownership.go:809`) must short-circuit
   `IsHeld()` and return the persisted `ComputerURL`/`State` without registering
   `pendingWaiters` or entering the readiness/assignment wait. A held, serving
   computer must resolve instantly.
3. **Bootstrap resolve must not be owned by the browser request context.** A
   client abort must cancel an HTTP exchange, not the lifecycle operation. Use a
   server-owned bounded context (`context.WithoutCancel(r.Context())`-style) for
   the vmctl resolve, or expose a durable bootstrap-status surface the browser
   polls, so a 15s abort cannot cancel a legitimate in-flight assignment.
4. **Owner's computer is a normal always-warm computer (owner directive 2026-08-28
   — supersedes the sealed-artifact / fresh-computer framing).** New doctrine:
   there is no "primary" computer; all computers are homogeneous and differ only
   by keep-warm tier. `yusefnathanson@me.com`'s computer is a normal
   `premium_always_on` (always-warm) computer with its files recovered and email
   continuity wired. The prior "seal `computer-0333528…` immutably and cut over
   to a fresh computer" exit is superseded. The owner's computer should be made
   live, always-warm, and usable; the durable decision is whether to reuse
   `computer-0333528…` in place (un-hold + crash-loop fix + File-CAS restore +
   Track M) or mint a fresh ComputerID and migrate files/mail. This requires
   owner ratification; it is the durable resolution, not a patch.
5. **Acceptance must prove stable boot, not a sampled 200.** Require product-path
   browser convergence, two consecutive matching bootstrap `computer_id`s, and
   no unresolved `pendingWaiters`/`resolve_error` (as the hold Definition
   required "no restart loop"). A single `/health` 200 sampled during a 4s
   serving window is not proof.

## 4. Problem Classification & Ceremony

- **Mutation class:** `red` (vmctl lifecycle, guest runtime startup, route
  authority, browser protocol).
- **Protected surfaces:** `internal/actorruntime/adapter.go`,
  `internal/vmctl/ownership.go`, `internal/proxy/handlers.go`,
  `internal/autoputer/run.go`, `frontend/src/lib/Desktop.svelte`,
  `docs/definitions/choir-0333528-stabilize-and-hold-2026-08-24.md` (permanent
  hold authority).
- **Pre-fix documentation rule:** this receipt is the problem record. No repair
  code is committed before this artifact. Fixes (3.1/3.2/3.3) are reversible
  code repairs; 3.4 requires owner ratification.
- **Rollback:** revert the specific repair commit(s) on `main`; the hold and the
  sealed computer remain untouched.

## 5. Consensus Panel

Agentic consensus (divergent, 10/12 succeeded; devin/x-preview-f/luna failed on
model/resource limits, not dissent) — `.agentic-consensus/
agentic-consensus-20260827-213813/` — foregrounded the same deep theme as this
receipt: the guest crash-loop (Options: guest-runtime lifecycle), the host
resolve no-stability-contract (host `waitForPendingAssignmentLocked`), the
request-scoped-context protocol defect (browser abort cancels the resolve), the
missing held short-circuit (`activeOwnershipNeedsReadinessCheck` ignores
`IsHeld`), and the doctrine contradiction that 0333528 should not be the live
primary. hy3 reinforced the missing product-path / read-only-artifact option and
the acceptance-criterion meta-fix. Failure of repair invariants 3.1–3.3 masks
the real defect (3.4).

## 6. Next Safe Probe

On Node B, run `journalctl -u go-choir-vmctl --grep "runtime startup refused"`
to capture the exact `log.Fatalf` line and confirm it fires on each restart
cycle. Then authorize a repair under invariants 3.1–3.3 (guest-fence benign
crash fix + held resolve short-circuit + request-context decoupling), and
separately record the owner decision on 3.4 (reuse `computer-0333528…` as a
normal always-warm `premium_always_on` computer with files recovered and Track M
email continuity, vs mint a fresh ComputerID and migrate files/mail — owner
directive 2026-08-28 supersedes the seal / fresh-computer exit) before any red
route mutation. The maintenance hold was set under the superseded exit; clearing
it is now an owner-directed, doctrinally-permitted operation but still requires
owner ratification and the crash-loop fix first. Do not delete or rewrite
`computer-0333528…` or its `data.img.*` snapshots.

## 7. Refined Doctrine & Fix-Order (full 8-model panel, 2026-08-28)

The owner refined the directive again (verbatim): no "owner"/"primary"
computer — anyone registers an account and gets one VM; CLI/UI must not treat any
user (incl. `yusefnathanson@me.com`) specially; always-warm is a general
per-computer tier to be gated by premium pricing (TBD); recover the account's
machine state + email (`000@choir.news`), mechanism flexible.

Full agentic consensus (8 models: gemini37, hy3, nemotron, opencode, cursor,
muse-spark, cursor-grok46, claude) — `.agentic-consensus/agentic-consensus-20260827-221528/`.
Code-grounded facts that reshape the fix (Claude, verified against source):

1. **No per-user special-casing in product code.** Go/Svelte has no `yusef`/
   `5bd6de97` special-case except the `yusefmosiah` module path. Special-casing
   lives only in host env (`VMCTL_ALWAYS_ON_USER_IDS`, `cmd/vmctl/main.go:491`)
   and host operator verbs. The doctrine's "no special-casing" is already true
   in product code.
2. **General always-warm tier already exists.** `VMOwnership.WarmnessClass`
   (`ownership.go:102`) is a per-computer field honored by
   `warmnessClassForOwnership` (`warmness_policy.go:86-101`); `AlwaysOnUserIDs`
   is an override on top of it. Doctrine Q3 is a *deletion*, not a build. **Trap:**
   deleting the map falls back to `WarmnessClassPrimary` — deletion without a
   backfill silently demotes the account mid-recovery.
3. **`RouteSlotID(ownerID, computerID)` already names the arg `computerID`**
   (`routeledger/ledger.go:94`) while every caller passes `PrimaryDesktopID`.
   The doctrine change is partly a correction of what the code already claims. But
   `ParseRouteSlotID` round-trip validation means a half-migrated ledger fails
   closed for *every* account → red-class, needs a dual-read window.
4. **Two independent holds.** Host `HoldStatus` (`ownership.go:135`) and guest
   `RUNTIME_MAINTENANCE_HOLD=1` (`agentcore/runtime.go:612`). The crash-loop is
   the **guest** hold; clearing only the host hold reports success and leaves the
   loop running.
5. **Email recovery is strictly downstream of compute recovery** — every mail
   drain terminates at the guest `/api/mail/inbound` (`maild/drain.go:87`).
6. **The 15s browser abort cancels vmctl's pending assignment** — a one-line
   `BOOTSTRAP_PROBE_TIMEOUT_MS` change may restore reachability with no VM work,
   but it masks the 14s store-open regression rather than fixing it.

Must-pin before any red code (unanimous): (i) is `computer-0333528…` the live
computer or a sealed archive — owner said **live always-warm** (supersedes seal);
(ii) "one computer" = a live VM pointer or ComputerID uniqueness; (iii) always-warm
is **capability-open-to-all + entitlement-gated** (premium) vs default-on;
(iv) mail SoR is host `mail.db` or guest Maildir for acceptance.

## 8. Landing Status (2026-08-28 UTC, verified)

- **fix (a) landed:** commit `eb27cac8` ("treat maintenance hold as benign on
  Texture reconcile") on `main`; push CI `33141788363` **success** (all agentcore/
  textureowner race shards green), Node B `Deploy to Staging` with active VM
  refresh; `/health` `deployed_commit` = `eb27cac8`. Guest is no longer expected
  to crash-loop on the hold (the fatal `log.Fatalf` path is removed).
- **Live symptom confirmed pre-fix:** guest flapped — api-key `/api/shell/bootstrap`
  returned 200 (probe 1, guest briefly up) then 502 (probes 2-5) over a ~40s
  window, matching the documented ~18s crash-loop while the guest churned.
- **Post-deploy state:** `/health` `deployed_commit` `eb27cac8`, but the computer
  now reports `state: stopped` (was `active`/epoch 804). The deploy's active-VM
  refresh recycled the VM; a **held** computer is never auto-started on resolve
  (`resolveDesktopContext` refuses auto-start under `IsHeld()`), so it stays
  stopped. The api-key bootstrap now 502s because `target.State != "active"`.
- **Blocker for (c):** clearing the host+guest holds is the owner-directed step to
  make `computer-03335285…` a live always-warm computer. `SetHold`/`ClearHold`
  are only reachable through internal vmctl control (`isInternalCaller`); there is
  **no product-path unhold verb** (no proxy endpoint, no `choir computer unhold`),
  and this shell has no SSH to Node B (`go-choir-node-b` unresolvable). Either the
  authorized operator clears the host hold (which stops passing
  `RUNTIME_MAINTENANCE_HOLD=1` on the next guest boot), or a general
  product-path recovery verb is built (doctrine gap, red). `fix (a)` must be on
  the guest before the guest boots without the fence so the hold-fatal is gone.
- **fix (b) staged locally, not pushed:** `34896b7e` (held computer
  short-circuits `activeOwnershipNeedsReadinessCheck` so a held, serving computer
  resolves instantly without a pendingWaiter race). Held locally to avoid another
  deploy cycle while the computer is stopped+held, since it only helps the
  held+`active` state. Land it as part of the recovery commit once unhold is
  authorized or as a standalone.
- **Acceptance not yet met:** no stable browser boot, no unheld live computer,
  no guest Maildir / SMTP. Problem receipt remains open.
