# VText Agentic Invariants - 2026-06-13

## Status

Doctrine and guardrail document. This records the working VText semantics after
the M3 regression review, where an acceptance probe accidentally turned VText
researcher delegation into a forced runtime workflow.

This document inherits [choir-doctrine.md](./choir-doctrine.md). Choir Doctrine
is the apex; this file specializes it for VText. If this document is used to
justify hidden workflow forcing, first update the doctrine-level conjecture and
protected-surface evidence packet.

VText is currently the most fragile core of Choir. It is also the key dependency
for almost every higher-level product path: owner-readable mission state,
canonical documents, appagent coordination, source-to-story work, publication,
promotion review, and long-running self-development all rely on VText behaving
as an agentic document system rather than a workflow runner.

Read this before changing VText tools, prompts, routing, revision creation,
coagent wake behavior, trace/VText projection, run acceptance, or any mission
that uses VText as its owner-readable narrative.

## Core Invariant

> VText owns canonical document versions inside a multi-agent system. It is not
> a workflow engine, a route script, or a role-sequence executor.

VText may write, revise, wait, ask researcher, ask super, ask both, ask neither,
request clarification, or report a blocker. The correct choice is part of the
VText agent's obligation and authority envelope. Runtime may expose tools,
durable evidence, pending work, and policy constraints; runtime must not force a
semantic delegation merely because text or metadata mentions another role.

## Non-Negotiable Rules

1. **Canonical text is VText-owned.** User revisions and VText appagent
   revisions are canonical document versions. Researcher findings, super
   updates, trace moments, search results, and worker evidence are inputs to
   VText, not canonical text until VText incorporates them into a revision.

2. **Delegation is agentic.** VText decides whether to call `spawn_agent`,
   `request_super_execution`, both, neither, or a future coordination tool. A
   prompt saying "researcher" is evidence about owner intent; it is not a hard
   runtime command to spawn a researcher.

3. **No semantic forced continuations from `edit_vtext`.** `edit_vtext` stores a
   document revision. It must not require a subsequent researcher, super,
   verifier, or other semantic appagent call. Deterministic app protocol
   handoffs, such as persisting an email draft for owner approval, must be
   explicit, narrow, and documented separately.

4. **Required tool choice is not policy.** Exact next-tool enforcement is
   allowed only for mechanical tool protocols whose second call is part of the
   same protocol state, for example a worker allocation followed by a
   worker-start handshake. It must not be used to steer VText's semantic
   choices.

5. **Prompts and metadata do not replace agency.** Prompt flags, revision
   metadata, and route hints may inform VText. They must not become hidden
   workflow edges. If the product needs explicit commands, create a visible
   command grammar and still record the resulting obligation as state VText can
   settle honestly.

   Durable metadata forcing is an explicit violation: persisted flags such as
   `explicit_researcher_request`, base-revision content scans, or carried
   request-intent fields must not re-derive a required researcher/super
   delegation across turns. Prompt-pipeline forcing is also a violation:
   prompts and revision builders may describe obligations and affordances, but
   must not mandate "call spawn_agent now" or similar semantic role sequences.

6. **Trace and VText have different jobs.** Trace is the causal ledger for tool
   calls, LLM content, events, and agent messages. VText is the owner-readable
   narrative and canonical document surface. Do not turn VText into a Trace-like
   topology/status dump, and do not use Trace role sequences as a substitute for
   VText semantics.

7. **Acceptance verifies outcomes, not role choreography.** A test may require
   researcher participation only when the product behavior under test is
   researcher participation. Lifecycle missions must verify lifecycle evidence:
   open obligations, passivation, rewarm, delivered updates, settlement, and no
   stranded work. They must not force a particular VText delegation sequence as a
   proxy.

8. **Harness minimalism protects VText.** Do not add VText-specific branches to
   the core tool loop, provider loop, continuation machinery, or run acceptance
   unless there is a documented invariant, a simpler prompt/policy/tool
   alternative has been rejected, focused regression tests exist, and a human has
   explicitly approved the divergence.

9. **Structured edits are the default for long documents.** Whole-document
   rewrites are exceptional and require rationale. VText must preserve document
   structure, provenance, revision history, and source/citation semantics.

10. **VText regressions are architecture regressions.** Treat unexplained VText
    failures as mission-level blockers. Do not patch around them with one-off
    workflow enforcement unless the invariant being protected is explicitly
    named and reviewed.

## Allowed Runtime Help

Runtime may:

- expose `spawn_agent`, `request_super_execution`, source tools, edit tools, and
  other affordances to VText;
- preserve owner intent, source refs, revision metadata, and trajectory/work
  evidence durably;
- wake VText from pending coagent updates or assigned work items;
- debounce/coalesce updates before waking VText;
- surface pending obligations and missing evidence in prompts;
- prevent duplicate revision writes and protect owner approval boundaries;
- reject invalid edits or unsafe operations.

Runtime may not:

- convert a role mention into a forced next tool;
- require VText to ask researcher/super/verifier after storing a revision;
- silently satisfy VText obligations through another agent's route;
- mark exact internal role sequence as acceptance unless that sequence is the
  product requirement;
- hide role-specific control policy in generic tool-loop continuation code.

## Regression From M3

During M3, the deployed restart proof required Trace to show conductor, VText,
researcher, and super before vmctl refresh. When researcher did not appear, the
mission drifted from durable-actor lifecycle proof into trying to force VText to
spawn researcher. The final shape returned `next_required_tool=spawn_agent` from
`edit_vtext` and relied on the generic tool loop to enforce exact `spawn_agent`.

That was a regression. It made a probe precondition the runtime semantics.

Correct recovery:

- remove hard researcher continuation from VText;
- document this invariant in worker-facing docs;
- test that VText is not forced by role mentions;
- redesign M3 acceptance around lifecycle evidence;
- keep researcher participation as a possible VText choice, not a runtime
  workflow step.

## Required Tests For Future Changes

Any behavior-changing VText coordination change should include tests proving:

- `edit_vtext` does not emit semantic `next_required_tool` values;
- prompts mentioning researcher or super do not force a delegation;
- VText still has access to researcher/super affordances and can choose them;
- researcher findings remain non-canonical until VText incorporates them;
- public/product acceptance observes outcomes and obligations, not hidden role
  sequence;
- long-document revisions preserve structured-edit defaults and operation
  evidence.

Tests to invert or delete when M3.1 repairs H010/H024/H026:

- tests that expect `edit_vtext` to emit `next_required_tool=spawn_agent`;
- tests that preserve researcher intent through durable revision metadata as a
  forced follow-up;
- tests that treat base-revision content mentioning researcher as a required
  delegation oracle;
- tests that require VText's first tool to be `request_super_execution` because
  a prompt matched super keywords;
- prompt-default assertions that encode a fixed VText -> researcher -> super
  role sequence instead of obligations and evidence.

## Protected Surface Rule

VText canonical writes, revision metadata, prompt routing, coagent wake
behavior, Trace/VText projection, and acceptance involving VText are protected
surfaces under Choir Doctrine. Before changing them, name the mutation class,
conjecture delta, evidence class, rollback path, protected surface touched, and
heresy delta (`discovered`, `introduced`, `repaired`).

## Short Rule For Agents

If a proposed VText change makes the sentence "VText must call X next" true for
a semantic agent role, stop. You are probably turning the multi-agent system
into a workflow. Document the problem, shift the observer, and protect VText's
agency before writing code.
