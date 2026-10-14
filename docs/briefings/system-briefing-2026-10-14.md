# Choir System Briefing, October 14

Written on Wednesday evening, October fourteenth, at about ten at night UTC. It covers the day from midnight UTC, about twenty-two hours and three hundred and one commits, and it is the first briefing, so it has no earlier one to settle.

Today Choir got harder to break and easier to measure. Staging now runs every line of code on main, including tonight's security work, and much of what remains is switching on what is built, or landing what sits on branches.

Here is where things stand. The paper is live and better ranked: a one-source stub can no longer lead, and titles no longer read as desk instructions. Computers hibernate to a memory snapshot and wake in about a tenth of a second. The host's internal routes, databases and service units were hardened this evening, and the Firecracker jailer is deployed with its switch off. Three things did not land. The self-development drill still cannot reach Freeze, because the capsule cannot build the binary in time. The prompt-cache and database index fixes sit on a branch. And the experiment that tests your core thesis, in-context reinforcement learning, is pre-registered but not deployed. The most valuable decision waiting on you is about backups, because everything still lives on one host.

## How to listen to this

There are five chapters and a short list at the end. Each claim about the system carries one of four spoken labels. "Built and deployed" means the code is running on staging. "Built, not landed" means it is on a branch, or on main but not proven. "Decided" means a ruling or an accepted design that is not built. "Hypothesis" means I could not verify it from the repository. A short reference follows in parentheses so the claim can be checked. A reference is either a commit hash or a document path under the docs folder. A path with no date and no file ending stands for the file of that name dated October fourteenth. Your text-to-speech reader can skip the parentheses.

## Chapter one: what changed today

The day had a shape. It began with building, turned at midday to efficiency, and ended with security and with teaching. Your own direction moved three times along the way, and each move is visible in the work.

### Overnight: the paper's intake and the computers' sleep

Before dawn UTC, the synthesis intake work landed. Its station is called L2a, and its goal is one story per event, across languages, instead of one article per capture. Jev, the cheap typed judge, now labels each new capture against the story shortlist as its own object, without ever blocking the capture's arrival. Management sees those labels and pages the whole story library. On a test computer with a fixed set of twenty-seven multilingual items, merging was perfect: every same-event pair ended in one story and nothing merged that should not (commit a9d87870e). Labels were switched on for the live paper at about seven in the morning UTC (commit d609f6193).

The same night, computers learned to sleep properly. A hibernated computer now keeps a memory snapshot instead of being killed. On staging it hibernated in six seconds and woke to healthy in a hundred and three milliseconds, against nearly ten seconds for a cold boot, with its open work kept (commit 7ad5607f4). A follow-up mints a fresh credential on wake, so a computer can sleep past its credential's expiry (commit da3d8903a). Later in the day you ruled that hibernation must survive a night, and the maximum snapshot age went to thirty-six hours (commit 97d7a5733).

Two cost levers landed too. Very long cell output no longer floods the transcript: one research print of about a hundred thousand characters had raised every later call's input from seven thousand to thirty thousand tokens (commit 34f3a75fe). And Choir now prices the models it calls, so the costs page reports dollars (commit d07e4cd32).

### Morning: your rulings, and a lesson about restarts

Your twelve roadmap rulings were recorded just after midnight UTC (reports/owner-roadmap-rulings). The ones that shaped the day: architecture and resource efficiency before source scaling; spending tracked but never capped; the paper publishes around the clock, even after crashes; general news is vertical zero and Taiwan is the first vertical.

The morning then supplied a live example of why ruling six mattered. A planned refresh of the paper computer closed all fourteen source schedules, and the paper went silent for about an hour until you restored them by hand (problems/planned-refresh-closes-paper-schedules). The fix makes a source schedule a standing intent: a crash closes only the poll in flight, never the schedule (commit ce59d4068).

The Go module proxy for engineering capsules also went live this morning. That was your choice of option B in the capsule decision memo: the host serves modules, and capsules get no general internet route for them (commits 3c6347e81 and 03084116a).

### Midday: efficiency before scaling

At about noon UTC you changed the order: "we really shouldn't be scaling our utilization until we improve our efficiency to the point of diminishing marginal returns" (ACTIVE.md). The afternoon's work followed that sentence.

