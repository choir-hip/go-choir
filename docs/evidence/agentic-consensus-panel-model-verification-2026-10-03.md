# Agentic-consensus panel model verification — 2026-10-03

Mutation class: **yellow** (review-panel configuration and skill tooling;
changes future review pressure, no product runtime behavior).
Protected surfaces touched: none.
Problem record: [`docs/problems/agentic-consensus-panel-anchor-rot-2026-10-03.md`](../problems/agentic-consensus-panel-anchor-rot-2026-10-03.md).
Rollback: `git revert` of the panel commit.

## Why

Owner direction 2026-10-03: refresh the consensus panel onto current models
(add `opencode-go/space-bunny-free`, replace the 5.6-generation GPT pins with
GPT-6/6.1, remove the `cursor` 4.6 anchor), and verify the whole line rather
than assuming it works.

## Method

`omp --model` fuzzy-matches, so a retired id can silently resolve to an older
generation and still exit 0. Every probe therefore ran in `--mode json` and
graded on `message_end.message.provider` / `.model` — the model that actually
served the turn — not on whether text came back. New durable script:

```text
skills/agentic-consensus/agentic-consensus-model-probe.sh
```

Verdicts: `OK` (served == requested, output contract met), `MISROUTE` (exit 0,
wrong model served), `WRONG-ANSWER`, `DEAD` (non-zero exit), `TIMEOUT`.

## Before — the panel's pinned models, probed 2026-10-03

| runner id | pinned model | verdict | served / error |
| --- | --- | --- | --- |
| `omp-gpt6-sol` | `openai-codex/gpt-6-sol` | OK | `openai-codex/gpt-6-sol` |
| `omp-gpt6-luna` | `openai-codex/gpt-6-luna` | OK | `openai-codex/gpt-6-luna` |
| `omp-gemini38` | `google-antigravity/gemini-3.8-flash` | OK | `google-antigravity/gemini-3.8-flash` |
| `omp-glm53-flash` | `opencode-go/glm-5.3-flash` | OK | `opencode-go/glm-5.3-flash` |
| `omp-gpt6-terra` | `openai-codex/gpt-6-terra` | **MISROUTE** | **`openai-codex/gpt-5.6-terra`** |
| `omp-cursor-grok46` | `cursor/cursor-grok-4.6-high` | **DEAD** | 429 named models unavailable |
| `omp-muse-spark` | `opencode-zen/muse-spark-1.3-contributor-free` | **DEAD** | 403 free tier |
| `omp-nemotron-3-ultra` | `opencode-zen/nemotron-3-ultra-free` | **DEAD** | 403 free tier |
| `omp-ling` | `opencode-zen/ling-3.0-flash-fin-free` | **DEAD** | 403 free tier |

Four of nine pinned OMP models were dead and one had silently downgraded a
generation. External CLI panelists: `codex`, `devin`, `claude` OK; `opencode`
broken by flag rot (`opencode run --dir` no longer exists in v2.0.14, exit 1
printing usage); `cursor` blocked on account quota.

## After — the full verified anchor set, `probe.sh --all`

All 14 anchors `OK`, each twice (two independent sweeps), each with the serving
model equal to the requested model:

```text
VERDICT        SECS  REQUESTED                                      THINK   SERVED
OK                5  openai-codex/gpt-6.1-sol                       xhigh   openai-codex/gpt-6.1-sol
OK                4  openai-codex/gpt-6-luna                        max     openai-codex/gpt-6-luna
OK                6  google-antigravity/gemini-3.8-flash            high    google-antigravity/gemini-3.8-flash
OK                5  opencode-go/space-bunny-free                   high    opencode-go/space-bunny-free
OK                5  opencode-go/muse-spark-1.3-contributor         high    opencode-go/muse-spark-1.3-contributor
OK                5  opencode-go/qwen3.8-max                        high    opencode-go/qwen3.8-max
OK                2  opencode-go/glm-5.3-flash                      high    opencode-go/glm-5.3-flash
OK                5  openai-codex/gpt-6-sol                         medium  openai-codex/gpt-6-sol
OK                6  openai-codex/gpt-6-astra                       high    openai-codex/gpt-6-astra
OK                4  google-antigravity/claude-opus-4-6             high    google-antigravity/claude-opus-4-6
OK                4  opencode-go/grok-4.7                           high    opencode-go/grok-4.7
OK                4  opencode-go/deepseek-v4-pro                    high    opencode-go/deepseek-v4-pro
OK                3  opencode-go/kimi-k3                            high    opencode-go/kimi-k3
OK                3  opencode-go/minimax-m3                         high    opencode-go/minimax-m3
```

CLI versions probed: `omp 18.4.9`, `codex-cli 0.159.0` (default
`gpt-6-astra`), `claude 2.1.284`, `opencode 2.0.14`, `agent 2026.09.28`.

## Default panel change

Was 13 members with 6 non-functional. Now 11 members, all verified:

