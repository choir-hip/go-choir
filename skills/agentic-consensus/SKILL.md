---
name: agentic-consensus
description: Run a prompt across a default panel of agent CLIs and OMP models using the bundled runner script, then synthesize consensus, dissent, and recommendations for planning, code review, architecture, debugging, or product decisions.
---

# Agentic Consensus

Use this skill when the user wants multiple independent model/agent opinions: "consensus", "ask several agents", "run this by Devin/Claude/Codex/Cursor/opencode", "model panel", "multi-agent review", "planning review", or "code review across models".

The skill bundles a script:

```text
skill://agentic-consensus/agentic-consensus-runner.sh
```

Use the script instead of hand-assembling commands unless the user explicitly requests a one-off command. The script handles panel selection, model overrides, parallel execution, output capture, and a manifest.

## Default Panel

The default panel is:

1. `codex` CLI with its configured default model.
2. Devin CLI/API with its configured default model/agent.
3. Cursor `agent` CLI with its configured default model.
4. `opencode` CLI with its configured default model.
5. OMP `openai-codex/gpt-5.5` with `--thinking high`.
6. OMP `google-antigravity/gemini-3.5-flash` with `--thinking high`.
7. OMP `zai/glm-5.2` with `--thinking high`.

`claude` is supported but intentionally excluded from the default panel because its token rate limits are lower. Add it explicitly with `--include claude,...` when needed.

External CLIs intentionally use their default model unless the user asks for a model override. OMP entries are pinned because they are the stable built-in comparison anchors.

Supported runner ids:

```text
codex
devin
claude
cursor
opencode
omp-gpt55
omp-gemini35
omp-glm52
```

## Verified CLI Invocation Contracts

These flags were checked from local CLI help.

### Codex CLI

Non-interactive command:

```bash
codex exec [OPTIONS] [PROMPT]
```

Runner contract:

```bash
codex exec --cd "$CWD" --sandbox read-only -c 'approval_policy="never"' --ephemeral --skip-git-repo-check "$PROMPT"
```

Optional model override:

```bash
-m MODEL
```

Notes:

- `codex exec` reads from stdin if prompt is omitted or `-` is used, but the runner passes the prompt as an argument.
- `--sandbox read-only` and `-c 'approval_policy="never"'` keep consensus runs non-interactive and review-oriented.
- `--ephemeral` avoids session persistence.
- If the configured default model is unavailable, pass `--codex-model` to override it.

### Devin CLI

Non-interactive command:

```bash
devin -p [PROMPT]
devin --print [PROMPT]
```

Runner contract:

```bash
devin --permission-mode auto -p "$PROMPT"
```

Optional model override:

```bash
--model MODEL
```

Notes:

- `--permission-mode auto` auto-approves read-only tools.
- Help says non-interactive mode disables workspace-trust prompting by default.

### Claude CLI

Non-interactive command:

```bash
claude -p [OPTIONS] [PROMPT]
claude --print [OPTIONS] [PROMPT]
```

Runner contract:

```bash
claude -p --output-format text --permission-mode plan --no-session-persistence "$PROMPT"
```

Optional model override:

```bash
--model MODEL
```

Notes:

- `--permission-mode plan` makes the run read-only/planning-oriented.
- `--no-session-persistence` avoids saving sessions.
- Use `--output-format json` only when downstream parsing needs Claude's JSON wrapper; the runner defaults to text for uniform raw outputs.

### Cursor Agent CLI

Non-interactive command:

```bash
agent --print [OPTIONS] [prompt...]
```

Runner contract:

```bash
agent --print --output-format text --mode ask --trust --force --approve-mcps --workspace "$CWD" "$PROMPT"
```

Optional model override:

```bash
--model MODEL
```

Notes:

- `--mode ask` is read-only Q&A style.
- `--trust` suppresses headless workspace trust prompts.
- `--force` automatically approves all commands/permissions.
- `--yolo` is an alias for `--force`; passing both is unnecessary.
- `--approve-mcps` automatically approves all MCP servers.
- The runner redirects stdin from `/dev/null` so the agent never sees a TTY;
  without this, Cursor detects an interactive terminal and prompts for command
  approvals despite `--force`.
- `--workspace` points Cursor at the review/planning root.

### opencode CLI

Non-interactive command:

```bash
opencode run [message..]
```

Runner contract:

```bash
opencode run --dir "$CWD" "$PROMPT"
```

Optional model override:

