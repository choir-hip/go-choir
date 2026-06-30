# Global Wire SourceMaxx Newsroom Architecture Diagram - 2026-06-07

**Status:** review diagram before implementation.  
**Purpose:** make the intended architecture concrete enough to critique before
more code lands.

## System Shape

```text
+----------------+        +----------------------------+
| SourceMaxx     |        | VText Corpus               |
| Ingestion      |        |                            |
|                |        | - platform article VTexts  |
| GDELT          |        | - user-owned VTexts        |
| RSS/Atom       |        | - published user VTexts    |
| Telegram       |        | - processor note VTexts    |
| Search         |        | - reconciler note VTexts   |
| Curated feeds  |        | - researcher packet VTexts |
+-------+--------+        | - Style.vtext artifacts    |
        |                 | - native Dolt versions     |
        v                 | - per-version sources      |
+-------+----------------+| - multimedia transclusions|
| Deterministic Ledger   || - VText transclusions      |
|                        ||                            |
| - SourceItems          |+-------------+--------------+
| - fetch runs           |              ^
| - dedupe/echo evidence |              |
| - source metadata      |              |
| - routing hints        |              |
| - provider health      |              |
+-------+----------------+              |
        |                               |
        | routed source batches         |
        v                               |
+-------+----------------+              |
| Processors             |              |
| long-running agents    |              |
|                        |              |
| - hot context/KV cache |              |
| - source handles       |              |
| - active developments  |              |
| - watch items          |              |
| - unresolved questions |              |
| - compaction chain     |              |
| - note/brief VTexts    |--------------+
+----+-----------+-------+
     |           |
     |           | VText write/update requests
     |           v
     |     +-----+----------------+
     |     | Existing VText Agents |
     |     | writing/editing      |
     |     |                      |
     |     | Inputs:              |
     |     | - processor notes    |
     |     | - reconciler notes   |
     |     | - researcher packets |
     |     | - current VText      |
     |     | - matched styles     |
     |     |                      |
     |     | Outputs:             |
     |     | - article VTexts     |
     |     | - new versions       |
     |     | - user-owned forks   |
     |     +-----+----------------+
     |           ^
     |           |
     | research requests / packets
     v           |
+----+-----------+--------+
| Existing Researchers    |
| bounded evidence agents |
|                         |
| - verify claims         |
| - find missing sources  |
| - compare accounts      |
| - source standing       |
| - evidence packets      |
+-------------------------+

+-------------------------+       +-----------------------------+
| News App Surface        |<----->| Reconcilers                 |
|                         |       | corpus-level live agents    |
| - newspaper columns     |       |                             |
| - source chronology     |       | They range over:            |
| - filters               |       | - published VTexts          |
| - compact provenance    |       | - active platform VTexts    |
| - Open in VText action  |       | - user VTexts if allowed    |
| - no contribution dash  |       | - processor notes           |
| - no nested panels      |       | - source state              |
+------------+------------+       | - researcher packets        |
             |                    | - VText traversal indexes   |
             v                    |                             |
+------------+------------+       | They produce/request:       |
| VText Traversal Index   |       | - consensus notes           |
| rebuildable accelerator |       | - contradictions            |
|                         |       | - open questions            |
| - VText/version ids     |       | - article updates           |
| - per-version sources   |       | - follow-up article ideas   |
| - transclusion edges    |       | - researcher requests       |
| - style citations       |       | - VText requests            |
| - publication state     |       +-----------------------------+
| - user-published refs   |
+-------------------------+
```

## Key Corrections

A story/article is a VText. Platform articles, style-shaped projections,
user-owned versions, and counterstories are not separate object classes. They
are ordinary VTexts with ownership, publication, style, citation, and version
state.

VText is the provenance-bearing object. Sources are per-version, not
per-VText. Native Dolt-backed VText versioning carries version provenance, and
VText markup can transclude sources, multimedia, styles, and other VTexts.

The VText transclusion/version system creates the implicit graph. A traversal
index may be needed for performance, especially for future reading/navigation
surfaces, but it is rebuildable from VText/source state and is not authority.

Reconcilers are not downstream of processors. Processors absorb new source flow
and may request researcher/VText work. Reconcilers review the wider corpus,
including existing published VTexts and current source state, to find
consensus, contradictions, drift, corrections, and follow-up ideas.

## Agent Loop Assumption

Processors and reconcilers should use the same underlying agentic loop as other
Choir agents:

```text
role prompt + durable state + tools + channel/request records
-> tool calls / researcher requests / VText requests
-> compaction when needed
-> continuation with source and state handles
```

They should not require a custom harness unless a documented invariant proves
the shared loop cannot support them.

## Deterministic vs Agentic

Deterministic substrate owns:

- source ingestion;
- SourceItem identity;
- fetch provenance;
- dedupe and echo metadata;
- simple routing;
- ownership/publication boundaries;
- native VText version records through the existing VText/Dolt path;
- rebuildable VText traversal indexes.

Agents own:

- live understanding;
- evidence interpretation;
- contradiction/question discovery;
- research requests;
- writing/revision requests;
- publication-quality VText drafting through existing VText agents;
- compaction of their own working context.

## Out Of Scope For This Mission

Autoradio is a future consumer of VText traversal, not a deliverable here. It
will require TTS/STT model exploration and a separate design for turning a path
through VText graph space into a single fluid narrative.

## Review Questions

- Should processors ever receive existing VText state proactively, or only
  when a routed source batch appears related?
- Should reconcilers run continuously over the whole corpus, or on triggers
  such as new processor notes, publication events, user edits, and source
  bursts?
- Should VText agents receive notes directly from processors, reconcilers, or
  both with an editor arbitration step?
- What is the first honest product proof: processor hot-context continuity,
  reconciler over existing published VTexts, SourceMaxx ingestion volume, or
  VText traversal index queries?
