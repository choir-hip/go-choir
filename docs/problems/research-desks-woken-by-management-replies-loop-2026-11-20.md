# Research desks are re-woken by management replies that say Relate refuses "central", which no ledger shows (2026-11-20)

The file name carries the account the paper's own records give. The reading
below is that the account is not what the run ledgers show; the loop it drives
is real.

## Problem

Management's replies to research desks say that every research desk is blocked
because `choir.Relate` is refused for `role="central"` by the v2 vocabulary
guard (the replies quote `vocabfence: og:choir.event:body.payload.attachments[0].role="central"
not in v2 vocabulary`), because the System 1 judge is unavailable, and because
some story heads have no text; that the finish rule
(`requireStoryEntitiesRecorded`, `internal/agentcore/inbox_records.go:718`)
therefore holds the desks' final Reports; and that this is a platform defect
management cannot repair. Each reply is a record in the desk's inbox. Each
unread record wakes a full research run.

Two waves woke research desks on 2026-11-20: 14:48Z (6 runs seen, all
searched) and 19:34Z to 19:36Z (21 runs seen; briefs logged `skipped: no
objective in the unread records` from 19:36Z to 19:45Z; the last ended 19:50Z).
Earlier reply-woken runs were read at 2026-11-19 18:37Z, 2026-11-20 03:14Z,
03:15Z, 06:49Z, 06:50Z, 09:06Z and 10:40Z.

## Evidence

Run ledgers (`GET /api/runs/{id}/events` on the paper, read key, read only),
390 runs from 2026-11-15T11:48Z to 2026-11-20T20:28Z, every cell source and
output searched for `vocabfence`, `vocab write guard`, `not in v2 vocabulary`
and `choir.Relate`:

- **No `choir.Relate` cell was refused by a vocabulary guard.** 0 of 390 runs.
- Relate returned `attached` with `"role":"central"`, 51 times, first
  2026-11-15T11:50:20Z (run 39e727b5), last 2026-11-20T20:08:07Z (1afe7b07).
  For example run dde7058f at 2026-11-15T14:29:19Z recorded three central
  entities 15 minutes after the management reply (seq 1512) that says the
  guard rejects central; d98df616 at 2026-11-17T07:02:04Z, `exported:true`;
  922c0f64 at 2026-11-19T00:23:46Z; c75555cd at 2026-11-20T14:19:06Z.