```text
codex, devin, claude, opencode,
omp-gpt61-sol, omp-gpt6-luna, omp-gemini38,
omp-space-bunny, omp-muse-spark, omp-qwen38max, omp-glm53-flash
```

Owner-requested changes: `opencode-go/space-bunny-free` added as
`omp-space-bunny`; the 5.6-generation GPT pin replaced by
`openai-codex/gpt-6.1-sol` as `omp-gpt61-sol`; the `cursor` 4.6 anchor
removed, with `omp-grok47` (`opencode-go/grok-4.7`) available for an xAI
voice because the `cursor/*` provider cannot name models on a free plan.

Retired: `omp-gpt6-terra`, `omp-cursor-grok46`, `omp-nemotron-3-ultra`,
`omp-ling`. `omp-gpt6-sol` is retained as a supported id (in-flight `/goal`
files pass it in `--include`) but is no longer the default flagship.

`cursor` is supported but excluded from the default panel while the account is
out of Agent usage; `--include ...,cursor` restores it.

## Guards added

1. `agentic-consensus-model-probe.sh` — identity-graded, repeatable anchor
   verification.
2. Runner startup self-check: every supported `omp-*` id must map to a
   non-empty `_MODEL` and `_THINKING` pin, so a renamed anchor fails loudly
   instead of fuzzy-matching onto the nearest model.
3. The 26 hand-written `--omp-*` parse cases collapsed to one table-driven
   arm, and 14 hand-written `build_cmd` OMP arms to one mechanical
   id→pin mapping — removing the repetition that let the flag surface and the
   panel drift apart.
4. `~/.agents/skills/agentic-consensus` was a stale **copy** of the repo
   skill, last synced 2026-09-23, still pinning `hy3-free` and `gpt-6-terra`.
   Replaced with a symlink to the repo (matching the existing `definition`
   skill) so the two can no longer diverge.

## E2E acceptance

Full default panel run, real prompt, exit 0, 11/11 `ok`:

| agent | status | secs |
| --- | --- | --- |
| claude | ok | 12 |
| omp-space-bunny | ok | 14 |
| devin | ok | 15 |
| omp-muse-spark | ok | 17 |
| codex | ok | 20 |
| omp-gpt6-luna | ok | 21 |
| opencode | ok | 22 |
| omp-glm53-flash | ok | 25 |
| omp-qwen38max | ok | 30 |
| omp-gpt61-sol | ok | 40 |
| omp-gemini38 | ok | 50 |

All 11 outputs non-empty, no rate-limit/free-tier/quota error text. `opencode`
returned `Ling (InclusionAI) Ling 3.1 Flash` — that is the CLI's own configured
default, intentionally not overridden.

Reproduce:

```bash
skills/agentic-consensus/agentic-consensus-model-probe.sh --all
skills/agentic-consensus/agentic-consensus-runner.sh --dry-run --prompt probe
```

## Residuals

- Model ids keep retiring. This makes rot detectable on demand; it does not
  make it self-healing. A scheduled re-probe is a deliberate deferral.
- `opencode-zen/*-free` is unusable from OMP (403). `opencode-zen/space-bunny-free`
  and `opencode-zen/grok-4.7` did answer, so the refusal is not a blanket
  provider ban; it is undocumented and inconsistent. Prefer `opencode-go`.
- `cursor` needs purchased Agent usage before it can rejoin the default panel.

## Follow-up — owner trim, same day

Owner direction later on 2026-10-03: remove `omp-grok47`, `omp-gpt6-astra` and
`omp-gpt6-sol`; `gpt-6.1-sol` is the wanted OpenAI flagship. These three pins
were healthy when measured above — this is a panel trim, not a rot repair, and
the tables above are left as measured rather than rewritten.

Removed: `omp-grok47` (`opencode-go/grok-4.7`), `omp-gpt6-astra`
(`openai-codex/gpt-6-astra`), `omp-gpt6-sol` (`openai-codex/gpt-6-sol`).
Supported ids 16 → 14 (default panel unchanged at 11).

`omp-gpt6-sol` had been retained specifically because 14 in-flight `/goal`
files name it. Checked before removing: all 14 occurrences are inside `review:`
frontmatter recording panels that **already ran** on 2026-09-30 / 2026-10-01,
with `evidence_ref` manifests. None is a live invocation — no doc in the repo
passes `--include` to the runner. They are receipts of what actually served,
so they are left untouched; rewriting them would falsify the record.

Re-verified after the trim: 11/11 anchors OK with serving model == requested,
and a full default-panel run 11/11 `ok`, exit 0.

Also found and fixed while checking the above: **`--include` failed open.**
`--include omp-gpt6-sol,omp-gpt6-luna` ran 2 agents and reported
`Failed/skipped: 0` — a retired id silently shrank the panel. An unsupported
id in `--include` is now a hard error (exit 2) listing the supported ids;
`--exclude` stays forgiving. This is root cause 4 in the problem record.
