# Claude Delegate, Slice 1: `claude -p` in One Engineering Capsule

Date: 2026-10-14. Mutation class: **red** (credential custody, vmctl,
capsule egress, the guest image). Source of the design:
[claude-agent-sdk-in-choir-2026-10-14.md](../reports/claude-agent-sdk-in-choir-2026-10-14.md)
(owner ruling at its top: Option T now, Choir's own harness on the API
later). This note records what Slice 1 builds, the failure modes written
before the code, and the red-class ceremony.

Slice 1 is an experiment, not an architecture. Everything is behind one
host switch that is off by default (an empty allowlist), and the whole
slice deletes cleanly: the files it adds are listed in §7.

## 1. What it does

When the owner's token is on Node B and one ComputerID is on the
allowlist, that computer's engineering desk can hand code-writing to
headless `claude -p` running inside its implementation capsule,
signed in with the owner's own `claude setup-token` token. Engineering
starts the run, polls it, then verifies the result with its own test
re-run. Nothing else changes for any other computer.

The path of the token, end to end:

1. The owner writes `/var/lib/go-choir/secrets/claude-oauth-token`
   (root, 0400) on Node B. No agent ever touches it.
2. vmctl sees it through a read-only bind
   (`/run/go-choir-claude-delegate/claude-oauth-token`; the secrets
   directory itself stays hidden from vmctl and its VMMs).
3. When vmctl cold-boots a computer on `CHOIR_CLAUDE_DELEGATE_COMPUTERS`
   before `CHOIR_CLAUDE_DELEGATE_EXPIRES`, it writes the token and a
   grant record (`claude-delegate.json`: computer, expiry, status,
   reason, never the token) onto that VM's root-only credential disk
   (`credential.img`, the same disk that carries the gateway token).
   For an allowlisted computer without a usable token it writes only the
   grant, with the reason. For any other computer it writes nothing.
4. In the guest, the trusted runtime (guest core, root) reads the grant
   and token from `/run/choir-bootstrap` only when it spawns a
   `claude`-mode capsule, and re-checks the expiry each time.
5. The executor hands the token to that capsule's broker in the broker's
   own process environment. The broker reads it once, removes it from
   its environment, and keeps it in memory.
6. The broker serves `/run/capsule/delegate.sock`. A cell runs
   `choir-delegate start` (through `choir.Exec`), which asks the broker
   to start a run. The broker starts `claude` through a hardening stage
   of its own binary (workload Landlock layer, capability drop, the L2
   seccomp filter) with `CLAUDE_CODE_OAUTH_TOKEN` in that process's
   environment and nowhere else.

No relay injects the token into requests (Anthropic's terms forbid
intermediating subscription credentials). The token sits in the
unmodified binary's process environment, which is the carve-out shape.

## 2. The switch

All on vmctl, in `nix/node-b.nix` (`go-choir-vmctl` `Environment`):

- `CHOIR_CLAUDE_DELEGATE_COMPUTERS`: ComputerIDs, comma or space
  separated. **Empty in this commit.** Empty means the slice is off.
- `CHOIR_CLAUDE_DELEGATE_EXPIRES`: RFC 3339 time or `YYYY-MM-DD` (UTC
  midnight). Missing or unparseable means off. Past means off.
- `CHOIR_CLAUDE_DELEGATE_TOKEN_FILE`: where vmctl reads the token (the
  bind target above). Fixed by the nix config.

The guest re-checks the same expiry from the grant record, so a computer
resumed from a memory snapshot (which keeps its old credential disk)
still loses the delegate when the window closes.

## 3. The `claude` capsule network mode

An implementation assignment on a computer whose delegate is available
gets network mode `claude` instead of `ecosystem_proxy` (L2), unless the
Cast withholds the network (`forbidden`/`none`/`off`, as today) or asks
for `"network": "ecosystem_proxy"` explicitly (npm, GitHub and PyPI are
not reachable in `claude` mode). In `claude` mode:

- egress: CONNECT to `api.anthropic.com:443` only, plus the host Go
  module proxy route (`goproxy.choir.internal`). Every other host is
  refused and recorded in the capsule's egress ledger;
- budgets: upload 512 MiB (L2: 16 MiB; Claude Code resends the whole
  conversation on every request), download 1 GiB, 4096 connections;
- capsule: 3 GiB and 2 CPUs and 512 pids when the guest's capsule budget
  holds 3 GiB (an 8 GiB interactive guest does; a 4 GiB guest's budget is
  about 2.85 GiB), else the ordinary 2 GiB, 1 CPU and 256 pids;
- the spawn fails closed if the token is not available at that moment.

## 4. `choir-delegate`

`choir.Exec` stops at 30 s and a cell at 60 s, so a run is detached and
polled. `choir-delegate` (on the guest PATH, `cmd/choir-delegate`) is a
thin client of the broker's socket and the run directory:

- `choir-delegate status`: is the delegate available here, and why not;
- `choir-delegate start --spec-json '<json>'` (or `--spec <file>`):
  starts one run, returns its id at once;
- `choir-delegate wait --id <id> --seconds <=25`: run state, progress,
  and once finished the result record;
