# Five days of work, and where we stand

I wrote this on Monday the twenty-third of November, in the middle of the afternoon Eastern. It covers the five days from Thursday the nineteenth to today, with the most detail on last night and this morning. It replaces this morning's letter about the night. I corrected it once, an hour after publishing, when you pointed out that it treated as open several things you had already ruled on in our design sessions. It is long on purpose, so you can listen to it while you work out and then answer all of it at once.

The system moved forward on almost every front this week, and today showed that the paper's sense of what matters is the weakest part of it.

Here is the short version. The phone app is now mostly shared code, and that version is on your phone as Autoradio One, waiting for your ears. The paper has vectors, an event layer, a search over its own stories, and a voice ask that uses that search. Main is green and staging runs it. Against that, three things are wrong in ways you can feel. The radio has silences and rough edges in the voice pipeline that nobody has measured stage by stage. The paper wrote today's biggest story and then ranked it near the bottom. And you already redesigned the part that failed today, two nights ago, and none of that redesign is built. The older documents around it still contradict it in places. This letter walks through all three, and ends with what I think the next mission is.

## The five days in order

On Thursday the nineteenth, the foundations went in. Every published story and every capture got a vector. The event layer deployed, which links captures to events in the world. The search over the corpus passed its bar. The first shared-code version of the phone app landed on main. That was also the day I worked out why the paper was sending four times the web searches its credits allow.

On Thursday night and Friday the twentieth, I ran the overnight mission on Exact, search and entities. Thirteen changes landed. Demo mode was deleted from the phone. The search fixes, the entity read and two deleted settings deployed. Three pieces stopped and are still stopped: the old root desk, which review sent back six times, the snapshot change, sent back three times, and the Texture editor's move to shared code, also three times.

On Saturday the twenty-first, a capture began joining a story by its main point. Management now writes one sentence when it opens a story, saying what the story is about, and later captures are judged against that sentence. Research began reading what it already holds before searching again. The takes page moved to shared code.

On Sunday the twenty-second, the work was mostly instruments and storage. Every model call now names its desk and its run. There is a command that shows one run step by step with its reasons, which you asked for. The store can be measured by kind. Compaction and restore each lost a full copy of the store. Research is now given the paper's graph before it searches. The first design went to its fourth draft. Its offline trial failed all four of its own kill lines, and I recorded that as a failed prediction.

Sunday night into Monday, I ran the ten hours you gave me. I will take that in more detail.

## Last night and this morning

Eleven things landed between your good night and now.

A real defect behind a red build, in the index that links objects to their kinds. A second repair used to give up quietly and report success.

The server half of signing in from a sheet. A stranger's proof of it passed on staging.

The phone train. Five reviewers read it. They found a skip that could leave the next entry silent, a loop that asked an empty desk for more every second, and an update that could land after sign-out. All fixed. It merged to main and I installed it on your phone as Autoradio One a little after eight in the morning Eastern.

The voice ask that searches the paper. When you ask the radio for something, it now searches published stories by keyword and by meaning, ranks them, and gives the best ten to the model. Its first deploy failed its own proof. The meaning half of the search never answered in time, because it asked for a hundred and thirty-five passages and the database switches to a slow plan above about eighty. I fixed that, and the proof then passed three times of three, in a quarter to half a second.

A fix to a validator that accepted a number no decoder can hold. The fuzzer found it.

And a hole in our own security gate. A change that widens what a guest computer can reach on the host could pass with no security statement. That took four review rounds, because each round found more unlisted places. I changed the approach from listing functions to listing whole packages, and it landed this afternoon.

One thing I landed and took back. A research prompt change deployed at three in the morning Eastern, and research runs went from about thirteen an hour to about sixty. I reverted it at ten thirty. I had written down a prediction first: runs under twenty-five an hour in each of the next two hours. The first hour had forty-four. The second had twelve. So my prediction was false as written, and the direction was right an hour late. I do not know why the lag.

This afternoon you gave me a new search key. It is on the server and answering. I should tell you what I saw when I checked: it is the only search provider answering. The other three paid ones are all out of quota.

## Autoradio One and the voice pipeline

You said the voice problems are hard to put into words. So here is the pipeline as it is built, stage by stage, with what a failure at each stage sounds like. When you answer, you can name the stage.

Voice is a switch. It is off at launch, you turn it on, and it turns itself off after three minutes without speech. There is no push to talk and no wake word. Both are only sketched.

