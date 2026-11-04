# Textures whose Cast was refused at stack_base_moved are not woken when it clears (2026-11-04)

Evidence (staging): two rounds were refused engineering Casts with
`stack_base_moved`; the platform's U4 rebase then applied (claude arm 12:44:53,
selfdev-076fbae0; crouton arm 12:43:49, selfdev-52a24b31). Neither waiting
Texture resumed; each needed an owner note (12:48:11, 12:48:12) to re-Cast.

Cause: the refusal (`sourceProjectionCastGateFor`) is an error to the management
cell and leaves nothing durable; the rebase operation's apply wakes only the
rebase operation's own document.

Fix: the refused Cast registers its document in a file under the selfdev updater
root (`source-projection/base-waiters.json`, `stack_base_wake.go`), because the
rebase apply goes through a planned restart. A system Note (deterministic id) is
committed into each registered Texture inbox by (a) the live reconcile that moves
the stack off `stack_base_moved`, or (b) the boot reconcile of a planned restart
(marker present) that finds the stack off it. A crash boot wakes no one and
deletes the entries registered before that boot. Entries registered during the
boot are untouched.
