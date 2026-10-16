# Texture opens stories blind to the capture it was cast, and a story can stall for good (2026-10-16)

Status: documented. Fix to follow.

Found about 10:25 UTC during the hourly V1 check. Two paper stories held
only their seed: `80e6896d` (The Register, school staff and ransomware,
opened 07:34) and `c5aaaf86` (Wired, Jabra hearing aids, opened 10:05).
At 08:25 the handoff called `80e6896d` "legitimately waiting". Its
trace says otherwise.

## Evidence (run traces, the paper)

**What Texture is given.** Management opens a story with a Cast to
`texture:new` that names one capture. The frame lists it as a source with
only its capture id:

```
Sources:[{Handle:r1.c1 URL: CaptureID:obj:choir.web_capture:…:key-01dae7c0… Title: Excerpt:}]
```

`texturewrite.RecordsFrom` (`internal/texturewrite/frame.go:314`) gives
an opening Cast's captures a handle and a capture id, nothing else. The
frame renders them as `r1.c1 capture <id> (captured)`. Texture's surface
has no capture reader. `choir.Help("Read")` answers "There is no
choir.Read for this desk", and `ReadCapture` is on management,
management tasks and research only (`internal/yaegikernel/choir.go:328-355`).
Texture's own Write reads the capture's blocks to check quotes
(`rt.TextureCaptures()`), but the cell cannot.

**How often Texture goes looking.** These are the 78 first Texture runs
of stories opened on a capture, from 07:12 to 10:24:

| Behavior in the opening run | Runs |
|---|---|
| Looked for a reader (`Help("Read")`, `"ReadCapture"`, `"ReadSource"`, `"Fetch"`…) | 44 (56%) |
| Asked management for the passage | 20 |
| Cast research to verify (research returns a packet with excerpts) | 36 |
| Transcluded the capture whole (`{{embed r1.c1}}`) | 24 |
| Write refused: quote "not found in the capture of r1.c1" (it quoted the headline it was given) | 7 |

Most stories recover. Texture casts research, research returns sources
with excerpts, and the story is written from those. The capture the
story was opened on never gets quoted.

**How `80e6896d` stalled.** It had five runs, 07:34:32 to 07:37:30, then
none:
1. A Write quoting the headline was refused: not in the capture. Texture
   then Asked management for "the exact passage from leased capture r1.c1"
   and waited.
2. Management answered that `r1.c1` "is a lease-local alias, not an
   accepted capture ID for choir.ReadCapture". `r1.c1` is Texture's
   frame handle and means nothing on management's side. Texture asked
   again, and again (runs 2-4). Run 4 also tried `Ask("conductor", …)`,
   which was refused: Texture dispatches only to research or management.
3. Run 5: a Cast to management was refused, "round 1 is open and cast by
   management … inbox record refused". The next cell committed a bare
   `Wait` with nothing staged beside it.

That Wait is the last act. No Ask or Cast from this document is
outstanding, so nothing will wake it, and management's round 1 stays
open. The story is still at its seed three hours later.

`c5aaaf86` is at step 1: its only run Asked management for "the captured
Wired review's exact passage" and waited (10:05). Whether it recovers
depends on management's reply.

## Classification and clustering

This is substrate: the Texture desk surface lacks the evidence its own
opening record names. It is the third Texture stall symptom documented
this week, which triggers the clustering rule:

| Symptom | Doc | Shared cause |
|---|---|---|
| Done after a refused Write closes the turn at the seed | `texture-done-after-refused-write-stalls-story-2026-10-16.md` (fixed, 1e13ab348) | a turn can close leaving nothing that will revise or wake the document |
| Copied example handle `r1.c1`; one pSupported per citation | same doc (surface wording fixed, 7869cdef3) | the model is guessing at a surface it cannot see into |
| Opened blind: quote refusals, Asks management cannot answer, a bare Wait | this doc | both of the above |

Two causes underlie all three:
1. **Texture cannot read what it was asked to write about.** The design
   already names the fix and it never landed:
   `docs/autopaper-app-design-2026-10-11.md` §4.4, "Needed on the desk
   surface: `ReadCapture` on Texture and management (read-only, shared
   content)", slice C. Management got it; Texture did not. Texture
   already embeds captures and its Write already reads them, so reading
   them in a cell grants no world authority. This is the existing
   implementation to connect.
2. **A turn may close with nothing that will wake the document.** The
   Done fix covers the case inside one cell. A bare Wait in a later cell,
   after an Ask or Cast was refused, is the same stall. The Wait summary
   says "pair it with the Cast, Ask or Note that asks for the evidence",
   but nothing enforces it.

## Fix direction

1. **`ReadCapture` on Texture.** Read-only, same contract as the other
   desks. It also accepts the frame handle (`r1.c1`) so the model does
   not have to copy a long capture id, and the opening frame line points
   at it. This should remove most blind Asks and headline quotes, and
   the cells spent looking for a reader.
2. **A Wait must leave the document waiting on something.** A Wait is
   refused unless a cell's tray also stages an Ask or Cast (or a Note to
   research or management), or the document already has an outstanding
   Ask or Cast. The refusal names the alternatives: Ask or Cast for the
   evidence, Reject the record you cannot answer, or Write what is
   supported. Whether the outstanding-request check can be made from the
   reducer is to be established from the code before this lands.
   Otherwise this part is deferred and named as a residual.

Ways the fix could fail (to test first):
- ReadCapture on Texture resolves a handle to a different record's
  capture, or reads a capture from a writer outside the trusted set
  (the publish path's trust rule, §4.4).
- A legitimate Wait is refused: a follow-up wake while a research cast
  is still running, with nothing new to ask.
- A refused Wait leaves the model with no legal act and the run loops:
  the refusal must name Reject and Write as ways out.

Measure after deploy (paper, 3h): opening runs that look for a reader
(56% before), quote refusals on the opening capture (7 of 78 before),
Asks to management naming a frame handle, and seed-only stories older
than 30 minutes.

## Not fixed here

- `80e6896d` stays stalled. The paper is the owner's live computer, and
  this work does not act on its documents.
- **The desk jail root is the guest's whole /tmp when a run has no
  working directory.** `deskWorkerRoot` falls back to `os.TempDir()`
  (`internal/agentcore/tools_desk.go:788`), and `ListDir`/`ReadFile`
  resolve under it. One Texture opening run (`f1a2ace0`, 09:02) listed
  `ListDir(".")` and got the guest's `/tmp` (`systemd-private-…`,
  `.X11-unix`). This is a separate residual (broker jail width), not part
  of this fix.
