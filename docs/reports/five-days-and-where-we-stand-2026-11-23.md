# Five days of work, and where we stand

I wrote this on Monday the twenty-third of November, in the middle of the afternoon Eastern. It covers the five days from Thursday the nineteenth to today, with the most detail on last night and this morning. It replaces this morning's letter about the night. I corrected it once, an hour after publishing, when you pointed out that it treated as open several things you had already ruled on in our design sessions. It is long on purpose, so you can listen to it while you work out and then answer all of it at once.

The system moved forward on almost every front this week, and today showed that the paper's sense of what matters is the weakest part of it.

Here is the short version. The phone app is now mostly shared code, and that version is on your phone as Autoradio One, waiting for your ears. The paper has vectors, an event layer, a search over its own stories, and a voice ask that uses that search. Main is green and staging runs it. Against that, three things are wrong in ways you can feel. The radio has silences and rough edges in the voice pipeline that nobody has measured stage by stage. The paper wrote today's biggest story and then ranked it near the bottom. And you already redesigned the part that failed today, two nights ago. I have put that whole redesign into this letter, because it is the foundation, and almost none of it is built. This letter walks through all three, and ends with what I think the next mission is.

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

## The redesign, as you ruled it

This chapter is the foundation, so I will go slowly. Over the nights of the twenty-first and twenty-second you redesigned the paper from the top. You called it the first design, and said why: every version until now was a test to prove it could be done, the system works at a prototype level, and this is what gets it to production. It is written as draft four of the first design. Nothing in it is ratified and almost nothing is built. Where I quote, the words are yours.

What it is for. "To cover the world's stories, to personalize into individual autopaper editions, and serve as a foundation for autoradio, while using minimal resources enabling scaling of sources and agents." And: "the goal of any run shouldn't just be to write the best story, but to be building on and using our corpus, and then the story adds to our corpus."

How it is measured. Two numbers side by side. The surprise rate: how much of what arrives the paper did not expect. And the corpus share: of what a run relies on, the part the paper already held, with rungs at fifty, seventy-five, eighty and ninety percent. Web search volume is not a target. In your words, "the point isn't to cut search to zero", and "now we need to scale corpus utility". The reason it matters outside the paper: "the paper should become, at scale, more economical than exposing raw web search for any agent looking for current events."

The things. There are four. An account is a text about the world: a capture is an outlet's, a story is ours. A subject is "one bucket from which any symbol can be associated with a story". There are no kinds. Events and entities are not modelled apart. A mention says this account refers to this subject, with the passage, the time and the words as written. What the passage says is that source's claim, and the graph holds claims and settles none. A prediction is a claim by a story or a root about what sources will report, and it registers a subscription. Stories cite and transclude stories: "textures reference textures is the essence of it." No two stories have the same subjects and the same claims.

The hot path, when a capture arrives. No agent session. Fetch, clean, embed. Candidate subjects come from rules, with no model: known names, new proper names and rare phrases, and the subjects of the nearest captures by vector. Then one System One call for the whole poll selects among the candidates for every capture at once. An unmatched capture is the common case and does not escalate by itself. Unmatched captures are grouped by vector, and a group reported by several distinct origins is named once and offered to the root as a subject that matters and has no story. Then mentions are written and matched against waiting predictions. The budget is one embedding and a share of one call per capture. This replaces three mechanisms that each ask today what a capture belongs to.

The warm path, when a story is woken. A story's Texture predicts what its subjects will do next. "Stories predict next events and get woken with them. Surprises are key." It is woken, debounced, by mentions that answer a prediction and by surprises. It finds its context one way: vector to enter, the graph to move, sampled, with the full object one read away. It revises, cites other stories, states new predictions, and asks research only for a named gap. One session per wake, not per capture.

The root. The root Texture holds the whole view and does much of what management does now. It is one long session, resettable at any time, and its memory is its own Texture and that Texture's revisions. It is woken by a digest of its predictions answered and of surprises. It opens stories, which run as its sub-calls. "Concurrency is to be achieved by desk subcalls." It merges two stories that have become one. And it is where the index is held and learned: "counting importance, that sounds like something that gets done via the root RLM texture. That's how we hold and learn an index over time, in an LLM's REPL." Its digest carries aggregates of the capture process, not captures. In its REPL it may change the index with code: add a subject, merge duplicates, change a topic's importance. Those edits are the control on how sources are processed. Every edit is a revision, so it is seen and can be undone. I proposed, and you have not ruled, that the paper's root Texture is the front page.

