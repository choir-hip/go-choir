# Claude Code as the Engineering Desk's Coding Delegate


**Owner ruling (2026-10-14, late):** T now, own harness later.

- **Short term: T.** The owner's own subscription token runs inside one
  test capsule as an explicit, owner-only exception to the rule that
  credentials never enter a capsule. It is limited to one computer and a
  bounded time, and it is revoked when the experiment ends.
- **Long term: the API through Choir's own harness, not `claude -p` or
  the Agent SDK.** Once Choir pays API prices, Anthropic models are called
  through the gateway like any other provider, from Choir's own
  multi-agent harness. The harness uses far less memory, and it already
  has precommitment records, multi-agent messaging and RLM desks built
  in. Slice 1 is therefore an experiment, not a long-term architecture.
- **The installation path in step 3 below needs `sudo`.** The secrets
  directory is root-only, so use
  `pbpaste | ssh node-b 'sudo sh -c "umask 077; cat > /var/lib/go-choir/secrets/claude-oauth-token; chmod 0400 /var/lib/go-choir/secrets/claude-oauth-token"'`.
Date: 2026-10-14. Author: Claude (research subagent of the lead), for the
owner. Status: research and design only. No code, secrets or deploys.
Mutation class of this document: **green**. Every build step it proposes
is **red** (capsule egress policy, credential custody, provider calls, the
guest image).

The question: can Choir's engineering desk hand the actual code-writing to
Claude, run through the Claude Agent SDK or headless `claude -p` inside an
engineering capsule and paid for by the owner's Claude Max subscription?
The owner's goal for 2026-10-15 is to develop Choir through Choir's own
self-development. A Claude Code session on the owner's Mac drives the
`choir` CLI or MCP. Management and engineering run inside Choir on cheaper
models. Engineering hands the code-writing to Claude Opus 5.5.

Facts about Anthropic's products were checked on 2026-10-14 against
code.claude.com, support.claude.com and anthropic.com/legal. Pages were
read through a fetch tool that summarizes, so the quotes outside §2.2
should be re-read on the live page before anyone relies on them. The
§2.2 quotes are verbatim. Facts about the local CLI come from
`claude --help` on Claude Code 2.1.299. Facts about Choir come from the
source at this commit. Anything not confirmed on a page or in code is
marked UNVERIFIED.

## 0. The answer first

**Policy.** For the owner's own development, Anthropic's terms allow
the unmodified Claude Code binary, signed in with the owner's own
subscription, to run on a platform the owner hosts. They do not allow
Choir to "collect, store, or intermediate" the owner's subscription
token, and they point developers who build on the Agent SDK to API keys.
So the owner's Max subscription can power **headless `claude -p`
holding its own token**. It should not power the Agent SDK as a library,
and it should not go through a Choir proxy that injects the token into
requests. Other Choir users may not use the owner's subscription at all.
Each must authenticate with their own Anthropic API key, their own
subscription signed in through Anthropic's flow, or a cloud-provider
credential. Usage is billed to them. If Choir preinstalls Claude Code
for other people, Anthropic's Commercial Terms apply.