Stage one. When you turn voice on, the phone switches its audio into a call-style mode with echo cancellation, and holds it there. The cost is that the radio sounds thin, like a phone call, for as long as voice is on. If that is one of the things you noticed, it is by design and it is a design I would question. A trial that keeps normal sound until you actually speak exists on a branch and was never merged.

Stage two. The phone decides whether you are speaking. Today that is a simple loudness detector. A television, a cough or a clatter can look like speech. A check that rejects other people's voices is built, but it only works once it has learned your voice, and I do not believe it has.

Stage three. Your words are transcribed, on the phone, by Apple's recogniser. If you are misheard, it is here. If it hears nothing, the turn is thrown away silently, so it can feel like the radio ignored you.

Stage four. The phone decides you have finished. It waits for about one second of silence. If it answers while you are still thinking, or cuts off the end of a slow sentence, it is here. A smarter model for this exists and is not installed.

Stage five. Cutting in. If the radio is talking and you speak, the rule is three tenths of a second of speech and two words that are not in the sentence the radio is saying. If the radio talks over you, or stops for no reason because it heard itself, it is here. Nobody has measured self-hearing on a real phone.

Stage six. A quick judgement on the server, with a two second limit, about whether you asked anything. I have no timing for this stage at all.

Stage seven. An instant answer made on the phone. A small model on the phone writes two or three sentences from passages the app finds. Those passages are only headlines and descriptions from the front page, your queue and your open pages. It never reads full stories. If nothing is relevant it says a holding line. So a vague or off-target first answer is here. I also do not know whether that model is actually downloaded on your phone.

Stage eight. The desk's real answer, from the server. This is the one that now searches the paper. Its first words arrive in about a second and a half by an old measurement, before the search was added.

Stage nine. Speaking. The voice is a small neural voice running on the phone, one sentence at a time, with Apple's plain voice as a fallback. Warming it up took thirteen seconds at launch in your first listening session. So a long wait before the first word of the day, or a sudden flat robotic voice, is here.

Stage ten. Back to the radio. After a turn, the radio usually does not resume what it was playing. That is designed, and it may feel like an abrupt change of subject.

Now the radio itself. The phone writes its own opening, about thirty seconds, and speaks it. While that plays it asks the server desk for a stretch of entries. The first stretch takes forty to fifty seconds to arrive. So there are ten to twenty seconds of silence after the opening, at every launch. This is the fault I am surest you heard. I did not fix it, because the fix is a choice. I would fill the wait with the top story from the paper, because that works even when the desk is slow.

Here is what else I know is wrong or unproven with Autoradio One.

Nothing has been tested on a locked phone. The shared-code framework does not promise to keep running when the phone locks. The player is built to carry on by itself, holding up to a hundred entries. Only your phone can prove it.

The microphone on a first launch has not been tested. That needs one delete and reinstall.

The sign-in sheet has never run on a phone.

The developer menu is on in the build you carry. It can reload the shared code underneath the app. I would turn it off.

The build on your phone is from one in the afternoon universal time. A few later fixes are on main and not in it.

Once in your first listening session, the radio lost its audio for twenty-five seconds while the bar said playing. A repair is on main. The cause was never understood.

The Files page failed once in the last simulator proof, saying it could not reach your computer. I do not know if that was real. Open Files and see.

Audio from a podcast or clip is marked as heard when it reaches the end, whether or not sound came out.

A double tap on the record button for a take can leave the microphone open.

About fifty-eight hundred lines of Swift that are not voice remain, in forty-nine files. The in-place web page and the follow-along state are the main pieces.

And the account boundary is still open. Signing out does not yet end the process and wipe on the next launch. That work had twelve review rounds and five failed drills, and I stopped it for a structural assessment that I have not written.

The honest summary of the voice pipeline is this. Ten stages, and I have a real measurement for two of them. That is why it is hard to articulate. I think the first piece of work is not a fix. It is a recording of each stage's timing and decision for every turn, on your phone, that you can open after a bad turn and see which stage did it.

## The story that was there and ranked last

This afternoon you opened the paper and did not find the day's biggest story, that Jensen Huang is back at Nvidia. You concluded captures were backed up. I checked, and the truth is different and I think worse.

The story was there. Sixty-three sources, eighteen hundred words, sixty-eight revisions. It sat tenth in a list ordered by update time.

Captures are not backed up. About two hundred an hour are being judged.