The readers. A person's computer has its own root, which predicts its owner and is settled by what they do. I proposed its Texture is their edition. Autoradio becomes a desk that runs Go cells and composes by walking the graph, and it talks to the listener's root, not to individual stories. A listener's ask is a retrieval over the paper, ranked by new, important and cited. That last part is the one piece built: it is the voice search that deployed today. And you widened the shape: "don't overfit for text research and writing. Autoradio will be a conduit for general computer use." An important email interrupts the programme, is read aloud, and is answered by voice. So an arrival is anything that reaches a computer with a subject and an importance.

The loop that learns. Predict, subscribe, be woken, be settled, see the score. Settlement is host code, not an agent. Only captures settle, and copies of one wire report count once. The paper's own stories never count as evidence. A prediction whose time passes is settled false. The score is skill against a base rate, so predicting the near-certain earns nothing. The score is shown to the predictor.

Novelty and importance. Two signals, kept apart. "Sparse semantic space is novelty." It is computed from the density of what we hold around an arrival, never asked of a model. And "importance is something that you learn continuously. Otherwise you're just answering from model priors." It is accumulated state on a subject or region, moved by arrivals from distinct origins, by citations, by what readers and listeners do, and by the root's edits. "We do need to score captures, not stories." And the sentence that describes today exactly: "follow ups to key stories will be important but not sparse. Novelty needs to be squared with importance." The ten-minute timer that scores stories is on the list of things retired. "These scheduled jobs are a real hack. That's the stuff that works but doesn't scale."

Learning over retrieval. "Predictions, then retrieval, then scoring predictions. And the next prediction gets an accumulated sentiment of the past predictions' scoring." Before a search, the asker predicts whether the corpus holds the answer. The retrieval settles it. Sparsity is the thing predicted.

Sources and the scaling law. The capture path is designed together with the sources. No source is added before this path exists. The law to hold: a new source costs its fetch, its embeddings and a share of one call per poll. Model sessions grow with surprises, not with captures. Today it is the other way. About nine model calls per capture and fifty-eight per published story, and the fifty gigabyte store is mostly the machinery's own log.

What goes. Per-capture management tasks. The intake label's same-event question. The event layer as a separate store. The entity layer as a separate design. The old root desk. The importance timer. The spacing between research children. And the rule that nothing is retired until its replacement is proved.

The order of work, and where it stopped. Step zero, the desk named on every model call, is built and I read it in the logs today. Step two was an offline trial with four kill lines set before the run. It failed all four. The rules found names and not what happened, and candidate lists were mostly page furniture. That changed two steps of the hot path on paper. The design then says what comes before the next trial: a read-only measurement of whether density actually separates outcomes. If it does not, the novelty and importance section is rethought before anything is built on it. That measurement has not been run. A panel of five read draft two and said proceed with changes. No panel has read draft four.

Three earlier rulings still stand underneath all this, from the twentieth and twenty-first.

On when a story ends, you gave an election as the example. Before the vote, momentum and ads and debates are different stories. Election night is its own story. The day after is another. "A story stops growing when the main point, the main beat changes." And when it is unclear, "lean to revising".

On trajectories: "the more that we send a capture to existing trajectories with accumulated context, the better."

On research: "we shouldn't need rules. We should be able with prompts to encourage models to search the object graph first, and then the web."

Of these, the main-point judgement is built and running, and research is given the graph first.

## What today's failure says about the design

Read against your design, the Nvidia story is not a new problem. It is the design's own example, arriving before the design is built.

By your election example, a man reinstated is a changed beat from a petition demanding it. It should have opened a successor. It was fed. I have not read the run that made that choice, so I do not know whether management judged it unclear and leaned to revise, or was never really asked. That is a trace to read, not a ruling you owe.

By your importance ruling, a follow-up to a key story is important and not sparse. The timer that exists cannot know that. The learned state that would know it is not built.

So nothing about today needs a new idea. It needs the path from this design to proof. There is one thing no document has, and I would add it: a measure of whether the paper is current against the real news. Nothing compares our front page with the world's. That is why I could not tell you this afternoon whether throughput is down from last week, and I still cannot.

What is honestly unsettled is short. Whether density separates outcomes. Whether the changed hot path passes its trial. The update rule for importance, and what stops the root declaring a subject important by fiat. Whether importance lives on subjects, on regions, or both. Who is custodian of subjects and mentions. Whether the root Texture is the front page. And how a private computer's root learns importance for its owner without its data crossing owners.

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

Fourth, the rulings the design still waits on: whether the root Texture is the front page, who is custodian of subjects, and whether a panel reads draft four before anything red starts.

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