**The tension.** Choir's doctrine says the opposite about custody.
Credentials stay on the host and never reach a capsule (`I19`: "Spawned
processes never inherit Choir capabilities, credentials, ..."). The
gateway already applies that rule to `codex-auth.json`. Anthropic's terms
allow exactly one shape for subscription auth, and in that shape the
token sits inside the capsule's Claude Code process. The host-held,
proxy-injected shape that Choir's doctrine prefers is the one the terms
forbid for subscription tokens. It is fine with an API key. The owner
has to pick between these. §3.3 sets out the three options and
recommends one.

**The minimal slice.** There are two steps.

- **Slice 0 needs no deploy and can run tomorrow.** The owner's Mac
  session writes the code with its own Max login, the plain carve-out.
  It pushes a branch to the public repo. Through `choir self-dev start`
  it asks Choir to adopt that exact commit. Engineering fetches the
  commit in its capsule and checks it. It records rule precommitments,
  applies the change, runs the tests, freezes and reports. Management
  verifies, and the owner approves from the CLI. This covers every part
  of the loop except Claude running inside the capsule.
- **Slice 1 is the shape the owner asked for: `claude -p` in the
  capsule.** It needs a red deploy. The parts are:
  1. a new capsule network mode `claude` (CONNECT to `api.anthropic.com`
     only, plus the host Go module proxy route, with a larger upload
     budget);
  2. a Claude Code binary in the guest image;
  3. a just-in-time path that takes the owner's token from a root-only
     host file into that one capsule's environment, for one ComputerID
     only;
  4. a small trusted wrapper, `choir-delegate`, that starts
     `claude -p` detached with fixed flags and lets a cell poll it.

  Engineering sends an objective, the allowed paths, a test command and
  limits. It gets back the Claude result record (turns, session id,
  notional cost, token usage, permission denials, error subtype), the
  capsule diff and its own re-run test receipts. The existing rule
  precommitments score the result: `tests_pass`, `paths_confined`,
  `freeze_within`, `verify_pass` and `completes`.

A blocker that is not about Claude: the engineering capsule still cannot
reliably build `bin/autoputer`, which Freeze requires. It has no C
toolchain on its PATH, and 1 CPU with 1 GiB is too small.
[selfdev-drill-capsule-cannot-build-autoputer-2026-10-14.md](../archive/problems/selfdev-drill-capsule-cannot-build-autoputer-2026-10-14.md)
records this, and the lead chose fixes A then B there. Until those land,
both slices end at "tests pass, release build blocked" for Go changes.

## 1. Agent SDK versus headless `claude -p`

Both run the same engine. The TypeScript SDK "bundles a native Claude
Code binary for your platform as an optional dependency"
([TypeScript reference](https://code.claude.com/docs/en/agent-sdk/typescript)).
The quickstart says both SDKs do
([quickstart](https://code.claude.com/docs/en/agent-sdk/quickstart)).
The SDK is a typed driver for the CLI process. It adds in-process hooks
and tool-permission callbacks (`can_use_tool`), Python in-process MCP
servers (`create_sdk_mcp_server()`), and typed message objects. The
Python SDK needs Python 3.10 or later and the TypeScript SDK Node 18 or
later ([hosting](https://code.claude.com/docs/en/agent-sdk/hosting)).
Versions seen today: `claude-agent-sdk` 0.2.167 on PyPI and
`@anthropic-ai/claude-agent-sdk` 0.3.300 on npm.

Choir is Go, and there is no Go SDK. A yaegi cell can only reach either
form through `choir.Exec`. An SDK host would be one more runtime in the
capsule, a Python or Node driver, spawning the same binary. The policy
in §2 also names the SDK as a developer tool that should use API keys.
**So the design uses the CLI.** The SDK becomes worth it only if Choir
wants in-process permission callbacks that a hook script cannot give.

The headless surface, from the
[CLI reference](https://code.claude.com/docs/en/cli-reference),
[headless docs](https://code.claude.com/docs/en/headless) and local
`claude --help` (2.1.299):

- **Streaming output.** `--output-format stream-json` (with `--verbose`)
  writes one JSON event per line. `--include-partial-messages` adds
  token deltas. `--input-format stream-json` accepts streamed user
  turns. The last line is the result message. Its fields are `subtype`,
  `is_error`, `num_turns`, `session_id`, `total_cost_usd` and `usage`.
  TypeScript adds `modelUsage`, `permission_denials` and `stop_reason`.
  The `subtype` values are `success`, `error_max_turns`,
  `error_during_execution`, `error_max_budget_usd` and
  `error_max_structured_output_retries`
  ([Python reference](https://code.claude.com/docs/en/agent-sdk/python)).
  `usage` "covers the main agent loop only and excludes subagent and
  other nested or auxiliary model calls", so `modelUsage` is the
  accounting field to use.
- **Sessions.** `--resume <id>`, `--continue`, `--fork-session`,
  `--session-id <uuid>`, and `--no-session-persistence`. Sessions are
  stored under `~/.claude/projects/` in plaintext and kept for 30 days by
  default ([data usage](https://code.claude.com/docs/en/data-usage)).
  In a capsule that directory is in the throwaway overlay.
- **Permissions.** `--permission-mode` takes `default`/`manual`,
  `acceptEdits`, `plan`, `auto`, `dontAsk` and `bypassPermissions`.
  `--dangerously-skip-permissions` is the same as `bypassPermissions`.
  `dontAsk` refuses anything not pre-approved.
  `--permission-prompts none` refuses any prompt in an unattended run.
  `--allowedTools`, `--disallowedTools` and `--tools` (with `""` for
  none) shape the tool set, and `--permission-prompt-tool` sends prompts
  to an MCP tool.
- **Hooks.** These include `PreToolUse` (allow, deny, ask or defer),
  `PostToolUse`, `Stop`, `SubagentStart` and `SubagentStop`. In the CLI
  they come from settings files. In the SDK they are callbacks, and the
  default hook timeout is 600 s
  ([hooks](https://code.claude.com/docs/en/agent-sdk/hooks)).
- **MCP.** `--mcp-config` takes files or JSON. `--strict-mcp-config`
  ignores every other MCP source.
- **Subagents.** `--agents` takes JSON, or with `-p` a file path. Their
  model calls are missing from `usage`, as noted above.
- **Limits.** `--max-turns` "exits with an error when the limit is
  reached". `--max-budget-usd` works in print mode only. It checks a
  "client-side cost estimate" that "can differ from your bill", and
  "spend can pass the cap". Under a subscription that dollar figure is
  notional (UNVERIFIED that it is reported at all for OAuth sessions;
  the field is documented as an estimate).
- **Working directory and sandboxing.** There is no top-level `--cwd`.
  The process's own working directory is the project, and `--add-dir`
  widens file access. Claude Code's own sandbox (bubblewrap and socat
  on Linux) "covers shell commands only. Claude's file tools, MCP
  servers, and hooks run outside it"
  ([sandboxing](https://code.claude.com/docs/en/sandboxing)). Choir
  should not count on nesting that sandbox inside a capsule, because
  user namespaces inside a seccomp and Landlock capsule are UNVERIFIED.
  The capsule is the sandbox. Anthropic's own
  [secure-deployment guide](https://code.claude.com/docs/en/agent-sdk/secure-deployment)
  recommends this shape: a VM such as Firecracker, no external NIC, and
  a host proxy.
- **`--bare` cannot be used here.** Local help and the
  [authentication page](https://code.claude.com/docs/en/authentication)
  agree that bare mode "never reads OAuth credentials" and "does not
  read `CLAUDE_CODE_OAUTH_TOKEN`". Its auth is "strictly
  ANTHROPIC_API_KEY or apiKeyHelper".

## 2. Authentication and Anthropic's policy

### 2.1 Credentials and precedence

The [authentication page](https://code.claude.com/docs/en/authentication)
gives this order:

1. cloud-provider variables;
2. `ANTHROPIC_AUTH_TOKEN`, sent as a Bearer token;
3. `ANTHROPIC_API_KEY`, sent as `X-Api-Key`, and "in non-interactive
   mode (`-p`), the key is always used when present";
4. `apiKeyHelper`;
5. `CLAUDE_CODE_OAUTH_TOKEN`;
6. federation credentials;
7. the saved `/login` session.

So a stray `ANTHROPIC_API_KEY` in the capsule environment would silently
switch the run to API billing. The wrapper must clear it.

`claude setup-token` "generate[s] a one-year OAuth token". It needs a
Pro, Max, Team or Enterprise plan, and the token "can only make model
requests": no Remote Control and no claude.ai connectors, though local
MCP servers still work. It is supplied as `CLAUDE_CODE_OAUTH_TOKEN`,
which the [env-vars page](https://code.claude.com/docs/en/env-vars)
calls an "alternative to `/login` for SDK and automated environments".
Claude Code warns within three days of expiry. After expiry it fails
with "Login expired". The fix is to generate a new token and restart.
No refresh flow runs on a headless host. Contrast the ChatGPT auth file,
which Choir's gateway refreshes itself (§4).

### 2.2 What Anthropic allows (verbatim)

From [Legal and compliance](https://code.claude.com/docs/en/legal-and-compliance),
section "Authentication and credential use":

> OAuth authentication is intended exclusively for purchasers of Claude
> Free, Pro, Max, Team, and Enterprise subscription plans and is designed
> to support ordinary use of Claude Code and other native Anthropic
> applications.

> Developers building products or services that interact with Claude's
> capabilities, including those using the Agent SDK, should use API key
> authentication through Claude Console or a supported cloud provider.
> Anthropic does not permit third-party developers to offer Claude.ai
> login into their own applications, or to route requests through Free,
> Pro, or Max plan credentials on behalf of their users. Moreover,
> developers may not collect, store, or intermediate Claude.ai
> credentials or session tokens — sign-in to a Claude account must
> complete through Anthropic's own flow.

> [...] Nor does it prevent an end user from signing in to the unmodified
> Claude Code binary with their own Claude subscription, including where
> a platform hosts Claude Code as described under *Can customers offer
> Claude Code in their products?* above.

The same page, "Can customers offer Claude Code in their products?":
"preinstalling or running Claude Code in your products or services (e.g.
in hosted sandboxes or other agent infrastructure) requires agreeing to
our Commercial Terms of Service" and two conditions. First, "The Claude
Code binary must not be modified". Second, "Customers may not pay for,
resell, or intermediate Claude usage on their end users' behalf. Each end
user must authenticate with their own Anthropic API key, Claude
subscription plan credentials, or 3P inference provider credential".
And under "Acceptable use": "Advertised usage limits for Pro and Max
plans assume ordinary, individual usage of Claude Code and the Agent
SDK." The page ends: "Anthropic reserves the right to take measures to
enforce these restrictions and may do so without prior notice."

The Agent SDK [overview](https://code.claude.com/docs/en/agent-sdk/overview)
says the same: "Unless previously approved, Anthropic does not allow
third party developers to offer claude.ai login or rate limits for their
products, including agents built on the Claude Agent SDK."

There are two more sources, both read through the summarizer, so
re-check their wording. The [Consumer Terms](https://www.anthropic.com/legal/consumer-terms),
which govern Pro and Max, forbid sharing account credentials or "mak[ing]
your Account available to anyone else". They also forbid automated
access "except when you are accessing our Services via an Anthropic API
Key or where we otherwise explicitly permit it". Claude Code's headless
mode and `setup-token` are such explicit permission.

The [Usage Policy](https://www.anthropic.com/legal/aup) page shows a
version marked "Effective November 12, 2026". It forbids reselling or
proxying access to Claude "through unauthorized means, including
services that route requests through consumer subscriptions". Whether
that version is already in force is UNVERIFIED. It takes effect in about
four weeks either way.

### 2.3 The policy answer, plainly

**(a) The owner's own development is allowed in one shape.** That shape
is the unmodified `claude` binary, signed in with the owner's own
`setup-token` (Anthropic's own sign-in flow), running headless on
infrastructure the owner controls, doing the owner's work. The carve-out
names this case, "including where a platform hosts Claude Code". Three
things are not allowed or are doubtful:

- **Not allowed:** a Choir service, such as the gateway or a new relay,
  that holds the token and adds it to requests on the binary's behalf.
  That is to "store, or intermediate Claude.ai credentials or session
  tokens".
- **Doubtful:** the Agent SDK with the subscription token. The SDK is
  framed as the developer path that "should use API key authentication".
  The "ordinary, individual usage of ... the Agent SDK" line suggests
  personal SDK use is foreseen, but the CLI is the clearer case.
- **Unclear:** whether round-the-clock or high-volume automated use
  counts as "ordinary, individual usage". It is UNVERIFIED and not
  defined anywhere. Keep delegation owner-initiated and capped (§5.4).

**(b) Other Choir users may not use the owner's subscription in any
form.** Each brings their own credential, and usage is billed to them.
For Choir that means a per-user Anthropic API key or a cloud-provider
credential. Choir may hold and inject an API key: the same page says the
rule "does not restrict how customers provision and manage their own API
keys ... in a ... secrets manager". A user's own subscription could only
work if that user signs in through Anthropic's own flow into an
unmodified binary on their own computer. Choir must not collect the
token for them. Shipping Claude Code preinstalled in Choir's shared
guest image for other users is "preinstalling or running Claude Code in
your products". That needs the Commercial Terms and the
no-modification condition. The repo is Apache-2.0 and the GitHub repo is
public, which changes none of this.

If the owner wants the proxy shape with a subscription, the page's own
advice is to [contact sales](https://www.anthropic.com/contact-sales).

### 2.4 Lifetime, refresh and rate limits

The `setup-token` token lasts one year and is not refreshed (§2.1).
Revocation and token exchange go through `platform.claude.com`, per the
[network-config page](https://code.claude.com/docs/en/network-config).
The exact steps to revoke a `setup-token` token from the account UI are
UNVERIFIED. The owner should confirm them before installing one (§5.7).

Rate limits: "Both Pro and Max plans have a five-hour session limit and a
weekly limit", and "usage in Claude and Claude Code counts against the
same limits" ([support](https://support.claude.com/en/articles/11145838-use-claude-code-with-your-pro-or-max-plan)).
The Max 5x/20x multipliers, per-window message counts, weekly reset
timing and any Opus-specific sublimit are UNVERIFIED, because the
support article on Max usage returned 404 today.

The practical consequence: **Choir's delegated runs and the owner's own
lead Claude Code session draw on one quota.** An eager engineering
delegation can rate-limit the owner's interactive work.

## 3. Running it in a Firecracker guest capsule

### 3.1 Runtime and binary

The native CLI needs no Node. The npm package "installs the same native
binary ... The installed `claude` binary does not itself invoke Node"
and needs Node 22 or later only to install
([setup](https://code.claude.com/docs/en/setup)). The local macOS arm64
binary of 2.1.299 is 237 MB, and the four kept versions are each
236-248 MB. The Linux x86_64 size is UNVERIFIED but likely similar.
The guest image already has `nodejs`, `go`, `git` and `curl` on the
system path (`nix/autoputer-vm.nix:867-891`). The capsule bind-mounts
`/nix/store` (`internal/capsule/executor.go:2008`), and its PATH starts
with `/run/current-system/sw/bin`, so any package in
`environment.systemPackages` can run in a capsule.

Two ways to get the binary there:

- **Put it in the guest image** as a fixed-output Nix fetch of a pinned
  release with its hash, or nixpkgs' unfree `claude-code`. The nixpkgs
  version at the pinned revision is UNVERIFIED. The policy reading in
  §2.3 says an image that every computer boots counts as
  "preinstalling", so gate it to the owner's computer: on PATH
  everywhere but runnable only in `claude` mode, or a separate store
  path that only that mode mounts.
- **`npm install` inside the capsule** from `registry.npmjs.org`, which
  the L2 allowlist already permits. That is about 240 MB per capsule
  per run. Whether the npm package then fetches the native build from
  `downloads.claude.ai`, which L2 refuses, is UNVERIFIED. If it does,
  this path fails.

Resources set the real limit. Anthropic's hosting guide suggests "1 GiB
RAM, 5 GiB disk, and 1 CPU per agent" for the agent alone. The
engineering capsule is exactly 1 GiB, 1 CPU and 256 pids, with no swap
(`internal/agentcore/engineering_assignment_runtime.go:27-29`), and Go
builds and tests share that budget. An interactive guest has 2 GiB and
2 CPUs in code (`interactiveVMMemSizeMib`) and 8 GiB and 4 CPUs on Node B
(`VM_INTERACTIVE_MEM_MIB=8192`, `nix/node-b.nix`; corrected 2026-10-22,
RES-21), and capsules may use up to three quarters of guest memory
(`internal/autoputer/capsule_executor_linux.go:44-60`;
`internal/vmctl/ownership.go:1685-1694`). A Claude-mode capsule needs
about 2-3 GiB and 2 CPUs. So a computer on the code default (2 GiB) would
need the 4 GiB size that platform computers get; Node B's interactive
computers already have 8 GiB. How to resize one computer is UNVERIFIED.

### 3.2 Network egress

From the [network-access list](https://code.claude.com/docs/en/network-config#network-access-requirements):

- `api.anthropic.com` serves the model API, WebFetch's domain check,
  feature flags and telemetry.
- `claude.ai`, `claude.com` and `platform.claude.com` handle sign-in and
  token exchange. A run that already holds a `setup-token` token should
  not need them, but that is UNVERIFIED and should be probed.
- `downloads.claude.ai` serves updates.
- Datadog intake hosts receive telemetry.
- `github.com`, `registry.npmjs.org` and `storage.googleapis.com` serve
  plugins and marketplaces.

`CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1` turns off auto-update,
telemetry, error reporting, feedback, release notes and feature-flag
fetches. `DISABLE_AUTOUPDATER=1` or `DISABLE_UPDATES=1` blocks updates.
Claude Code honours `HTTPS_PROXY`/`HTTP_PROXY`/`NO_PROXY` but not
SOCKS. A subscription login also turns on error reporting by default
(v2.1.198+), which the non-essential switch disables.

So a capsule running Claude needs one host, `api.anthropic.com:443`,
over CONNECT. Choir's L2 proxy already speaks CONNECT for allowlisted
hosts (`internal/capsule/egress.go:320-445`). Two limits do not fit:

- **The upload budget is 16 MiB per capsule** (`EcosystemEgressPolicy`,
  `egress.go:121-128`). Claude Code sends the whole conversation on
  every request. A 40-80 request session with 150-500 KB of context per
  request uploads tens of MB. This is an estimate and is UNVERIFIED.
  The `claude` mode needs its own budget, for example 512 MiB.
- **Server-side tools bypass the capsule.** WebSearch runs on
  Anthropic's servers, not through the capsule proxy. It must be
  disabled with `--disallowedTools`, or the capsule's network rule
  stops meaning anything. WebFetch would be refused by L2 anyway, but
  should be disabled too.

### 3.3 Where the credential lives: three options

The rule in Choir's own docs is `I19` (above). The design note
[engineering-network-grants-2026-10-10.md](../design/engineering-network-grants-2026-10-10.md)
§6 Q6 says credentials "stay mirror-side (proxy-side injection ...).
They never enter a capsule". The ChatGPT pattern (§4) is exactly that:
the token sits on the host, the host's gateway makes the calls, and
guests hold only a gateway bearer.

**Option R: a host relay injects the subscription token.** It would
mirror the Go module proxy. A route `claude.choir.internal` maps to a
host service. Inside the capsule, Claude Code runs with
`ANTHROPIC_BASE_URL=http://claude.choir.internal` and a placeholder
token. The relay replaces the bearer with the real token, keeps the
OAuth `anthropic-beta` value (the
[gateway protocol](https://code.claude.com/docs/en/llm-gateway-protocol)
says dropping it gives 401), and forwards to `api.anthropic.com`. The
token never enters the guest, and the relay can count every token
host-side. It is the best security design. **It is also the shape §2.2
forbids for a subscription token** ("store, or intermediate Claude.ai
credentials or session tokens"). Reject it for Max. The design is good
and should be kept for API keys (Option K).

**Option T: the binary holds the owner's token inside the capsule.**
This is the carve-out shape and the CI pattern
(`CLAUDE_CODE_OAUTH_TOKEN` in a job's environment). Its cost is a named
exception to `I19`: code running in the capsule can reach the token.
Mitigations reduce that risk without removing it:

- **`CLAUDE_CODE_SUBPROCESS_ENV_SCRUB=1`** removes credentials from the
  environment of Claude's Bash commands, hooks and stdio MCP servers
  ([env-vars](https://code.claude.com/docs/en/env-vars)). Whether a
  child can still read the parent's `/proc/<pid>/environ` under the
  capsule's seccomp, Landlock and user namespace is UNVERIFIED. Probe it.
- **Egress narrowed to `api.anthropic.com`** in this mode. GitHub, npm
  and PyPI are dropped. With GitHub open, an injected prompt that
  carries an attacker's GitHub token could push the secret out.
  `api.anthropic.com` itself stays an exfiltration path. An injected
  command could upload a file to the Files API using an attacker's API
  key, and the proxy cannot see paths inside TLS. Keep task text
  owner-authored to make that unlikely.
- **A host-side kill switch and a one-computer allowlist.** Only the
  owner's ComputerID ever receives the token.
- **A Freeze-time scan** refuses any bundle whose files contain the
  token's value. The repo is public, and a token committed into a
  candidate that later lands on GitHub would leak.
- **The token is inference-only.** `setup-token` gives no account
  administration, so a stolen token buys model calls on the owner's
  quota until revoked. The account-level risk is that Anthropic treats
  a leaked and shared token as account sharing.

**Option K: an API key through the relay.** This is Option R with a
Claude Console API key instead of the subscription token. It is clean on
both sides: Anthropic explicitly permits keys in a secrets manager, and
the custody matches Choir's doctrine. It costs API rates. It is the
right long-term shape and the only one open to other users (§2.3 b).
Each user's own key would sit host-side per computer.

**Recommendation.** Use Option T for a time-boxed owner experiment: one
computer, one week, then rotate the token. The owner must explicitly
accept it as an `I19` exception. Build the relay code once so Option K
reuses it, and move there when the experiment ends or when other users
need it. This is a credential question, which CLAUDE.md reserves for
the owner, so it is listed in §5.7 and not decided here.

### 3.4 How engineering capsules run today

Read from the code and the 2026-10-14 drills:

- **Admission.** Management admits engineering through a delegated
  `choir.Cast`. `planEngineeringCast`
  (`internal/agentcore/engineering_assignment_runtime.go:163-266`) binds
  one assignment, one capsule and a network mode taken from the Cast
  spec (`engineeringCapsuleNetwork`/`castWithholdsNetwork`, `:361-389`).
- **The capsule.** It has private namespaces, a cgroup v2 budget,
  seccomp and Landlock. Its overlayfs has a pinned, content-addressed
  source tree at `/workspace/platform` and the guest root as lower
  layers (`internal/capsule/executor.go:213-417`).
- **The tool.** `capsule_go_eval` is engineering's only JSON tool, and
  a stateful yaegi cell runs inside the capsule. `choir.Exec(cmd, args)`
  runs a direct-argv command, and each cell yields a host-signed
  receipt.
- **Time bounds.** `choir.Exec` defaults to 30 s (yaegi broker
  `DefaultTimeout`, `internal/yaegikernel/broker.go:63-65`; the sidecar
  sets none, `sidecar.go:92`). A cell is bounded at 60 s by the capsule
  broker (`cmd/capsule-broker/main.go:407`, `session_worker.go:430`).
  The 18:02 drill confirmed both. Long work has to be started in the
  background and polled, which is how that drill ran `go build`.
  **A multi-minute `claude -p` therefore cannot be one synchronous
  `choir.Exec`.**
- **Network.** In L2 mode the capsule's only interface is loopback. The
  broker forwards `127.0.0.1:3128` to the executor's egress socket, and
  the proxy enforces the `ecosystemHosts` allowlist plus "routes"
  (today only `goproxy.choir.internal`). Routes accept only plain-HTTP
  GET/HEAD (`egress.go:89-99`, `350-375`). The guest VM itself has NAT
  egress; only capsules are restricted.
- **Results.** Freeze extracts the overlay diff, requires the built
  release tree at `/var/lib/artifact/release/` (with `bin/autoputer`),
  and opens a self-development operation and bundle. Complete mints the
  final Report with capsule-diff evidence and receipts. Management runs
  `Verify`, and the owner approves with `choir self-dev approve`, which
  is CLI only since 5b17bb572.
- **Two prompt rules matter.** The engineering overlay ends "Never start
  another agent or create an alternate candidate world"
  (`internal/runtimeprompts/overlays/rlm_engineering_runtime.yaml:58`).
  Delegating to Claude Code starts another agent, so that sentence needs
  an owner-ratified carve-out: a delegate inside the same capsule
  creates no new candidate world. The overlay also says the cell must
  fail when its test command fails, because `tests_pass` reads cell exit
  codes (R7).

## 4. Choir's own patterns

**The gateway.** `go-choir-gateway` runs on Node B and loads provider
keys from `EnvironmentFile=/var/lib/go-choir/gateway-provider.env`
(`nix/node-b.nix:1200-1260`). Providers are built in
`provider.ResolveAll` (`internal/provider/provider.go:3152-3208`):
`chatgpt`, `bedrock`, `zai`, `fireworks`, `opencode-go` and
`opencode-zen`. Guests receive `choir.gateway_url=http://<tap>:8084` on
the kernel command line. The host DNATs it to loopback. The gateway
requires a bearer from the guest's root-only credential disk
(`/run/choir-bootstrap/gateway-token`) and a Choir guest source address
(`internal/gateway/handlers.go:650-673`, `760-767`). Every host unit
hides every secret path except the ones it names (`hostSecretPaths`,
`nix/node-b.nix:311-325`). `/var/lib/go-choir/secrets` is already on
that list, so a token file there is invisible to every unit unless one
opts in. `LoadCredential=` is read by systemd before the unit's mount
namespace exists, so the file stays hidden even from the unit that
consumes it.

**`codex-auth.json`.** The operator refreshes `~/.codex/auth.json`
locally. `nix/deploy-provider-creds.sh` copies it to
`/var/lib/go-choir/codex-auth.json` (mode 0600) and sets
`CHATGPT_AUTH_PATH`. `ChatGPTAuth` (`provider.go:2740-3029`) refreshes
the token itself 45 minutes before JWT expiry against
`auth.openai.com`. It writes the file back atomically and shares one
mutex per path, because refresh tokens rotate. Guests never see it.
Their child processes have credential-like variables removed
(`internal/autoputer/terminal.go:252-281`,
`cmd/capsule-broker/main.go:490-502`). **Do not copy this pattern to
Claude.** It is the gateway making provider calls with a subscription
token on the product's behalf, which §2.2 forbids for Anthropic
subscriptions. Whether OpenAI's terms allow it is a separate question
this report does not answer. Discovered on the way:
`go-choir-front-importance` also keeps `codex-auth.json` visible
(`nix/node-b.nix:864`). The survey agent read that unit's comment as
needing only the OpenRouter key. If so, that is an unneeded exposure.
UNVERIFIED intent; worth one line in a hardening pass.

**Routing and `prompt_cache_key`.** Per-computer model policy maps each
desk to `pool:default`, which hashes the thread onto one pool member.
The other members become failover (`internal/modelpolicy/model_policy.go:208-239`).
The pool is `opencode-go/deepseek-v4.1-flash`, `opencode-go/cat-jam-free`
and `chatgpt/gpt-6-luna` (`internal/modelcatalog/catalog.go:211-219`).
The cache key is `choir-<label>-<scopehash>-<prompt+tools hash>`
(`internal/toolregistry/toolloop.go:170-181`). It is sent as
`prompt_cache_key` to ChatGPT and as `x-opencode-session` to OpenCode
(`provider.go:978`, `1089-1150`), and gets a `-failover[N]` suffix on
failover. The gateway falls back to sending `claude*` models to Bedrock
(`handlers.go:508-552`). None of this changes for delegation. The
engineering desk's own turns stay on the cheap pool. Only the delegate
spends Claude Opus 5.5, and it does so outside the gateway.

**Shell-outs to agent CLIs.** No runtime code invokes `claude`, `codex`
or `opencode` to write code. The only Go call is `codex login status`
as a token-refresh fallback (`provider.go:2989`). The
`agentic-consensus` runner script calls all three, but it is a developer
skill. The guest terminal runs Zot, an external Go coding agent, with
the gateway token deliberately withheld (`terminal.go:108-320`). That
is the nearest precedent for "an external agent inside a Choir
computer". `git log --all` shows no earlier attempt at an external
coding-agent executor. The "claude" and "codex" hits are review panels
and agent-authored branches.

**Where an executor plugs in.** There is no executor interface or
registry. `Runtime.capsuleExecutor` is a concrete `*capsule.Executor`.
That is fine, because the delegate does not need a new executor kind.
It runs as one more command inside the existing capsule, and its results
flow through the existing diff, receipt, Freeze and Report path. The
seams it touches are:

- the Cast spec's network mode, which adds `claude`;
- `EgressPolicy`, which adds a policy for that mode;
- the capsule workload environment, built by the broker after the
  cell's own environment, the same way `EgressProxyEnv` is;
- the guest image;
- one trusted wrapper binary.

## 5. Design

### 5.1 Slice 0: the loop with Claude outside the capsule (no deploy)

This runs tomorrow on deployed code. It is what "Claude Code drives the
`choir` CLI" means at the lowest risk.

1. The owner's lead Claude Code session on the Mac writes the change in
   a worktree with the full dev shell (cgo, ICU). It runs the tests and
   pushes a branch to the public repo. That is Claude Max under the
   carve-out with no question attached.
2. The lead runs `choir self-dev start --prompt "..."`. The prompt names
   the branch, the exact commit SHA, the sha256 of
   `git diff <base>..<sha>`, the allowed paths and the test command.
   Carry identifiers, not the patch text. Texture and management
   paraphrase, and a patch passed through two model hops arrives
   damaged.
3. Engineering precommits in its first cell (§5.3). It then fetches the
   commit in the capsule (`github.com` is on the L2 allowlist),
   verifies the diff hash and applies it. It re-runs the tests in a
   cell that fails when they fail, builds the release (blocked until
   fixes A/B land), freezes and Reports.
4. Management verifies. The lead watches with `choir self-dev wait` and
   `choir lifecycle capsule-evidence`. The owner approves or rejects
   with `choir self-dev approve|reject`.

What it proves: the self-development loop accepts Claude-written code,
and engineering's precommitments are scored against it. What it does
not prove: engineering choosing what to delegate, or Claude acting
inside the capsule.

### 5.2 Slice 1: `claude -p` inside the engineering capsule

These are the smallest set of changes. Each is red and lands as one
deploy behind a host flag.

1. **A `claude` network mode.** The Cast spec gains
   `"network": "claude"`. The executor builds its policy:
   - hosts `api.anthropic.com` only, CONNECT to 443;
   - the existing Go module proxy route, so Go builds still resolve;
   - an upload budget of 512 MiB, download 1 GiB, connections 4096;
   - a capsule budget of 3 GiB and 2 CPUs.

   The mode is refused unless the host flag is on and the ComputerID is
   on the host allowlist.
2. **Token delivery (Option T).** The owner puts the token in
   `/var/lib/go-choir/secrets/claude-oauth-token` (root, 0400). The
   gateway gets it through `LoadCredential=`. A new runtime-peer-only
   gateway route hands it to the trusted guest runtime of an
   allowlisted ComputerID, and only when that runtime spawns a
   `claude`-mode capsule. The executor adds it to the workload
   environment as `CLAUDE_CODE_OAUTH_TOKEN`, along with
   `CLAUDE_CODE_SUBPROCESS_ENV_SCRUB=1`,
   `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1` and `DISABLE_UPDATES=1`.
   It removes `ANTHROPIC_API_KEY` and `ANTHROPIC_BASE_URL`. Execution
   receipts already store digests, not environments; a test must pin
   that no receipt, trace or Report contains the value. Freeze refuses
   a candidate containing the value. Removing the file or turning off
   the flag stops new delegations at once.
3. **The binary.** The guest image carries a pinned Claude Code
   (fixed-output fetch with hash), reachable only in `claude` mode.
4. **`choir-delegate`, a small trusted Go wrapper in the guest image.**
   It exists because a cell's exec is bounded at 30 s and because the
   model must not choose the flags. It has three verbs:
   - `start --spec <file>` reads a JSON spec, starts `claude` detached
     in its own process group under a wall-clock bound, and returns a
     run id at once;
   - `wait --id <id> --seconds <=25` returns the state (`running`,
     `done`, `timeout`, `killed`) and, once finished, the parsed result
     record;
   - `stop --id <id>` stops the run.

   The flags are fixed in the wrapper:

   ```text
   claude -p "<objective + constraints>" \
     --model claude-opus-5-5 --output-format stream-json --verbose \
     --max-turns <N> --max-budget-usd <B> \
     --permission-mode dontAsk --permission-prompts none \
     --allowedTools "Read,Edit,Write,Glob,Grep,Bash(go test:*),Bash(go build:*),Bash(go vet:*),Bash(gofmt:*),Bash(git diff:*),Bash(git status:*)" \
     --disallowedTools "WebSearch,WebFetch" \
     --strict-mcp-config --mcp-config '{"mcpServers":{}}' \
     --append-system-prompt "<capsule context>"
   ```

   The process runs with cwd `/workspace/platform`, and stdout goes to
   `/tmp/delegate/<id>/events.jsonl`. The appended prompt says: this is
   a disposable capsule; never commit, push or deploy; the only network
   is the model API; edit only under these paths; run this test
   command; stop when it passes. That last part is needed because the
   repo's CLAUDE.md, which Claude Code will read, tells agents to push
   and deploy. Whether repo settings or hooks under `.claude/` load in
   `-p` mode with no settings flag is moot today, since the repo has no
   `.claude/` directory. Pin `--setting-sources user` anyway.

   For the very first probe, before the wrapper exists, engineering can
   start the same command line through
   `choir.Exec("sh", ["-c", "... &"])`, as it did with `go build`. That
   puts the flags in model hands, so it is for a probe only.
5. **Prompt.** Amend the overlay's "Never start another agent" sentence
   (§3.4) to allow the delegate, and state the delegate verbs as data,
   not as a branch.

**What engineering sends.** The spec:

- `objective`: owner-authored text taken from the Cast, never research
  output;
- `paths`, the directories under `/workspace/platform` the delegate may
  edit;
- `test`, an argv;
- `max_turns` (default 40), `max_budget_usd` (default 20, notional) and
  `wall_clock_min` (default 30);
- optionally `resume_session` for a follow-up such as "the tests still
  fail; here is the output".

**What comes back.** From `wait`, capped at 2 MiB:

- the result record: subtype, `is_error`, `num_turns`, `session_id`,
  `total_cost_usd`, `usage`/`modelUsage` and `permission_denials`;
- the final message, truncated;
- the duration;
- the sha256 and byte count of the events file.

Engineering then gathers what Choir trusts. Claude's word is not
evidence:

- the capsule diff (`git diff --stat` in a cell, and Freeze's own
  extraction);
- its own test receipts from a cell that panics on failure;
- the egress records for `api.anthropic.com` (bytes up and down, as a
  coarse spend signal).

It Freezes and Reports to management. The Report carries the delegate
summary, usage, session id and transcript digest as evidence.

### 5.3 Scoring as precommitments

The rule precommitments that landed tonight (`internal/types/commitment_rules.go`,
`internal/agentcore/engineering_precommit_rules.go`) fit without
changes. In its first cell, before it starts the delegate (R2 refuses a
precommit in a cell that stages Freeze or Complete, and once the
candidate froze), engineering records:

- `paths_confined` with the same `paths` it gives the delegate. This is
  the direct check on Claude staying inside its brief, because the
  runtime settles it from the frozen diff's source paths (R8, R9).
- `tests_pass`. It settles from the receipts Freeze cites, which must
  be engineering's own re-run (R7). Claude's internal test runs are not
  receipts and do not count.
- `freeze_within` with, say, 60 minutes from the assignment opening
  (R1 stamps `Since`).
- `verify_pass` and `completes`.

Settlement is by `system:rule` from receipts, never from a model. The
Brier view `RulePrecommitSayDo` stays off engineering's context (R12).

Two gaps, deferred and not built now:

- **Attribution.** A precommit record names the model of the run that
  staged it (engineering's pool model), not the delegate. To compare
  "with delegate" against "without", the reducer could add
  `subject_refs["delegate"] = "claude-code/claude-opus-5-5"` when the
  assignment's capsule ran `choir-delegate`. The executor knows this
  from its own exec receipts.
- **Cost.** No rule kind settles cost. A `cost_within` kind would be a
  reviewed change to a closed vocabulary. Until then cost travels in the
  Report. If a host-side count is wanted under Option T, the only
  host-visible signal is egress bytes, because tokens are visible only
  to the binary.

### 5.4 Budget and timeout guards

The guards are layered, innermost first:

- **Claude.** `--max-turns` (hard), `--max-budget-usd` (a soft, notional
  estimate that can overshoot) and the wrapper's wall clock (SIGTERM,
  then SIGKILL to the process group after a grace period). One delegate
  runs at a time per capsule.
- **The capsule.** cgroup memory, CPU and pids are physical backstops.
  The egress byte and connection budgets of the `claude` mode act as a
  traffic ceiling. The capsule dies with the assignment.
- **The assignment.** There is the 6 h deadline
  (`engineeringAssignmentDeadline`) and the `freeze_within` precommit as
  the declared target.
- **The owner's quota.** It is shared with the owner's own sessions.
  Allow one `claude`-mode assignment per computer, which the sequential
  admission phase already enforces. Add a host-side daily cap on how
  many capsules the gateway hands the token to, for example 10. A 429
  from Anthropic ends the run as `error_during_execution`. Engineering
  then Reports a blocker rather than retrying in a loop.
- **The kill switch.** Turn the host flag off, remove the token file, or
  revoke the token.

### 5.5 Failure modes, written before any code

- **F1: token leak through code in the capsule** (a child reads the
  parent's environment, a prompt injection, a debugging `env` dump).
  Mitigations are the subprocess scrub, `api.anthropic.com`-only egress,
  owner-authored objectives and revocation. Residual: the Files API
  route needs an attacker's API key and a successful injection.
- **F2: token in a frozen candidate or a public commit.** The Freeze
  scan refuses it. The owner's approval is the second gate.
- **F3: token in receipts, traces or Reports.** A test pins that the
  value never appears in them. Cell sources never carry it, because the
  executor injects it.
- **F4: a stray `ANTHROPIC_API_KEY`.** It silently switches the run to
  API billing (`-p` always prefers it). The wrapper removes it.
- **F5: `--bare` used by mistake.** OAuth is then ignored and the run
  fails with an auth error. Do not use it.
- **F6: the run outlives the cell bounds.** The wrapper detaches and
  `wait` polls in at most 25 s slices.
- **F7: OOM or pids exhaustion.** Claude plus `go test` in 1 GiB is the
  likely first failure, hence 3 GiB and 2 CPUs for the mode. The cgroup
  OOM counters (`memory.events`) go in the Report.
- **F8: Claude edits outside its brief, or "fixes" tests by deleting
  them.** `paths_confined` and management's Verify catch the first.
  The second needs Verify to read the diff of test files. Name test
  deletion in the Verify prompt.
- **F9: Claude claims the tests pass.** Only engineering's own re-run
  receipts count (R7).
- **F10: network surprises.** A startup call to sign-in or feature-flag
  hosts that L2 refuses (UNVERIFIED) could hang or fail startup. Probe
  with the non-essential switch on and read the egress records for
  refusals.
- **F11: the server-side WebSearch bypass.** It is disabled by flag. Pin
  that in the wrapper's tests.
- **F12: Max limits reached.** The run fails, and the owner's own lead
  session may be rate-limited too. Mitigated by the cap and by keeping
  delegation owner-initiated.
- **F13: crash restart mid-run.** The capsule is gone, and the
  assignment closes as "interrupted by a restart", as the restart rule
  requires. The Claude session is never resumed at boot.
- **F14: token expiry after a year, or revocation.** The run fails
  with "Login expired". The gateway route reports the token's age, and
  the owner renews before the three-day warning.
- **F15: a Claude Code version change** alters flags or result fields.
  The binary is pinned, and an upgrade is a reviewed image change.
- **F16: policy change.** The November 12, 2026 Usage Policy names
  subscription routing. A single-owner Option T does not "provide
  access" to anyone, but re-read the terms at each renewal. Option K is
  the ready fallback.

### 5.6 Class, protected surfaces, rollback, heresy delta

- **Mutation class.** This document is green. Slice 0 adds nothing
  beyond a possible overlay wording change (yellow). Slice 1 is red.
  It touches capsule egress policy and capsule isolation (`I19`, O17),
  credential custody on the gateway, a provider call path, the guest
  image, and the Freeze bundle check.
- **Admissible evidence.** Pure unit tests for the egress policy, the
  environment builder and the wrapper's argument construction (any OS).
  Linux-gated executor tests for the scrub and `/proc` probe. A
  deployed drill on the owner's computer through product routes only,
  with an exported trace bundle.
- **Rollback.** Turn the host flag off. Remove
  `/var/lib/go-choir/secrets/claude-oauth-token`. Revoke the token.
  Revert the commits. The new mode is additive, and frozen candidates
  are inert until the owner approves them.
- **Heresy delta.**
  - Discovered:
    - the 30 s exec / 60 s cell bounds mean any long tool needs
      detach-and-poll (already recorded in the build-drill problem doc);
    - the 16 MiB capsule upload budget is too small for any LLM client;
    - `front-importance` can see `codex-auth.json`.
  - Introduced, with owner ratification:
    - a credential inside a capsule process, a named, time-boxed `I19`
      exception for one ComputerID;
    - the delegate's own tool calls are not broker receipts, a partial
      breach of "one actuator surface". The transcript digest, the
      capsule diff, engineering's re-run receipts and the egress records
      partly make up for it;
    - the overlay's "never start another agent" gains a carve-out.
  - Repaired: none.

### 5.7 What the owner must do

The owner does the following. No agent touches the token at any point.

1. **Decide custody.** Option T (owner-only experiment, `I19`
   exception, one computer, one week) or Option K (API key, clean,
   billed at API rates). Also ratify the overlay carve-out and a daily
   cap.
2. **Generate the token yourself**, in a plain terminal on your Mac, not
   inside a Claude Code or other agent session, whose transcripts would
   capture it: `claude setup-token`. It prints a one-year token once.
3. **Install it on Node B as a root-only file without echoing it.** For
   example, copy it to the clipboard and run
   `pbpaste | ssh node-b 'umask 077 && mkdir -p /var/lib/go-choir/secrets && cat > /var/lib/go-choir/secrets/claude-oauth-token && chmod 0400 /var/lib/go-choir/secrets/claude-oauth-token'`.
   Then clear the clipboard. Check it with
   `ssh node-b 'stat -c "%a %U %s" /var/lib/go-choir/secrets/claude-oauth-token'`,
   which shows mode, owner and size, never the content. Do not put it
   in `.env`, `gateway-provider.env`, the repo, a goal file or a chat.
4. **Find out how to revoke a `setup-token` token** in your account
   before installing it (UNVERIFIED in §2.4). Note the steps where you
   keep the token's creation date.
5. **Check your consumer data-use setting.** Pro and Max sessions may be
   used for training if you allowed it, and are kept 5 years in that
   case ([data usage](https://code.claude.com/docs/en/data-usage)).
6. **Size your computer** to 4 GiB or more for Slice 1, and expect that
   delegated runs share your Max limits with your own sessions.

## 6. UNVERIFIED, collected

- Linux binary size.
- Whether the npm install fetches from `downloads.claude.ai`.
- The nixpkgs `claude-code` version at the pinned revision.
- Whether a token-holding headless run contacts sign-in or feature-flag
  hosts at startup.
- Whether `total_cost_usd` is reported for OAuth sessions.
- Whether `/proc/<ppid>/environ` is readable from Claude's Bash child
  inside a capsule.
- Whether bubblewrap can nest in a capsule.
- The upload-volume estimate for a session.
- Claude Code's resident memory.
- How to resize one computer.
- How to revoke a `setup-token` token.
- Max multipliers, window counts and weekly reset.
- Whether the November 12, 2026 Usage Policy is in force.
- Whether autonomous delegation is "ordinary, individual usage".
- Whether `front-importance` needs `codex-auth.json`.
- The exact wording of Consumer Terms and Usage Policy quotes (read
  through a summarizer).

## 7. Sources

**Anthropic docs.**

- [Agent SDK overview](https://code.claude.com/docs/en/agent-sdk/overview)
- [quickstart](https://code.claude.com/docs/en/agent-sdk/quickstart)
- [TypeScript reference](https://code.claude.com/docs/en/agent-sdk/typescript)
- [Python reference](https://code.claude.com/docs/en/agent-sdk/python)
- [hooks](https://code.claude.com/docs/en/agent-sdk/hooks)
- [hosting](https://code.claude.com/docs/en/agent-sdk/hosting)
- [secure deployment](https://code.claude.com/docs/en/agent-sdk/secure-deployment)
- [CLI reference](https://code.claude.com/docs/en/cli-reference)
- [headless](https://code.claude.com/docs/en/headless)
- [authentication](https://code.claude.com/docs/en/authentication)
- [environment variables](https://code.claude.com/docs/en/env-vars)
- [LLM gateway](https://code.claude.com/docs/en/llm-gateway)
- [gateway protocol](https://code.claude.com/docs/en/llm-gateway-protocol)
- [network config](https://code.claude.com/docs/en/network-config)
- [sandboxing](https://code.claude.com/docs/en/sandboxing)
- [setup](https://code.claude.com/docs/en/setup)
- [data usage](https://code.claude.com/docs/en/data-usage)
- [legal and compliance](https://code.claude.com/docs/en/legal-and-compliance)
- [Claude apps gateway](https://code.claude.com/docs/en/claude-apps-gateway).
  The `claude gateway` subcommand is an enterprise IdP gateway in front
  of Bedrock, Vertex, Foundry or an API key. Subscription OAuth is not
  an upstream, so it does not help here.

**Support and legal.**

- [Use Claude Code with your Pro or Max plan](https://support.claude.com/en/articles/11145838-use-claude-code-with-your-pro-or-max-plan)
- [Consumer Terms](https://www.anthropic.com/legal/consumer-terms)
- [Commercial Terms](https://www.anthropic.com/legal/commercial-terms)
- [Usage Policy](https://www.anthropic.com/legal/aup)

**Choir.**

- [computer-ontology.md](../computer-ontology.md),
  [agent-product-doctrine.md](../agent-product-doctrine.md) and
  [choir-doctrine.md](../choir-doctrine.md) (`I18`, `I19`, `I24`)
- [engineering-network-grants-2026-10-10.md](../design/engineering-network-grants-2026-10-10.md)
- [capsule-go-module-proxy-design-2026-10-14.md](capsule-go-module-proxy-design-2026-10-14.md)
- [c1e-b1-engineering-assignments-cancelled-on-arrival-2026-10-14.md](../archive/problems/c1e-b1-engineering-assignments-cancelled-on-arrival-2026-10-14.md)
- [selfdev-drill-capsule-cannot-build-autoputer-2026-10-14.md](../archive/problems/selfdev-drill-capsule-cannot-build-autoputer-2026-10-14.md)
- the code paths cited inline
