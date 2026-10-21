# Failed tool calls on the paper, before the dual pool (2026-10-21)

Status: open. Problem documentation first; no fix in this commit.
Owner direction (owner notes section 35): evaluate the traces for failed
tool calls; fix what a tool description or the Go function can fix; run the
same kind of task on deepseek-v4.1-flash and gpt-6-luna and fix where both
fail, rather than tune prompts to one model.

## Baseline: gpt-6-luna only

Every paper run created on 2026-10-21 from 00:00 to about 13:50 UTC and
listed by `/api/runs` (500 most recent), with its events. A failure is a
`tool.result` with `is_error` or a `desk.cell` with an error. All desk runs
were on gpt-6-luna (the pool was luna-only until the dual pool lands).

| desk | runs | cells | failures | per cell |
| --- | ---: | ---: | ---: | ---: |
| management | 159 | 1392 | 208 | 14.9% |
| research | 46 | 509 | 65 | 12.8% |
| texture | 38 | 101 | 44 | 43.6% |

(249 more Texture runs with no cells are deterministic turns, not counted.)

Failure families, largest first:

### T1. `choir.Stories` times out the cell (management, 64)

64 cell errors "context deadline exceeded" in 58 runs. In a sample of 14,
every one was a cell calling `choir.Stories(50..200, cursor)` with the
model's 10, 20 or 30 s cell timeout; the call took longer. From code
(`internal/agentcore/intake_view.go` `storiesEgress`,
`internal/store/stories.go`): `ListStoryDocuments` lists every Texture
document object, then reads each candidate's trajectory one at a time to
keep the stories; `StorySummaryOf` then reads each listed story's head
revision one at a time for its lede and version. A page of 200 is up to
about 400 serial object reads on a busy paper. Same shape as the corpus
search scan (research-more-slow-and-hidden-2026-10-21.md E1). Not a model
error: both models will hit it.

### T2. A story open refused because the library moved (management, 78)

78 `texture:new` Casts in 51 runs refused with "the story library moved
since this run last read choir.Stories()". The guard (R2-5,
`internal/store/management_tasks.go` `planStoryLibrary`) refuses whenever
any story opened after the run's last read, whether or not it overlaps.
That is by design (the same event can open through different captures),
and the refusal lists the new stories, but each refusal costs a turn and a
re-read, and T1 makes the re-read slow. Candidate, not decided: refuse only
when a story opened since shares a capture or a resolved event with the
Cast.

### T3. Packet map literals that do not compile (research 39, others)

`expected type, found ','`, `expected ';', found ':'` and similar in
`ReportPacket`/`Cast` map literals (problem E8 in
research-more-slow-and-hidden-2026-10-21.md). Typical causes from the
cells: `"claims":[] ,` (an untyped empty slice in a `map[string]any`
literal) and a nested map without its type. The overlay already says
"Go-native values: pass map/slice literals or equivalent JSON strings".
Candidate: accept a JSON string packet everywhere and say so first, or
make the cell's compiler hint name the fix (`[]any{}` for an empty slice).

### T4. Texture cites with `{{ref N}}` (texture, 15)

"`{{ref 1}}` names no source of this document; `{{ref}}` only keeps a
citation the frame shows". Texture writes numbered refs, as the published
text renders them, instead of the frame's citation tokens (E9 there).

### Smaller families

Source manifest count mismatch on a Texture Write (8), capture lease cap
on a delegate (6), a quote not found in its capture (5 texture), a wrong
field or method on a result (`p.Results`, 12 across desks), reflect and
nil-assertion panics in yaegi (9), arity mismatches (8).

## First dual-pool census (runs since 2026-10-21 14:00)

The pool hashes Texture desks and pooled trajectories. Of the last 120 runs,
18 of 92 pool-hashed runs were deepseek-v4.1-flash (Texture 15 of 71). The
management task runs are pinned to gpt-6-luna. Failures per cell in the
sample:

| Model, desk | Runs | Cells | Failures |
|---|---|---|---|
| luna, management | 35 | 246 | 24 (9.8%) |
| luna, research | 12 | 145 | 10 (6.9%) |
| luna, texture | 8 | 18 | 11 |
| deepseek, texture | 2 | 9 | 5 |

The deepseek sample is too small to compare rates. Families seen so far:

- Both models: a wrong field on a result value (`r.Summary`, `u.Payload`
  for deepseek; `p.Results` and others for luna in the baseline). The hint
  already names the fields of the value's type.
- deepseek: a quote not found in its capture (1).
- luna: `{{ref 1}}` (4; two before the T4 text deployed), packet map
  literals (T3, 4), and T5 below.

### T5. A capture id copied with a chunk missing (texture, luna 3)

Texture cited a capture by its full id rather than the frame handle, and
dropped a chunk from the middle of it:
`obj:choir.web_capture:ZTU5…NGUx:key-a36e…` for the frame's
`obj:choir.web_capture:ZTU5…NGUxZGIx:key-a36e…`. The refusal says "is not
captured … Ask research to Read it", which is wrong: the source is captured
and in the frame, so the advice sends Texture off to ask research again. The
id is about 140 characters; its owner segment is the same base64 for every
capture in the computer, and its `key-<sha256>` suffix is unique.

Candidate (tool API, model-independent): when a cited id is not captured
but exactly one frame source has the same `key-<hash>` suffix, cite that
source; otherwise the refusal names the frame handle of the nearest source.

## Next

1. Fix T1 in the store (model-independent, latency).
2. With the dual pool deployed, rerun the census on runs from both models
   and compare T2 to T4 and the smaller families per cell. A family both
   models hit at a similar rate is fixed in the tool surface; one only one
   model hits is recorded, not prompted around.

## Heresy delta

- discovered: T1 to T5.
- introduced: none.
- repaired: none.
