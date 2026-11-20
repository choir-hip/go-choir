# The takes page in Exact was sent back three times; the approach to old data is changed

Date: 2026-11-20, recorded 19:41Z by the lead. Lane `exact-takes`, last
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

## The handle is not proof; a shared take id is (2026-11-20T19:42Z)

The builder read the server before building and stopped, as briefed. A
handle is "voice-" plus 24 bits of a hash of the user id
(`internal/types/autoradio_takes.go:35-38`), and the server's own comment
calls it display only: two accounts can share one
(`internal/platform/takes.go:141-142`). So the rule in "Belief" above would
have been a fourth guard with a window.

A take id is 64 bits of a hash over the account's publisher key and the
audio. The old own file holds ids the old build copied from the server's
list of the account's takes. The rule built instead: the old files are
adopted only when at least one id in the old own file is in the signed-in
account's own list from the server. The cost is the same as stated above
and a little wider: an account with no live take (never published, or all
deleted or withdrawn) does not carry its old mutes.

## The changed approach was sent back too; the lane is stopped for the owner (2026-11-20T20:18Z)

Lane `exact-takes`, last commit `c534f89c0`. Nothing of it is on main. Claim
C180 (this review accepts, p 0.5) settled false. Review in the session
scratchpad, `review-takes/r4`.

What the fourth review verified sound: the ownership proof for the ids
(the server's list holds only the caller's live takes; an id cannot be
another account's), the generation guards around the write and delete, the
order of the one-step adoption, the live take discarded at sign-out, and
the Swift, which it compiled and tested. The queue's proof run passed at
`c534f89c0` (20:12Z), driving only the refusing branch: d@e has no live
take, so the adopting branch has never run on a simulator.

The blocking finding: the proof is for the own-ids file and the adoption
takes the muted-voices file with it. They are two files with two silent
deletes. If account A's muted file outlives A's sign-out on the old build
and account B then publishes on the old build, the upgrade proves B owns
the ids file and hands B the mutes that were A's. The reviewer reproduced
the adoption. The reverse also holds: a current account whose old ids file
is someone else's loses its own mutes.

Belief. The muted-voices file has no owner written in it and the format
cannot carry one. No rule adopts it safely. Two choices remain, and they
are the owner's because one of them loses a listener's own settings:

1. Carry the old ids on proof and carry no old mutes. Every upgrading
   listener mutes again. No leak. (The reviewer's fix, and the lead's
   recommendation: the mute list is short and the listener can see it is
   empty.)
2. Carry the mutes with the proven ids and accept the residual: after an
   old-build sign-out whose delete failed, followed by a second account
   publishing on the old build, that second account inherits the first's
   mutes.

Five should-fixes stand either way (a previous account's publish can
unblock the next account's recorder and write its notice; the programme
copy pushed without a generation check; stale comments describing the
removed marker; test gaps; the proof script's check passes when the kept
file is missing).

Default taken: nothing lands; the lane is kept; no fifth round without the
owner's choice.
