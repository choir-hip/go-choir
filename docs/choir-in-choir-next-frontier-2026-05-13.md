# Choir-in-Choir Next Frontier

Date: 2026-05-13
Preceding mission: `docs/mission-choir-in-choir-deformation-v0.md`

## What Comes After Run Memory

Run memory preserves continuity. The next object is run control memory: a durable controller state that decides what work should happen next, in which world, with which authority, and under which verifier contract.

The next frontier is not more summarization. It is closed-loop self-development:

```text
evidence -> objective proposal -> candidate world -> verifier contract -> promotion decision -> continuation proposal
```

Run memory answers: "What happened and what matters?"

Run control memory answers: "Given that, what should run next, where, and how will we know it helped?"

## Current Capability Edge

The system now has enough primitives for a low-resolution closed loop:

- source runs can compact;
- continuations can be recorded and started;
- candidate patchsets can be queued, verified, and promoted;
- `vsuper` can represent candidate-world authority;
- worker export results become promotion queue records.

The first low-resolution synthesis slice now exists: `docs/run-control-memory-synthesis-proof-2026-05-13.md`. It chooses verifier-first continuation objectives from promotion candidates, or falls back to the mission-gradient document when no candidate signal applies.

The missing part is now broader synthesis and product operation: choosing from more durable signals, exposing proposals in UI/Trace, and eventually auto-starting bounded next objectives without Codex supplying them.

## Next Mission Gradient

The next run should optimize one continuous artifact: the Choir self-development controller.

At low resolution, the controller is now a deterministic selector over:

- queued promotion candidates;
- failed promotion candidates;
- verified candidates requiring owner review;
- mission doc fallback.

The next expansion should add:

- mission doc open obligations;
- failed verifier contracts beyond promotion records;
- missing product proof artifacts;
- run memory compaction summaries.

At higher resolution, the selector becomes a super/appagent decision loop that writes an auditable continuation proposal with objective, reason, authority, lease, verifier contract, stop condition, and rollback point.

## Recommended Next Product Pressure

Use the real launcher/uploads/themes/files path rather than a marker patch:

- bottom-left desktop button opens an app launcher;
- launcher includes desktop app entries and open-state affordances;
- Files app supports upload UI and server-side file-root enforcement;
- Settings exposes a theme editor backed by the existing theme schema;
- a small preset set demonstrates NeXT, classic Mac, Aqua, Frutiger Aero, Linux/GTK, and Y3K as editable examples, not hard-coded finality.

This is still the right pressure source before podcast/radio because it exercises desktop UX, file APIs, validated user config, and promotion without the full semantic media stack.

## Next Verification Contract

A skeptical reviewer should see:

- Playwright opens Choir and uses the prompt/product path to request the work;
- a background worker exports a patchset;
- promotion queue shows candidate metadata and verifier results;
- integration branch passes Go/frontend/Playwright checks;
- promotion is explicit and blocked on divergence;
- automatic continuation creates the next bounded run after success.

Current evidence covers deterministic continuation selection, not the full visible product-path controller.

## One-Line Goal

`/goal Use MissionGradient to execute docs/mission-choir-in-choir-controller-v0.md, turning run memory plus promotion queue into a self-development controller that selects, verifies, promotes, and continues one real launcher/uploads/themes/files product slice through the Choir product path.`
