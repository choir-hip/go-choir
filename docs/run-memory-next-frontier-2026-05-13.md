# Run Memory Next Frontier

Date: 2026-05-13

## Position

Run memory v0 gives Choir a durable context substrate for tool-loop runs. This
note originally pointed at patchset-based candidate-world promotion. That path
has been pruned. The remaining frontier is the durable memory/control pattern:
background candidates mutate isolated state, publish typed evidence, and only a
verified AppChangePackage/adoption transition can change active state.

The practical next frontier is a narrow Choir-in-Choir demo where Choir changes
Choir through a background candidate path, then publishes, adopts, verifies,
promotes, and can roll back the result with durable evidence.

## Candidate Worlds

Candidate worlds should be first-class records, not just spawned processes. Each candidate needs:

- owner ID, foreground desktop ID, parent run ID, candidate run ID, VM ID, and purpose;
- base repo SHA and base artifact graph checkpoint;
- branch/worktree identity;
- lease deadline and authority profile;
- package publication/adoption path;
- verification contract and results;
- promotion decision and rollback point.

The governing rule stays the same: foreground is stable, background mutates, canonical state changes only by promotion.

## Git And Rollback Geometry

Use branch-per-candidate-VM as the default mental model. A VM can have a local
worktree or clone, but its product object should be a typed package/adoption
delta with:

- `base_sha`;
- `candidate_head_sha`;
- dirty-state manifest;
- files changed;
- commands run;
- tests passed/failed;
- uncommitted files, if any, explicitly named;
- package/adoption verification instructions.

Rollback should be simple because canonical state is not mutated until promotion. Before promotion, rollback means discard or archive the candidate. After promotion, rollback is a normal git revert or reset to the recorded promotion parent, depending on whether the delta landed as a commit or patch application.

User divergence is handled by treating foreground user changes as canonical
input, not as merge noise. If the user changes the same file while a candidate
is working, super should not blindly apply the candidate delta. It should route
the package through recipient adoption, merge/rebuild in the candidate
computer, and run verification there.

## Promotion Protocol

The minimal protocol:

1. Super creates a candidate-world record and requests a background VM.
2. Vsuper mutates only inside that VM and publishes an AppChangePackage.
3. Super routes the package into recipient adoption.
4. Verification contracts run against the integrated candidate.
5. Appagents accept semantic artifact changes when relevant.
6. Super promotes the verified computational delta into canonical state.

Promotion should be non-blocking and owner-mediated. Workers produce candidates. Super integrates. Appagents accept semantic changes. Canonical state stays single-writer or owner-mediated.

## Verifier Contracts

Do not create a verifier caste. Define verifier contracts:

- target;
- purpose;
- invariants;
- required checks;
- capability profile;
- independence requirement;
- result schema;
- evidence paths.

For coding candidates, the initial contract should require `go test ./...` with local ICU flags when Go code is touched, frontend build when frontend code is touched, and focused tests for the claimed behavior. For UI candidates, add Browser/Playwright screenshots and interaction proof.

## Skills-Native Super And Cosuper

Super and cosuper should support skills natively because skills are compact operational policy. The useful primitive is not "agent has a prompt"; it is "agent has a bounded capability profile plus reusable procedures."

Run memory summaries should include active skill context:

- skill name and version/path;
- why it was invoked;
- constraints it contributed;
- unresolved obligations from the skill;
- whether a later worker can continue without rereading the full transcript.

This is how mission-gradient becomes an internal control primitive rather than a Codex-only wrapper.

## Choir-In-Choir Feasibility

A narrow demo is feasible next if it avoids pretending to solve full self-hosting. The demo should be:

- foreground super starts a background VM;
- background co-super makes a small Choir improvement;
- worker publishes a typed change package with evidence;
- super routes it through recipient adoption;
- verifier contract runs;
- user reviews promotion report;
- canonical repo changes only after explicit promotion.

The first target should be a small product-visible improvement with meaningful tests. The best wedge is probably the launcher/uploads/themes cluster because it is user-visible and bounded. The second-best wedge is podcast/radio, because it aligns with vtext as semantic substrate but risks scope growth. Browser backend/Obscura is important but has more infrastructure uncertainty.

Historical note: the original recommendation was patchset-based candidate-world
promotion. That direction was pruned. The carried-forward lesson is stable
foreground state, candidate mutation, independent verification, owner review,
promotion, and rollback through typed AppChangePackage/adoption records.

## Next Goal

`/goal Use MissionGradient to execute docs/mission-campaign-compiler-selfdev-v0.md end to end, proving a typed AppChangePackage/adoption loop before widening to longer Choir-in-Choir campaigns.`
