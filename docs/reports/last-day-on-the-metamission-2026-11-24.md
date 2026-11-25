# The last day on the metamission

Written Tuesday evening, November twenty-fourth, about eight o'clock your time. It covers the twenty-four hours from Monday evening, and it covers the work of the earlier session that ran out of its weekly limit as well as mine since I picked it up.

The app's sign-up path is now real on the server, but the App Store submission is not ready tonight, and it will miss the target of about nine thirty tonight.

## Where things stand

Main took about two hundred commits in the day. The biggest change is that making an account now works the way you asked. The phone and the website both email an eight-digit code first, and an account exists only once the address is proved and the first passkey is made, in one step. Sign-out on the phone clears it at once and takes the key back from the server. Sign-up is now limited by the room on the server's disk, and an account without a computer no longer counts against that room. The phone's first run, its consent screen, the store build and the move of more phone pages into shared code also landed.

Four things still stand between you and an App Review submission. In-app account deletion is not landed. Apple requires it, and it is in review. The reviewer's sign-in is approved and queued. The new privacy policy is approved and queued behind the Z.AI removal. The voice setup work, which decides which speech models the phone recommends, is not landed either.

What I would ask of you is small. Read section nine of the new privacy policy, about deleting an account, when it goes live, because for now it promises that you will do deletions by hand when someone writes in.

## Monday night: the proof of an address

The earlier session spent most of Monday night on accounts. The problem it was fixing was that the passkey screen let someone claim an email address they did not own. The fix was a rule. An account is made only when the code from the email has been typed and the first passkey is saved, both in the same database transaction. Every way of using an account, whether a token, a key, a refresh, or a device approval, now checks that rule where the credential is checked. A long series of tests drove each route with an unproved account and saw it refused.

The same night took apart a smaller problem with sign-out. A phone that signed out while offline used to keep a key the server still honoured. Now the phone records which key it held, forgets it at once, and keeps asking the server to revoke it until the server answers.

The lesson there is that the review seats kept finding a new way around each fix until the rule moved to the one place every credential passes through. That is the same pattern the store build guard showed. It needed three rounds before the guard checked what was actually being uploaded, rather than the command that made it.

## Tuesday morning: the server's room and the leftovers

Early Tuesday, the sign-up limit landed. A new account's computer is four gigabytes, and the decision to admit someone is made from the real free disk, the reserve, and what admitted accounts will still take. In the afternoon a follow-up found that accounts which never got a computer were still holding room, which had closed sign-up on staging. That fix landed and is live.

The disk-leftovers work also landed in the early morning. It is the tool that deletes the stored history of computers that have been removed. It is built to refuse rather than guess. It takes an exact list of computers, proves each one is dead under a lock, marks it, and only then deletes. Any failure stops the whole run. It is live on the server, but it has not been run against anything yet. The list of five removed computers is still waiting for you to look at.

Two investigations that morning are worth your ear. One found that the paper has been quietly losing captures. Some aged out of the queue undecided, and about two percent a day were left behind by management tasks that closed without deciding them. That belongs to the capture desk design, which must not let a capture leave the queue without a decision. The other found that the phone's voice path is not the one you chose weeks ago. Fluid Speech Mini was selected in October, but no plan ever scheduled it, and a stopgap default stood for nine days. The voice model work is the fix, and it is in review.

## Tuesday afternoon: the spacer, the corpus, and the reviewer account

The forty-calls-a-minute spacer you objected to is gone, along with the switch that could bring it back. In its place, the gateway now tells the computer the truth when a provider refuses. The computer waits out the provider's own retry time once, instead of retrying blind.

The research corpus now reaches the research desk as values inside its working cell, rather than through a tool call. It landed tonight after about ninety minutes of checks, mostly a race test of the largest package. Its tests are running on main now.

The reviewer's sign-in for Apple took ten rounds of review. It is a user name and password for one account named in a file on the server. Typed into the ordinary email field without an at sign, it shows a password field. There is no link that would confuse your testers. The last review found only two wording errors, and both are fixed.

## This evening: Z.AI, the policy, and the queue

You asked me to take Z.AI out of the code. It is gone from the gateway, the model list, and the deploy script. The gateway's journal showed no Z.AI calls at all, and the other GLM model, which runs through OpenCode, still works. One review seat accepted it after three small fixes.

The new privacy policy had been sent back by its review with three real errors. It said the app asks for no codes, which is false. It left out Z.AI. And it described in-app deletion that does not exist yet. I fixed those and a list of smaller ones over four more rounds, and the review accepted it. Section nine now says that deletion is by email to you, until the deletion lane lands and rewrites it.

One thing went wrong on my side. The two background jobs I left to land the next lanes died quietly when the chat paused. I noticed an hour later and restarted them as a detached process.

## What to remember

The rule that finally held, for accounts and for the store build, was to check at the single place everything passes through, and not at each entrance.

If I were holding the phone, I would let the queue run tonight. In order, it lands the purge tool, the reviewer sign-in, the Z.AI removal, and the privacy policy. Then I would put the account deletion lane first in line tomorrow, because App Review cannot start without it. Each landing takes roughly an hour, so batching the last three into one landing would save two hours. Say the word and I will do that.

## Names and receipts

- Report scope: origin/main from about 2026-11-24T01:00Z to 2026-11-25T01:00Z, 215 commits.
- Proof of address and passkey in one transaction: 8be8c8bbe, ac897e61a, e75125fcf, 1f85b0c12. Sign-out revoke: 43d4ab5bd, 750d84cac, f55ac2031.
- Sign-up bounded by room: ab5a89564, 6f75635af. Room holders: a7a22c298, 563cdeb54, f724713d5 (CI green, staging serves f724713d5).
- Disk leftovers sweep: 884df9328 through ea8929540. Not yet run on Node B (ledger C209).
- Spacer removed: 36ef46900, 346df3e8f, e2a4a67b9, 46e78938f.
- Corpus as values: f35902963, pushed 00:31Z, CI in progress.
- Problem records: docs/problems/captures-dropped-and-stalled-2026-11-24.md, autoradio-voice-pipeline-not-the-one-chosen-2026-11-24.md, computer-disks-hold-self-development-leftovers-2026-11-24.md, store-build-guard-checks-the-route-not-the-artifact-2026-11-24.md.
- Queued, in order: lane/purge-unproved, lane/demo-signin (seat 2 round 10 accept), lane/drop-zai at b4467ff1b (seat 2 accept), lane/privacy-v2 at 968c6e0e2 (seat 2 round 5 accept). Waiter logs in the session temp folder: after-queue.log, after-zai.log.
- Not landed and needed for App Review: lane/account-delete (36 commits, in review), lane/voice-models, lane/ship-asc.
- Housekeeping noticed: docs/changes.md carries three near-duplicate entries for the disk-leftovers sweep, still marked "not landed" although it is on main.
