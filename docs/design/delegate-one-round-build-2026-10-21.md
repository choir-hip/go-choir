# The delegate builds in one round: a one-way registry latch

Date: 2026-10-21. Status: design, written before the code; panel pending.
Owner decision: owner notes section 34 ("yes, ill go with your
recommendatation for #1"), option 1 of failure 26 in
[engineering-completion-path-cluster-2026-10-20.md](../problems/engineering-completion-path-cluster-2026-10-20.md).
Mutation class: red (capsule egress, a network grant).

## 1. Problem

A Claude delegate round runs in a `claude`-mode capsule whose egress
admits the model API and the host Go module proxy only
([claude-delegate-slice1-2026-10-14.md](claude-delegate-slice1-2026-10-14.md)
section 3). A frontend rewrite needs `npm ci` and `npm run build`, so every
delegate rewrite needs a second, `ecosystem_proxy` round, and that
handoff takes three model-mediated desk turns. Eight SlidesApp drills
stopped in it (failures 19 to 26); the delegate's own work never failed.

## 2. Why "jobs get the registry, the delegate does not" is not a proxy rule

The capsule has one network namespace and one egress socket, and the
proxy cannot tell which process opened a connection. The delegate's Bash
tool is limited to go, gofmt and make, but `go run` and `make` execute
arbitrary code, and a process the delegate starts can outlive its turn:
the delegate runs with `Setpgid` only, so a child that calls `setsid`
leaves its process group. Its files also outlive it, and some of them
execute during the build: `npm ci` reads `package.json`,
`package-lock.json` and any `.npmrc`; Vite executes `vite.config.*` and its
plugins. Registry egress that is open while delegate code can run, or
while files the delegate wrote can execute, is egress for the delegate.

## 3. Design

One round, in the `claude`-mode capsule, in this order:

1. **Delegate phase.** As today: egress is the claude policy, the
   delegate writes within its spec's `paths`. Change: the delegate's
   `claude` process runs in its own PID namespace (the broker already
   starts itself with `CLONE_NEWPID`), so killing that namespace's init
   kills every process the delegate started, `setsid` or not.
2. **Latch (one-way, per capsule).** Engineering starts its build job with
   `network: "registry"`. Before the job starts, the trusted runtime asks
   the broker to close the delegate: the broker refuses every later
   `choir-delegate start` for the capsule's life, kills the delegate PID
   namespace and waits until it is reaped. The latch is never reopened.
3. **Path gate.** Outside the capsule, the executor diffs the capsule's
   workspace against its base (`DiffTreesToPatch`, as the carried patch
   does) and also lists the overlay's upper layer. The registry is
   granted only if every changed path is inside the delegate spec's
   `paths` and inside `frontend/src/`, the tree Vite bundles but never
   executes at build time, and nothing changed anywhere else in the
   upper layer except the delegate's own run directory
   (`/tmp/choir-delegate/`). This is an allowlist, so the files that do
   execute during the build stay unchanged by construction: `package.json`
   and its lockfiles, `.npmrc`, `vite.config.js`, `svelte.config.js`,
   `frontend/scripts/` (the build runs
   `node scripts/generate-source-contract.mjs --check` before
   `vite build`), the job's HOME, `/etc` and the toolchain. Otherwise
   the job is refused with the offending paths, and the capsule stays on
   the claude policy.
4. **Widen.** The executor swaps the capsule proxy's policy to a
   registry policy: `registry.npmjs.org` only (not the full L2 list: no
   GitHub, PyPI or Hugging Face), upload budget 16 MiB counted from zero,
   download 2 GiB. The model API route is dropped (the delegate is gone).
   The swap is recorded in the egress ledger.
5. **Build, freeze, complete.** The overlay's build job runs unchanged
   (`npm ci && npm run build` and the release copy), then `choir.Freeze`
   and Complete, in the same round. No carried patch, no second cast.

The authority to widen sits outside the capsule: the runtime calls the
executor, and the executor checks the latch report and the path gate
itself. Nothing inside the capsule can ask for the wider policy.

## 4. What this grants, honestly

After the latch, code that runs in the capsule is the build: npm's
install scripts for the locked dependencies (integrity-checked against
the unchanged lockfile), the unchanged source-contract check script, Vite
with the unchanged config, and the Svelte
compiler over the delegate's component, which compiles but does not
execute it. That is the same code today's round 2 runs with the full L2
list, after a model only read the diff. So the grant is narrower than
today's round 2 on three counts: one registry host instead of twenty, a
mechanical path gate instead of a model's reading, and no delegate
process alive.

## 5. Failure modes (the tests, written before the code)

1. A delegate child that called `setsid` is still running when the
   latch reports closed.
2. `choir-delegate start` succeeds after the latch.
3. The registry is reachable while a delegate run is live.
4. A changed `package.json`, lockfile, `.npmrc`, `vite.config.js`,
   `svelte.config.js` or file under `frontend/scripts/`, or any changed
   file outside both `paths` and `frontend/src/`, passes the gate.
5. A changed file under the job's HOME or `/etc` passes the gate.
6. The widened policy admits a host other than `registry.npmjs.org`, or
   still admits the model API.
7. The upload budget the delegate spent makes the build's first
   connection fail (the counters must restart at the swap).
8. A capsule that is not `claude` mode, or a capsule with no delegate
   spec, can take the registry latch.
9. Something inside the capsule (a cell, the delegate, a job) can trigger
   the widen.
10. A refused gate leaves the capsule half-widened.
11. A restart during the latch resumes the round (it must not: the crash
    rule closes the work).

## 6. Rollback

Git revert of the mission commits. With the change reverted, the
two-round path (claude round, carried patch, `ecosystem_proxy` round) is
unchanged and still works.

## 7. Open questions for the panel

- Is the upper-layer listing plus the base diff enough to see every file
  the build reads, or is there a read path (a cache, an env file, a
  `node_modules` planted by the delegate) the gate misses? The build job
  runs `npm ci`, which deletes `node_modules` first.
- Is `frontend/src/` really never executed at build time? The
  source-contract script reads it; does it import any of it?
- Is killing the delegate PID namespace enough, or can a delegate process
  leave state that runs later without a process (a `systemd`-style timer
  does not exist in the capsule; cron does not either)?
