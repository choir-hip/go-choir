# Deployed proof of the self-development blocker fixes and the delegate gateway (2026-11-03)

Staging https://choir.news served eb1fb6408ab8437ff01053b93c0681818d0c2479 (guest image
s7gylyyw5s4k..., layered 8hd8ld9crqkix...). Metadata only; no keys, tokens or contents read.
Problem record: docs/problems/selfdev-round-blockers-cluster-2026-11-03.md. Mutation class of this doc: green.

## Result table

| # | Proof | Result |
|---|-------|--------|
| 1 | Expedite | PASS for the walk and for d@e, owner, paper, pool; claude, deepseek, luna deferred (hibernated) |
| 2 | Fix 10 rebase notice | PASS on pool, claude and deepseek |
| 3 | Fix 11 publish | PASS on pool |
| 4 | Fix 13 post-Freeze record | NOT EXERCISED (no apply, reject or fail since the deploy) |
| 5 | Delegate via gateway | (a) PASS; (b) and (c) NOT EXERCISED (no delegate run, no capsule) |

## 1. Expedite

Walk walk-s7gylyyw5s4k-1793702159, started 10:35:59.998Z, expedited by trailer eb1fb6408ab8
(caller ci.ring-expedite). All rings opened at 10:36:00Z. No breach: no ring outcome other than
clean, skipped_no_exposure (R3) and skipped_empty (R4); status complete at 10:46:02.66Z.

| Computer | Reached eb1fb6408 | Deploy-to-reach |
|---|---|---|
| d@e computer-59c902c5... | 10:38:00.25Z | 120 s (woke 10:37:00) |
| owner computer-03335285... (observe) | 10:37:00.31Z | 60 s |
| paper computer-1355b83b... (observe) | 10:37:00.31Z | 60 s |
| pool computer-4d38420c... | 10:41:01.00Z | 301 s (hibernated; cold boot 10:40:32Z, release s7gylyyw5s4k) |
| claude computer-a875023a... | not reached | deferral: hibernated since 09:58Z; guest-release.json still j43n6hj07w75... (2795b8ee); boots the new image at next wake (not yet shown) |
| deepseek computer-e80d229b... | not reached | deferral: hibernated since 07:35Z; same old release |
| luna computer-2bf3cab1... | not reached | deferral: hibernated; same old release |

The walk closed with only four members reached; the three arms are `last_off_target` 10:39:00Z in R1
and were never woken (I sent no request and did not start them). Note: the walk counts a hibernated
computer as satisfied at close, so "walk complete" does not mean those arms run eb1fb6408.

## 2. Fix 10 (pool PASS)

Pool boot: autoputer startup log 10:40:38Z. Console log (vm-04a4e430...):
`2026/11/03 10:45:39 rebase notice: minted rebase-notice:selfdev-c4ac7712467862d5b7cd527ed6287034:eb1fb6408ab8437ff01053b93c0681818d0c2479:1 (attempt 1)`
(306 s after boot; the previous boot logged `none minted: rebase_needs_root` at 07:12:49Z and 07:14:56Z).
Record: the stack operation's Texture document b1ad3952-5ab4-54e0-80ba-94d312f88385 (trajectory
5dc6a6ff-41d6-5ee6-9025-151c57bb5eff) logged `texture_turn_committed` at 10:45:41.95Z, 2 s after the mint,
in `choir lifecycle events`. The Texture revision history (`texture history`) had no new revision by then
(newest 8e1caca8, 07:19:36Z); the answering Cast is in flight or the turn made none. Not checked further.

Update 10:54Z. The coordinator started claude and deepseek at about 10:50Z (guest-release.json
s7gylyyw5s4k, launched 10:47:41Z and 10:47:50Z; startup logs 10:47:48Z and 10:47:56Z):
- claude: `10:52:48 rebase notice: minted rebase-notice:selfdev-40faf6ee056fec39eb534c07226ffb95:eb1fb6408ab8…:1 (attempt 1)`, 306 s after boot. Stack Texture ffb9e093-b523-567d-a48c-ce21357a802c (trajectory b48525f9-60e6-57ac-923e-a842ca7437b3): `texture_turn_committed` 10:52:56Z, `co_super_assignment_opened` 10:53:00Z, `co_super_assignment_bound` 10:53:05Z, so the notice's Cast reached management.
- deepseek: `10:52:57 rebase notice: minted rebase-notice:selfdev-574077b31eef7feb3d5ce023ad0499ff:eb1fb6408ab8…:1 (attempt 1)`, 307 s after boot. Texture ddbdd1ae-2ae1-5c2d-ab8f-89058f98d6f6 (trajectory 8048fb68-0be2-59b0-b8d8-8b83c8319d4c): turn committed 10:53:03Z, assignment opened 10:53:08Z, bound 10:53:13Z.
- Both operations are state applied (05:34Z and 06:12Z). No new Texture revision yet on either document (newest 07:16Z and 07:17Z); the follow-on assignment is running.
Proof 2 therefore PASSES on all three stacked arms.

## 3. Fix 11 (pool PASS)

`choir package publish --operation selfdev-c4ac7712467862d5b7cd527ed6287034 --package files`
refused, correctly: `409 package_change_not_confined` (the operation's package is files-next; the
first run used the wrong name). With `--package files-next` (no --test-command):
release_id 0f8733c219d90904c44a86fc7981698a302c0540e9408fa580b3a54bf2698601, published 10:44:56.54Z,
listing state listed/published, seq 1, track c63af0bc..., parent kind `base` commit 2795b8ee,
changed_paths 5 files under frontend/src/packages/files-next, built_on 2795b8ee. Lane C's 06:43Z
refusal was package_parent_unpublished; it no longer occurs.

## 4. Fix 13

Rechecked 10:54Z: still no apply, reject or fail on any arm since the deploy; the new assignments on claude and deepseek have not decided.
The only outcome on pool's Texture (b1ad3952) is the 06:43Z apply, before the fix; its newest revision
is 07:19:36Z. No arm has applied, rejected or failed anything since 10:36Z, and reading the hibernated
arms' Textures would wake them. NOT EXERCISED; recheck after lane C's queued requests settle.

## 5. Delegate via gateway (claude)

- (a) PASS. After claude's fresh boot (launched 10:47:41Z; credential.img written 10:47:54Z), the host-side
  names-only listing (`debugfs -R "ls -l /"`, no reads) is: computer-event-envelope (995 B), gateway-token (101 B).
  No claude-oauth-token and no claude-delegate.json. Baseline on the pre-boot disk had both (153 B and 109 B).
  The running vmctl unit no longer carries CHOIR_CLAUDE_DELEGATE_TOKEN_FILE or _EXPIRES (only _COMPUTERS, a grant list);
  the gateway unit carries GATEWAY_ANTHROPIC_OAUTH_TOKEN_FILE and a relay identity list that includes claude.
- (b) `journalctl -u go-choir-gateway` since 10:47Z has no `anthropic relay` line; no delegate run
  occurred. Not exercised, not forced.
- (c) No claude-mode capsule log exists on this boot. Not exercised.

## Needs a fix or a follow-up

1. Proof 4 and 5(b), 5(c) wait for a decision and a delegate run or claude-mode capsule on the arms; re-run then.
2. Luna is still hibernated on the old release.
3. The ring walk completed while hibernated arms held the old release; "complete" does not mean they run eb1fb6408.