- `choir-delegate stop --id <id>`.

The spec: `objective` (required, at most 16 KiB), `paths` (the
directories under `/workspace/platform` the delegate may edit),
`test` (the argv engineering will re-run), `max_turns` (default 40, at
most 200), `max_budget_usd` (default 20, at most 100, notional under a
subscription), `wall_clock_min` (default 30, at most 120),
`resume_session` (optional session UUID from a previous run).

The flags are fixed in `internal/claudedelegate` and every one of them
was found in the 2.1.197 binary (the nixpkgs pin); an unknown option
makes Claude Code exit at once, so the list is pinned by a test:

```text
claude -p --model claude-opus-5-5 --output-format stream-json --verbose
  --max-turns N --max-budget-usd B
  --tools Read,Edit,Write,Glob,Grep,Bash
  --allowedTools Read,Edit,Write,Glob,Grep,Bash(go:*),Bash(gofmt:*),Bash(make:*),
                 Bash(git diff:*),Bash(git status:*),Bash(git log:*),Bash(git show:*)
  --disallowedTools WebSearch,WebFetch
  --strict-mcp-config --mcp-config {"mcpServers":{}}
  --setting-sources user --settings {"disableAllHooks":true}
  --append-system-prompt <capsule context>
  [--resume <uuid>]
  --permission-mode dontAsk
```

The prompt goes on stdin. Never `--bare` (it ignores the OAuth token).
`--permission-prompts` is not used: 2.1.197 does not have it. The run's
cwd is `/workspace/platform`, `HOME` is a fresh per-run directory, and
the environment is built from an allowlist: `ANTHROPIC_API_KEY`,
`ANTHROPIC_AUTH_TOKEN`, `ANTHROPIC_BASE_URL` and the cloud-provider
switches are never present (an API key silently switches `-p` to API
billing). Added: `CLAUDE_CODE_SUBPROCESS_ENV_SCRUB=1`,
`CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1`, `DISABLE_AUTOUPDATER=1`,
`DISABLE_UPDATES=1`, the proxy variables and the capsule toolchain.

Files under `/tmp/choir-delegate/<id>/` (outside the frozen tree):
`spec.json`, `prompt.txt`, `events.jsonl` (stream-json, every line
redacted), `stderr.log` (redacted, capped), `status.json` (state,
progress, and at the end the result: subtype, `is_error`, turns, session
id, `total_cost_usd`, `usage`, `modelUsage`, `permission_denials`, final
text truncated, events sha256 and size).

## 5. Failure modes, written before the code

Each names where it is pinned. "Unit" tests run on any OS; "Linux" ones
need the guest kernel; "staging" ones are the probe.

| # | Failure | Guard | Pinned by |
|---|---|---|---|
| F1 | Token file absent on Node B | vmctl writes a grant with status `unavailable`, reason "token file absent", no token; the guest reports it; engineering stays on L2 | unit (vmmanager, capsule, agentcore) |
| F2 | Computer not allowlisted | vmctl writes nothing; guest reports "not allowlisted"; mode stays L2 | unit |
| F3 | Window expired (host, or a snapshot resumed with an old disk) | vmctl refuses; guest re-checks the grant's expiry at every spawn | unit |
| F4 | Token in a desk cell's environment or a `choir.Exec` child | broker unsets it on read; session worker and direct exec envs are built from allowlists that never carry it | unit (env builders), staging (probe status) |
| F5 | Token in logs, records, traces, egress ledger, delegate files | vmctl and guest log status only; the supervisor redacts every events and stderr line; status/result built from redacted lines; `choir-delegate` prints only those files | unit (fake `claude` that prints its env and the token) |
| F6 | Token in a frozen candidate | Freeze scans every changed file of a `claude`-mode capsule and refuses on a match | unit (scanner), Linux path |
| F7 | `ANTHROPIC_API_KEY` (or base URL, auth token, Bedrock/Vertex switches) present | environment built from an allowlist; those keys removed even when the base carries them | unit |
| F8 | `--bare` or WebSearch/WebFetch | args pinned: no `--bare`; `--tools` excludes them and `--disallowedTools` names them | unit |
| F9 | Run outlives its window | wall clock: SIGTERM to the process group, SIGKILL after 10 s, state `timeout`, result recorded | unit (fake `claude` that sleeps) |
| F10 | Egress to a host other than the model API | `ClaudeEgressPolicy` allows `api.anthropic.com` only plus the Go proxy route; refusals recorded | unit (proxy harness) |
| F11 | A flag the pinned binary does not know | the arg list is pinned to flags found in 2.1.197; `claude` exits at once and the run reads `failed` with the stderr tail | unit + strings check (§6) |
| F12 | Two runs at once, or a spec that escapes `/workspace/platform` | one run per capsule; spec validated | unit |
| F13 | A cell reads the token from `claude`'s `/proc/<pid>/environ` | `claude` runs in a Landlock domain beside the cells', not under it; Landlock refuses ptrace-class access across that line | UNVERIFIED on the staging kernel (§6) |
| F14 | A cell plants hooks or settings for `claude` | fresh per-run `HOME`, `--setting-sources user`, `--settings {"disableAllHooks":true}`, no project settings in the repo | unit (args) |
| F15 | Claude edits outside its brief or claims tests pass | `paths_confined` and engineering's own test receipts (R7); Claude's word is not evidence | existing rules |
| F16 | Crash restart mid-run | the capsule dies with the assignment; nothing resumes at boot | existing restart rule |
| F17 | Memory or pids exhaustion | 3 GiB / 512 pids when the budget allows; cgroup counters | unit (size choice) |
| F18 | The delegate section of the prompt shows where the delegate is absent | overlay section is conditional on the run's capsule mode | unit (runtimeprompts) |

