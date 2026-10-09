# Texture research turn cannot cite the research it received (2026-10-09)

Found by Texture acceptance suite run 7 on staging (build 733bec77;
receipt `evidence/texture-acceptance-2026-10-09T22-03-56-155Z.json`), fresh
disposable `computer-1df030fc…`, research document `38779851…`. Written
22:17Z. Problem first; hypotheses, not findings, where marked.

## Run 7 in one line

T1–T5 pass (T2b now idles in 21 s: the research budget works), T7 and T8
pass for the first time (crash reads `crash_or_stop`; the document is
interrupted, not pending, and gains no revision after the crash; nothing
owed, retrying or exhausted). T5b fails (known). T6 fails: no research
revision in 6 minutes.

## T6 trace

- 22:06:08 first draft; it opens two research assignments (the creation
  budget).
- 22:06:09 run `738f8866…` reactivated "for coagent_result".
- 22:06:12 owner's research request; Texture run `9c895e2c…` starts.
- 22:06:25, 22:07:05 (×2) research reports queued.
- 22:06:27 a Texture turn commits "desk cell completed with no authoring
  act; consuming the owner revision".
- 22:06:12–22:12:25 run `9c895e2c…`: 25 iterations, 660K input tokens.
  Its first cell printed empty INBOX and UPDATES. It then searched for a
  way to cite (`choir.Sources`, `choir.SourceEntities`, `choir.Evidence`,
  `choir.Outline`: none exist), probed ApplyTexture with invented fields
  (`uri`, `title`, `kind`) and ids (`__probe__`, `__nope__`), and got
  `source_entity_id or source_entity is required` and `not present in
  the current structured source_entities`. Still working when T7 crashed
  the computer.
- At the crash boot all four occurrences (22:06:12, 22:06:25, 22:07:05
  ×2) were still unprocessed and became `interrupted_by_restart`.

## What this shows

1. The citation contract exists and the desk is told about it
   (`textureprompts/overlays/revision_policy.yaml`: cite *listed*
   source_entities with `insert_source_ref`). The citing run had no
   listed source entities to cite.
2. Hypothesis H1 (not confirmed): two Texture runs were live on one
   document, and the 22:06:27 commit of the other run consumed the queued
   research packets by the consume-at-commit default, so the run serving
   the owner's request never saw them.
3. Hypothesis H2 (not confirmed): research packets that arrive after a
   run's first cell are not surfaced to that run's later cells.
4. A turn that cannot find what it needs spends its whole budget probing
   (25 calls, 660K tokens).

## Next observation

The update records for this trajectory (which turn each research packet
was delivered or consumed to) and whether one document can hold two
active Texture runs. Read from the guest before any fix.

## Update records read (22:30Z)

The trajectory's update records: the three research reports to Texture
(22:06:25, 22:07:05 ×2) are all still `pending`; nothing consumed them.
**H1 refuted.** The citing run's later cells printed update ids from the
research runs, so it saw the reports. **H2 refuted.** Two of the three
reports carried no sources; one carried one source.

H3 (current): the citing turn had one packet source and no listed source
entity, and could not find how to turn a packet source into a citation.
The edit contract allows it (`insert_source_ref` with an inline
`source_entity` whose `target` holds `kind` and `uri`; the runtime mints
the id), but the desk put `uri`, `title` and `kind` at the top level and
the errors (`source_entity_id or source_entity is required`) never showed
the nesting. Same class as the research packet problem: a strict
contract, an error that does not show the shape, and a desk that spends
its budget probing. Fix direction: return the minimal
`insert_source_ref` shape in those errors and in the revision guidance
for packet sources.

## Why the citing run had nothing listed (22:41Z, code reading)

The listed pool (`texture_available_source_entities` in run metadata) is
built once, when the owner revision starts the run
(`texture_agent_revision.go`, from the commitment records at that
moment), and ApplyTexture resolves `source_entity_id` only against it
(`tools_texture.go`). The per-cell view (`choir.Updates()`) carries each
packet's sources with their URIs but no entity id. In T6 the owner's
request started the run at 22:06:12 and the research reports arrived at
22:06:25 and later, so the pool was empty for the whole run. Research is
by design the thing that arrives during a run, so any research turn that
starts before its report lands can cite only inline.

The inline path already works: `insert_source_ref` with
`source_entity: {target: {kind: "web_url", uri}}` and no id; the runtime
mints the id and fills display, evidence and provenance defaults. The
desk was never shown that shape.

Two fixes, in order:

1. (yellow, now) The citation guidance and both citation errors show the
   inline shape for a source from a research update.
2. (red, named residual `texture-source-pool-refresh`) Refresh the
   listed pool from the commitment records at each apply and show entity
   ids beside packet sources, so mid-run sources are citable by id.