Measurement came first. The artifact store was growing about twelve and a half gigabytes a day, half of it file chunks re-uploaded after every boot, and garbage collection had only ever run as a dry run. Both are fixed and collection is on (problems/artifact-store-grows-12gb-a-day; commit fa3b3336e). A profile found the paper computer's CPU going almost entirely to database scans (problems/guest-cpu-is-unindexed-store-scans). Prompt-cache hits were measured for the first time (commit 34a3e524f). And the search plane stopped wasting calls: a result cache, joined duplicate searches, and free providers first, with one paid provider only on a shortfall (commits 7250d36bf and 0e8248b5c).

Autopaper changed visibly too. A story with fewer than two sources can no longer lead the front page, a rule that came straight from a Mauritius riots story that led with one source and a hundred and twenty-one words (commit 1f84fa908). Sections arrived: World, U.S. Politics, Business and Finance, and Tech and AI. The paper went from fourteen sources to twenty-eight, and management's intake moved to sub-desks called management tasks, four at a time (commit 433ce4d6e). And Choir got a license: Apache 2.0, your call (commit 750bd259d).

### Afternoon and evening: security, then teaching

A full isolation review of the host found that every virtual machine monitor runs as root without a jailer, that the guest firewall covered one of sixteen guest subnets, and that any process on the host's loopback address could stop computers or read credentials (reports/computer-isolation-security-review). By about twenty-one forty UTC, five fixes were on staging, though three of them do not yet enforce anything: the firewall over the whole guest pool, the jailer with its switch off, an internal credential in observe mode, a staged database password, and tightened service units.

At about seventeen thirty UTC you gave your strategy takeaways, and twenty-five minutes later you corrected them: "We need to deepen foundations." Growth is not the goal this month. Production grade for Autoputer, Autopaper and Autoradio in about a month is (reports/strategy-takeaways). You also asked how to unbottleneck yourself and learn faster. The answers to that question are the six teaching primers in the learn folder (commit c0f676362) and this briefing.

The evening produced designs rather than code: the in-context reinforcement learning experiment, the agentic email desk, the Autoradio programming engine, and the docs prune, which took the docs folder from one thousand three hundred and thirty-eight files to seven hundred and sixty-one (reports/docs-pruning-plan). One test ran and failed. The self-development drill, rerun after the module proxy, got every module from the proxy and still did not reach Freeze (problems/selfdev-drill-capsule-cannot-build-autoputer).

## Chapter two: the state of the system

This chapter walks the system part by part, each claim with its label. Where a document and the code disagree, I say so, because the disagreement is itself worth knowing.

### Staging and deploys

Built and deployed: staging's build and platform both report the last code commit on main. Everything after it is documentation (commit 0b59a9c89, read from the health endpoint at about twenty-one thirty-eight UTC).

Built and deployed: a deploy no longer takes the site down while the gateway drains. In the proof deploy, all four hundred and seventeen health probes answered (commit 2db4bc020). Hypothesis: model calls from computers are still refused during the drain, for up to eleven minutes (reports/l1-deploy-without-site-outage-design).

A caveat on the word "deployed": it means the host platform. Guest code reaches the live paper only when that computer refreshes, and the paper is never idle, so it never refreshes by itself. Decided: guest updates by pull, where each computer takes a pending update at a quiet moment or after a bounded drain, never forced (reports/lu-guest-updates-design). No code exists yet; two slices are cleared to build.

### Autopaper

Built and deployed: the front page ranks by Jev's importance score times depth times recency. One source multiplies a story's score by a half, two sources by three quarters, and under a hundred and fifty words by a further eight tenths (commit 1f84fa908).

Built and deployed: fifteen of a hundred front-page titles had read as desk instructions, like "Write the story of". Texture now titles a story from its lead headline, and holds a revision with none (commit 201fe33ca).

Built and deployed: a free model's rate limit no longer opens the whole provider's circuit, and an automatic publish that cites no source is held (commits bde0a169f and dae91894a).

Built and deployed: the synthesis intake's labels are on for the live paper. Here is a correction worth hearing. The state-and-vision report says live merge recall is seventeen percent. That number came from a scorer that read only the newest fifty stories. Re-read properly, the first live window merged twenty-nine of forty-six cross-outlet same-event pairs, about sixty-three percent (commit 17820594e). And during that window Jev labelled nothing, because the gateway had pinned Jev's credential to the paper's first guest address, which changes on every boot (commit 2cc8ab8fa fixes it).

