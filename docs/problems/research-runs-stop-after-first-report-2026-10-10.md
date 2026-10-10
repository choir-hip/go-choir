# Research runs stop within a minute, after their first report (2026-10-10)

Status: documented; prompt fix prepared, not yet landed. Mutation class of
the prompt fix: `yellow`. The fan-out half is a separate mission (see the
last section).

## Evidence

All eight research runs in the saved demo traces (arxiv, ms3, arxiv4,
arxiv5; session scratchpad traces/, events read at limit=1000):

| run | span (UTC) | cells | WebSearch | Fetch/Import | report cells |
| --- | --- | --- | --- | --- | --- |
| 5a8925a3 | 17:54:03-17:55:06 | 18 | 4 | 6 | 4 |
| 3afd32ce | 17:56:01-17:56:38 | 14 | 0 | 0 | 2 |
| 26769ec0 | 19:08:18-19:09:11 | 8 | 3 | 2 | 3 |
| 02b8a5db | 19:17:18-19:18:04 | 7 | 2 | 1 | 1 |
| 0eaa4215 | 20:38:47-20:39:24 | 6 | 3 | 0 | 2 |
| ee53b793 | 20:38:47-20:39:31 | 6 | 2 | 2 | 2 |
| a496efc7 | 21:13:50-21:14:43 | 6 | 2 | 2 | 3 |
| 64f09172 | 21:17:01-21:17:35 | 6 | 2 | 1 | 2 |

Every run lasted 35-63 seconds and made at most four web searches. The
run is not ended by the runtime. In 64f09172 the last provider call stops
with `end_turn` at 21:17:35, four seconds after a report whose own text
says what is still open: "public code URLs, smallest CPU-only
recipe/runtime, stronger candidates, and independent replication evidence
remain unverified". The desk runs `gpt-5.6-luna` at reasoning effort
`low` (`[roles.research]` in internal/modelpolicy/model_policy.go).

For comparison, five Claude Haiku researchers given one slice each of
"the news on and around AI for the last week" made about 50-60 tool calls
each (about 37-38 searches for the two that counted) in 3-4 minutes.

## Cause (from the prompt and the trace)

The research prompt (internal/promptstore/defaults/research.yaml) asks for
saturation ("Saturation, not one pass") and for checkpointing as material
lands. It never says what comes after a checkpoint, so the model treats
its first report as the end of the job. Nothing tells it to break a broad
question into subtopics, and nothing sets a scale for "comprehensive". Its
own 2-5 s cell timeouts also killed half its searches (R2 in
desk-cell-ergonomics-from-demo-traces-2026-10-10.md), which made the few
probes it did run thinner still.

## Belief state

Before: research saturates within one run, as its prompt says. After: research
stops after one report, and Texture publishes from 2-4 searches. Remaining
error field: whether a prompt alone moves a low-effort model to sustained
depth, or whether depth also needs the model/effort or the fan-out below.
The research-only demo ("Research the news on and around AI for the last
week."), compared against the five-Haiku baseline, measures it.

## Fix direction

1. Prompt (yellow): deep by default; list the subtopics first; several
   searches and primary-source reads per subtopic; report after each
   subtopic; a report is a checkpoint and the run continues; end only at
   saturation or a budget refusal, saying what remains.
2. Owner direction 2026-10-10: research recursively fans out to multiple
   models (RLM pattern). The settled design is the 2026-08-29 RLM
   consensus D7b and M6.5 (docs/reviews/rlm-implementation-consensus-2026-08-29.md):
   a nested activation of research-or-smaller authority, with a
   carve-out of the parent's budget and caps on width and depth. The
   2026-09-15 ontology consensus adds that its results return as values
   inside the parent's cell, not as mailbox deliveries. Not built yet;
   it gets its own goal file.
