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
rebase apply goes through a planned restart. Every reconcile whose result admits
a cast (`Result.Refusal() == ""`; pending and unavailable states do not count)
commits one system Note into each registered Texture inbox (live or settled
trajectory, not cancelled or archived) and removes the entry after the note
commits. The first reconcile of the process, of any reason, first runs the boot
pass: a crash boot (no planned-restart marker) deletes the entries registered
before the boot and wakes no one; a planned boot keeps them for the first
admitting reconcile. A registration that finds the last reconcile already
admitting wakes at once. The note id includes the clearing projection, so a
later wake is not swallowed by an earlier one.
