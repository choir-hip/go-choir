# The Daemon Answers a Desk Worker's Frames Without Checking the Desk's Role

Date: 2026-10-21. Found while auditing desk isolation for the
[security, density and architecture checklist](../security-density-architecture-checklist-2026-10-21.md)
(item SEC-02). Read from code at `origin/main` 8f71f9c64; nothing was run.
Status: documented; no fix yet. Mutation class of any fix: **red** (desk
authority boundary, product API reachability from model code).

## The problem in one sentence

Desk-cell confinement (F4,
[desk-cells-run-as-guest-root-and-egress-is-open-2026-10-14.md](desk-cells-run-as-guest-root-and-egress-is-open-2026-10-14.md))
assumes "every effect is a frame the host answers", but for most actions the
daemon does not check which desk sent the frame, so a cell that escapes the
Yaegi verb surface can ask the daemon for any action any desk may use.

## Evidence (code)

- `serviceEgress` decodes the worker's `BrokerRequest` and calls the egress
  handler with `req.Action` and `req.Payload`. It never reads `req.HandleRef`
  or `req.Epoch` (`internal/yaegikernel/desk_session_worker.go:439-452`).
- The capability handles are minted and verified inside the worker: the
  issuer secret is random per worker (`internal/yaegikernel/sidecar.go:76-80`)
  and the check is in the in-worker broker (`internal/yaegikernel/broker.go:150`).
  A handle is therefore evidence only to code that runs in the worker, which
  is the side being contained.
- `deskEgress` (`internal/agentcore/tools_desk.go:1069-1090`) gates by role
  only for `System1` (`system1_verb.go:135-137`) and `Stories`
  (`intake_view.go:68-75`). `ProductAPI` (`tools_product_api.go:127-141`,
  route allowlist only), `Research` children (`research_children.go:140`),
  the evidence tools and `researchDeps.HostEgress` take no role.
- Every non-research, non-task desk handle carries `ActionExec` and
  `ActionWriteFile` (`internal/yaegikernel/choir.go:133`), and the in-worker
  broker executes them (`broker.go:158-192`). Only the module export sets
  (`deskModuleSets`, `choir.go:320-373`) keep them from Texture and
  management cells.

## Why it matters

- Confinement alone (seccomp without `socket`, an empty network namespace,
  Landlock) does not stop a compromised cell from writing frames on fd 3.
  Whatever the daemon answers is the cell's real authority.
- Research reads untrusted web content and is the likeliest
  prompt-injection entry. Today a research cell that reaches raw frames can
  ask for `ProductAPI` actions the research desk is never offered.
- App desks (Mail next; owner notes §36) multiply desk roles. A per-action
  role table checked by the daemon is the one place that scales with them.

## Failure modes any fix must close

1. A frame names an action outside the sending desk's role table and is
   served.
2. The daemon trusts a role or desk name carried in the frame payload rather
   than the run record it owns for that worker.
3. A handle minted in one worker is replayed by another worker or after the
   worker's session ends (epoch not bound).
4. Research children (`choir.Research`) inherit the parent's actions instead
   of the narrower child set.
5. `ActionExec` / `ActionWriteFile` remain reachable for desks that never
   run commands or write files, so removing an export is the only barrier.
6. A new desk (app desk) is added without a role-table entry and falls open
   rather than closed.

## Fix direction (not designed)

The daemon binds each worker to the run record it spawned it for, and
checks every frame action against that desk's role table before serving
it; unknown desks fail closed (as `choir.go` already does for unknown roles
in the worker). The table should come from the desk manifest the checklist
proposes (ARCH-02), so app desks get it by construction.