Hypothesis: that labels make one story per event on the live paper is untested, because no second window has been scored. In the first window, production was close to linear: about two and a third captures per new story, and about one outlet per story (problems/l2a-merge-recall-on-the-paper).

Built and deployed: management runs up to four intake tasks at once (commit 433ce4d6e). Hypothesis: whether that clears the backlog at twenty-eight sources is not recorded. With one task at a time, a hundred and forty-nine of a hundred and fifty-four captures were still waiting after thirty-four minutes (problems/management-intake-backlog-at-28-sources).

Built and deployed: research searches the computer's own corpus before the web, though only titles of the newest thousand captures, about five hours (commit db9f56c9f). Built, not landed: indexed retrieval over the whole corpus. A local benchmark at a million captures takes today's query from five seconds to under thirty milliseconds (problems/corpus-retrieval-windowed-unindexed-untimed).

### Desks, records and the object graph

Built and deployed: engineering runs on records, so its state lives in the same commitment ledger as every other desk (commit a06191f9e).

Built and deployed: one write gate covers every durable object-graph write (commit 7f287a135). More on this in the concepts.

Built and deployed, as an offline laboratory wired to nothing: the claims study. Its first result does not stop the claims layer, since ninety-four percent of extracted assertions can be routed for settlement. Extraction is not ready: modality, meaning whether a sentence asserts, predicts or promises, scored seventy-nine and seventy hundredths for the two models, against a bar of eighty-five (reports/claims-e0-e6a-results).

The same study produced the desks' first box score. Management is underconfident, as the concepts chapter explains. Research quotes faithfully but answers about two thirds of what it is asked. Forty-seven percent of published citations were never judged. And the field naming which model produced a desk record was empty on all two thousand nine hundred and thirty-one records. Built, not landed: that field's fix.

### Computers and resources

Built and deployed: the snapshot budget counts real on-disk bytes. A compressed eight-gibibyte snapshot takes about five hundred and sixty megabytes, but the budget had counted the full eight, so it held about five snapshots instead of about seventy (commit 9001e9842). Hypothesis: the new estimate is a bound, and how close it gets is unknown.

Built and deployed: admission reserves each computer's measured memory instead of a flat eight gibibytes, and reclaim waits for sustained pressure (commit b6f19a284). That follows a real incident in which reclaim hibernated a user's computer during a disk scan and its cold boot hung for five minutes (problems/pressure-reclaim-wedged-a-user-computer).

Built and deployed: artifact garbage collection. Hypothesis: its first active run should reclaim between eight and twenty-four gigabytes, but no receipt of a run is in the repository.

Built, not landed: the database index fix for the paper's CPU. In a benchmark, management's end-of-run check fell from about fifty-three seconds of CPU to about a tenth of a second (branch guest-perf-land). My open prediction puts sixty percent on it taking the paper below half a core (calibration/claude-ledger.md, row sixteen). The prompt-cache fix is on the same branch; the measurement that will judge it is deployed.

### Security

Built and deployed: guest forwarding is default-deny over the whole guest address pool, and a hundred and fifty-seven stale rules were swept (commit efbd69e27). Decided, not built: the review's "established-only inbound" rule into each guest.

Built and deployed, switched off: the Firecracker jailer, slice one (commit 9ac656298). Its only proof is local, in a container without KVM.

Built and deployed, in observe mode: a host-internal credential on the virtual machine controller and the other host services. It logs "would refuse" and still allows, and enforcement comes after a clean day (commit 44656ef90). Built and deployed, stage one of three: database passwords. Passwordless root still works until the last stage (commit 9ab0c6b7b). Built and deployed: service units that hide secrets from units that do not use them (commit fda386b47). Built and deployed: a read-only verifier for all of this (commit 0b59a9c89). Hypothesis: that it passes, since no post-deploy run is recorded.

Built, not landed: desk cells running as their own unprivileged users under Landlock and seccomp, and an egress policy for guests (branch fix/desk-cell-confinement). Today, management, Texture and research run model-written Go as guest root, and only the interpreter's import list stands in the way (problems/desk-cells-run-as-guest-root-and-egress-is-open).