```bash
-m MODEL
```

Notes:

- `opencode run` also supports `--format json`, `--agent`, `--variant`, and `--auto`.
- The runner does not pass `--auto` by default; consensus should gather opinions, not mutate the workspace.

### OMP CLI

Non-interactive command:

```bash
omp -p --model MODEL --thinking LEVEL --no-session "PROMPT"
```

Runner contracts:

```bash
omp -p --mode text --model openai-codex/gpt-5.5 --thinking high --no-session "$PROMPT"
omp -p --mode text --model google-antigravity/gemini-3.5-flash --thinking high --no-session "$PROMPT"
omp -p --mode text --model zai/glm-5.2 --thinking high --no-session "$PROMPT"
```

The runner also passes `--auto-approve` and `--max-time` to OMP so a tool call
cannot block on an invisible approval prompt or run without a deadline.

Optional overrides:

```bash
--omp-gpt55-model MODEL
--omp-gpt55-thinking LEVEL
--omp-gemini-model MODEL
--omp-gemini-thinking LEVEL
--omp-glm52-model MODEL
--omp-glm52-thinking LEVEL
--no-tools-omp
```

Notes:

- Do not use `--no-tools` for OMP if the OMP agent needs to see skills; OMP only lists skills when the `read` tool is available.
- Use `--no-tools-omp` for pure opinion prompts where tool use would be wasteful.

## Runner Usage

Basic default panel:

```bash
skill://agentic-consensus/agentic-consensus-runner.sh \
  --prompt "Review this plan for correctness and hidden risks."
```

Long prompt from file:

```bash
skill://agentic-consensus/agentic-consensus-runner.sh \
  --prompt-file /tmp/consensus-prompt.md \
  --cwd /path/to/repo
```

Run a subset:

```bash
skill://agentic-consensus/agentic-consensus-runner.sh \
  --include codex,claude,opencode,omp-gpt55 \
  --prompt-file /tmp/consensus-prompt.md
```

Exclude unavailable or unwanted agents:

```bash
skill://agentic-consensus/agentic-consensus-runner.sh \
  --exclude devin,cursor \
  --prompt-file /tmp/consensus-prompt.md
```

Override selected models:

```bash
skill://agentic-consensus/agentic-consensus-runner.sh \
  --claude-model opus \
  --opencode-model anthropic/claude-sonnet-4-5 \
  --cursor-model gpt-5 \
  --prompt-file /tmp/consensus-prompt.md
```

Dry-run exact commands:

```bash
skill://agentic-consensus/agentic-consensus-runner.sh \
  --dry-run \
  --prompt "whats the 42nd prime"
```

List supported runner ids:

```bash
skill://agentic-consensus/agentic-consensus-runner.sh --list-agents
```

## Runner Output

The script writes:

```text
<out-dir>/prompt.md      exact prompt sent to agents
<out-dir>/manifest.tsv   agent, status, exit code, duration, output path, command
<out-dir>/<agent>.out    combined stdout/stderr for each run
<out-dir>/<agent>.cmd    shell-quoted command for reproducibility
```

Default output directory:

```text
/tmp/agentic-consensus-YYYYmmdd-HHMMSS
```

Use `--out-dir DIR` to pin the location.

`/tmp` is session diagnostics, not resumable evidence. When resumption or audit
requires durability, archive the prompt, manifest, candidate identity,
adjudicated findings, and reviewer-health telemetry under a durable referenced
identity. Raw transcripts need not become their own Git commit.

Every agent has a 180-second hard deadline by default. Override it with
`--timeout-seconds N`.
The runner requires GNU `timeout` (provided by `coreutils`), verifies that
implementation before launch, and fails rather than running an unbounded panel.

Manifest statuses:

```text
ok                     agent completed with exit 0
failed                 agent command exited non-zero
timed-out              GNU timeout ended the agent at its deadline
skipped-missing-cli    required CLI binary was not found
dry-run                command was rendered but not executed
```

Default exit behavior:

- exits `1` if any selected agent fails or is missing,
- exits `0` only if every selected agent succeeds.

Use `--keep-going` when a partial panel is acceptable:

```bash
skill://agentic-consensus/agentic-consensus-runner.sh \
  --keep-going \
  --prompt-file /tmp/consensus-prompt.md
```

## Prompt Construction

Every consensus prompt should include:

