# The overnight run on Exact, search and entities

Written on Friday, November twentieth, 2026, after about ten hours of work through the night while you slept. It covers the one mission you set before bed: the phone app, the paper's web searches, entities, the last settings, and the host problems.

About half the mission landed and is deployed, three pieces are stopped and waiting for you, and your phone has a new build.

Your iPhone now carries a build with demo mode deleted from the product. The paper is on a release with the search fixes, the entity read, the gateway change and two settings removed. I landed thirteen changes to main overnight. One of them went in red and I repaired it within half an hour, with nothing deployed from the broken commit.

Three pieces did not land and will not without you. The Texture root desk was sent back by review six times. The snapshot change was sent back three times. The Texture editor's move to shared code was sent back three times. I stopped all three, wrote down why, and left the code on its branches. One more item, turning owner acts from observe to enforce, is built but held, because it contradicts your ruling from October.

Four decisions are yours this morning. They are at the end of this letter. The most urgent one is the paper's snapshot, which is about two days from failing.

## What landed in the first hours

The first landings were the ones already close. Two settings came out, the ones for intake events and wide dispatch slots. The Textures list on the phone moved from Swift into shared Exact code. The research brief now takes an objective from the unread record when it has none, so no brief is skipped for want of one. Before the change, two thirds of briefs were skipped. Afterwards, none of sixteen were.

Then the demo fix went onto your phone. Demo mode can no longer turn on on a real phone, and its Settings row is gone.

I got one prediction wrong here. I had written that the brief would take its objective from the conversation or the assignment. Every brief took it from the unread record instead. The fix worked by a path I had not named, so I marked that claim false.

## The middle of the night

Three more changes went out together. The paper now keeps every web search result it is handed, as plain text with a word index, kept thirty days. In its first two and a half hours it held about seven and a half thousand of them, with no failed write. Captures also get their vectors sooner, because the store now tells the vector worker which captures it just wrote.

The design rules came next. Five violations on the phone were fixed, and the rule tests moved from Swift to shared code. I looked at the signed-out screens myself. Every signed-out page shows a banner saying you are signed out, with Sign in as the one coloured action.

Then demo mode itself was deleted. That removed about seven and a half thousand lines. The cleanup of what demo mode left on a phone stayed, and a second, wider cleanup followed it. A reviewer caught that the wider cleanup could have removed a real recent-files row. It now matches only on three facts at once, which a real file cannot share.

The gateway change also landed. It is the first step toward deploys that do not drop model calls. The two spend files are now safe for two gateway processes writing at once. Both review seats sent the first version back, because a corrupt file would have erased spend and reset your daily cap. The second version was accepted by both.

## Where it went wrong

The third group of changes went to main and failed there. It held the entity work and the two deleted settings. My tests before landing covered the packages the change touched and the packages that import them, which is what the contract asks. Two checks sit outside that set. One test scans the whole tree for edge kinds that are not registered. The other is the pinned hash of the Go modules in the Nix build. Both failed. I fixed both in one small commit, had it reviewed, and main was green again in thirty-two minutes. I now run those checks before every landing.

The entity read then failed its own proof on staging. It answered with no stories for an entity on the front page. I guessed the cause and guessed wrong. The builder found the real one: a text range in the query matches nothing under the production database's sort order, while the test databases use a different order where it works. I confirmed that on the host. Three reviews had accepted that code. None of them could have seen it, because no test ran under production's sort order. The fix landed and is deployed. The proof now passes: fourteen entities from the front page each list their stories. I asked the reviewer to look for other queries of the same shape. It found none broken, because every other such column is declared with the safe sort order. The edge table is the one that is not, and that is worth fixing at the root later.

## What stopped

The root desk is the larger disappointment. After four rounds I changed the structure, as the contract's three-round rule requires, so that the store is the one authority. Both seats agreed the new structure is sound. They sent it back twice more anyway, for the same underlying reason each time. The store has no sequence number assigned when a document commits, so the digest uses clock time as its cursor, and every patch for clock time opened another way to lose or repeat a document. The real fix is in the store, not in the desk. That is several days of careful work, and it is your call whether to do it.

The snapshot change stopped for a similar reason. Streaming the store without a scratch copy means holding one long read open for about seventy minutes on the paper. That builds a write-ahead log of several gigabytes, and every way of draining it touches the one write connection that the health check also reads through. Each round fixed the last finding and exposed the next. Both seats said it is not fit to deploy everywhere at once. It needs one measured capture on a large test store, and we have no way to make a test store that large yet.

