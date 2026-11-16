# A topic prompt gets no fast v1, and research delivers once at the end

Recorded 2026-11-16 15:20Z from the owner's report and a read-only trace of
the run on the owner's computer (prompt "History of SoftBank", 12:31:00Z).

## Problem

The documented behaviour
([ordinary prompt path, 2026-11-09](../definitions/choir-ordinary-prompt-path-2026-11-09.md))
is: v0 the raw prompt; v1 fast; research dispatched; an interim packet to
Texture within about 30 s of research's first citable read; revisions as
messages arrive. The owner's statement of it: v1 is a fast response from the
Texture model's weights; v2 follows research's first-round message; after
that, revisions follow messages early and often.

In this run v1 arrived 12 minutes after the prompt, already a full article
with 62 source refs. v2 added no evidence.

## Evidence (the run's own records)

| Time (Z) | Event |
|---|---|
| 12:31:00 | Prompt routed to Texture. v0 = raw prompt, metadata `asked: story` |
| 12:31:02 to 12:32:08 | Texture turn 1: one 57 s model call that cast research. Turn type `wait`. No revision |
| 12:32:18 on | Research searches (14 queries in two batches, about 1 s each; more at 12:35 and 12:36) |
| 12:33:32 to 12:34:30 | Research's first packet takes three attempts; two refused by the strict decoder (`unknown field "source_refs"`, then `"status"`) |
| 12:34:30 | First research message, 2 m 23 s after the cast |
| 12:34:31 to 12:36:15 | Texture reads captures |
| 12:36:15 to 12:43:06 | Two model calls, 140 s and 271 s; the second is one `Write` of the whole article (19.7k output tokens) |
| 12:43:06 | v1 committed |
| 12:43:13 | Research's second and last packet. Nothing was sent between 12:34:30 and here |
| 12:47:06 | v2 committed with `done: true`: a citation repair. One inbox record still unread; the trajectory settles |

Models: `devin/swe-2-flash` for both desks, about 70 output tokens a second,
unchanged since 2026-11-10. Jev calls in the run were trivial.

## Belief

1. **No fast v1** (about 85%; the model's reasoning is not in the trace). The
   prompt was classed as a story ask. That path replaces Texture's first
   objective with "open the researcher on the world's evidence, then write the
   document from what is found" (`internal/textureowner/ask_story.go`,
   `storyAskObjective`, applied in `texture_handoff.go`). Plain-route prompts
   on the same computer still got a v1 in 8 s and 20 s (2026-11-12,
   2026-11-10). The ask path landed after the 2026-11-09 proof: `68f863a5d`,
   `fb3d62925`, `e7b180b2d`. The proof never covered it.
2. **Even the plain route's v1 is not a from-weights answer.** Texture's V1
   rule says "fast acknowledgment… plan… No answer from model priors"
   (`internal/textureprompts/texture.yaml`). That contradicts the owner's
   statement of the policy.
3. **Research delivers late and rarely.** Its prompt asks for a packet after
   the first capture and after each round; nothing enforces it, and the strict
   packet decoder cost about a minute in refusals.
4. **Texture closes with mail unread.** A turn may commit `done: true` and the
   trajectory may settle while its inbox holds an unread record.
5. **One monolithic write.** 6.8 of the 12 minutes were Texture generating
   28.5k output tokens in two calls.

Refuted by the trace: the Jev one-entry-point change of 2026-11-15, the Jev
dial and intake fix, and the inline-body reader had no part in this run.

## Remaining error

- Why v2 was started is believed (support-judgement repair), not confirmed.
- Whether the unread record was the 12:43:13 packet is not confirmed.
- The 2026-11-09 proof ran on the plain route only; an acceptance run of a
  topic prompt against the documented sequence does not exist yet.