```text
You are one member of an independent agentic consensus panel.
Do not assume other agents agree with you.
Return concise, decision-useful output.

Task:
<user task>

Context:
<repo paths, diff, plan, requirements, constraints, or "none">

Output format:
1. Verdict / recommendation
2. Top findings or proposed plan
3. Risks / edge cases
4. Evidence or assumptions
5. Confidence: high / medium / low
```

For code review, use:

```text
Review this diff/code for correctness, security, maintainability, performance, and test gaps.
Prioritize concrete blocking issues over style.
For each issue include file/path, exact failure mode, severity, and suggested fix.
If you find no blocking issue, say so explicitly and name the main residual risk.
```

For planning, use:

```text
Review this plan for architecture, sequencing, hidden dependencies, scope risk, test strategy, and user impact.
Identify missing decisions and propose the smallest robust execution plan.
Separate must-fix blockers from optional improvements.
```

For adversarial challenge, use:

```text
Try to break this proposal. Find false assumptions, edge cases, race conditions, security/privacy failures, operational risks, and ways the implementation could satisfy tests while failing users.
Return only actionable risks and fixes.
```

## Workflow

When reviewing work governed by a Definition, consensus is an assurance
operation, not a separate Git beat. If the assurance profile requires a panel
for a canonical Define or Implement boundary:

1. Prepare the decision-complete diff and its available evidence.
2. Bind review to a frozen identity containing base revision, complete included
   and excluded path scope, content digest, and evidence refs. Use a
   content-addressed patch/bundle, read-only snapshot, or isolated candidate
   commit; a candidate commit is review substrate, not canonical mission state.
3. Freeze scoped mutation, then run the risk-tiered panel. Map panel breadth to
   the project's existing mutation classes and ceremony; never substitute the
   panel for required evidence, rollback, protected-surface, or authority work.
4. Read `manifest.tsv` and successful outputs, synthesize rather than
   concatenate, and locally verify high-impact claims.
5. Adjudicate findings into the candidate. A confirmed new behavior problem
   requires a code-free problem/Define boundary before repair when the project
   uses problem-documentation-first.
6. Compare the accepted candidate identity before the canonical commit.
   Material semantic change makes the review stale; rerun proportionately.
   Record why any deterministic formatting or generated delta is content-neutral.
7. Durably bind the accepted candidate identity, consensus/evidence refs,
   adjudication, and any no-rerun rationale in the reviewed artifact or commit
   metadata. Do not create standalone consensus-intent, output, or adjudication
   commits.

After commit, CI, deployment, production, migration, restart, credential,
provider, VM, promotion, rollback, and other external observations may be newly
available. Run post-boundary consensus only when interpreting such evidence or
a surprise can change the graph, evidence class, authority, mutation boundary,
escalation, route, or stopping condition; otherwise fold receipts into the next
natural Define boundary.

For any other consensus task:

1. Build a prompt file when the prompt is long, quote-heavy, or includes diffs.
2. Run the bundled script with the chosen panel.
3. Read the manifest and successful outputs; treat intentional partial-panel
   failures as metadata.
4. Synthesize and locally verify high-impact claims.

## Synthesis Template

Return this structure unless the user asks for another format:

```text
# Agentic Consensus

## Panel
- <agent/model>: <ran/skipped/failed> <reason if skipped/failed>

## Consensus
- <finding agreed by 2+ agents>

## Dissent / Disagreements
- <where agents disagree and why it matters>

## Unique High-Value Findings
- <single-agent finding worth keeping>

## Low-Confidence / Unverified Claims
- <claims not locally verified or based on assumptions>

## Recommendation
- <specific action, with rationale>

## Raw Outputs
- <output directory and important files>
```

## Review Rules

- Agent outputs are leads, not proof.
- Do not let majority vote override a demonstrated local fact.
- One severe minority finding beats five low-value majority nits.
- If an agent cites a file, diff, command, or API behavior, inspect or test it locally before reporting it as confirmed.
- Mark unverified claims as unverified.
- Never paste secrets, private tokens, `.env` content, or unrelated proprietary context into remote agents.

## When to Shrink the Panel

Use a smaller panel when:

- the task is trivial,
- the prompt contains large proprietary context,
- the user wants fast turnaround,
- credentials are missing,
- the decision is low-risk.

Use the full default panel when:

- the decision is architectural or irreversible,
- the code touches auth, payments, secrets, migrations, deployments, concurrency, or data loss,
- the user asks for review before landing,
- the team needs dissent or confidence calibration.
