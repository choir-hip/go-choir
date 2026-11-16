# Autoradio overnight: what is on your phone and what is not

I wrote this on Monday the sixteenth of November, at half past eight in the morning, universal time. It covers one night of work, about nine hours, from your order late on Sunday the fifteenth. It is my own report of work I directed and builders carried out.

The phone has a build with everything you asked for in it, and I have not heard a single second of it. That is the news. Every part compiles, every part passes its tests, and I have watched most of it work in the simulator against the live service. But sound, voice and the feel of it in the hand are yours to judge this morning, and you should expect to find things.

Here is where it stands. The app opens and the station starts by itself. There is one play queue, and the radio host can change it when you ask. There is a podcast app inside the app, a Reader for books and documents, and real podcast clips attached to real stories on the live paper. The pills are gone, and so are two other habits that were standing in for them. On the server, the host's queue edits are deployed and proven. Podcast clips are deployed and served, but they are matched by hand, not automatically, and the reason is a rule of yours that I broke and then chose not to bend. Three decisions wait for you. They are at the end.

## The phone

I installed eleven builds through the night. The last went on a few minutes before eight.

The queue is one structure now. Stories, web pages, documents and podcast audio all go through it. When I played an episode of N P R News Now in the simulator, the player showed thirteen seconds elapsed out of five and a half minutes, with the fifteen second skip buttons that only appear for audio. That is streaming, proven up to the point of sound.

The podcast app lists popular news shows, opens a show, lists its episodes, and plays them. A show page also carries that show's clips. The Reader exists and imports books and documents, but I have only seen its empty shelf. I could not put a real book in front of it with my test tooling, and opening a file from another app into Autoradio is not built.

Voice turns on when the app opens and off after three minutes of quiet, by test. I have not spoken to it.

## No pills, three times over

You said to get rid of the pills and then do it again, because there would be more than we realised. There were. The design builder made seven passes, and I photographed each one and sent back what I saw.

The first passes removed capsules, chips and outlined bubbles. The second hunt found what had replaced them: filled accent buttons. The rule now is that a filled block exists only on the setup pages, and every other action is a word in the accent colour. A test enforces it. The third hunt found boxed rows, lists drawn inside dark cards. Those are plain lists on the page edge now, with hairlines. Cards are kept only for objects: a story, a clip, a document.

On the live paper, the front page now opens on a story. The sign in prompt and the edition prompt are each one quiet line with a "not now", and only one shows at a time.

One thing the builder did right against my instruction. I asked for the section names to scroll sideways. You have ruled against sideways scrolling, and a test holds that rule. The builder followed you, not me, and made the sections one line that opens in place. Look at it and say whether you like it.

## The host can edit the queue

This landed, deployed, and passed its proof at half past three. On the drill computer I sent the words "put the long read next" with a queue of four. Three times out of three the host answered in one sentence and moved the right entry to the front, with the reason "you asked for the long read next". I asked "what's coming up" three times, and it read the queue back and changed nothing. A real model answered all six.

The phone side was then checked line by line against the live route. That found three mismatches that would have hurt on the device, and they are fixed.

## Podcast clips, and the two times I stopped

This was the hard one, and it taught the most.

The first lesson was about scope. Three review rounds failed, and every blocker was in the same part: letting accounts report their subscriptions to the platform. That part came from your idea, late in the evening, that we should transcribe what users subscribe to. It is a good idea. It is also a design problem about what a stranger with an account can make us do, and it was being designed by review rounds at night. After the third round I stopped patching, wrote that down, and deleted that part from the change. The fourth review passed.

The second lesson was about a rule of yours. The change landed at half past four, and the continuous integration run refused it. The automatic matching job called Jev directly, with its own request. Your rule from the fifteenth says every Jev call goes through the one job entry point. Four review rounds missed it because I never told the reviewers about the rule, and I had not read that section before ordering a Jev caller. The landing script missed it because that gate ran only on the server. I backed the change out. Main was red for about thirty five minutes and nothing bad deployed.

I then had a choice. Making automatic matching conform means sharing the entry point with a second service and teaching your gate to admit a new caller. I did not think that was mine to do while you slept. So I removed the automatic matcher and landed the rest, with a small tool that ranks candidate passages without any model and writes the ones a judge approves. The gate is in the landing script now.

Then the worker would not run where it was designed to run. The build machine has no direct route to the internet, only a proxy, and the worker insists on checking the address it dials. So the worker ran on this Mac instead. I also did not put your operator key on the build machine, because everything there runs as one shared user. The key stayed in a small local relay on the Mac that forwards only the podcast routes.

