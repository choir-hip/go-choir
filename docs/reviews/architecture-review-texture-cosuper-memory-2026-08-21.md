# Architecture Review: Texture Revision Semantics, CoSuper Cardinality, Memory Overcommit

Date: 2026-08-21
Status: v2 — corrected per owner review; ready for agentic consensus
Context: assignment supersession loop on staging (docs/evidence/effects-red-assignment-supersession-loop-2026-08-21.md). This review supersedes the active effects mission; a new definition mission sequence will be charted from its conclusions.

---

## 0. Owner corrections incorporated in v2

1. **Single writer:** Texture revisions are written by the Texture agent only. No other agent ever edits the document directly; Super/CoSuper/owner inputs arrive as structured updates that the Texture agent incorporates through its own turn loop. Single-writer is essential — multi-writer means chaotic thrashing.
2. **Revision = version:** every revision committed by the Texture agent is a new version number. A "revision" that does not bump the version is not a revision.
3. **Self-contained snapshots:** each revision fully captures the current semantic state. Reading prior versions is never required to act on the current one — prior versions exist for debugging and curiosity.
4. **Scale expectation:** dozens to hundreds of versions for agents running tens of minutes to hours.
5. **Document-driven supervision is load-bearing:** document-based (not chat-based) agent interface is deliberately off-distribution and is the mechanism that keeps multiagent supervision tractable as agent count and task horizon grow. Soft-realtime: the document reflects current state at all times, editable at any moment; an owner edit propagates downstream as a diff-shaped instruction via agent-to-agent messages.

Live staging evidence confirms the single-writer structure exists (~250 texture-turn commits per trajectory, all authored by the Texture profile) but **violates correction #2**: the head sits at v1/v2 despite hundreds of commits, because turns currently rewrite the same head without producing distinct versioned snapshots. The code contradicts the doctrine; docs-first, then code.

---

## 1. Texture revision semantics (the heresy and the fix)

`docs/texture-live-supervision-architecture.md` §2 presents a v₀ intake → v₁ scope → … → v₈ restore progression table. Read today, it implies a normative pipeline. The correct doctrine:

- Revisions are self-contained snapshots of semantic state.
- A new revision is committed whenever semantic state changes, at whatever cadence that demands — dozens/hundreds per session.
- Version numbers are monotonic ordering, nothing more. There is no prescribed stage sequence; any actor's authoritative update (Super report, CoSuper evidence, owner edit) may drive the next revision at any time.
- Prior versions are never required context for acting on the current version.
- The owner can edit the current version at any time; the edit reaches downstream agents as instructions derived from the diff.

**Doc fixes:**
- Reframe §2 as one non-normative example sequence for a single-candidate lifecycle.
- Add the normative statements above as the "Living Document Protocol" section.
- State explicitly: every Texture-agent turn that changes semantic state produces exactly one new version; turns that produce no semantic change commit no revision.
- Add document-driven supervision as a named design principle with the scaling rationale.

**Code gaps to fix after docs (recorded now, executed later):** turn-loop currently collapses many commits into one version (`texture_turn_committed` events ≈ 250 vs head at v2); `selfdev/operations.go` linear state machine must be documented as an effects-pipeline workflow instance, orthogonal to Texture versioning.

## 2. Actor model: computer-level supervisor, task-level actuators

- **Texture agent:** sole document writer. Incorporates updates from all directions into new revisions. Holds the document-editing tools exclusively.
- **Super:** computer-level supervisor. Orchestrates, delegates, reads reports; cannot mutate the computer or the document; communicates upward via typed report updates.
- **CoSupers:** task-level actuators. Execute capability-scoped work; report evidence upward; no upward channel beyond their typed update path.

The whiplash on staging is a scheduling-policy gap, not a reason to abandon the singleton Super. Multiple Supers would split single-writer/single-arbiter authority over the event chain for no benefit. What was missing:

