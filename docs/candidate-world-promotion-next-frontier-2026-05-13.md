# Candidate World Promotion Next Frontier

Date: 2026-05-13

## Position

Candidate-world promotion v0 now exists as a tested library boundary. The next frontier is turning it into the product path that super can actually use after `delegate_worker_vm` returns exported patchsets.

The missing object is a platform-side promotion queue: exported candidate worlds become reviewable promotion records, verifier contracts can run on demand, and the user can accept or reject a verified delta.

## Next Real System

Build promotion queue v0:

- super receives exported patchset metadata from a background VM;
- platform stores a candidate promotion record;
- user can inspect candidate, verifier contracts, report, rollback command, and changed files;
- platform can run verification on an integration branch;
- only verified candidates expose a promote action;
- foreground divergence blocks promotion and explains the merge/reverify path.

## Product Wedge

The launcher/uploads/themes cluster is still the right wedge. It is visible, bounded, and directly tied to onboarding. A good next Choir-in-Choir patch would be:

- start-button style app launcher;
- desktop app icons;
- file upload UI inside Files;
- theme creation/editing scaffolding that can later be user-promoted.

The promotion queue should be built first, then used to land one of those patches through the product path.

## Next Goal

`/goal Use MissionGradient to execute docs/mission-promotion-queue-v0.md end to end, wiring candidate-world promotion into a super/platform review queue and dogfooding one launcher/uploads/themes patch through verified promotion.`
