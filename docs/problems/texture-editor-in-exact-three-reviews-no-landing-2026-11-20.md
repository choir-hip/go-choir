# The Texture editor in Exact was sent back three times and is stopped for the owner

Date: 2026-11-20. Station X2 of the mission "Exact, search and entities".
Lane `exact-convo-edit`, last commit `4b58dba83`. Nothing of it is on main.
The phone still runs the Swift editor.

## Problem

Moving the Texture editor from Swift to shared Exact code changes how an
unsent edit is kept and posted. Three versions were reviewed; each was sent
back for a way the owner's document text could be lost or overwritten without
a sign. Rule 7 of the operating contract stops the patching here.

## Evidence

- Round one: a save could post over a newer head; the old build's unsent edit
  was not carried over.
- Round two: a merged save never reached the field, so the next keystroke
  reverted the other writer's change; text with no recorded base was posted
  over a newer head.
- Round three (review in the session scratchpad, `review-convo3`), two
  blocking findings:
  1. After a merged save the kept record holds the revision id from before
     the merge, not the field's text. After a relaunch the three-way merge
     uses the wrong common ground (`texture-edit.ts:397-398`, `263-267`,
     `375`).
  2. Reopening the editor seeds the field from the head, not from the kept
     text, and typing then overwrites the kept text
     (`members-content.contract:129`, `texture-edit.ts:301`).
- The real proof on the simulator passed each round. It does not exercise two
  writers, a relaunch after a merge, or a reopen with kept text.

## Belief

Each round fixed the named cases and left the model unstated. The editor
holds four texts (the field, the last text the server accepted from this
phone, the head, and text kept from an earlier launch) and the code keeps
them in fields whose meaning changes by path. A fourth patch would find a
fifth case. What is missing is one written state model (what is kept, under
what base, and which transitions are allowed) with a test per transition,
written before the code.

## Remaining error

- I have not checked whether the Swift editor on main has the same two
  defects; it may, since it posts whole text with no merge.
- The two conversation-row commits in the same lane (`c804a93e6`,
  `8924a32a3`) drew no blocking finding in any round. They could land alone
  after a rebase and a build; not done in this session.

## What the owner is asked

Whether the editor's move waits for a written state model (the lead's
recommendation), or the phone's editor stays in Swift for now and the other
pages move first.

Default taken while the owner sleeps: nothing lands; the lane is kept.
