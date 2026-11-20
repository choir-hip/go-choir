# The takes page in Exact was sent back three times; the approach to old data is changed

Date: 2026-11-20, recorded 19:45Z by the lead. Lane `exact-takes`, last
commit `b298cc0f4`. Nothing of it is on main. Claim C179 (the second review
accepts, p 0.4) settled false.

## Problem

Moving the takes page from Swift to shared Exact code has to carry a
listener's own take ids, handle and muted voices from two old native files
into the shared store. Three versions were reviewed by seat 2. Each was sent
back. From the second round on, every blocking finding is the same class:
state that belongs to one account reaches the next account on the same
phone.

## Evidence

- Review 1 (`e9d8ba6a9`): a discarded take could still be published (closed
  in round 2 and verified closed by the reviewer); the in-memory takes state
  survived a sign-out.
- Review 2 (`3ca74e7ae`): the live native take survived a sign-out; the
  carry-over of the old files could run again for a later account after a
  restart.
- Review 3 (`b298cc0f4`): the marker that makes the carry-over run once is
  written only after the first carry-over, so a phone that upgrades while
  signed out, or whose marker write fails, still hands account A's files to
  account B. Also: a publish can discard the next account's take when its
  queued write is skipped; the proof script's takes step fails on a rerun
  because an earlier step writes the marker (the queue's proof run at
  19:14Z failed at that step, as the review predicted from reading).
- Reviews are in the session scratchpad, `review-takes/r1` to `r3`. The
  reviewer checked the earlier repairs by breaking each and watching its
  test fail, so the closed findings are closed.

## Belief

The defect is not in the patches. The old files carry no proof of whose
they are, and the native delete that should remove them at sign-out can
fail without a sign. Every guard so far (a flag in memory, then a marker on
disk) is a statement about the phone's history, and each has a window where
the history is not what the guard assumes. Three rounds found three such
windows.

The change of approach (rule 7): stop inferring ownership from history and
prove it. The old own-ids file names a handle. The server names the
signed-in account's handle. The old files are adopted only when the two are
equal, in one step, and are deleted unadopted otherwise. No marker, no
"carried" flag, no per-process state. An account with no published take has
no handle on the server, so its muted voices are not carried: the listener
mutes them again. That loss is chosen over the leak.

## Remaining error

- I have not verified that the server's answer always carries the handle
  for an account that has takes, nor that a handle cannot be shared by two
  accounts. The builder is to read the server code and say.
- How many listeners have muted voices and no take is not known. On the
  owner's phone it is the owner's to notice.
- The native programme's copies of own ids and mutes are pushed from shared
  code. Between a restart and the first push they are whatever was left.
  The reviewer rated this a note (a wrong filter for a few programme
  requests; no voice published, no take lost).
- The double tap on Record that leaks a live recorder is pre-existing voice
  code, recorded in the lane and not fixed there.

## Next

One build on the changed approach, one review. If that review sends it
back, the lane stops and the question goes to the owner, as the Texture
editor did.
