# Agentic-consensus panel anchors rot silently; OMP fuzzy-matches model ids

**Status:** DIAGNOSED 2026-10-03 on the local dev host; fix follows in the
same change per `docs/memo-problem-documentation-first.md`.

## Symptom

The `agentic-consensus` default panel reported degraded results across
several stations without anyone naming a cause. Half of its OMP anchors were
dead and one was silently serving a *different model than the runner pinned*.

Measured 2026-10-03 with a per-model identity probe (`omp -p --mode json`,
which reports the provider/model that actually served the turn):

| runner id | pinned model | verdict | actually served |
| --- | --- | --- | --- |
| `omp-gpt6-sol` | `openai-codex/gpt-6-sol` | OK | `openai-codex/gpt-6-sol` |
| `omp-gpt6-luna` | `openai-codex/gpt-6-luna` | OK | `openai-codex/gpt-6-luna` |
| `omp-gemini38` | `google-antigravity/gemini-3.8-flash` | OK | `google-antigravity/gemini-3.8-flash` |
| `omp-glm53-flash` | `opencode-go/glm-5.3-flash` | OK | `opencode-go/glm-5.3-flash` |
| `omp-gpt6-terra` | `openai-codex/gpt-6-terra` | **MISROUTE** | **`openai-codex/gpt-5.6-terra`** |
| `omp-cursor-grok46` | `cursor/cursor-grok-4.6-high` | **DEAD** | — (429) |
| `omp-muse-spark` | `opencode-zen/muse-spark-1.3-contributor-free` | **DEAD** | — (403) |
| `omp-nemotron-3-ultra` | `opencode-zen/nemotron-3-ultra-free` | **DEAD** | — (403) |
| `omp-ling` | `opencode-zen/ling-3.0-flash-fin-free` | **DEAD** | — (403) |

Four of nine pinned OMP models were dead and one more had silently
downgraded a generation. Of the 13-member default panel, 7 were actually
answering.

Exact upstream errors:

```text
cursor/cursor-grok-4.6-high : Cursor RATE_LIMITED_CHANGEABLE: Named models
    unavailable: Free plans can only use Auto. (429, after 10 retries)
opencode-zen/*-free        : 403 OpenCode's free tier can only be used from
    within OpenCode (type=FreeTierError)
```

A separate flag rot hit the `opencode` panelist: `opencode run --dir` is no
longer a flag on `opencode v2.0.14`, so the member exited 1 printing usage
text on every run. `cursor` (`agent` CLI) additionally returns
`ActionRequiredError: You've hit your usage limit` on this account.

## Root cause

This is a substrate problem, not nine stale strings.

**1. Nothing binds a pinned model id to the model that actually served the
turn.** `omp --model` fuzzy-matches. `openai-codex/gpt-6-terra` is not in the
catalog any more — GPT-6 retired `terra` in favour of `astra` — but omp
resolved it to `gpt-5.6-terra` and exited 0. Text-mode probing cannot see
this: the runner recorded `status=ok`, `exit=0`, plausible prose. The
misroute is *more* dangerous than an outage, because it corrodes the panel's
independence claim while reporting success. `omp -p --mode json` emits
`message_end.message.provider` / `.model` for the serving turn, which is the
only identity signal available.

**2. Pin decay is invisible and unowned.** A pinned model id is a time-limited
claim about a third-party catalog. There was no scheduled or scripted check,
so retirement surfaced only as a consensus run quietly missing a voice —
attributed by readers to "the panel disagreed" rather than to "a member did
not exist".

**3. The panel mixes three independent failure domains with one retry
policy.** Credential/plan entitlements (`cursor` free plan cannot name a
model; `opencode-zen` free tier refuses non-OpenCode callers), catalog
retirement (terra, and every `opencode-zen/*-free` tier model), and CLI
flag drift (`opencode run --dir`) all surface as the same `status=failed`
row. They have different fixes and different recovery times.

## Fix shape (same change, after this record)

- Retire the dead anchors; repoint each slot at an identity-verified model
  (`opencode-go/*` routes serve the same families that `opencode-zen/*-free`
  no longer will).
- Add a served-identity contract: `agentic-consensus-model-probe.sh` runs each
  anchor in `--mode json` and reports `OK` only when the serving model equals
  the requested model, so a future retirement fails loudly as `MISROUTE`
  instead of quietly downgrading.
- Document each remaining anchor's failure domain (plan entitlement vs
  quota vs catalog) so a red row is diagnosable without re-probing.

## Residual

Model ids will keep retiring. This change makes the rot *detectable on
demand*; it does not make it self-healing. A scheduled re-probe is a
deliberately deferred residual, not an oversight.