Two things went wrong. First, the news arrived as revision sixty-something of a two-week-old story about a petition demanding his return. By our own rule, a story should stop growing when its main beat changes, and "demands his return" becoming "is reinstated" is a changed beat. Management fed the old story anyway. The rule says "when unclear, feed", and you ruled to lean toward revising. That lean has a price and this is it.

Second, its importance score was three out of ten. I told you at first that the score was stale, set when it was a petition story. That was wrong, and you had it right. I read the record. The score was made fresh at revision sixty-eight, twenty minutes before I looked. A timer asks a model one question, how important is this from one to ten, and shows it only the title and the first paragraph. Not the sources. Not the history. Not how many other stories surround it. The prompt also says a company's own announcement scores one to three. A headline about a chief executive reinstated after a petition read like company news, so it got a three. Meanwhile a second, thin story about short sellers being wiped out by the same event got a six.

So importance is a guess from model weights, made again from nothing each time. Nothing in it accumulates.

And the list the server returns is ordered by update time alone. The web page and the phone do multiply importance by depth and freshness, so a three sinks.

## The design ideas, gathered

You asked me to review every design idea we have so the plan can be consolidated. Three readers went through the documents. Here is what they found, in plain terms.

There are five families of ideas.

The first family is about what a story is. One main point per story, fixed at opening. Four answers for each capture: same story, next story, new story, or decline. That is built and running. A daily check for stories that have blended two points is designed and not built. It would have caught today's case after the fact.

The second family is about importance, and here you have already ruled. What is built is the timer I just described. What you designed on the evening of the twenty-second, in section six A of the first design, replaces it. Your words were these. "Importance is something that you learn continuously. Otherwise you're just answering from model priors." "These scheduled jobs are a real hack. That's the stuff that works but doesn't scale." "We do need to score captures, not stories." And "follow ups to key stories will be important but not sparse. Novelty needs to be squared with importance."

So the design is: importance is accumulated state on a subject or a region, moved by arrivals from distinct origins, by citations, by what readers and listeners do, and by the root's own edits. It is held by the root Texture, which keeps the index of topics over time. It is assigned to captures, not only to stories. Novelty is a separate signal, computed from how sparse the neighbourhood is in vector space, never asked of a model. And the timer is on the list of things to retire.

Today's failure is that design's own example. A follow-up to a key story, dense and not novel, and important. The timer could not see it. Your design would have.

None of it is built. What is still open inside it is the update rule, what stops the root from declaring a subject important by fiat, and whether importance lives on subjects, on regions, or both. Older documents still carry two other definitions, a count of stories and a forecast settled by readers. Your ruling supersedes them, and they should be marked so.

The third family is about the front page. The first design asks whether the root Texture simply is the front page, and leaves that until the root exists. Until then nothing says how the front should be ordered. Sections as yes or no questions are built. A reader's own sections are designed only.

The fourth family is about events and entities as the unit instead of the document. The event layer is deployed, but eighty-four percent of captures mint their own event, so it groups almost nothing. Entities have a list and a public read, and the quality is poor. You ruled on this too, on the twenty-first: one bucket of subjects, no separate events and entities. The first design replaces both layers with subjects and mentions. That is the trial that failed its kill lines.

The fifth family is about learning. Forecast, outcome, lesson, trust. Forecasts are written at fourteen places. Many are settled by the party that made them. The lesson stroke is not built. The radio's notebook is the one place where a score is shown back to the thing that made the prediction, and it has had no real listener yet.

Now what is actually unsettled. I first wrote this part as a list of contradictions for you to rule on. Most of them you had already ruled on, and I had not checked. So here is what is settled, and then what is not.

Settled by you. Importance is learned state, not a count and not a question. Subjects are one bucket. The aim for search is the share of what a run takes from the corpus, not fewer searches. Nothing is retired before its replacement is proved.

Not settled. First, feed or successor. Your rule is that a story stops growing when its main beat changes, and also to lean toward revising. Today those two pulled apart, and the lean won. I think the rule needs an example like today's written into it. Second, the hot path. The offline trial failed all four of its kill lines. By the rule set before the run, the later stations do not start. The design says what comes next: a read-only measurement of whether density separates outcomes, then a second trial of the changed path. Neither has been run. Third, no panel has read draft four. Fourth, two settings and two rate limits still exist that break our own rules.

