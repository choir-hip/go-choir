# Claude delegate: `claude` crashes at start under the capsule seccomp filter

Date: 2026-10-15. Mutation class of the fix: **red** (capsule isolation:
the seccomp filter of the broker and of the delegate stage). This note is
evidence only; the fix comes in a later commit. It closes the first item of
the UNVERIFIED list in
[claude-delegate-slice1-2026-10-14.md](../../design/claude-delegate-slice1-2026-10-14.md) §6
("Claude Code (Bun) runs under the capsule's seccomp allowlist and
Landlock"): it does not.

## Evidence (staging)

- Computer `computer-33594053a05f46e93d80e7e15ce8a0d2` (allowlisted,
  delegate `available`, model `claude-opus-5-5`), staging build `4bf25d82a`.
- Engineering run `run:assignment-ddc336b09bd55699415b1325` (started
  01:59:14Z) called `choir-delegate start` twice. Both delegate runs failed
  at once: Bun crashed, exit 134, `panic: us_create_timer returned null`.
- `scripts/claude_delegate_probe.py` reported "no engineering run appeared
  for the trajectory in time", although that engineering run existed. That
  is a second, separate bug (the probe), below.

## Reproduction (local, same binary, same filter)

- Binary: Claude Code 2.1.197, the nixpkgs pin's own download
  (`downloads.claude.ai/claude-code-releases/2.1.197/linux-arm64/claude`),
  sha256 `fb48473c467c27615ac799a754f4ef0b68c363e4596cefbb59c3815d51a0cc8a`, which matches nixpkgs' `manifest.json` at the
  flake.lock rev `ec840546`. It embeds Bun 1.4.0.
- Filter: a throwaway harness (not committed) loads exactly what
  `runDelegateStage` (cmd/capsule-broker/delegate.go) runs under: the
  broker's L2 filter it inherits (`BrokerInetSeccompFilter`), then the
  workload L2 filter it stacks (`WorkloadInetSeccompFilter`), then execs
  `claude` with the delegate's real argv (`claudedelegate.Args`) and
  environment (`claudedelegate.Env`, with an obviously fake token).
- Platform: Docker Desktop, linuxkit 6.10, **aarch64**. The allowlist is
  written in x86_64 names; go-seccomp-bpf refuses to assemble it on
  aarch64 (`open`, `stat`, `epoll_wait`, `arch_prctl`, ... do not exist
  there), so the harness drops the 33 x86_64-only names (each has an
  `*at`/modern twin already in the list, or, like `arch_prctl`, no
  aarch64 counterpart) and maps `newfstatat` to aarch64's `fstatat`. An x86_64 container under emulation cannot load a
  seccomp filter at all (`EINVAL`), so x86_64 was not run.
- `strace -f` records every refusal (`= -1 EPERM`).

Result with the current filter, `claude --version` and the delegate argv
alike: exit 134,

```
panic: us_create_timer: returned null: 1
oh no: Bun has crashed. This indicates a bug in Bun, not your code.
```

the staging failure exactly.

## What the filter refuses

The delegate stage inherits the broker's filter and stacks the workload
filter on it; seccomp returns the strictest answer of the stack, so a call
must be in **both** to run. Neither has the calls below.

| Call | Who, when | Effect of the refusal |
|---|---|---|
| `timerfd_create` (then `timerfd_settime`) | uSockets' loop timer, at startup (`us_create_timer`) | **fatal**: Bun panics, exit 134 (the staging failure) |
| `rt_sigsuspend` | JavaScriptCore parks a thread for the garbage collector: SIGPWR to the thread, whose handler calls `sigsuspend` until resumed (5 refusals in one short run, all with mask `~[PWR RTMIN RT_1]`) | **silent**: `sigsuspend` returns at once, so the "suspended" thread keeps running while the collector scans its stack. No crash at start; a heap-corruption risk for the whole run |
| `epoll_pwait2` | Bun's event loop, first wait | Bun falls back to `epoll_pwait` (observed) |
| `preadv2` (`RWF_NOWAIT`) | Bun reading stdin | falls back to `ppoll` + `read` (observed) |
| `pidfd_open` | Bun's spawn | falls back to its waiter thread + `wait4` (observed: `rg`, `ps`, `bash` children ran and were reaped) |
| `sched_setscheduler` | a Bun/JSC thread priority | ignored |
| `socket(AF_NETLINK, …)` | glibc address checks | refused on purpose (L2 admits unix/inet/inet6 only); no effect seen |
| `fadvise64` | coreutils children (`cat`, `head`) | ignored |
| `process_vm_readv` | Bun's crash handler, only after the panic | none |

With `timerfd_create`, `timerfd_settime`, `timerfd_gettime` and
`rt_sigsuspend` added to both layers, `claude --version` prints
`2.1.197 (Claude Code)`, and a full `-p` run with the delegate's argv
against a fake Messages API on loopback (one Bash tool call, then a text
answer) completes: `"subtype":"success"`, `num_turns` 2, exit 0, and no
refusal left except the fallbacks above. `timerfd_gettime` was not
called in these runs; it is the read half of the same timer API and is
admitted with it.

## The next failure, already visible locally

With the timer fixed, the delegate starts, talks to the API and edits
files, but **every Bash tool call fails**:

```
bwrap: Creating new namespace failed: Operation not permitted
```

The delegate's environment sets `CLAUDE_CODE_SUBPROCESS_ENV_SCRUB=1`
(spec.go, `Env`), and 2.1.197 then runs each Bash command inside
bubblewrap (without `bwrap` on PATH it refuses to start: "bubblewrap is
required for subprocess env scrubbing and isolation"; the nixpkgs wrapper
puts `bwrap` on PATH). As uid 0 (the capsule's root), bwrap takes its
privileged path, `clone(CLONE_NEWNS|CLONE_NEWPID)`, which needs
`CAP_SYS_ADMIN`; the workload keeps only `CAP_DAC_OVERRIDE` and
`CAP_FOWNER`. The same `bwrap` call fails the same way with no seccomp
filter at all in an unprivileged container and succeeds only with
`--privileged`, so this is the capability floor, not seccomp; and with
the capability, the filter would still refuse `mount` and `pivot_root`.
The same flag also logs "Permission mode forced to default —
CLAUDE_CODE_SUBPROCESS_ENV_SCRUB is set"; tools stay limited to
`--allowedTools`. With `CLAUDE_CODE_SUBPROCESS_ENV_SCRUB=0` the Bash call
runs (`/bin/bash: go: command not found` in the test container; no `go`
there) and the mode stays `dontAsk`.

This is a credential-custody decision, not a syscall one, and is left
open as a named residual: (a) keep the scrub, and the delegate edits
while engineering runs the tests in its own cell (its re-run is the
verifier anyway, F15); (b) turn the scrub off and accept that Bash
children see the token in their own environment, a small step beyond
the ratified residual (§5: children can already read the parent's
`/proc/<pid>/environ`); (c) give the delegate stage the namespace
calls and capability bwrap needs (a wider red change).

## UNVERIFIED

- x86_64: the refused set above is from aarch64. Bun and JSC take the
  same paths on x86_64, and every call in the table has the same name in
  both tables, but the staging kernel's exact set was not traced.
- Landlock: linuxkit has no Landlock, so the delegate's workload Landlock
  layer was not in the reproduction; a Landlock refusal would read
  `EACCES`, not `EPERM`. The capability drop also failed in the
  unprivileged container; it does not change seccomp.

## Second bug: the probe cannot find the engineering run

`scripts/claude_delegate_probe.py` looks for
`r.get("trajectory_id") == tid` in `GET /api/runs?limit=200`. The run
list's JSON (`runStatusResponse`, internal/agentcore/api.go) has no
top-level `trajectory_id`: an engineering run, opened by management's
Cast as `engineering:<assignment>` / `run:<assignment>`, carries the
Texture document's trajectory in its trajectory column and in
`metadata.trajectory_id` (`buildAssignedEngineeringRun`). So the match
never succeeds, whatever the run does. The probe also reads the run id
as `loop_id`; the field is `run_id`.

## Fix shape

1. A delegate-only profile: the four calls above go into a new delegate
   filter (workload L2 + delegate calls) that `runDelegateStage` loads,
   and into the broker filter **only in a capsule that holds a delegate
   token** (the delegate stage inherits it). Cells, session workers and
   jobs keep stacking the unchanged workload filter, so they gain nothing;
   other capsules' brokers are unchanged.
2. The probe asks `GET /api/runs?trajectory_id=<tid>` (which matches the
   column and `metadata.trajectory_id`), reads the trajectory from
   `metadata` as well, reads `run_id`, and on timeout lists the
   engineering runs it did see.

## Fix (branch `fix/claude-delegate-bun-seccomp`, not deployed)

- `internal/capsule/seccomp.go`: `delegateExtraSyscalls`
  (`timerfd_create`, `timerfd_settime`, `timerfd_gettime`,
  `rt_sigsuspend`); `DelegateSeccompFilter` (workload L2 + those) and
  `BrokerDelegateSeccompFilter` (broker L2 + those), with loaders.
- `cmd/capsule-broker`: the broker loads the broker-delegate filter only
  when it will serve the delegate (`delegateServed`: token, binary and
  egress); `runDelegateStage` loads the delegate filter. Session workers
  and jobs still load the workload filters, unchanged.
- The `epoll_pwait2`, `preadv2` and `pidfd_open` refusals are left in
  place: Bun's fallbacks ran in every trace.
- Tests (`internal/capsule/seccomp_delegate_linux_test.go`, failure
  modes D1-D6 written first): both delegate filters admit the four calls;
  each adds exactly those four to its base, with the same socket
  families; the workload, workload L2, broker and broker L2 filters admit
  none of them; and in helper processes a timerfd is created, armed and
  read under the broker-delegate + delegate stack, while a cell's stack
  (broker-delegate + workload L2) and plain broker L2 + delegate both
  get `EPERM`. Run in `golang:1.26` on aarch64 with the same name
  mapping as the harness (the existing seccomp tests need it too there);
  CI runs them on x86_64 as written.
- Re-run of the reproduction with the production filters (mapped to
  aarch64): `claude --version` prints `2.1.197 (Claude Code)` with no
  refusal; the full fake-API `-p` run ends `"subtype":"success"`, exit
  0, with only the fallback refusals and the bwrap failure above.
- The probe: `engineering_run` (scripts/claude_delegate_probe.py).
