# Standing Questions — Agent Pre-Flight Entry Point

**Read this before authoring or executing any mission Definition.** It is an
entry point, not a doctrine source: it inherits `AGENTS.md` and
`docs/choir-doctrine.md`. Each question below caught a real, expensive failure
in this repository; the parenthetical names the receipt. Ask every question
that applies before your first code or doc mutation. A mission that cannot
answer these is not ready to execute.

## Before trusting a decision

**1. Who settled this — owner or orchestrator?**
Check the `source:` line of any `status: settled` node. Orchestrator-settled
syntheses are proposals until the owner ratifies them, no matter what their
status field says.
*(Caught: the phantom "vmctl ComputerVersion route ledger" — a third Dolt
store agents nearly built, invented by an orchestrator synthesis that even
wrote itself a defense clause against the owner's two-store directive.)*

**2. Does this mission's topology conform to every settled decision at
authoring time?**
Diff the mission's Real Artifact / architecture sections against the settled
decision registry before the first execution move.
*(Caught: the autopaper activation definition hard-coded embedded-Dolt VM
topology two days after D-WIRE settled sql-server mode — twelve attempts,
including a 12-hour run, faithfully rebuilt the rejected architecture.)*

## Before deleting or deprecating

**3. What still references the thing being deleted?**
Grep for the path/name across `docs/`, `specs/`, `skills/`, and code comments.
A deletion that leaves citers is not a deletion; it is a haunting. Repair or
redirect every citer in the same commit.
*(Caught: three deleted owner-doctrine docs — autoputer-before-autopaper,
universal-wire-stabilization, substrate-independent-audited-computer — still
cited as live sources by TLA specs while the owner repeatedly rediscovered
"I thought we decided this.")*

**4. Who consumes this in production?**
Audit non-test callers before building on, patching, or keeping a subsystem.
No callers means delete-first, not maintain.
*(Caught: 38 self-validating Base contract files with no production callers,
deleted by seam-repair after consuming attention as "architecture".)*

## Before building or fixing

**5. What is the single authority for this state?**
If two projections can disagree about a fact, name which one wins before
touching either. If none wins, that is the bug.
*(Caught: processor lifecycle split across five projections — run state,
trajectory, processor-resolution, sourcecycled ledger, admission counter —
each pairwise disagreement froze the pipeline a different way.)*

**6. What artifact proves this success claim?**
A run, deploy, or mission is complete when its required artifact exists and
is fetched, not when a process, agent, or log narrates success.
*(Caught: a reconciler that "completed successfully" after the runtime
cancelled its only mandatory write; a deploy verifier that failed six correct
deploys; edition "publication" that no human ever saw in the app.)*

**7. What does this read path fate-share with?**
Enumerate every component that must be simultaneously healthy for the read to
succeed. Durable product state must be servable without the live substrate
that produced it.
*(Caught: `/api/universal-wire/stories` traversing auth → proxy → vmctl →
VM boot → guest runtime → single-connection Dolt, so every deploy closed the
publication window and "working" was only ever true during curl-diagnostic
lulls.)*

**8. What happens to this state on restart?**
Process-local state is an availability decision, not a default. Name what a
restart erases and whether that is acceptable.
*(Caught: sourcecycled's MemoryStore losing its entire queue and poll ledger
on every deploy, triggering 4,900-item cold re-fetches.)*

## Before calling anything operable

**9. Could an agent do this without SSH?**
Every diagnosis, lifecycle action, and acceptance proof should be reachable
through the product API / choir CLI under a scoped key. SSH-shaped operations
are platform break-glass, not product paths. See the Introspection Contract
in `docs/definitions/choir-autoputer-cli-operability-2026-07-11.md` for the
safe limit (authority-scoped, receipts-not-shells, substrate-neutral
diagnostics).
*(Caught: twelve attempts diagnosed entirely via journalctl/systemctl on
Node B — a surface neither external agent operators nor co-supers will have.)*

## Registry hygiene contract

When a Definition is created, superseded, or settled, the same change must
update all three navigation surfaces — `docs/ACTIVE.md`,
`docs/mission-graph.yaml`, `docs/doc-authority-manifest.yaml` — or the next
agent executes stale topology. Verify no dangling references remain:

```bash
for f in $(grep -rhoE 'docs/[a-zA-Z0-9_/-]+\.(md|yaml)' docs/*.md docs/definitions/*.md specs/*.tla specs/README.md AGENTS.md | sort -u); do
  [ -f "$f" ] || echo "DANGLING: $f"
done
```

*(On 2026-07-11 all three registries still directed agents to execute the
superseded autopaper topology; this is how attempt 13 would have repeated
attempt 12.)*

## Provenance rule for new settled nodes

Every node promoted to `settled` must carry `settled_by: owner` or
`settled_by: orchestrator`. Orchestrator-settled nodes are binding for
execution ordering but are **proposals** for architecture/authority claims
until the owner ratifies them in a dated statement. When an owner statement
and an orchestrator synthesis conflict, the owner statement governs and the
synthesis must be demoted in place, not silently rewritten.