Built and deployed, as a live bug: account recovery creates a sign-in token and discards it. No email is sent and there is no recovery screen. In the problem record's words, you are one lost phone away from a permanent lockout (problems/account-recovery-token-discarded). Built, not landed: the fix.

Built and deployed, as the current shape: every durable home, every computer and the only copy of the ledger live on one host, Node B. The state report rates this the top technical risk (reports/choir-state-and-vision, section 9.1). Built, not landed: an encrypted backup stopgap to node A, with its timer off.

### Self-development

Built and deployed, and failing its drill. The module proxy did its job: a hundred and seventy-one modules from cache, no misses. Then the capsule spent more than fourteen minutes linking the binary on one CPU and one gibibyte, without a C toolchain, and the thirty-minute bound ended the run (problems/selfdev-drill-capsule-cannot-build-autoputer). Decided: reuse the running binary when a change touches no Go, then give capsules the real toolchain. Built, not landed: both, on a branch, with capsules sized to two gibibytes.

### Calibration and governance

Built: the goal ledger, an offline script over seventy-eight goal files, none of which recorded a probability. Twenty-one of twenty-six resolved goals met their finish, and always forecasting that base rate scores a Brier of about seventeen hundredths, the bar a planner has to beat (reports/goal-precommitments). Built: my own calibration ledger, twelve claims settled and seven open (calibration/claude-ledger.md).

Built, not landed: calibration level two, which lets any agent write claims and settlements into a computer through the command line or a model context protocol server, and the in-context reinforcement learning experiment's code. Decided: that experiment's pre-registration (reports/icrl-calibration-experiment).

Two documents disagree with the repository. The mission graph still marks two missions as entry points, the texture latency cutover and the app-development metamission, while the live work is the foundations push in the ACTIVE document. And the owner notes say the license file waits for your final pick, while the Apache license was committed the same afternoon (commit 750bd259d).

## Chapter three: decisions waiting on you

I collected the open questions from today's reports, about a hundred items across more than fifteen documents, and removed the ones you have already answered. The Go module route is settled: you chose the host proxy. Jev feeding high-confidence captures on its own is settled: you allowed it. The snapshot age is settled at thirty-six hours, the search budget router is built, the license is Apache, and who gets a track record is settled: public figures and people who publish, never private individuals. What remains, merged where several documents ask the same thing, is ranked here by what each answer unblocks. The first five matter this week.

### The five that unblock the most

First: backups, and a second home for the ledger. The cost-tracking report asks you to name a host for station SP0, the production-infrastructure work, and to set recovery targets, and there is a backup stopgap to node A waiting for your go-ahead and for the agent on node A's project to apply it (reports/cost-track-sa-s3-sp-investigation; reports/roadmap-sequencing-appdev-security). It unblocks the most because station L3, where hypotheses are settled and scored, has a recoverable off-host copy of the ledger as its entry condition, and because "one host" is the top risk. Your escrow key and platform signing seed were copied to your iMac today, which helps, but the data is still in one place.

Second: what a desk may see of its own settled record, and permission to write settlements. The precommitment document lists this as open, and the in-context reinforcement learning experiment cannot start without it (precommitment-records.md, section 8.3, question two; reports/icrl-calibration-experiment, section 7). Its tension is real. The precommitment design warns that a desk which sees independent scores can learn the scorer's quirks. The experiment answers by showing aggregates, not item verdicts, and by scoring the scorer on a second blind judge. Your answer unblocks the test of your core thesis.

Third: how a busy computer takes updates. This appears in four places: the guest-update design's open items, its escalated drain decision, management delegation's question about who requests a drain, and the problem record about the morning refresh (reports/lu-guest-updates-design, section 6; reports/management-delegation-design, section 15). The concrete choice in front of you: when a drain cannot finish, should the guest reboot itself, which the panels recommend, or should the host get a fifth patch of its kill protocol? Defaults exist for the rest: defer at the deadline and alert after three failures, ring order test then paper then your computer then others, and no opt-out for your computer. It unblocks the live paper ever running today's guest code without a manual refresh, and the next engineering epoch change.

