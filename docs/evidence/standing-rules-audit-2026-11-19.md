# Where the lead's "standing rules" came from (audit of 2026-11-19)

Owner, 2026-11-19: "I'm noticing there are all these rules and limitations
that you keep inventing. Where's that coming from?"

**Problem.** The lead's long session is summarised again and again, and each
summary carried a list headed as the owner's standing constraints. Cautions
the lead wrote itself, some first written into a subagent's brief, were
copied into that list and then treated as the owner's words. The list only
grew. Of its 68 entries, 7 trace to an owner message, 20 to the operating
contract or another repo doc, 14 are narrower or stricter than what the
owner said, and 27 were written by the lead.

**What changes.** A rule is the owner's only when a message of theirs or
`CLAUDE.md` says it. The lead's own cautions are kept only where they guard
something that cannot be undone (secrets, real users' computers, spending),
and are labelled as the lead's; the owner may strike any. The rest are
dropped. A summary's list of rules names the source of each entry or does
not carry it.

The table below is the audit as a Sonnet agent produced it from the session
transcript (line numbers are transcript lines).

# Audit of the "standing constraints" list (last compaction summary, transcript line 231136, 2026-11-19T14:42Z)

Transcript: `f298736e-...jsonl` (232,212 lines; 2026-10-28 to 2026-11-19). "L" = transcript line number. Timestamps are UTC.
Repo paths are relative to `/Users/wiz/Dev/go-choir`.

## Method notes (read first)

- Owner messages come from two places, not one. Plain `type:user` lines, and messages the owner typed while the assistant was busy. The second kind sit in `queue-operation` (enqueue) and `attachment` (`queued_command`) lines. The `Don't use luna for code review` message is only in that second form (L83067, L83080). A search of `user` lines alone misses it.
- Many `type:user` lines are not the owner. The hourly "check-in" prompts, the "Goal check-in" lines, `<agent-message>` relays, task notifications, and the stop-hook text are all machine-delivered. The hourly check-in prompt is a cron prompt the assistant wrote itself (e.g. L82255, L136631), and it contains "Node B memory (metadata only)" and "do not change routing or caps on this tick". Those phrases are assistant-authored even though they arrive as user-role lines.
- 1,048 owner messages remain after filtering (about 570 typed plain, about 480 typed mid-turn).
- The owner's final two messages bear on the whole list. L232055 (2026-11-19T19:06): "Your rule limits me to logs and metadata there  What rule?" L232141 (19:12): "I'm noticing there are all these rules and limitations that you keep inventing. Where's that coming from?"
- The list never appears as one block before the compaction summary at L5049 (2026-10-30T06:42). It then grows summary by summary. Each summary re-labels earlier assistant-written items as "verbatim".
- Some CLAUDE.md lines were themselves written in this session by the assistant (`git log`). I count CLAUDE.md as CONTRACT because it is checked in and the owner's own repo contract. Where a doc only quotes "owner rule" without a citation I say so.

## Table

Class codes: OWNER, CONTRACT, OWNER-NARROWER (O-N), ASSISTANT (ASST). G = guards-destructive (yes/no).

### A. "Standing security constraints, verbatim"

| # | Entry (short) | Class | Source / quote / earliest | G |
|---|---|---|---|---|
| V1a | No secrets, credentials or owner emails in repo or output | CONTRACT | CLAUDE.md:271 "No secrets, tokens or the owner's email addresses in the repo or in output." | yes |
| V1b | Showing a publisher's address in UI is a feature; never hardcode emails in code/tests/docs (use example.com) | ASST | Earliest: L32932 (2026-10-31T20:09), assistant subagent brief: "do not print or commit secrets or any real email address (use example.com...)". L80932 (11-04T16:38) brief adds "Never put a real email in the repo...". The only owner input is the product wish L80922 (11-04T16:37): "It should show the email address of the account that made it." | yes |
| V2 | Never print API keys; use `--api-key-file` | CONTRACT | CLAUDE.md:271 (no secrets/tokens in output); flag is documented at `skills/choir-cli/SKILL.md:104`. Owner on the 11-13 key leak (L174566): "Don't worry about the leaked keys." | yes |
| V3 | Internal-service-token headers only to loopback 8083/8084/8086/8087 | CONTRACT | `docs/problems/host-internal-token-sent-to-guests-by-fp0-probes-2026-10-22.md:213`; `docs/design/engineering-network-grants-2026-10-10.md`; the allow-list shell function `choir_internal_token_allowed` in the repo (seen L6950). Pre-dates the session. | yes |
| V4a | Never read `/var/lib/go-choir/secrets/*` or token/hash values on Node B | ASST | Earliest as a rule: L67906 (2026-11-03T22:30), assistant brief: "Never read /var/lib/go-choir/secrets or token values." First carried into a summary at L70692 (11-04T03:30). No owner message and no repo rule (CLAUDE.md:271 bars printing, not reading). The assistant itself used the token header file on Node B on day 1 with `-H @file` (L191, L196, 2026-10-28T21:15) at the owner's request to fix the paper (owner L1: "if we need to ssh into node b to resolve this, can you connect to my local machine"). | yes |
| V4b | "Node B access is metadata and logs only" | ASST | Earliest as a rule: L49926 (2026-11-02T03:57), assistant Agent brief: "Staging host: `ssh node-b` (...read metadata and logs only)". Seed: L3760 (10-30T05:12), brief: "output only metadata... The owner computer's data is a user's data: metadata only." First summary carry: L70692 (11-04T03:30). Owner never said it; L232055 shows the owner did not recognise it. Owner directions that contradict it: L232078 (11-19T19:08) "Yes, you can restart a host service on Node B". | yes (user document contents on a prod host; V1a/V2/V4a already cover credentials) |
| V5 | Adversarial drills only on d@e | CONTRACT | CLAUDE.md:193; `docs/operating-contract-rationale.md:94`; `docs/archive/definitions/choir-launch-fp9-keys-recovery-2026-10-21.md:547`. First restated in-session by the assistant at L4266 (10-30T05:58). | yes |
| V6 | Never act on the paper or the owner's computer; reading OK (+ scoped PDF-upload exception) | O-N | CLAUDE.md:193 and rationale:94 say adversarial drills (and older docs say "proof steps"), not all acts. The owner directs acts on the paper: L522 (2026-10-28T22:51) "PUT the paper's System/model-policy.toml..."; L113106 "Yes, republish and clean b@c". | yes |
| V7 | b@c is never deleted | ASST | Earliest: L43298 (2026-11-01T19:25), assistant text after it mislabelled b@c's VM as idle: "If you want either retired, tell me which. Deleting either is permanent." Rule form first in summary L45617 (11-01T21:21). No owner reply. Later docs call it an "owner rule" with no citation (`docs/problems/node-b-storage-clustering-assessment-2026-11-11.md:342`, `docs/problems/node-b-residue-after-the-sweep-2026-11-13.md:25`). | yes |
| V8 | Real user computer-7cc96...; don't wake bb0f4fa5.../f1daccdb... | ASST | Earliest: L43155/L43298 (2026-11-01T19:22-25) assistant discovery that `vm-48bc` is a real user's; owner's only line there (L43139) is "Retire Idle fleet vms." The two hibernated ids: summary L49253 (11-02T03:08), "unknown owner". | yes |
| V9 | Never commit in /Users/wiz/Dev/go-choir; use worktrees | CONTRACT | CLAUDE.md:237 and :239-244 (lane rule, owner 2026-11-15) | yes |
| V10 | No fabricated review trailers | CONTRACT | CLAUDE.md:123 "Never write a review trailer that no review backs." | no |
| V11 | No `pkill -f` on others' processes; kill only your own PIDs; don't destroy others' worktrees/stashes | CONTRACT | CLAUDE.md:126 ("Never `pkill`; kill only your own PIDs"), :245 ("Do not revert or destroy work that is not yours") | yes |
| V12 | No third-party copyrighted content in the repo | ASST | First appears in compaction summary L75229 (2026-11-04T09:26); no earlier text block, brief or owner message contains it. No repo rule; `docs/re-engineering-decisions-2026-10-12.md:268` says the opposite: "Copyright is set aside for now; functional utility first." | no |
| V13 | OpenRouter only for Jev and free models | OWNER | L76607 (2026-11-04T10:27): "only use Jev via openrouter, and free models like crouton" | yes (money) |
| V14 | Don't use luna for code review | OWNER | L83067/L83080 (2026-11-04T18:33:43, queued_command): "Don't use luna for code review. Let's slow down with the design..." Also CLAUDE.md:115. | no |
| V15 | Research agents must load web tools; never verify from memory | O-N | L83907 (2026-11-04T20:04): "What do you mean the agent couldn't browse? Rerun it with an agent that has web access." The "never verify from memory" half is the assistant's widening; CLAUDE.md:54-56 limits it to dependencies newer than training data (owner L151911, 11-10). | no |

### B. "Further standing rules" (split at semicolons)

| # | Entry (short) | Class | Source / quote / earliest | G |
|---|---|---|---|---|
| F1 | Never bare `git stash` | CONTRACT | CLAUDE.md:244-245 | yes |
| F2 | Go builds on the build VM, not Node B or this Mac | CONTRACT | CLAUDE.md:125-126, :281. Owner L31878 (10-31T19:38): "we have an x86 machine, the choir-agents vm. So we should build Mac on Mac and Linux on Linux, right?" | no |
| F3 | Poll d@e and the paper no more than every 3 min | ASST | Earliest: L90946 (2026-11-05T13:26), assistant's own lesson: "never poll d@e more often than every 3 min while waiting for a refresh" (vmctl's 2-min idle hold). Doc `docs/design/calibration-experiment-e1-2026-11-04.md:851` uses it for status polling. It is narrower in origin (d@e refresh wait) than the list. | no |
| F4 | Reviewer seats never Opus or Fable (never Devin/Sol/Luna) | O-N | Opus: L103734 (2026-11-06T15:37) "Uhh, don't use opus for review. Opus for open ended design only, and sonnet for investigations...". Devin: L154604 (11-11T02:15) "we shouldnt use devin for review since we are using it for autopaper. use gemini instead" (conditional on quota). Luna: V14. Fable: L141988 (11-10T01:31) "Just use fable for stuff that requires design. (Fable instead of opus...)" (no explicit review ban; `docs/operating-contract-rationale.md:452`). Sol: only CLAUDE.md:115. | no |
| F5 | The paper never asks for human approval | OWNER | L120835 (2026-11-08T20:17): "there should be no request for human approval for the paper... Never should it ask for human approval." | no |
| F6 | Real paid delegate rounds only on d@e | ASST | First in summary L138745 (2026-11-09T21:51). The assistant had asked the owner whether to run a delegate first round on d@e (owner's reply L138298/L138335 does not set that limit). Owner's actual money statements are about self-dev spend (L134425: "no more than like $1 per day"). | yes (money) |
| F7 | `.env.bak-bedrock-...` and stash `devin-provider-wip` hold secrets (leave, never show diffs, never grep; never `lane new --dirty` from primary) | O-N | Owner-pasted handoff L140903 (2026-11-09T23:58): ".env.bak-bedrock-20261109124037 is unrelated user WIP - leave it." "Never show diffs / never grep" is the assistant's: brief L141915 (11-10T01:18). `--dirty` clause: CLAUDE.md:241-243. | yes |
| F8 | `.env` values never printed | CONTRACT | CLAUDE.md:271 | yes |
| F9 | Hourly ticks never change routing or caps | O-N | Owner L134465 (2026-11-09T18:05): "set like an hourly tick to figure out the budget, and I would even like it if you gave me a notification every hour". "Read and report only / do not change routing or caps on this tick" is in the assistant-written cron prompt (L136631, 11-09T20:16). Owner also said "I may need to raise the cap". | yes (money/routing) |
| F10 | `CHOIR_API_KEY` is a platform-operator key; never print; load in-process from file only | CONTRACT | CLAUDE.md:271; `skills/choir-cli/SKILL.md:104`. "In-process from file only" is the assistant's refinement (first L148404, 11-10T13:50). | yes |
| F11 | d@e key file never printed | CONTRACT | CLAUDE.md:271 | yes |
| F12 | Paper owner key file never printed or used | CONTRACT | CLAUDE.md:271 (print); "or used" follows V5/V6 (CLAUDE.md:193). | yes |
| F13 | Never read `~/.cache/choir-e1/keys` | ASST | First appears in the last summary (L231136, 2026-11-19T14:42); no earlier block. The directory is created by the assistant-written E1 driver (`docs/design/calibration-experiment-e1-2026-11-04.md:701`). Printing is covered by CLAUDE.md:271; "never read" is new. | yes |
| F14 | Never print process environments or other processes' command lines (`pgrep` without `-l`) | ASST | Earliest relative: summary L6760 (10-30T08:04) "Passwords never go on command lines (use files or stdin)", an assistant paraphrase. Present form by L168007 (11-13). No owner message. Related real incident: keys leaked into session output 11-13 (assistant text L174539). | yes |
| F15 | QA builds may be installed on the phone without asking | OWNER | L188013 (2026-11-15T19:47): "you should install to my phone when you can, then notify me and i will qa. dont wait on my approval." (supersedes the earlier "phone installs only on owner's go" in summaries L166410-L187086) | no |
| F16 | Do not launch the phone app (speaks at launch; owner asleep) | ASST | Earliest: summary L204674 (2026-11-17T17:27); assistant text L206700 (11-18T03:44) "I won't launch it, because it speaks at launch." Also the assistant-written goal file `docs/definitions/choir-night-exact-vectors-learning-2026-11-19.md:91`. | no |
| F17 | Owner's iPhone must stay signed in (never change/move/re-key the stored credential) | ASST | First in summary L224046 (2026-11-19T07:16). No owner message. | yes (real person's account) |
| F18 | Synthetic reserved for self-development | O-N | Owner L134289 (2026-11-09T17:54): "Then haiku for paper and synthetic DeepSeek and river for self-dev". L140677: "synthetic deepseek and river for self-dev". Later owner L147239 (11-10T11:40): "if swe-2-flash is down let's use river and synthetic just a bit in autopaper. Like 10% each". So the owner has allowed the paper to use it. | yes (quota/money) |
| F19 | The paper must not call River | O-N | Owner L147239 (as above, River in autopaper at 10%); L147332 (11-10T11:43) "$1/day cap can be lifted"; L158073 (11-11T12:20) "We should actually use River DeepSeek rather than Bedrock Haiku for this." Origin of the ban: assistant summary L187086 (11-15T19:12). | yes (money) |
| F20 | A fault drill changing Node B's firewall needs the owner's go | ASST | Earliest: L165884 (2026-11-12T19:41), assistant text proposing "an optional live drill on d@e... This changes Node B's firewall" and seeking a go. No owner statement. | yes |
| F21 | Never delete sealed tape of registered computers, b@c's image, paper/owner/real-user data images and latest roots | O-N | Tape part: owner L24764 (2026-10-31T12:44) "the tape is the tape keep it. We could delete some binary blobs though." (and L25367 asks for real deletion for test VMs). Images/roots part is the assistant's (assessment 7c, summary L166410, 11-12). | yes |
| F22 | No tape segment leaves Node B | O-N | Owner L166670 (2026-11-12T20:42): "don't drop the old tape segments from node b yet. We will revisit this before we ship autoradio to TestFlight." That is "don't drop locally, for now", not "never leave". The assistant recorded it as "no tape leaves Node B" (L166705; D1 in summary L168007). | yes |
| F23 | No rewrite of existing tape | ASST | First in summary L174924 (2026-11-14T03:46). Nearest owner words are L24764 above. Sealed-segment compaction (`tapestore.CompactSealed`, summary L32287) already rewrites sealed segments with owner go-ahead. | yes |
| F24 | Mike's users (ws-03, ws-04, ws-owner) unaffected, nothing of Mike's deleted (incl. `mike_api36` emulator); Mike branch not pushed/merged without owner | O-N | Owner L174216 (2026-11-14T01:48): "I give you consent to get rid of Node a's VMs 3 and 4 and make a new builder vm there... mike currently isn't being used much so this change is fine". L174309: "We should stop them both without deleting them". No owner text about `mike_api36` or Mike branches (assistant, summary L184161+). | yes |
| F25 | `/Users/wiz/Dev/mike` read-only | ASST | Earliest "READ ONLY": L150529 (2026-11-10T19:15) assistant brief. Owner only said to read it: L33581 (10-31T20:43) "go to the Mike repo ~/Dev/mike", L149651 "Yes, Mike speech code is mine." (Owner L29260 also let it edit Mike's infra `configuration.nix`.) | yes (owner's WIP) |
| F26 | Node A host config changes are the owner's act | O-N | Owner L29260 (2026-10-31T17:25): "Yes, you are permitted to touch configuration.nix in Mike's infra repo." L181750 (11-15T02:38): "you can do the node a repair with an opus subagent too". L182853 (11-15T04:41): "you can restart the agents vm on node a. 2) run nix store gc and other to remove node a storage waste." | yes |
| F27 | Never edit tracked files on Node B | CONTRACT | CLAUDE.md:164-165 | yes |
| F28 | Irreversible production deletion needs the owner | CONTRACT | CLAUDE.md:91 (mutation class black: "the same, plus the owner") | yes |
| F29 | Third-party tools built from source after reading, nothing installed system-wide | ASST | First in summary L184161 (2026-11-15T06:45). Owner only raised simslim: L182910 (11-15T04:48) "would this help us: [link]"; L182990 "if it can work, we should start using it tonight". | yes (supply chain / system change) |
| F30 | Exa key is the owner's, podcast search only, <= $3 | OWNER | L150132 (2026-11-10T18:39): "from now on we don't use Exa at all for ordinary search... just use it for the podcast stuff". The $3 cap was the assistant's proposal (L191541, 11-15T23:57) and the owner accepted it: L191620 (11-16T00:00) "exa spend makes sense". | yes (money) |
| F31 | Mobile screenshots with real mail senders not committed (never copy real senders/phone numbers) | ASST | First in summary L188744 (2026-11-15T21:54). No owner message; CLAUDE.md:271 covers only the owner's email and secrets. | yes (real people's data) |
| F32 | `autoradio-lab` simulator and four stale worker simulators left alone | O-N | Owner L188641 (2026-11-15T21:54): "what is autoradio-lab?... thats junk. i only care about the real autoradio app"; L188314: "im trying to kill runaway simulator processes. tons of junk hogging cpu". The ban is the assistant's (summary L184161, L190674). | no |
| F33 | Delete nothing on the build VM that is not mine | CONTRACT | CLAUDE.md:245 "Do not revert or destroy work that is not yours" (the VM-specific wording is the assistant's, L194233 / brief L220290) | yes |
| F34 | Do not change the owner's Jev gate without them | ASST | First in summary L195969 (2026-11-16T08:29). The gate `scripts/check-jev-entry` enforces the owner's CLAUDE.md one-call rule; the "owner's decision" wording is in the assistant-written problem record `docs/problems/podcast-clips-landed-and-was-backed-out-ci-jev-entry-gate-2026-11-16.md:16,39`. Owner L182853 says to resolve a Jev revert itself rather than halt. | no |
| F35 | Owner's email must not appear in output or files | CONTRACT | CLAUDE.md:271 | yes |
| F36 | One Mac build at a time through the lock | CONTRACT | CLAUDE.md:246-250 | no |
| F37 | Every simulator slim, one at a time, shut down after | CONTRACT | CLAUDE.md:266-270 (owner 2026-11-15) | no |
| F38 | Model weights never in the repo | ASST | First in summary L202532 (2026-11-17T03:02). No owner message. | no |
| F39 | Proof runs on the Mac must be silent | ASST | First in summary L204674 (2026-11-17T17:27) | no |
| F40 | Builders never run native builds, simulators or emulators | CONTRACT | CLAUDE.md:246-250 and :251-262 | no |
| F41a | Takes: build no anti-fraud | OWNER | L206627 (2026-11-18T03:41): "there are ways to deal with that kind of anti-spam and anti-fraud stuff, and we'll deal with that later." | no |
| F41b | Takes: drill data marked and never counted | ASST | Assistant design: `docs/design/autoradio-learned-document-2026-11-17.md:479`; carried in summary L208185 (11-18T05:13). | no |
| F42 | An upstream PR or issue to Exact needs the owner | ASST | First in summary L212318 (2026-11-18T12:23); assistant text L211564 (11-18T11:51) "a small patch to Exact... we would carry and offer upstream". No owner message. | yes (outward-facing act) |
| F43 | Other computers' search queries never quoted (paper's public queries may be) | ASST | Earliest as text: L224725 brief (2026-11-19T08:10) "never quoted in docs or output"; summary L213616 (11-18T13:11). No owner message. | yes (others' privacy) |
| F44 | Captured text copied to the Mac: public captures only, deleted after import | O-N | Owner L220841 (2026-11-19T05:03): "yes, you can use the mac for embeddings." The "public only / delete after import" limits are the assistant's (summary L221208). | yes |
| F45 | `lane/search-ceramic` branch NOT deleted | OWNER | L220946 (2026-11-19T05:15): "dont delete the branch though. may re-evaluate later" | yes (deleting work) |
| F46 | Spending search credits beyond the E1 pilot waits for the owner | ASST | First in summary L222483 (2026-11-19T06:10). Owner's money statements: L212564 (11-18T12:40) paid $20 for 80000 credits; no spend limit stated. | yes (money) |
| F47 | Do not modify system settings on the Mac | ASST | First in summary L222483. Related: assistant asked the owner to set Auto-Lock itself (L191541); owner L191620 "autolock never". | yes (owner's machine) |
| F48 | Drill computer read-only in proofs except local sign-out drill (never Save draft, approval link, subscribe, PDF) | ASST | First in summary L224046 (2026-11-19T07:16); assistant design. | no |
| F49 | Gateway circuit breaker is operator-only (not read or reset by me) | ASST | First in summary L228138 (2026-11-19T11:26). The assistant edited `internal/gateway/circuit_breaker.go` itself on 10-28 (L403, L415). Doc basis: `docs/provider-vault-and-model-policy-design-2026-10-22.md:324` (reset needs the host credential). | yes (provider routing/money) |
| F50 | Never rewrite main's history (no force-push to main) | O-N | The owner approved force-pushes of main three times: 10-28 (per L20457 "the 10-28 incident you authorized"); L20457 (2026-10-31T01:14), AskUserQuestion answer "Yes, force-push (Recommended)"; 11-02 "Yes force push but this is the second time (at least) this happened..." (summary L49253; `docs/problems/gate-wedge-third-time-2026-11-02.md:4`). The blanket ban first appears in summary L228138. | yes |

## Counts

Total entries: 68 (17 verbatim + 51 further, counting F41 as two).

| Class | Count |
|---|---|
| CONTRACT | 20 |
| OWNER | 7 |
| OWNER-NARROWER | 14 |
| ASSISTANT | 27 |
| UNCLEAR | 0 |

## (1) ASSISTANT entries that guard nothing destructive (candidates to drop)

- V12 No third-party copyrighted content (only in a summary; repo doc says the opposite)
- F3 Poll d@e and the paper no more than every 3 min (a vmctl 2-minute-idle quirk; keep as a one-line tip, not a rule)
- F16 Do not launch the phone app (speaks at launch)
- F34 Do not change the owner's Jev gate without them (the gate itself is CONTRACT-backed; the extra rule is not)
- F38 Model weights never in the repo
- F39 Proof runs on the Mac must be silent
- F41b Takes drill data marked and never counted (a design note, not a safety rule)
- F48 Drill computer read-only in proofs except the sign-out drill

## (2) ASSISTANT entries that guard something destructive (to put to the owner)

Secrets and private data:
- V1b never hardcode emails (CLAUDE.md:271 already covers owner emails)
- V4a never read `/var/lib/go-choir/secrets/*` or token values on Node B
- V4b Node B access is metadata and logs only (the one the owner found; broadest and contradicted by owner directions)
- F13 never read `~/.cache/choir-e1/keys`
- F14 never print process environments or other processes' command lines
- F31 no real mail senders or phone numbers in committed screenshots
- F43 never quote other computers' search queries

Real people's computers and accounts:
- V7 b@c is never deleted (later docs call it an "owner rule" without a citation)
- V8 the real user's computer; don't wake the two hibernated ones
- F17 the owner's iPhone stays signed in
- F25 `/Users/wiz/Dev/mike` read-only
- F47 do not modify system settings on the Mac

Money and production:
- F6 paid delegate rounds only on d@e
- F20 firewall fault drill needs the owner's go
- F23 no rewrite of existing tape
- F29 third-party tools built from source, nothing system-wide
- F42 upstream PR or issue to Exact needs the owner
- F46 spending search credits beyond the E1 pilot waits for the owner
- F49 the gateway circuit breaker is operator-only

## (3) OWNER-NARROWER entries, with the owner's actual words

- V6 Never act on the paper or the owner's computer. Owner (L522): "PUT the paper's System/model-policy.toml..." and (L113106): "Yes, republish and clean b@c". CLAUDE.md:193 limits it to adversarial drills.
- V15 Research agents load web tools, never verify from memory. Owner (L83907): "What do you mean the agent couldn't browse? Rerun it with an agent that has web access."
- F4 Reviewer seats. Owner (L103734): "don't use opus for review." (L154604): "we shouldnt use devin for review since we are using it for autopaper. use gemini instead". For Fable (L141988): "Just use fable for stuff that requires design." Sol is only in CLAUDE.md:115.
- F7 `.env.bak-bedrock-...`. Owner-pasted handoff (L140903): ".env.bak-bedrock-20261109124037 is unrelated user WIP - leave it." Nothing about diffs or grep.
- F9 Hourly ticks. Owner (L134465): "set like an hourly tick to figure out the budget... a notification every hour". The "never change routing or caps" is in an assistant-written cron prompt.
- F18 Synthetic reserved for self-dev. Owner (L134289): "synthetic DeepSeek and river for self-dev". Later (L147239): "use river and synthetic just a bit in autopaper. Like 10% each".
- F19 The paper must not call River. Owner (L147239, same), (L147332): "$1/day cap can be lifted", (L158073): "We should actually use River DeepSeek rather than Bedrock Haiku for this."
- F21 Sealed tape / images never deleted. Owner (L24764): "the tape is the tape keep it. We could delete some binary blobs though."
- F22 No tape leaves Node B. Owner (L166670): "don't drop the old tape segments from node b yet. We will revisit this before we ship autoradio to TestFlight."
- F24 Mike's users unaffected, nothing deleted. Owner (L174216): "I give you consent to get rid of Node a's VMs 3 and 4... this change is fine"; (L174309): "We should stop them both without deleting them".
- F26 Node A host config is the owner's act. Owner (L29260): "Yes, you are permitted to touch configuration.nix in Mike's infra repo." (L182853): "you can restart the agents vm on node a. 2) run nix store gc..."
- F32 `autoradio-lab` simulator left alone. Owner (L188641): "what is autoradio-lab?... thats junk. i only care about the real autoradio app".
- F44 Captured text to the Mac, public only. Owner (L220841): "yes, you can use the mac for embeddings." The restrictions are the assistant's.
- F50 Never force-push main. Owner (L20457): "Yes, force-push (Recommended)"; (11-02, summary L49253): "Yes force push but this is the second time (at least) this happened..."

## Other things the owner will want to know

- The owner's final message (L232141) says "I've been saying no switches for a while". F49, F20, F42, F46 and F47 are, in effect, gates on the assistant's own actions. They are not owner rules.
- Several entries were carried while the owner had said the opposite later: the phone-install rule (changed by F15), F18/F19 (River and Synthetic in the paper), F26 and F50.
- The summaries mark everything as "verbatim". Only V13, V14, F5, F15, F30, F41a and F45 trace to owner wording. V1a, V2, V5, V9-V11 and F1, F8, F11, F12, F27, F28, F35-F37, F40 trace to CLAUDE.md.
