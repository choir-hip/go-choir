# The Texture supervision document stops at the decision boundary (2026-10-24)

Date: 2026-10-24, 04:4x UTC. Found by: the FP0 app-rewrite conductor, on the
test computer computer-6c50d6e5 (b@c.com), staging `0cbc52edc`. Run log:
`/Users/wiz/Dev/lanes/_runs/apprewrite/log.md` (outside the repo). Class of
this record: green. Class of a fix: orange (Texture desk wakes, the
self-development reconciler's observations). Status: problem recorded, not
fixed.

## The owner's requirement

Owner, 2026-10-24: "i expect self-dev to lead to live updated texture
supervision versioned documents. that i can view by opening the choir desktop
signed in at b@c.com ... i should be able to see the progress not just in the
desktop but in the texture docs".

Each app request went in through the prompt bar (`choir run start`, which is
`POST /api/prompt-bar`), so the request is a Texture document that Texture
owns. The request text asked Texture to keep the document as a live progress
record: the cast, engineering's progress, the build, freeze, verify,
awaiting approval, the owner's decision, applied.

## What happened (trace evidence)

From `choir trajectory <id>` (reducer events) and `choir texture history
<doc>`, read through product routes.

Slides attempt 1. Trajectory `dd644b24-ead1-5a51-94ed-b25adf17ece6`, document
`39f4d957-df99-5bbf-8951-63c2df9b61f0`.

| UTC | Reducer event | Texture revision |
|---|---|---|
| 04:11:14 | trajectory_started | rev 1 (user: the request) |
| 04:11:25 | texture_turn_committed ("Acknowledged ... plan") | rev 2 (appagent: plan) |
| 04:11:31 | co_super_assignment_opened (engineering) | none |
| 04:14:28 to 04:14:31 | capsule frozen, revoked; assignment reported | none |
| 04:15:07 | texture_turn_committed + **work_settled** ("verification ... owner decision remains pending") | rev 3 (appagent: "frozen and verified; owner decision is pending") |
| ~04:15:30 | operation selfdev-186e2bf3 awaiting_approval | none |
| 04:16:11 | owner **rejects** the candidate (decision_event 09861ddd…) | **none**: the document still says "owner decision is pending" |

Attempt 2 was an owner `texture revise` of the same document at 04:16:50. It
opened a new work item, and Texture wrote revisions at 04:17:33 (the
rejection and the new plan) and 04:20:04 (frozen and verified). Then the
reducer wrote **trajectory_settled at 04:20:05**. At 04:22:28 the owner's next
`texture revise` of that document was refused: `409 {"error":"durable
lifecycle state is unavailable or terminal"}`. The third attempt had to start
a new document (`b4f90909-3bad-541a-aec7-ace1fc8e5558`).

Attempt 3 followed the same shape: rev 1 04:22:44, rev 2 04:23:10, rev 3
04:25:23, with work_settled at the same instant. Approved at 04:27:04,
materializing at 04:27:13, applied at 04:27:21 (the served SPA carries the new
Slides chunk). Read at 05:04: the document still has exactly the three
revisions of 04:22:44, 04:23:10 and 04:25:23, and still reads "awaiting the
owner's decision". Nothing followed the approval or the apply.

## The three gaps

1. **Nothing after awaiting_approval reaches Texture.** The work item settles
   when management's verification report lands. The owner's decision
   (approve or reject), the host build, materializing, applied, a failed apply
   and a restore are not reported to Texture, so it never revises the document
   for them. Code: the self-development reconciler mints one observation at
   the decision boundary,
   `notifySelfDevelopmentDecisionBoundary`
   (`internal/agentcore/self_development_materializer.go`), addressed to
   **management** (`persistentManagementAgentID`). Nothing is minted for later
   states, and nothing addresses Texture. **[V: code read; trace above]**
2. **A settled trajectory cannot take another owner revision.** Once
   `trajectory_settled` is written, `texture revise` answers 409. The owner
   cannot continue the same app's story (a reject, then "do it again") in one
   document; each attempt is a new document, and the history is split.
   **[V: 409 above]**
3. **No revision during engineering.** Between the cast and the frozen report
   (2 to 3 minutes tonight, but 9 to 17 minutes for a Go change), the document
   says nothing. Engineering's job starts, Freeze and capsule revocation are
   reducer events but do not wake Texture. Tonight's rounds were short, so
   this matters less than 1 and 2. **[V: trace]**

## What the owner sees today

A document that reads "verified, awaiting the owner's decision" forever,
even after the candidate is rejected or applied. To see the outcome the
owner has to use the CLI (`choir self-dev list`), not the document.

## What a fix has to do (not decided; failure modes first)

- Route the operation's post-boundary transitions (decided, materializing,
  applied, failed, rolled back, restored) to the Texture desk that owns the
  request's document, as records Texture turns into revisions. One candidate
  is to extend the reconciler's boundary observation to later states and
  address the trajectory's Texture agent as well as management.
- Decide whether `work_settled` at awaiting_approval is right, or whether the
  work item should stay open until the operation is terminal.
- Failure modes: a wake for a trajectory that has settled (2); duplicate
  revisions on reconciler re-passes (use deterministic record ids, as the
  boundary observation does); an apply restart between the transition and the
  wake (the restart rule: a crash closes, a planned restart resumes).
