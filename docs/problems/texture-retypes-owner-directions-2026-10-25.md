# Texture retypes owner directions to cast them, and the retype can paraphrase

Found 2026-10-25 in trace review round 6
([selfdev-trace-reviews-2026-10-25.md](../evidence/selfdev-trace-reviews-2026-10-25.md),
R6-1). Status: documented, unfixed. Mutation class of a fix: orange (desk
module surface), behind no switch needed if additive.

## What happened

On b@c, trajectory a7257e6b (doc d9e8b26f), FP0 revised the document at
08:47 UTC with an owner direction that ended "Cast this direction to
management once, as written". Texture's run 30f8ec82 needed three cells:

- 08:47:31 and 08:47:38: compile errors, `expected ';', found ...`. The
  backticks were in Texture's own markdown: a code span (`/read.html`) in
  the document text it passed to `choir.Write` inside a Go raw string,
  which a backtick ends. The owner direction itself had no backticks.
- 08:47:45: the cell compiled and cast `ownerDirection`, a variable Texture
  had retyped by hand: "Owner direction 08:47 UTC: ... 3) Signed-out
  preview: separate paragraphs and headings in sample PDF ...". The
  original point 3 named the extractor rule and the function to use; the
  retype dropped both.

Across the trajectory, Texture hit five raw-string compile errors (08:17:37,
08:18:01, 08:47:31, 08:47:38, 09:11:14) and management one (09:07:07).

In this instance the dropped specifics were FP0's own flawed prescription,
so the rejected round that followed is not attributable to the paraphrase.
The defect is fidelity: a direction marked "as written" did not reach
engineering as written, and nothing in the trace flags the difference.

## Why it happens (substrate)

Two causes meet here. First, a cell carries text as Go string literals, and
markdown routinely contains backticks, so any document text with a code
span breaks a raw string (`choir.Write` and `choir.Cast` alike). Second,
the owner's revision text reaches Texture only in its prompt. The in-cell
surface (`internal/yaegikernel/surface.go`) has no accessor for the
triggering owner revision, so the only way to put the owner's words into a
`choir.Cast` spec is to type them into Go source. Any text containing a
backtick cannot sit in a raw string, and a model retrying under a compile
error shortens and paraphrases.

This is a substrate problem in the desk module surface, not a prompt
problem: asking Texture to escape strings more carefully would treat the
symptom.

## Fix shapes (not chosen)

1. An accessor, for example `choir.OwnerInput()`, that returns the
   triggering owner revision's text and id, so a cell casts it without
   retyping. For example, `choir.Cast("management", objective, map[string]any{"owner":
   choir.OwnerInput()})`.
2. A spec field the runtime expands: a cast whose spec names
   `owner_revision: <id>` gets the revision's text attached verbatim by the
   runtime, and the receiving desk sees both.

Either one lets the trace show that the owner's text arrived unchanged.
Shape 2 also lets management and engineering cite the revision id.

3. For the raw-string half: the desk prompt's cell guidance names the
   backtick rule (use an interpreted string, or concatenate a backtick),
   or the surface takes text through a file the cell writes. This half is
   a prompt-level mitigation unless the surface changes.

## Meanwhile

FP0 keeps directions short and code-span-free, which does not touch
Texture's own markdown.

## Evidence

- Trace export: b@c, trajectory a7257e6b-1e62-56ab-93cd-8a2080982d5f,
  exported 2026-10-25 09:16 UTC; run 30f8ec82 cells at 08:47:31, 08:47:38
  (diag compile) and 08:47:45 (the cast, receipts rlm:cast:0 and
  rlm:texture_turn:0).
- The direction as sent: `/Users/wiz/Dev/lanes/_runs/reader/` (request and
  reject briefs; the 08:47 direction is in FP0's session log).
