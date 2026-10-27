# A capsule cannot restore a path from its base tree (2026-10-27)

Status: open. Mutation class of any fix: red (capsule substrate).
Documented before any fix.

## Evidence

On b@c (computer-6c50d6e5…), in round trajectory 150b7de5… (Texture
citations toggle, the owner's request, 00:07 UTC):

- **The change.** Engineering (assignment-e288fc8d…) made the change, and
  the frontend build passed.
- **The deletion.** Along the way it deleted an unrelated file,
  `frontend/tests/texture-source-entities.spec.js`. It reported that it
  could not recover the file inside its capsule.
- **00:28.** Management refused to freeze a candidate that contained the
  unexplained deletion, which was correct. It revoked the capsule.
- **00:29.** Management settled the round as waiting on the owner to
  "provide a recovery source/tree or explicitly revise the scope". That was
  wrong: the base tree is the recovery source.

## Cause

A capsule's workspace is an overlayfs: the computer's source tree is the
read-only lowerdir, and the engineer's changes go to the upperdir
(`internal/capsule/namespace.go:229`). Deleting a file writes a whiteout
in the upperdir. The original stays in the lowerdir, but nothing inside the
merged mount can reach it:

- the capsule has no `.git` (it is excluded from the carried patch);
- the capsule has no read-only view of the base;
- the capsule has no "restore this path" act.

Only the host side reads the base tree (`carried_patch.go:145`).

## Second failure: the orchestrator's watcher

FP0's watcher waited only for a self-development operation in
`awaiting_approval`. A round that ends blocked creates no operation, so
the watcher stayed silent for 40 minutes after the round had stopped. The
owner noticed first. The rule from now on: watch the round's trajectory
turns and capsule fates, not only the operation list.

## Fix candidates

1. **A read-only base view.** Mount the lowerdir read-only inside the
   capsule as well, at a fixed path such as `/workspace/.base/platform`, so
   `cp` restores a file. This is the smallest change.
2. **A capsule act.** Add `restore <path>`, which removes the whiteout and
   any upper copy for that path.
3. **A prompt rule.** Tell management: an accidental change in a capsule is
   recovered by a fresh capsule from the base, never by asking the owner.
   This is cheap, and it applies even after 1 or 2 land.

## Workaround used

FP0 revised the round document at 00:36 to continue in a fresh capsule,
which starts from the clean base, with a rule against deleting any file.
