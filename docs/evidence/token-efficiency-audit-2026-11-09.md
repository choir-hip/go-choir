# Token efficiency audit (2026-11-09)

Owner, 2026-11-09: "is there a way that we could get the same or more paper
output using 50% or 10% or 2% of the tokens?"; "we should go really deep ...
every desk, pre-commitment records, all the communication, everything. Looking
at agent traces, retrospectively"; "It would be really cool if we could end up
getting it affordable enough that I can use Haiku at API rates"; "caching is
very important ... we should make sure that we are caching everything is
append only as much as possible".

This is the synthesis of eight read-only Sonnet investigations, one per
slice. The slice reports, with their commands and measured/inferred labels,
are in [token-efficiency-2026-11-09/](token-efficiency-2026-11-09/); the
shared brief is `brief.md` there. The first measurement is
[paper-token-budget-2026-11-09.md](paper-token-budget-2026-11-09.md), corrected
below. No paid calls were made and nothing was posted to any computer.

## Owner rulings on this audit (2026-11-09, afternoon)

The owner read the levers and ruled. Where a lever below conflicts, the ruling
wins; the affected levers are marked.

- **Never batch between Texture and other desks.** "We do not want to be
  batching the research to texture, interim wakes ... the interim messages are
  the whole system. That's the whole value prop. That's how you actually have
  relatively live updating supervision surface while you're working in the
  background."
- **Texture keeps its history.** "I don't want to be making texture stateless
  or forget its history. We just need to make sure that when another desk wakes
  texture, we're properly using the warm cache." Lever 5 is withdrawn. A wake
  appends to Texture's prompt context and reads the cache.
- **Never trim tool results.** "We absolutely should not be trimming tool
  results. That breaks caching." Caching is assumed on everywhere; crouton is
  free and does not matter. The "trim on routes that cannot cache" clause of
  lever 3 is withdrawn. Returning less in the first place is allowed; cutting
  it later is not.
- **Research: search, read and the interim send to Texture in one cell.**
  Lever 6 stands in that form: fewer model calls, the same interims, none
  delayed.
- **Progressive disclosure, cache-aware.** Prompts explain how the REPL works,
  and the model finds verbs from the REPL as it needs them (lever 8).
- **Review every system prompt** for simplification with behaviour held
  constant; management's is the first suspect.
- **Claude delegate: delegate more, not less.** It explores the repo before
  or instead of being briefed with pasted findings. Open investigation:
  Claude Code as a desk through an MCP server that sends and receives desk
  messages.
- **The corpus search 403 is a bug to root-cause now.**
- **Jev parallelism is tabled** until its own document is settled (levers 10
  and 11 wait on it).

Follow-up investigations for these rulings are in
[token-efficiency-2026-11-09/followup/](token-efficiency-2026-11-09/followup/)
as they land.

## The target

The test every lever is ranked against: **the paper runs on Haiku 5.5 or
River DeepSeek, at or under $20 a day, with the same or better output.**

Prices (USD per million tokens):