- The tape writes `entity_role`: 920 attachments in 190 `story.entities`
  `recorded` events in 187 runs on 2026-11-20, none with a field `role`, the
  latest at 20:28:07Z. Code: `internal/agentcore/story_entities_verb.go:83`;
  the guard walks leaves named `role`, `profile`, ... and takes the last path
  segment (`internal/store/vocab_migrate_og.go:827`, `:853`, `:964`;
  `internal/vocabmigrate/fence.go:106`), so `entity_role` passes. It is a
  structural walk, not a per-computer vocabulary record; the only per-computer
  state is `vocabCutover`. The refusal was real once: 2026-10-26, fixed by
  `d51ae036d` (2026-10-27 00:07 EDT), pinned by `010259229`
  (`internal/store/story_entities_vocab_fence_test.go`), recorded in
  `docs/problems/related-stories-not-showing-on-the-paper-2026-10-27.md`, which
  holds the exact error text management quotes. The ledger of 2026-11-08
  (`173aa80fb`) had already falsified the same hypothesis once ("research run
  f3e144b7 recorded central and incidental entities through choir.Relate").
- Relate failures that did occur are the host's argument checks answered to the
  model: `entity_id ... must be ent:<24 hex> or wd:Q<n>` (13:38Z, 14:18Z),
  `judgment_ref must be the receipt_ref`, `no System 1 answer with receipt`,
  `System 1 did not rank central first ... give a reason`, `"Ukraine" is not
  among the analysis's candidates ...` (2026-11-20 09:10Z); each was followed
  in the same run by a call that succeeded.
- The finish rule held no Report: of 284 runs with a `story.entities` event, 236
  (`recorded`) and 47 (`analysis_unanswered`, error "the story's head has no
  text") sent a final ReportPacket; 1 `recorded` run did not; 0 show the rule's
  refusal text. The 27 management-reply runs wrote no `story.entities` event and
  made no Relate call (their stories were recorded in earlier runs).
- Management's reply text is the only place the guard's refusal is named
  after 2026-10-27; the first reply read is seq 1512, seen by run dde7058f at
  2026-11-15T14:28:46Z. Desks then save notes ("Relate vocabulary guard rejects
  'central'", evidence 53f1debf, 2026-11-16T15:17:19Z; `platform_status` notes
  on desk 94226d5a) and read them back with `choir.ListEvidence`.
- The 27 management-reply runs (6 + 21): 26 delivered a ReportPacket; 639
  tool-loop iterations; about 265 minutes of run time; 31 web searches; 18.3
  million input tokens (12.3 million cached) and 0.48 million output tokens on
  `swe-2-flash`; 1 revision (dd22d999) and 0 publications in their windows.
- One real vocabulary refusal in the window, on another carrier: run a026d733,
  2026-11-20T14:05:07.711Z, `choir.Research(...)`: "the call's Cast could not be
  recorded, so the child did not run: vocab write guard: vocabfence:
  og:choir.commitment_record:body.subcall.role=\"research_web\" not in v2
  vocabulary". Once in 390 runs.
- One real stuck desk, on a different mechanism: run ae50d61b (desk
  `research:94226d5a`), 2026-11-20 19:47:22.034Z and 19:47:43.853Z, `ReportPacket`
  and `Report`: "disclosure refused: owner_private:autopaper (owner_private) at
  desk_message is ask; System 1's answer does not clear the rule, and publish-auto
  work never asks the owner, so this attempt is refused"
  (`internal/agentcore/disclosure_gate.go:517`, reached from
  `inbox_records.go:560`). The desk's notes say the same since 2026-11-19T14:33Z.
  The host's story publisher shows the same family: 10 story publishes refused
  between 19:55:10Z and 20:11:01Z for labels `owner_private:autopaper`,
  `computers`, `model-policy`, `prompt-bar`, `texture`.

## Belief

The account in management's replies is not true of the ledgers: Relate accepts
`central`, the tape field passes the guard, and no finish-rule refusal occurred.
The loop is real and is made of three parts that feed one another: a management
turn answers desk escalations with a platform-defect diagnosis (the text matches
the 2026-10-27 problem record); the reply, an unread record with no objective,
starts a full research run (`inboxUnread`, `inbox_records.go:969`;
`WakeRecordsInbox`, `:1036` and `:1093`) although the corpus brief already says
there is no objective (`research_brief.go:860`); and the desk saves a note that
repeats the premise and reads it back next time. It costs runs, tokens and 31
searches an observed day; it produced no publication. The real blockers are
different and smaller: stories whose head has no text (`analysis_unanswered`, 40
runs today, handled by the finish rule's third branch) and the disclosure gate's
labels (one desk found, 10 publishes in 16 minutes). I hold this at about 90%:
the weak point is that I read ledgers from 2026-11-15 only; a refusal before
that, or on a desk outside the 390 runs, is not excluded.

## Remaining error

- No Relate vocabulary refusal in any ledger since 2026-11-15T11:48Z. I did not
  read the management cells (seq 1512 to 1526) that wrote the replies, nor runs
  before 2026-11-15 11:48Z, nor the 14 other wave-1 desks.
- The source of the escalations management answers is not found: no desk cell in
  the window calls `choir.Escalate`.
- Whether the deployed guest equals origin/main (35a721615) for the files named
  was not read.
- The count of stories held by the disclosure gate is not known.

## Next

Not done here (this record writes no code):

1. Smallest code change, orange (runtime behaviour and routing; one review seat;
   rollback revert): a research desk's inbox with only management `reply` or
   `report` records that carry no objective and no cast does not start a run, and
   the cursor advances over them so `inboxUnread` does not stay true. Site:
   `WakeRecordsInbox` (`internal/agentcore/inbox_records.go:1093`) or
   `inboxUnread` (`:969`). Test first: a management reply with no objective in a
   research inbox starts no run and leaves unread 0. CI and
   `scripts/proofs/entity-e2-deployed.sh` could not have caught it (model turns;
   the proof tests the public entity read and the ready list).
2. The owner or lead decides on the data: the desks' `blocker` and
   `platform_status` evidence notes and management's consolidated escalation
   (disclosure 3adf5e4c) assert a defect the ledgers do not show.
3. A separate record, if the lead wants it: the disclosure gate's labels on
   `owner_private:autopaper` (desk 94226d5a, 10 refused publishes), which the
   gate's own message calls "a defect to fix at its labels".
4. Re-read: after 20:54Z no skipped brief appeared in the console (20:30:28Z to
   20:54:39Z); count the next wave when it comes.

## What the replies relate to (read 2026-11-20 21:50Z to 22:20Z)

Inbox records of 12 research desks (`GET /api/inboxes/research:<doc>/records`)
and the run records of the escalations' authors, read only. Every management
reply of the waves at 14:43Z and 19:33Z that carried related ids was read: 38
related ids on 11 desks, all alike.

| What the reply's related id is | Count |
| --- | --- |
| a `choir.Escalate` directive (`...:escalate:tray-N:packet`) written by a research run of the same desk | 38 of 38 |
| an Ask, or a record written by the system | 0 |
| written 1 to 25 days before the reply (2026-10-26 to 2026-11-19) | 38 of 38 |
| a record the desk had run past: a later record had been read before the reply, so the desk was not waiting | 38 of 38 |

Management answered the same open escalations three times today (06:44Z,
14:43Z, 19:33Z), from three of its own turns. The earlier note that no desk cell
calls `choir.Escalate` was true of the window read, not of the desks' history.

## Fix (lane/reply-wake-fix, orange, rollback revert)

A research desk is not woken for a management reply that has the quiet shape
(`store.IsQuietManagementReply`: an agent Reply from the management inbox to a
research inbox, not final, related ids, no Cast, directive, precommit, report
body or other typed body) when every record it relates to is either not one the
desk wrote, or one the desk wrote and has read past. "Read past" is: the
record was committed before the newest record at the desk's cursor, so a run
has read after it. A reply to an Ask or Escalate the desk wrote and has not
read past (it ended its turn waiting) wakes it; so does a reply whose related
record cannot be found or read. The judge is `staleReplyJudge`
(`internal/agentcore/inbox_records.go`), used by `WakeRecordsInbox` and by the
two post-turn re-checks, so the round-hold logic still has a loop coming for
every record it holds a round for. The store still mints the wake row.
`WakeRecordsInbox` logs `records inbox: wake declined desk=<desk> kind=reply
reason=management_reply_no_objective count=<n>`. The cursor does not move: the
reply stays unread, and the desk reads it in its next run. Not changed: the
disclosure gate, the desks' saved notes, the finish rule. Residual: a desk that
Asked, was woken by an unrelated record and ran, and is then answered, is not
woken by the answer (it has run past its Ask).
