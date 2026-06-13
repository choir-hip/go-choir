# MissionGradient Method

Last updated: 2026-06-13

Status: legacy baseline. New broad Choir missions use Parallax/paradocs; see
`docs/parallax-design-2026-06-11.md` and `AGENTS.md`. Keep this document for
historical MissionGradient mission documents and for the orientation-field
ideas Parallax superseded rather than deleting.

MissionGradient is a way to give long-running agents a **sense of direction** without pretending the route is already known.

A checklist says: do these steps.

A MissionGradient says: keep moving uphill, preserve these invariants, notice when the map is wrong, and prove each promotion before it becomes real.

At plain-language level, it is a compass plus field discipline:

- the **compass** says what kind of world the work should move toward;
- the **field discipline** says what not to break, what evidence counts, and when to stop rather than fake progress.

For a coding agent, that means replacing brittle instruction like:

```text
Build background agent support.
```

with a directional frame like:

```text
Move Choir toward reliable long-running background work.
Preserve user state, rollback, provenance, and promotion boundaries.
Prefer real traces over demos.
If the premise breaks, update the belief state before mutating more.
```

## The deeper mechanism

The banal version of MissionGradient is “write a better prompt.”

The deeper version is: **long-running agent work needs a control field, not merely a goal statement.**

A normal goal defines a destination. But once an agent runs for hours, the original route becomes obsolete. New errors appear. Tests reveal hidden state. The apparent target may turn out to be a fake island. At that point the agent needs something more fundamental than steps: it needs a way to re-orient under changing reality.

The load-bearing variable is **orientation quality under uncertainty**.

That is why MissionGradient borrows from optimization, cybernetics, and homotopy:

- from optimization: define what counts as improvement;
- from cybernetics: observe, act, compare, and correct;
- from homotopy: simplify the real object without changing it into a different object.

The point is not math decoration. The point is to prevent a common failure mode: the agent completes local tasks while drifting away from the real artifact.

## Accessibility and depth

MissionGradient needs two opposite virtues at the same time:

1. **Accessibility:** the agent and human operator need a usable handle. They should be able to say, “This move is uphill,” “this breaks an invariant,” or “this evidence is fake.”
2. **Depth:** the handle must point at the real structure, not a slogan. “Move fast,” “verify,” “ship,” and “simplify” all have shallow versions that can destroy the mission.

That is why the Cognitive Transform Portfolio matters here. Audience-Level Translation makes the method travel without losing its structure. Depth Extraction / Esoteric Upgrade prevents the method from collapsing into management-speak.

For MissionGradient:

```text
Accessible version:
Give the agent a compass, safety rails, evidence rules, and a stop rule.

Deep version:
Construct an invariant-preserving control field over artifact states, where evidence continuously updates the agent's belief state and admissible next moves.
```

Both are true. The useful method keeps them connected.

## Homotopy, not ladder

The central rule is:

```text
Use homotopy, not ladder.
```

A ladder says: first make a toy, then later make the real system.

Homotopy says: start with a low-resolution version of the real system and continuously increase realism while preserving identity.

A toy can be useful only if it is a projection of the real object. If it uses different state, different authority boundaries, different event semantics, or different proof semantics, it is not a smaller version of the real thing. It is a different island.

So the practical question is:

```text
Can this simplified path deform into production without crossing a trust-boundary cliff?
```

If not, the “MVP” is probably a demo trap.

## What a MissionGradient names

A full MissionGradient should name:

- **Requirements contract** — the spec or requirements doc that defines product
  invariants and acceptance semantics, or an explicit statement that the mission
  itself is the requirements contract;
- **Real artifact** — the durable thing being changed;
- **Invariants** — properties that define artifact identity and cannot be traded away;
- **Value criterion** — what “uphill” means;
- **Belief state** — what the agent thinks is true, and why;
- **Homotopy parameters** — realism axes that can increase without changing topology;
- **Dense feedback** — tests, traces, logs, screenshots, deployed checks, and artifacts that expose local error;
- **Forbidden shortcuts** — fake wins, bypasses, demos, and test-only paths;
- **Rollback policy** — how to return safely if promotion fails;
- **Learning side-channel** — where discoveries survive the run;
- **Stopping condition** — when proof is sufficient, or when continuing would become slop.

The skill file contains the full operational template. This document is the conceptual handle.

## How to use it in practice

Before sending an overnight or multi-hour agent run, write the mission so it can answer:

- What is the real artifact?
- What must remain true for this to still be the same artifact?
- What would a fake local win look like?
- What is the smallest real proof that preserves topology?
- What evidence would change the belief state?
- How far may the agent mutate before it must observe again?
- What counts as done, and what counts as a clean stop?

Then the `/goal` can be short:

```text
/goal Run docs/<mission>.md as MissionGradient.
```

The mission should then name its requirements contract near the top. This keeps
the slash goal short while avoiding disconnected redundancy between a spec and a
mission.

## What MissionGradient is not

MissionGradient is not:

- a longer checklist;
- a vibe;
- a permission slip for scope creep;
- a way to make fake demos sound principled;
- a substitute for tests, traces, source artifacts, or deployed verification.

It is a steering method for artifact-native work. It lets an agent adapt without losing the mission, and lets a human judge progress without micromanaging every token.

## One sentence

**MissionGradient is a control field for long-running agent work: preserve the invariant, update the belief state, increase realism, and prove promotion before changing reality.**