1. **Cardinality invariant:** one live CoSuper assignment per computer (v1). Generalization to N comes later, behind the resource policy (§4).
2. **Selection policy:** when multiple execution_requests pend, FIFO by arrival; unselected requests remain pending untouched. The Super prompt carries only the selected work item so the model cannot open competing assignments.
3. **Request settlement:** a pending execution_request settles only when its operation terminals or an owner correction supersedes it. An operation with a live assignment must not re-emit requests (join-path dedup by operation ID).

## 3. CoSuper↔capsule coupling

Doctrine: a CoSuper assignment MAY hold multiple capability-bound capsules (parallel shards, build+runtime split); bundle provenance binds the union of receipts.

Implementation today: 1:1 everywhere (binding schema carries one CapsuleID/capability/handle triple; spawn, reclaim, and fate paths assume bijection).

Decision: keep 1:1 implementation through candidate A; write the general form into the docs now so the schema extension (Binding.Capsules []CapsuleRef) is a planned, non-surprise change.

## 4. Memory overcommit replaces hard admission

Current: executor refuses spawn when sum-of-limits would exceed ¾ MemTotal; cgroups use memory.max with swap off. Result: 2-capsule ceiling and a deadlocked computer when scheduling thrash meets admission denial.

Target policy:

| Level | Mechanism | Behavior |
|---|---|---|
| Soft cap | cgroup memory.high = requested | throttle/reclaim under burst |
| Hard cap | cgroup memory.max = 2× requested | OOM kill only genuine runaway |
| Admission | Σ(requested) ≤ total × 1.5 overcommit factor | spawn allowed while factor holds; no denial cliff at Σ == total |
| Pressure response | monitor PSI memory pressure | pause lowest-priority capsule; resume when pressure clears |

With the §2 cardinality invariant, pressure should be rare; the policy exists so that when concurrency generalizes, resource contention degrades gracefully instead of deadlocking.

## 5. Texture agent scope (owner clarification, 2026-08-21)

The Texture agent does exactly two things: revise the document, and message
other agents. That is the whole job.

- **Human interface:** humans interface only with Texture. The document is
  therefore optimized for human-language writing — prose quality, structure,
  and readability are first-class concerns, not decoration. Agents never see
  chat; they see instructions derived from document state.
- **Owner edits:** an owner message does not need to reach agents verbatim.
  The edit changes semantic state; the Texture agent incorporates it and
  propagates downstream whatever is clear — verbatim text when precision
  matters, distilled instruction when intent matters. The diff is the input;
  the propagated form is the Texture agent's judgment.
- **Style guide textures (planned, not implemented):** any document can serve
  as a styleguide that shapes how the Texture agent writes future revisions.
  This keeps agent-to-agent comms and domain agents in their own registers —
  terse, machine-oriented — while the human-facing document stays in the
  owner's preferred register. A precursor exists in the wire-publish style
  catalog (`coagent_route.go` `WireStyleSource`), which routes content by
  style heuristics today; generalizing that into owner-designatable styleguide
  documents is the planned shape.

Doctrine consequence: "Texture" names both the document class and the agent.
The agent's authority is exactly: write revisions (single-writer) and send
agent-to-agent messages derived from document state. It holds no capsule, no
event-chain write authority, no provider-routing authority.

## 6. Mission supersession

The active candidate-proof mission is superseded by this review's conclusions. A new definition sequence starts from:

1. Docs-truth mission: rewrite texture-live-supervision-architecture.md (revision protocol + example demotion), current-architecture.md supervision section (actor model, cardinality, scheduling), computer-ontology.md note on capsule↔assignment cardinality direction.
2. Texture revision-cadence repair: every semantic-state-changing turn yields a new versioned snapshot; add detector for version-stall under continued commits.
3. Scheduling repair: FIFO selection, prompt narrowing, request-settlement dedup.
4. Resource policy: memory.high/max split, overcommit factor, pressure pause/resume.
5. Only then resume the candidate A authorship → consensus → promotion → falsification → restore arc under the new definitions.

Effects remain OFF throughout. Checkpoint 99949fe2 remains the restore fence.