So the honest state of the redesign is that the thinking is ahead of the proof. The design that answers today's failure exists and has your rulings in it. Its first trial failed, its next measurement has not been run, and nothing of it is built.

And one thing no document has at all: a measure of whether the paper is current. Nothing compares our front page against what the world's front pages say. That is why I could not tell you this afternoon whether throughput is down from last week. I still cannot.

## What I got wrong this week

I landed a prompt change whose proof was owed after a day, and the damage showed in fifteen minutes.

I told you the Nvidia story's score was stale without reading it. It was fresh, and the truth was more damning.

I then wrote this letter as though making importance stateful were a new idea, and listed your own rulings as open questions. You had designed it two nights ago, with me. This is the second time this week you have had to tell me we already redesigned something. The cause is that I answer from what is built and do not read the design first.

I made a prediction with a line that the old data crossed on its own two hours in twelve.

I switched a checkout under a running landing script. I queued a proof under a name the queue refuses. I told you the new train was slower, and then that it left silences between entries, and both were misreadings.

And while looking for a test computer's identifier, I printed a long list of local test identifiers into the session. No keys or addresses were in it.

## What I would do next

If I were holding the phone, I would answer four things.

First, what did you hear in Autoradio One. Use the stages. Thin sound is stage one. Ignored is two or three. Cut off is four. Talked over is five. Slow or vague is seven or eight. Flat voice or a long first wait is nine.

Second, the redesign. You already designed stateful importance. What it needs is the path from design to proof: the density measurement, the second trial, and a measure of whether the paper is current against the real news as its test. I think the next mission is that path, and not a tune of the scoring prompt.

Third, the two-day items. Your current app's signing ends Tuesday evening. The developer menu. The silence after the opening.

Fourth, the one ruling that is really open: when a running story's beat changes, does the lean to revise still apply, or does it open a successor.

The one thing to remember is this. The machinery works and is mostly proven. What it lacks is memory in its judgements, and a way to see for ourselves when it is wrong.

## Names and receipts

Main is at a2ba96767. Staging served f95d6f28b when I wrote this, with the gate change's deploy in progress.

Landed since Sunday night, in order: 3b5f6fd34 and 6229cd73a, the index lock. 977442ff8, device approval return. 3dcddbe86 through 33fc8ff10, the research prompts, reverted by 771fcc088. 2d44744e0, 95ce70334 and 42fa0d9f4, the voice ask's search. 0a397a684, the phone train merge. ba4c6d07b, the validator number fix. f95d6f28b, the vector fix. f43c1a282 and a2ba96767, the race rule and the security gate.

Every expedite: each of those that is not docs carried a Ring-Expedite line. The index lock, for a red build. Device approval, for sign-in in a sheet. Research prompts and their revert, for miscounted sources and then for quadrupled runs. Voice search and its vector fix. The train. The validator. The two gate commits.

Host acts on Node B: I replaced the search key in the gateway's provider file, with a backup beside it named for today, and restarted the gateway service at 19:19 universal time. Nothing else was changed there.

Proofs: scripts/proofs/device-approve-return-deployed.sh passed at 08:12 universal time. scripts/proofs/autoradio-ask-paper.sh on the drill computer failed at 14:04 and passed three of three at 17:05. Simulator proof train2p6: 167 pass, 1 fail.

Phone: Autoradio One, bundle news.choir.autoradio.lab.one, installed 13:08 universal time from dcc56f2cd. The first app was not touched.

Ledger: claim C201 settled false. Claim C202 is open until 22:30 universal time. C199 and C200 are open.

The Nvidia story: publication ending 429dfbf2d, revision 68, importance 3, scored 19:17 universal time. The scorer is internal/frontimportance/scorer.go. The feed or open rule is in internal/runtimeprompts/overlays/rlm_management_task_runtime.yaml.

Design documents read: first-design-2026-11-22, first-design-path, first-design-offline-slice-plan, semantic-layer-2026-11-20, entity-layer-2026-11-20, sections-as-questions, intake-desk-pace, autopaper-learning-materiality, corpus-first-retrieval-and-research, learning-engine, autoradio-voice-modes, autoradio-turns, autoradio-desk-fast-turns.

Problem records: research-runs-quadrupled-after-research-frame, ask-vector-half-misses-its-deadline, guest-reach-file-is-not-a-security-surface, broker-payload-accepts-a-number-no-decoder-holds, native-pushes-without-an-account-generation, autoradio-first-listening-session-2026-11-18.