What the barrier is and is not: Choir's own surfaces never carry the
token (F4, F5, F6, F7). Code that runs inside `claude`'s own process
tree (its Bash children) can still read the parent's environment through
`/proc`; `CLAUDE_CODE_SUBPROCESS_ENV_SCRUB` only clears the children's
own environment. That residual is the ratified I19 exception: one
computer, one week, owner-authored objectives, model-API-only egress,
revocation as the hard stop.

## 6. UNVERIFIED

- Claude Code (Bun) runs under the capsule's seccomp allowlist and
  Landlock (it may want a syscall the filter refuses).
- Landlock's ptrace scoping blocks a sibling cell from reading
  `claude`'s `/proc/<pid>/environ` on the staging kernel (F13).
- A token-holding headless run needs no host besides `api.anthropic.com`
  at startup (`platform.claude.com` is in the binary; the egress ledger
  will show any refusal).
- `claude-opus-5-5` is accepted as `--model` by 2.1.197 with OAuth.
- `total_cost_usd` is reported for OAuth sessions.
- The upload volume of a 40-turn session fits 512 MiB.
- Claude Code's resident memory beside `go test` in 3 GiB.
- systemd orders the token's read-only bind before the secrets
  directory's `InaccessiblePaths` mask (if not, the bind is skipped and
  the grant reads "token file absent": safe, but the delegate is off).
- nixpkgs patches the binary's ELF interpreter (`autoPatchelfHook`);
  whether that counts as "modified" under the Commercial Terms
  condition is a policy question for the owner, not a code one.

## 7. Red-class ceremony

**Conjecture delta.** Before: engineering writes code itself on the
cheap pool. Conjecture: an engineering desk that delegates the writing
to Claude Opus 5.5 inside its own capsule, and keeps verification
(paths, re-run tests, Freeze, management Verify, owner approval), lands
Choir changes the cheap pool cannot. Slice 1 tests that on one
computer; it does not change the long-term direction (Choir's own
harness on the API).

**Protected surfaces touched.** Credential custody (vmctl reads a host
secret; the per-VM credential disk carries a new file); capsule
isolation and egress policy (a new network mode, a credential in a
capsule process); the capsule broker (a new socket and a hardening
stage); the guest image (unfree `claude-code` 2.1.197, about 245 MB
uncompressed, not on PATH); the engineering prompt (a conditional
section and a carve-out to "never start another agent"); Freeze (a
refusal scan in `claude` mode).

**Admissible evidence.** Unit tests for the material, policy, args,
environment, redaction, supervisor and prompt (any OS). Linux-only
paths (broker stage, Freeze scan wiring) by cross-compile here and by
the staging probe. Deployed: `scripts/claude_delegate_probe.py` on the
allowlisted computer, which refuses to run unless the token exists.
Local proof is not claimed for vmctl, the credential disk, the capsule
or the model call.

**Rollback.** 1) Empty `CHOIR_CLAUDE_DELEGATE_COMPUTERS` (or let the
expiry pass) and deploy: no new credential disk carries the token, and
the guest refuses a lapsed grant. 2) Stop the test computer so its next
boot is cold. 3) Delete the token file on Node B and revoke the token in
the owner's Anthropic account (the hard stop for a token already in a
running guest). 4) Revert the commits. Frozen candidates stay inert
until the owner approves them.

To delete the slice: `internal/claudedelegate/`, `cmd/choir-delegate/`,
`cmd/capsule-broker/delegate*.go`, `internal/capsule/claude_delegate*.go`,
`internal/vmmanager/claude_delegate*.go`, the `claude` network mode
(`types`, `agentcore/engineering_assignment_runtime.go`), the overlay
section, the nix lines marked "claude delegate", the probe, this note.

**Heresy delta.**
- Discovered: the 16 MiB L2 upload budget is too small for any LLM
  client; a guest-root process (the owner's terminal, Zot) can read every
  file on the credential disk, the gateway token included; vmctl's
  secret hiding makes any vmctl-read host secret need an explicit bind.
- Introduced, ratified by the owner on 2026-10-14 for Option T: a
  credential inside a capsule process (an `I19` exception for one
  ComputerID and a bounded window); the delegate's own tool calls are not
  broker receipts (partly offset by the transcript digest, the capsule
  diff, engineering's re-run receipts and the egress ledger); the
  engineering overlay's "never start another agent" gains a carve-out
  for this delegate only.
- Repaired: none.