The result is real. The worker recorded one hundred and eighty three episodes from sixty one shows and transcribed them at about forty five times real time. In two passes an agent read four hundred and seventeen ranked candidates and passed thirty. I wrote twenty eight, the server refused none, and the public route now serves twenty six clips for eleven front page stories, from nine shows. On the phone, the story about the aviation ground stop shows a passage from Up First under the words "Hear it".

And the host does mix them in. On the drill computer I asked for a stretch of radio with ten stories, four of them offering clips. It wrote fifteen segments and chose three clips, each placed straight after its story with a spoken lead in. After the ground stop story it said, "Here is how Up First described the moment the order came." What I have not seen is the phone playing that stretch while signed in, because my simulator runs were signed out.

## The paper, through the night

The paper published every hour. Devin's fast model carried between eighty one and one hundred percent of its calls, and the paid fallback was never used. Bedrock spend was zero.

The disk scare from midnight is explained. The forty gigabytes were the snapshot's one whole copy of the store, paid once, and the space came back after the next deploy.

Two things I found and did not fix. The paper is publishing a few stories whose headline is a failed search, such as "no opinion piece found". The radio read one of them to me as news. And five paid search providers are out of quota, so search is living on two.

## What I would remember

The night went wrong twice in the same way. I ordered work that touched a rule I had not read. The reviews were thorough and still could not catch what nobody told them to look for. The fix that worked both times was to delete, not to patch.

If I were holding the phone, I would open the app and just listen for five minutes. Then ask it to put something next. Then open a story with "Hear it" and play the clip. Tell me what is wrong, in any order.

Three decisions are yours. First, how a second service may ask Jev, so matching can be automatic. Second, where the transcription worker lives and what key it holds, because tonight it stops when my session ends. Third, whether to design the subscription intake properly, which is where your amortising idea lives.

## Names and receipts

Phone build: branch `lane/autoradio-qa`, tip `816d7aadf`, installed 07:57 UTC. Rollback build: `1ec3b7b1c`. The lab profile expires Tuesday 17 November, 21:49 UTC.

Server, in order: `5841da812` (Jev dial fix, intake fix, inline body reader; deployed 02:15), `a99e07d3b` (queue desk; deployed 03:29), `93004c45f` (podcast clips, refused by CI run 43804540972), `af31e4fd0` (the revert), `53747dcca` (podcast clips without the matcher; CI run 43816903927; deployed 06:39).

Expedites, every one carried a trailer: the Jev dial fix and intake fix (also a path expedite through `internal/agentcore`), the inline body reader, the queue desk series, the podcast series both times, and the revert.

Claims in `docs/calibration/claude-ledger.md`: C103, C104, C105, C107, C108 and C109 settled true. C106 settled false.

Proof commands: the queue proof script is `/private/tmp/claude-501/-Users-wiz-Dev-go-choir/f298736e-cabb-51ff-9cea-34588a4989cb/scratchpad/prove-queue.py`. Clips: `curl https://choir.news/api/platform/podcast-clips`.

Problem records written tonight, all under `docs/problems/`:
- `podcast-clips-review-did-not-converge-structural-assessment-2026-11-16.md`
- `podcast-clips-landed-and-was-backed-out-ci-jev-entry-gate-2026-11-16.md`
- `podcast-worker-cannot-fetch-from-the-build-vm-proxy-only-egress-2026-11-16.md`
- `paper-publishes-stories-whose-headline-is-a-failed-search-2026-11-16.md`
- `status-hourly-reports-zero-search-answers-while-search-answers-2026-11-16.md`
- `paper-data-image-grew-40-gib-in-eleven-minutes-2026-11-16.md`

Goal file: `docs/definitions/choir-autoradio-full-experience-overnight-2026-11-15.md`, with its ledger beside it.

Not built or not verified: sound, voice and clips on the device; the phone playing a stretch with clips while signed in (the server side is proven: three clip segments of fifteen; the script is `prove-programme-clips.py`, beside the queue script); the Reader with a real book; opening a file from another app; captions fetched on the device from video sites (design only); the search step with Exa (not started, nothing spent); automatic matching; subscription intake; the worker as a durable service. One feed address that carried a public app identifier was removed from the seed list but remains in git history. The Jev call volume from the paper is about three hundred an hour, which I did not examine. Other gates may run only on the server and not in the landing script; I did not compare them one by one.

Rollback for each server change is a revert of its commits; the inline body reader is roll forward only, as its commit says.