Fourth: the claims layer's three foundation rulings. D1: a public claim is the speaker's precommitment, recorded by Choir. D2: events, not captures, are the merge key, and a story is a view over an event cluster. D3: the new record kinds. Alongside D1 there is a sharp follow-on: scheduled captures are ninety-five percent feed summaries, so track records would rest on headlines unless you approve fetching full text or counting research reads (reports/claims-events-entities-ontology, section 13; reports/claims-e0-e6a-results, section 6). These unblock the claims layer's next experiments and start the one-month clock you said track records need.

Fifth: storage on Node B. The search report estimates a hundred and sixty-two gigabytes free and three to four months to full from captures alone, and the artifact store has two owner steps queued, the larger being a twenty-three-gigabyte corpus dump to delete or move to node A first (reports/search-efficiency-and-reuse, section 6; problems/artifact-store-grows-12gb-a-day). It unblocks scaling sources without a disk emergency.

### Security choices that gate red teaming

Sixth: the jailer rollout and the enforcement dates. These are operator acts more than design questions, but they are yours to schedule: flag on for one test computer, then all; enforce internal credentials after a clean day; lock database root. Each step needs no new code.

Seventh: account recovery. Recovery restores sign-in but not data wrapped by the old passkeys, which needs a choice between a recovery protector and explicit re-keying (problems/account-recovery-token-discarded). Related and still open from the sequencing report: whether the red-team station's first targets include the live paper's host services or only forks.

### Product and money

Eighth: search spending. Which spent paid accounts to restore, whether ChatGPT search stays in every search, whether the result cache is shared across computers, a per-run search budget, suggested at twelve, and a deepen budget of three important stories an hour (problems/search-providers-cooling-most-of-the-time; reports/autopaper-deepen-important-stories).

Ninth: the email desk. Twelve questions. The ones that unblock a phase: ratify "app-agent desks" as a tier under the four root desks, which unblocks the first useful slice; whether the mailbox lives in the computer; and the address domain and what a paid account is, which unblock per-user addresses (reports/agentic-email-desk, open questions).

Tenth: the scoring of public people under data-protection law. You accepted the defamation risk. The claims design asks separately whether profiling natural persons under the GDPR, Europe's General Data Protection Regulation, is also an accepted risk, with the reply path as the objection channel (reports/claims-events-entities-ontology, D6).

Eleventh: the briefing itself. The Autoradio design asks whether you want a standing daily briefing, whether you want to commit your own probabilities on pathways and keep a private calibration record, and whether "check" questions are welcome (reports/autoradio-programming-engine, section 10.2, questions eleven, twelve and fourteen). The other eleven Autoradio questions can wait, since you ruled Autoradio comes after the architecture.

### Can wait

The business questions (freemium, an agent API, media, funds as the first cash customer, government intelligence), the docs prune's seven proposed edits to the operating contract, and the throughline amendment that would put a probability on every acceptance item, so the goal ledger finally has something to score.

## Chapter four: pathways for the next week

These are four alternatives for where the week's main attention goes, through Wednesday, October twenty-first. Work will happen in all of them, but one has to come first when deploys go one at a time. Each has a type, a probability, a prediction, and the named signal that would show it is working.

One rule from the Autoradio design applies here: a probability is spoken only with its predictor and that predictor's record. The predictor for all four is me, the lead agent writing this briefing. My record today is twelve settled claims, and on the eleven with a clear outcome my mean Brier score is about thirty-seven hundredths, which is poor. Most of that came from three claims made at ninety-five percent that turned out false: two "tests pass" claims and one "it is live" said before the build had finished. So treat these numbers as honest but not yet earned, and score them next week.

### Pathway A: finish the foundations, in the landing order

Type: substrate. The remaining items of the foundations push land one deploy at a time: the database index fix, the jailer on one test computer and then all, desk-cell confinement and guest egress, the prompt-cache fix, Store B retrieval, and account recovery (ACTIVE.md, the foundations push).

Probability: fifty-five percent that at least five of those six are on staging with a recorded proof by October twenty-first.

What it predicts: the paper's CPU falls below half a core; the cached share of input rises above eighty-five percent; the confinement verifier reports every virtual machine monitor jailed; and recovery sends a working sign-in link.

The signal: the host CPU sample script's reading for the paper after the index deploy, and the costs page's cache hit rate the day after the prompt-cache deploy. If the CPU does not fall by half within a day of the index deploy, the substrate diagnosis was incomplete, and the next move is another profile, not another patch.

### Pathway B: start the thesis test