## What is still moving

The re-ask holds for search are deployed. They took three reviews. The first version would have broken the hunt for a second outlet, which is how stories get published, so that review was worth its time. Whether the paper's searches actually fall is not known yet. I gave it about one chance in three of showing a fifteen percent drop in the first three hours. The day count under one thousand cannot be read until a full day has passed.

The themes are in their last review round, with a passing proof that your old theme choice carries over.

The Texture editor's third review came back as I wrote this, and it was sent back again. Each round found a new way your text could be lost or overwritten without a sign. This time it was a relaunch after a merge, and reopening the editor over kept text. I stopped it. The editor keeps four versions of a text, and the code never states which is which. It needs that model written down first, with a test for each step.

The dispatcher fix, for the twelve full slots on the paper, is still failing its own storm test.

## What I need from you

First, the snapshot. The paper's capture took eighty-three minutes against a ninety-minute budget, and the store grows about two and a half gigabytes a day. I recommend raising the budget to one hundred and fifty minutes now. It is one constant. It buys time to do the streaming change properly.

Second, the root desk. Either build a commit sequence into the store first, or take the root desk out of this mission. I also decided, without you, that the root desk does not digest stories. Please confirm or reverse that.

Third, the editor. Either it waits for a written model of what it keeps, which I recommend, or it stays in Swift while other pages move first.

Fourth, owner acts. Enforcing them is built and waiting. Your October ruling says observe. I left it alone.

Two smaller things. Earlier you asked why you were prompted for permission when bypass is on. I read the settings and found no rule that should prompt, so I do not have an answer. And two builders each used a broad kill command once on the build machine against their own jobs, which breaks our rule. Nothing else was affected.

The one thing to remember: review caught every dangerous defect it could see, and the two it could not see were both about production differing from the test environment. I would spend the next hour on your phone. Open it, check that it shows your real paper and nothing from demo mode, and tell me what you see.

## Names and receipts

Landed on main, in order: 6a873149b and 7f5ad41e1 (two settings), 3b0bdaae7 and de7aeb7f7 (Textures list), 84a853ce4 (brief objective, search gauge, entity status script), 6bdddb530 (demo fix), b1257d04a (embed wake and search leads), 49cbd976d (design rules), a7e0e6535 (demo mode deleted), f1aa4df9d and f883ea88d (gateway spend files), dc584af3b (entity station E2 and its follow-up, the DAMON and event-wakes removals; red), e33d9df5f (the repair), bf478ee52 (wider demo cleanup), 20af586d3 (re-ask holds and desk protocol lock 18.9.0).

Every one of these was expedited under the standing rule, with a Ring-Expedite trailer. The expedites were: the two settings, the Textures list, the brief objective train, the demo fix, the embed wake and search leads, the design rules, the demo deletion, the gateway change and its follow-up, the entity train, the red-main repair, the wider cleanup, the re-ask holds, and the entity read fix.

Later: 0eef16ec7 (the entity read's sort-order fix), deployed at 13:38 UTC, proof script scripts/proofs/entity-e2-deployed.sh exit 0.

Phone builds: 6bdddb530 at 08:24 UTC, f1aa4df9d at 11:25 UTC, 20af586d3 at 13:18 UTC.

Claims settled in docs/calibration/claude-ledger.md: C167 true, C148 to C150 void, C171 false as worded, C172 true, C173 false, C175 true by a narrower reading, C170 false narrowly (unembedded captures fell from 78 percent to about 12, against a bar of 10), C174 false on the repair clause. Open: C165, C176.

Stopped for you: lane root-desk-r3 at 7a6d66dc4, record docs/problems/texture-root-desk-six-reviews-no-landing-2026-11-20.md. Lane snapshot-stream at 0873aa1fc, record docs/problems/paper-snapshot-will-pass-its-ninety-minute-budget-2026-11-20.md. Lane owner-act-enforce at ba32aa571. Lane exact-convo-edit at 4b58dba83, record docs/problems/texture-editor-in-exact-three-reviews-no-landing-2026-11-20.md.

The red main: docs/problems/train-landed-red-objectgraph-registry-and-vendor-hash-2026-11-20.md. The entity read: lane entity-e2-ids at 42fb5625b.

Not done from the mission: the remaining Swift pages (takes, edition picks, the books shelf, the reader), search leads stations two to four, entity stations three onward, gateway stations two to five, the event join rate, and the day under one thousand searches. The paper key was used for reads only. I restarted no host service. The pilot's arm L overlay could not be deleted: the server answered 403.
