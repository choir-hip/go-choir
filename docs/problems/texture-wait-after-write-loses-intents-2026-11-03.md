# Texture: Wait after Write errors, and a panic on the error loses the cell

Date: 2026-11-03. Class: green (this doc). Source: docs/evidence/selfdev-trace-review-synthesis-2026-11-03.md, item 4.

## Evidence

At 17:41:27Z the pool arm's Texture cell ran choir.Cast, choir.Write,
choir.Wait, each with `if err != nil { panic(err) }`. Wait after Write returned
"this cell already called Write; Wait is a turn without a revision"
(internal/yaegikernel/choir.go, ChoirScope.Wait). The panic rolled the cell
back, so the Cast and the Write never committed (`intents:0`). The model ended
with prose saying it had dispatched, and the owner revision's trigger was then
refused as "trigger not disposed" (17:42:03Z). The same error appeared in 7
Slides cells (5 harmless, since they did not panic). The same trap hit at
11:29.

## Cause

Intended exclusion, incidental trap. A turn either Writes or Waits (a Wait is
a turn without a revision), so Wait after Write is a contradiction, and the
error is correct as a statement. But two things teach the model to call it:
the Cast help says a Cast "commits as a Wait", and the Texture prompt says
"Close each turn in the same cell as your Write", listing Wait among the
closers. The error then reads as fatal and the model panics, which discards
the other staged acts.

## Fix chosen

A Write already disposes the turn, so a Wait after it asks for nothing the
Write does not already give. Wait after Write becomes a no-op that returns
without error and says so. Write after Wait stays refused (the Write would be
lost silently otherwise). Help and prompt text say the order rule.