Type: measurement. Deploy the model field on desk records, the self-score instruments and calibration level two, then run phase zero of the in-context reinforcement learning experiment, at least forty-eight hours and three hundred settled hypotheses, and start phase one.

Probability: forty percent that phase one is running by October twenty-first. It needs your answer on what a desk may see, a deploy slot, and settlement writes that work on the first try.

What it predicts: the model field is present on every new desk record; the prompt sentence asking for calibrated probabilities moves management's average stated probability toward the seventy-nine percent that actually holds, even before any digest is shown; and the digest arm's system prompt stays byte-identical to the control arm's.

The signal: the share of new desk records with a model named, which should be all of them, and the first calibration settlement records arriving on the paper's tape. The experiment itself cannot finish inside a week.

### Pathway C: get self-development to Freeze

Type: capability. Land the two capsule fixes from the branch, reusing the running binary when no Go changes and giving capsules the real toolchain, then rerun the drill until a frontend-only change reaches Freeze.

Probability: sixty percent that a drill reaches Freeze for a frontend-only change by October twenty-first. It matches my open ledger row eighteen.

What it predicts: the drill's receipt shows Freeze reached, with the module proxy still at zero misses, and a change touching Go still fails or runs long, because the link step is the bottleneck.

The signal: the tracer's receipt for the engineering scenario. A pass for frontend-only and a fail for Go is the predicted pattern, and it would say the next lever is the build budget, the drill's open option C.

### Pathway D: one story per event

Type: product quality. Score a second live window now that Jev's labels are actually flowing, turn on the deepen tick once search is healthy, and adjudicate the misses.

Probability: thirty-five percent that a scored live window reaches merge recall of eight tenths or better by October twenty-first.

What it predicts: label coverage near all captures; captures per new story rising from about two and a third toward four; outlets per story rising above one and a half.

The signal: the window scorer's three numbers, recall, label coverage and the production ratio. Your principle is that articles grow far slower than sources. The production ratio is the direct measure of that sentence, and it should be read before recall.

### Which I would pick

If I were holding the phone, I would run A as the deploy lane, make the decision B needs tomorrow, and start C once the capsule branch lands. D is the most worth doing and the least likely to finish this week, because it depends on search health, management throughput and Jev labels at once. And A makes the others cheaper: the prompt-cache and index fixes cut the cost of every run the others depend on.

## Chapter five: concepts

Six ideas sat under almost everything today. For each: the general idea, the part an expert generalist usually has not met, and one concrete Choir example.

### Precommitment records and calibration

A precommitment record freezes a prediction before the evidence exists: a hypothesis, its assumptions, the alternatives, and a probability, bound to an actor, a task and a time. Later, a different record settles it. The reason is hindsight: once an outcome is known, a capable model can explain why it was obvious, and you cannot tell updating from rationalization. Philip Dawid's prequential principle says the same: judge a forecaster only by forecasts made before the outcomes.

Frozen predictions can be scored. The Brier score is the squared gap between the probability and the outcome, one or zero. Say ninety percent and be right, and you score one hundredth. Say ninety percent and be wrong, and you score eighty-one hundredths. Lower is better, and always saying fifty percent scores a quarter.

Calibration asks a narrower question: when you say seventy percent, does it happen seven times in ten? Expected calibration error, or ECE, measures it. Sort predictions into bins by stated probability, compare each bin's average probability with the share that came true, and average the gaps, weighted by how many predictions fell in each bin.

The part that is easy to miss: calibration alone can be gamed. A forecaster who always says the base rate is perfectly calibrated and useless. Allan Murphy's decomposition splits the Brier score into reliability, which is the calibration error, and resolution, which is how far your forecasts move away from the base rate when they should. Good forecasters are calibrated and sharp. Choir's design names this guard: report calibration with discrimination, because calibration can improve simply by becoming vaguer (precommitment-records.md, section 5.3).

The Choir example is management's hypotheses on the paper. Over five hundred and fifty-nine resolved hypotheses, management's average stated probability was sixty-three percent, and seventy-nine percent held. That is underconfidence. Forty-five percent of its probabilities sat between forty and sixty percent, the hedging zone. Its ECE was about fourteen hundredths (reports/icrl-calibration-experiment, section 1). My own ledger, from chapter four, carries a sharper lesson: "tests pass" claims failed in production two times out of three when production differed from the test environment (calibration/claude-ledger.md).

