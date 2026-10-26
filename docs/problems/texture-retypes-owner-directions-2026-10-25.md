# Texture retypes owner directions to cast them, and the retype can paraphrase

Found 2026-10-25 in trace review round 6
([selfdev-trace-reviews-2026-10-25.md](../evidence/selfdev-trace-reviews-2026-10-25.md),
R6-1). Status: fix chosen (below). Mutation class of a fix: red (the
Texture cast path), additive, behind no switch.

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

## Chosen fix (2026-10-25, lane sd07-owner-verbatim)

Shape 2, with shape 1's id. The owner's ruling (06:3x, "fix what we have:
Texture should hold the trajectory and prioritize the user intention")
favours closing the cast path over a new projection. Mutation class red
(Texture cast path); the residual register carries it as SD-07.

- **The cell names, the runtime attaches.** A Texture Cast's packet names
  owner revisions by id: `"owner_revision": id` or a list of ids, at most
  eight. At commit the reducer reads each revision from the lifecycle
  store and adds `owner_directions` to the packet: one entry per revision
  (`revision_id`, `kind` request or direction, `at`, `text`), the text
  byte for byte from the revision (its `owner_prompt`; for the document's
  first revision, its request). The cell never types the owner's words,
  so no backtick or retry can touch them.
- **The cell learns the id without typing it.** `choir.ReadDoc()` gains
  `OwnerRevision`: the revision that woke this run, when the owner wrote
  it. The request turn prints the revision id beside "The owner's
  direction for this revision" and the original request, as it already
  does for earlier directions.
- **Who sees it.** Management reads the Cast record's body, which now
  carries `owner_revision` and `owner_directions`. Engineering reads the
  same body through the request management's Cast is bound to
  (`engineeringRequestView`, digest-checked), so the owner's words reach
  the engineer without passing through management's typing. The record
  on the tape is the trace.
- **Authority check.** Only the Texture reducer attaches. Each named
  revision must load from this computer's lifecycle store under the
  run's owner, belong to this desk's document and the run's trajectory,
  be author kind `user`, and carry an owner direction (an owner edit with
  no `owner_prompt` is refused, as is any other revision). A refusal
  names the revision and the reason, and the cell commits nothing, like
  every reducer refusal. A packet that sets `owner_directions` itself is
  refused on every desk (`validateCellIntents`), so no cell can label its
  own text as the owner's.
- **Replay.** Revisions are immutable, so a replayed cell attaches the
  same bytes and the fold stays a function of its inputs.
- **Switch.** None: the change is additive and inert until a cast names
  `owner_revision`, which only the updated prompt teaches. An older
  reader of a committed record ignores the extra packet keys (unknown
  fields are tolerated; `castBodyFromPacket` reads only its own).
- **Rollback.** Revert the red commits (code and prompt together, one
  act). Records already committed keep `owner_directions` as an inert
  packet key.
- **Not in this fix.** The raw-string half (shape 3) for Texture's own
  markdown in `choir.Write`; management forwarding a mid-flight
  correction to the bound engineer, which still goes through a Note it
  types; engineering's 16 KiB bound on the request body
  (`engineeringRequestBodyMax`), which cuts a very long request and says
  so (`truncated`).

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