| route | input | cached input | output | caches? |
|---|---|---|---|---|
| Haiku 5.5, Bedrock | 0.10 | ~0.01 (estimate) | 0.50 | yes, explicit breakpoints; confirmed live (12,003-token prefix written, then read back whole) |
| DeepSeek V4.1 Flash 1017, River | 0.12 | 0.0024 | 0.48 | yes (River's published cached rate) |
| DeepSeek V4.1 Flash 1017, Synthetic | 0.60 | 0.03 | 1.20 | yes, 79–87% measured |
| crouton-alpha, OpenRouter | 0 (free until about 2026-11-10) | none | 0 | **no**: endpoint metadata `supports_implicit_caching:false`; 0 cached on every paper call |

Today the paper reads about 50M input tokens an hour, none of them cached
(crouton). On Haiku that is about $5 an hour, $126 a day. The target needs
about 6x less spend; caching does part of it and token cuts do the rest.

## Where the tokens go (corrected)

Paper, 2026-11-09, about 49.5M input tokens an hour:

| slice | share | what it is | slice report |
|---|---|---|---|
| management | ~51% | the wedge: frame over 4 MiB and the final-checkpoint loop, retried about 180 times since 2026-11-03 | management.md |
| research | ~30% | 18.5 calls a run, 413k tokens a run; 41% one identical 9.2k prefix, the rest append-only | research.md |
| texture | ~13% (about 25% after the wedge fix) | 46% carried conversation from earlier wakes; prompt only 4.4k tokens | texture.md |
| intake | ~10% | 5.2M/h: 2.1M Jev labels and events, 3.0–3.3M full-model intake tasks | intake-and-jev.md |
| output | ~1% of input | not a lever | |

Corrections to the first measurement: the system-prompt sizes map to roles as
research 37,943 chars (~8.5k tokens), texture 17,637 (~4.4k), conductor
49,908, intake 11,986; texture is not "78% fixed prompt"
(overhead-and-caching.md, texture.md). Fixed prompts are about 29% of paper
input, and the size is the `choir.*` verb surface written into each prompt,
not tool schemas (every request carries one tool, about 300 tokens).

Output does not track spend: the paper updates about 30 stories an hour
whether it spends 16M or 94M tokens an hour (retrospective.md). Tokens per
story-update ranged from about 5M (morning of 11-08) to about 1M now.

## Ranked levers

Savings are against today's 49.5M/h unless stated; "after the wedge" means
against the ~25M/h left once the management fix is live.

### Landing now (built, in review or deploying)

1. **Management wedge** — final-checkpoint delivery once per activation and a
   bounded re-injection (`7b4bbebd8`, Devin r2 ACCEPT, deploying), and a
   bounded, oldest-first desk-cell frame where nothing is consumed unless it
   was shown whole (three commits, Sonnet r2 ACCEPT, Devin r2 next). Saves
   **35–51%**, no quality risk.
2. **River priced at River's rate** (Devin reviewing): the River budget had
   been counting Synthetic's price, five times River's.

### Tier 1: caching and the shape of the transcript

3. **Run the paper on a caching route, append-only.** Every desk's prefix is
   already byte-stable (one system hash per desk), and research and intake
   transcripts are append-only, so a caching endpoint cuts fresh tokens
   70–93% with no change in raw tokens and no quality risk: research ideal
   hit rate 92%, intake 86%, texture 73% (overhead-and-caching.md,
   research.md). Crouton cannot cache, so this is a routing decision, and the
   natural moment is when crouton's free week ends. Rule from the measurements:
   **keep history append-only and never trim it** (trimming breaks the
   cached prefix and costs more). *Owner ruling: no trimming on any route.*
4. **Management retry budget.** A wake that commits no act after two
   activations closes with a visible fate instead of being retried. The wedge
   was retried 175–183 times; this caps any future wedge at about 0.2M tokens
   (management.md L1). An independent guard against repeated tool-less
   identical-input calls belongs beside it (retrospective.md).

### Tier 2: fewer calls per unit of work

5. **Withdrawn by owner ruling (Texture keeps its history; wakes append to
   the warm cache).** As first written: Texture as a stateless single-call
   wake. Fresh run per wake, source
   excerpts and the Cast shape in the first turn, the verb list trimmed: about
   8k tokens per wake against 45k, **−80% of Texture**, about 10% of the paper
   now and 20% after the wedge (texture.md, communication.md). Quality risk
   low to medium: the frame must carry the owner's directions and open casts
   that the carried conversation carries today. Diff-based revisions are not
   a lever (81% of each revision is rewritten; documents are small).
6. **Research: one search-wait-read-send cell** (owner ruling: the interim
   send to Texture is in the same cell and is never batched or delayed). Most of a run's 18.5 calls
   are 100-token micro-steps that each resend 10–28k tokens; batching to
   about 8 calls saves 55–65% of research, 17% of the paper now, 35% after the
   wedge (research.md L2). With it: return only the blocks that match the Cast
   from reads (−16% of research) and a deduplicated search digest (−9%); 41%
   of returned URLs were already delivered (retrospective.md).
7. **Preload intake and management context.** Intake runs spend 6.6 read cells
   before the first act; putting the capture, Jev's label and candidate
   stories in the first turn saves 55–75% of intake task tokens (3.5–4.7% of
   the paper). Management's opening `choir.Records()`, its Pack/Context reads
   and the call that only ends the turn are 35–40% of a healthy activation
   (management.md L3).
8. **A lazy verb surface for every desk.** A one-line index plus the existing
   `choir.Help` instead of the full verb list in the prompt: about 6% of the
   paper. Writing the verb signatures that models currently look up (12% of a
   healthy management activation) is the same lever from the other side.

### Tier 3: let code and Jev decide the routine

9. **Host rules for routine management dispositions** — clean task reports,
   residual captures, identical rebase shots, a Verify the platform always
   refuses — so activations become exceptions (management.md L7; one refusal
   loop on d@e cost 2.9M tokens, half that day's management spend).
10. **Intake through Jev.** Merge labels and the events layer into one batched
    call of up to 12 captures (2.07M → about 0.4M Jev tokens an hour), let the
    host feed a capture to the story Jev names as the same story (the model did
    so 24 of 24 times), and keep the model for "open". The events layer is a
    shadow nothing reads and does not predict publication (1 of about 70
    opened stories); delete it (intake-and-jev.md). Jev batch fan-out stays at
    4 in flight in its own rate scope, because one Jev failure holds every Jev
    caller.
11. **Never-publish work.** About 27% of recent research runs are on stories
    that do not reach the front, at almost the cost of published ones. A Jev
    publishability score to rank or skip is worth up to about 2% of the paper,
    after an AUC probe (intake-and-jev.md).

### Smaller and correctness items found on the way

12. **Failed cells in research**: 9.3% of cells error (39 of them
    `string slice index out of bounds`), and the call after each costs 7.7% of
    research input. A safe slice helper and documented result fields fix most
    of it.
13. **Cell-result envelope** is 15–22% of tool-output characters; shorten it
    and the "data, not instructions" marker (about 2.6%).
14. **Runs that end without a result** (quota, passivation): 8.4% of research.
    Hold a Cast until a provider is healthy.

## Self-development (d@e, b@c)

A b@c theme round is 100–143 DeepSeek calls, 10–15M input at 95% cache-read,
about $0.94 at list rates — one round per River $1 a day (selfdev.md). The
levers:

- **Delegate first.** 51% of a round goes before the Claude delegate is briefed
  (the engineer explores and pastes the finished theme into the brief) and 44%
  after (re-running the delegate's checks). A delegate-first round is about 14
  calls and $0.04–0.06, 16–23x cheaper.
- **A deterministic pack path**: a recipe plus a platform `CheckPack` verb, so
  contrast and key-set checks are code, not hand-written calculators each
  round: about 2–3 model calls a theme.
- **Synthetic sends no cache or session key** (the ChatGPT provider does); 23
  of 723 calls miss completely and re-bill 70–165k prefixes, 28% of a round's
  dollars. Hypothesis with a named probe.
- **A per-assignment budget**: the failed first OLED attempt on b@c ran 734
  calls and 116M input tokens.

## What this adds up to

Physics, after the wedge, with levers 3, 5, 6, 7 and 10 as first written
(lever 5 is now withdrawn; with Texture carried and cached instead, its 5.5M/h
stays raw but is mostly cache-read, which adds about $1 a day on Haiku):

- research about 14.6M/h → about 5.5M/h raw; texture 5.5 → about 1.1; intake
  model runs 3.2 → about 1; healthy management 1–2; so about 9M/h raw against
  50M/h today.
- On a caching route at the measured 75% hit rate, about 2.3M fresh and 6.7M
  cached tokens an hour.
- On Haiku 5.5: 2.3 × $0.10 + 6.7 × $0.01 + output about $0.05 ≈ $0.35 an
  hour, **about $8 a day**. On River about the same. Against $126 a day on
  Haiku today, about 15x.

So the target — Haiku at API rates, inside $20 a day, same output — is
reachable with tuning-sized changes plus caching. 50x is not a tuning result:
it is a pipeline redesign where the events layer and Jev select, and one model
call writes per published version (research.md, retrospective.md).

The measure the next mission should optimise is **fresh tokens (and dollars)
per published story-version**, held against story-versions per hour and a
quality read, not raw tokens: today's raw-token number moved 4x with flat
output.

## Defects and open questions found on the way

- **Published-story search fails**: 80% of the paper's `SearchWireCorpus`
  calls return `corpusd returned 403: internal caller required` for the
  published half, so research loses dedupe and related-story candidates.
  Cause is a hypothesis (the recent guest-graph auth change). Needs a problem
  doc and a fix.
- **The search cache served 5 of 15,192 searches.**
- **`/api/costs` misses most Jev spend**: 266 calls and 0.53M tokens where the
  gateway shows 640 calls and 3.55M for the same 90 minutes; labels and events
  calls have no run to attribute to.
- **A five-hour publishing outage, 2026-11-08 14:00–19:00 UTC**: no story
  updated while the paper spent 215M tokens (7.8% of 43 hours). Cause not yet
  read from the deploy or vmctl journals.
- **Texture reasoning effort** shows high in events while policy says low;
  check after the routing release.
- **15 Texture Write cells did not make a revision** (76 writes, 61 revisions).
- **Capture gap**: 188 new captures an hour, 96 leased to intake tasks.
- **Whether carried Texture runs write better revisions than fresh ones** —
  the quality question behind lever 5.
- **Whether extra research sources after the first packet matter** — the
  quality question behind a stop rule; 70% of research tokens come after the
  first packet.