### In-context reinforcement learning

Ordinary reinforcement learning changes a model's weights from rewards. In-context reinforcement learning changes nothing in the model. It puts feedback about the model's own past outputs into its context and asks whether the next outputs get better. In-context learning shows the model examples of the task. In-context reinforcement learning shows it its own track record. The working bet in the precommitment design is that learning from feedback on your own outputs is far more token-efficient than learning from examples (precommitment-records.md, section 1.2).

The danger is the same mechanism pointed the wrong way: feedback on a proxy produces reward hacking. The design's answer is to feed back prediction error about consequences, which is hard to game without predicting better, and to keep the independent scorer out of the desk's reach.

The Choir example is the experiment written today, before any code ran (reports/icrl-calibration-experiment). The hypothesis: showing a desk the aggregate calibration of its own settled predictions makes its later probabilities better calibrated than a randomized control arm's. Each desk thread joins the digest arm or the control arm by a hash of its name, so news, time of day and model health balance across arms. The digest is a user turn appended after the history, never the system prompt, so it does not break the prompt cache. Evidence for the hypothesis needs all three of these: an ECE gain of at least three hundredths with its interval excluding zero; a Brier score not worse by more than five thousandths; and resolution not down by more than one hundredth. That third condition is the anti-hedging guard from the previous concept. My pre-registered probability that the evidence comes out for it is forty-five percent (calibration/claude-ledger.md, row nineteen).

### The jailer and virtual machine confinement

Each Choir computer is a Firecracker microVM: a small virtual machine whose hardware is emulated by a user-space program, the virtual machine monitor, using the kernel's KVM interface. Hardware virtualization isolates the guest. The security review asked a different question: what if the guest escapes into the monitor process? Then the attacker has whatever that process has. On Node B today, that process runs as root, in the host's own network and process namespaces. One escape that also gets past Firecracker's seccomp filter would reach every computer's disk image, the provider credentials, the signing key and the database files (problems/firecracker-runs-as-root-without-jailer).

The jailer is Firecracker's own wrapper. Before the monitor starts, it builds a private root directory holding only that one computer's disk images, kernel and two device nodes. It moves the process into new mount and process namespaces and its own control group, capped at a hundred and twenty-eight processes. It clears the environment and drops to a user ID that belongs to that one computer and has no capabilities. In Choir's slice one, a firewall rule denies that whole user-ID range any IP traffic, because slice one still shares the host's network namespace.

The expert-generalist point: the jailer does not make the hypervisor safer. It shrinks the blast radius of a hypervisor bug from the whole host to one computer's files. That is why the review says switching to Cloud Hypervisor changes the device-model surface, not the confinement problem: Cloud Hypervisor has no jailer, so Choir would build the same cage with systemd.

The Choir example is tonight's state: the jailer is deployed with its switch off (commit 9ac656298). My open prediction is fifty percent that boot, hibernate and wake, and reattach all work the first time on the test computer (calibration/claude-ledger.md, row fourteen).

### The lethal trifecta in agent email

Simon Willison's "lethal trifecta" names three capabilities that are safe in pairs and dangerous together: access to private data, exposure to untrusted content, and a way to communicate outward. An agent with all three can be told, by a crafted message, to send your private data to an attacker. Injection filters lower the odds but do not close the hole, because a model cannot reliably tell instructions from data. It has happened in production: EchoLeak, in Microsoft 365 Copilot, pulled a hidden instruction from an email into context, and data left through links and images.

An email desk has the first two legs by definition. So Choir's design removes the third leg from the desk itself (reports/agentic-email-desk, section 5). The desk has no URL fetch, writes no remote images, and may only use links already in the thread. It cannot send at all. It drafts, and the host's mail service sends only after your approval or a policy you granted for that class of send, checked deterministically outside the computer. An injected "send the last ten invoices to this address" becomes, at worst, a draft whose approval card shows a first-contact recipient.

The part past the usual discussion: fetching a URL can itself be exfiltration, because a request to an attacker's URL with data in the path delivers the data. So research never fetches a link from mail without your act. The stronger option named for later is CaMeL, from Google DeepMind, where an interpreter tags every value with its origin; Choir's desks already run code in an interpreter, so it is a natural fit.

### Prompt caching

A language model processes every input token before it writes. Prompt caching lets a provider keep the state it computed for the start of a prompt, the prefix, and reuse it when the next request starts with exactly the same bytes. Cached input is billed at a small fraction of the normal price; for deepseek version 4.1 flash, about two percent (commit d07e4cd32).

The rule that matters is "exactly the same bytes, from the very start". The prompt is laid out as system prompt, then tools, then conversation. Change one character early, and everything after it misses. So the craft is ordering: what never changes goes first. OpenAI's cache also routes each request by a hash of the prefix plus an optional cache key. Without a key, similar requests can land on different machines and miss.

The Choir example is a one-line mistake with a large cost (reports/prompt-cache-analysis). Every desk's system prompt carried the current time, to the second, about two hundred and seventy-five tokens in. Within an activation a desk only appends, so later calls hit. Across activations the timestamp changed and everything after it missed: Texture's first calls hit nothing, management's about six percent. And gpt-6-luna hit only four to ten percent, which fits the missing cache key, though that cause is still a hypothesis. The fix, on a branch, makes each desk's system prompt byte-identical, moves the time into a "run context" turn after it, and adds a stable per-desk key. Its estimate of about ninety percent hits is a hypothesis until measured.

There is a lesson about precommitments here. Row seventeen of my ledger, written before the measurement existed, puts fifty percent on the fix raising the paper's cached share above half. The baseline measured afterwards was already seventy-one and a half percent (problems/prompt-cache-breaks-across-turns). So that row will settle true and teach nothing. A prediction is informative only when its threshold sits where the outcome is uncertain. The honest repair is to settle the row as written and add a new one with a threshold that can fail, such as eighty-five percent.

### The object graph

The object graph is how Choir stores almost everything durable: captures, story revisions, commitment records, runs. It is a property graph, nodes and edges each with a kind and properties, stored in two tables of a Dolt database (learn/01-ledger-and-store.md, section 5). Dolt is a SQL database built on prolly trees, which are B-trees you can hash, so it has commits, branches and queries as of an earlier point in history.

The design choice an expert will recognize is generic rows with JSON properties, the document-store pattern. It is the fastest way to build, and its bill arrives later: every question about a property inside the JSON is a full scan unless someone indexes that exact expression. Today's CPU profile found the bill. The paper computer used about one and a quarter cores, nearly all in its main daemon, and about half of the daemon's time was one JSON filter run over every commitment record, mostly at the end of every desk run. Model-call work was under one percent (problems/guest-cpu-is-unindexed-store-scans). Records became the message bus between desks, so a ledger built for "append, and occasionally audit" is now read like a mailbox on every run.

The second idea is the registry, the list of every legal kind of object and edge. Until today, registration was a convention, and a scan found twenty-six object kinds and twelve edge kinds written without it, commitment records among them. The write gate made it a check: new writes of unknown kinds are refused before they reach the tape, while tombstones and replay pass, so retiring old data and replaying history still work. Strict on new writes, permissive on the past (commit 7f287a135).

The third idea is the name, which you set today: the shared graph of published objects that computers adopt onto their own tapes is the Autopaper object graph.

## What I predict you'll ask next

This list is the learner model's precommitment, written before your answer exists. The next briefing will mark each one asked, not asked, or unscorable, and score it. Each line carries my probability that you raise it, in any words, before the next briefing.

1. When can the in-context reinforcement learning experiment start, and what exactly do I need to approve? Sixty percent.
2. Why can the capsule not build the binary, and what is the fastest path to Freeze? Fifty percent.
3. Can we turn the jailer on for every computer now, and what could go wrong? Forty-five percent.
4. Can this briefing come every morning, as audio, without anyone asking? Forty-five percent.
5. Which decisions should I make first, and can you give me the list in one place? Forty percent.
6. How do we get off one host: backups to node A, or a second host? Thirty-five percent.
7. Why is merge recall still low, and is sixty-three percent good or bad? Thirty percent.
8. Give me the math behind calibration: the Murphy decomposition, or how the bootstrap interval works. Twenty-five percent.
9. Why did the old row in your ledger, the one about the prompt cache, count as a bad prediction if it will come true? Twenty percent.
10. How do other systems handle the lethal trifecta, and should we build the CaMeL pattern now? Fifteen percent.
