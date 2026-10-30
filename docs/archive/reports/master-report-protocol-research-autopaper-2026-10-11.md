# Choir master report: the protocol, research, and Autopaper 2026-10-11

> **Revised 2026-10-12.** The owner's line-by-line review changed several proposals here: no runtime-owned research job and no budgets, async search instead of the deadline ladder, a permalink per version, no runtime scoring, and "research has no loop" corrected (it loops within a run; what it lacks is a way to end or continue its own work). The research-job and deadline-ladder drafts named below are deleted. See [../re-engineering-decisions-2026-10-12.md](../re-engineering-decisions-2026-10-12.md).

Written Sunday, October eleventh, late evening UTC. It covers the whole day since you decided that precommitment records are the protocol between agents, about ten hours of work. The last two hours were research and design only, at your request, done by seven research agents whose key claims I checked against the code.

Research's real problem is not search: it has no loop, so each round ends after one short report, and almost everything it finds is dropped before Texture sees it.

## Where things stand

The protocol is real on staging. Every request one desk sends another is now a record on the ledger. Texture uses the same verbs as every other desk. Only the Texture desk can write a Texture. The old side channels, Message and Emit, are gone. Two live demos proved it, with every dispatch joined to its record and no failures.

The next protocol station, which lets management feed any story's Texture, is designed but held. Its third review found a structural question about how a Texture run takes work from outside the owner's request. That question is yours.

Seven design documents are waiting for you. The most important finding is about research. It explains why the AI news output stays thin, and it means prompt tuning could never have fixed it. I stopped all tuning, as you asked.

## What landed today

In the morning you decided that agents talk only through precommitment records. I designed five stations from that, and you approved the plan.

The first station made each of Texture's dispatches mint a record in the same commit as the dispatch. The second, in three slices, gave Texture the ordinary verbs Cast and Ask, folded into its document turn. It stopped management from writing a Texture through the product API. And it retired Message and Emit. The first demo made eight dispatches, all joined to their records. The regression demo on the final build made five more. The limit of four research rounds per request held both times.

Along the way I found a misroute and fixed it. Any Cast through the general act path opened an engineering assignment, whoever it was addressed to. Now only a Cast to engineering does.

The station that lets management feed a story's Texture went through three review rounds. Each round found something real. The third found that a Texture run is bound to exactly one piece of work. So work arriving from outside the owner's request either never closes, or it collides with the owner's request. The most promising shape copies an existing pattern: one standing inbox per Texture for directives, like the supervision surface engineering already has. The alternative is letting a run carry several pieces of work, which changes the Texture root you reserved for review. I stopped there.

## Research has no loop

This is the finding I would most want you to hear.

A research "round" is one short model run, about one to two minutes long. It does two to ten searches, sends one report, and ends itself within seconds. Only Texture can start the next round, and each restart costs a Texture turn of one to four minutes. Then the four-round limit stops everything. In the first demo, research was busy only about half the time, and it stopped ten minutes in. Four versions of the prompt all said "keep going". The runs still ended after one report. A prompt cannot keep alive a run that the runtime ends.

Two smaller causes make it worse. First, a leftover instruction from the deleted tools fires on every search result inside a cell. It tells research to submit a short packet of two to four claims before searching again. I confirmed in the code that its guard looks for tool names that no longer occur, so it never switches off. Every sourced report in both demos had two to four claims. Second, a report can only carry claims backed by a page research imported. So the first demo turned more than a thousand search result cards into ten sources. Unconfirmed leads have nowhere to go, which is the opposite of what you asked for. One chips story was found and then lost when the run ended.

Search itself was healthy. Searches returned their full result sets, with no rate limits, and Texture incorporated every report that had sources. Research blamed the search providers in eight of eleven reports. Part of that is a real search problem, described next. Part is that the search contract labels any search "degraded" when one provider fails.

The design sketch replaces rounds with a research job the runtime owns. The job runs to a budget and stops when new items stop arriving. Search results feed a durable list of leads. Unconfirmed leads stream to Texture on a timer and are confirmed in parallel. "Recursive" means follow-up queries rather than trees of agents. Building it is red-class work, so it waits for you.

## Search, in plain words

Three layers each allow thirty seconds: the computer asking, the gateway, and each search provider. When one provider is slow, the gateway waits right up to its thirty seconds and then returns what it has. By then the computer has already given up. So the computer sees a timeout for a search the gateway logged as a success. That happened to one search in ten over the last day. The slow provider is most likely ChatGPT's web search, at medium to high confidence. When it stalls, the result is usually just one other provider's ten results. And the gateway never charges the slow provider for it, so it stays in rotation.

The first fix is cheap. Order the deadlines so each layer gives up before the one above it: about ten seconds per provider, twenty-five for the gateway, thirty-five for the computer. The prediction is that no search outlives its caller, and the slow tail drops from thirty seconds to about twelve. That mostly benches ChatGPT search, which makes it a routing decision, so I did not build it.

Your requirement, fast, broad, deep, iterative, recursive and comprehensive, splits cleanly. The gateway owns fast and broad. The research job owns deep, iterative, recursive and comprehensive. The search design document lays out fourteen changes with their risk class.

## The Autopaper app

The design follows your description. A news front page. Each story is a Texture. The sources are transcluded as quoted passages from captured pages, and management feeds the stories through records, never by editing them.

The existing Universal Wire app should be rewritten, not reused. It is built on a retired schema. It stamps the same two invented "claims" on every story, which I confirmed in the code. And on staging it currently shows nothing. The Texture renderers, on the other hand, already do most of a story page: numbered sources with quote popovers, and links between Textures with a "newer version" marker.

Three gaps stand between today and the first real story page. Today's demo sources are bare links with no quoted text, because fetched pages are not kept and only research can read a capture. Publishing mints a new permalink on every publish, so a living story would multiply on the front page. And no route reads records yet, so claims and scores cannot be shown. The design proposes a reader slice that needs none of the held protocol work: one permalink per story, a public front page, and citations that quote captures. That way something visible can land early, while predictions and scores follow the protocol stations.

## Predictions and scoring

Predictions are mandatory, as you decided. The key idea in the design is to separate the answer from the score. Once a prediction resolves, scoring is arithmetic. So the real question is who supplies the answer.

The cheapest source is the runtime counting what happened. Did an answer arrive? Were there new sources? Was there a source on the vendor's own site? Did the next revision cite it? Where a question cannot be counted, the default answerer is Jev, the fast decision model, recorded as management's scoring. Self-scores and the answering desk's scores are recorded, but they do not govern.

The experiment that will settle your open question about scorers is this. Jev also answers a quarter of the countable questions without seeing the count. That measures how good it is, and the system switches per question type once there is enough ground truth. My expectation is that most questions turn out to be countable. If so, the effort belongs in more counting rules, not better judges.

The design found two defects in today's code. The asker's probabilities are copied into the packet the answering desk reads, which ruins blind answering. And every reply scans up to ten thousand records. I confirmed both.

## Signing and computer keys

You asked what blocks real computer keys. One host-held platform key signs every computer's event receipts. That witness already proves when a record was committed. A computer's own key would prove who wrote a record, not when.

Real per-computer keys are blocked by several missing pieces:

- a decision on whether a key follows the computer across rebuilds or belongs to one realization;
- any binding between a computer and its key, and any publication of keys;
- rotation, which exists only as unused scaffolding;
- fork re-keying;
- a frozen commit body to sign.

One limit no key design removes: on staging the host operator can read every guest's disk. So the plan is to do three things now. Add a commit hash over a frozen record body, point each record at its witness receipt, and label records "platform-witnessed, unsigned". Before any public track record, publish commitment hashes to the global store and anchor them to an outside timestamp. Build real keys as a later station, and never re-sign old records.

## Assign, and engineering's route to the owner

You asked whether Assign is a needless subsystem. It is. I confirmed that it writes into a map inside the worker process and returns "dispatched", and nothing else happens. A management Cast to engineering already opens the real assignment. The recommendation is to delete it, with a refusing stub for one release.

On your routing proposal, the review agrees that engineering should report only to management. Today the one exception is an owner revision on an engineering-bound document. That is the path self-development and the M11 runs use. Retiring it removes about eight hundred lines and leaves management as the only way in and out.

The review disagrees in four places, and I agree with it.

- **The test.** It should be whether a report matters to you, not how technical it is, since you asked for code diffs in the document.
- **Jev's role.** Jev should only sort reports into pass through, restate, or hold. Restating needs a real model turn.
- **Never hold.** Results, blockers, questions for you and approval points should never be held.
- **What Texture gets.** The Texture should get management's plain words plus a link to the engineering report, not the raw report.

Jev cannot do this sorting today, because no desk has a verb for it.

## Idle computers

The test computers were not working at all. A staging setting from May keeps every ordinary computer warm while the host has spare capacity. That quietly cancels the October decision that idle computers take the new build. So no active computer on staging, user computers included, ever updates on its own. Pressure reclaim used to hide this by stopping computers, but the host has not been under pressure since yesterday evening. The recommendation is to defer the fix and fold it into hibernation and wake, as you suggested. Meanwhile, demos should check their computer's build and use a fresh one if it is stale.

## What I would remember

Understanding beat tuning today, three times over. The research desk's thin output, the search timeouts and the idle computers each looked like a parameter problem. Each turned out to have one structural cause visible in the code and the logs. Tuning any of them would have hidden the cause.

## Decisions waiting for you

If I were holding the phone, I would decide research first. The question is whether research becomes a runtime-owned job with a budget, a lead list and streaming unconfirmed leads, replacing the four-round limit. Everything about Autopaper's quality sits downstream of that.

Second, the search deadline fix, which is also a decision about keeping ChatGPT search in the fast path.

Third, the shape of Texture's inbox for directives, which unblocks living stories.

Then the smaller ones:

- whether the Autopaper reader slice lands before living stories;
- deleting Assign;
- retiring the owner-revision path into engineering;
- whether predictions are shown publicly from day one;
- the signing anchor;
- and, still from this afternoon, provider routing for when the ChatGPT account hits its limit.

## Names and receipts

- Staging: b172895b (CI 38170958509 green). Docs pushed through ba32e0cb.
- Protocol goal files: docs/definitions/choir-protocol-p1-record-first-controls-2026-10-11.md; docs/definitions/choir-protocol-p2-one-surface-2026-10-11.md (receipts P2a-deployed, P2c-deployed; landed); docs/definitions/choir-protocol-p2d-texture-addressing-2026-10-11.md (P2d1 landed; P2d2 held; receipts P2d-review1 to P2d-review3); docs/definitions/choir-precommitment-protocol-metamission-2026-10-11.md; design docs/precommitment-records.md (owner answers recorded in section 9).
- Research and design documents from tonight:
  - docs/problems/research-desk-structural-flaw-root-cause-2026-10-11.md
  - docs/archive/search-plane-design-fast-deep-broad-2026-10-11.md
  - docs/autopaper-app-design-2026-10-11.md
  - docs/precommitment-records.md
  - docs/precommitment-records.md
  - docs/archive/protocol-review-assign-and-engineering-routing-2026-10-11.md
  - docs/problems/idle-test-computers-keep-old-builds-root-cause-2026-10-11.md
- Other problem records: docs/problems/guest-search-timeout-equals-gateway-provider-timeout-2026-10-11.md (corrected); docs/problems/chatgpt-usage-limit-stops-research-and-management-2026-10-11.md; docs/problems/texture-raced-activation-refused-first-turn-2026-10-11.md.
- Verified code points: researchtools.go shouldRequireResearchUpdateAfterTool and its next_instruction; broker.go handleAssign; universal_wire.go fixed claims; platform service.go new publication id per publish; rlm_reduce.go distribution notes in the question packet; lifecycle_commit_act.go 10,000-record scan; gateway handlers.go 30-second search deadline; node-b.nix keepalive mode and warmness_policy.go primary skip.
- Demos: test computers demo-p2a-2026-10-11 (document 07968375) and demo-p2c-2026-10-11 (document 34f4781e).
- Operating rule added to CLAUDE.md: understand before tuning; panels for uncertainty, not ceremony.

## Added later the same night

After this report was first published, I kept to design work and drafted three more things for your review. Nothing was built.

The research job is now a drafted station. It keeps one research run alive as a budgeted job instead of ending it after each report. Search results go into a lead list in the computer's evidence store. The runtime streams unconfirmed leads to Texture about once a minute. Its first slice only deletes the stale instruction, and that slice doubles as a measurement. It lives at docs/definitions/choir-research-job-2026-10-11.md.

The search deadline ladder is a drafted station too, at docs/definitions/choir-search-deadline-ladder-2026-10-11.md. It holds the ChatGPT search routing decision for you.

The directive inbox for P2d2 has a design answer, at docs/archive/texture-directive-inbox-design-2026-10-11.md. A directive should carry no work item of its own. It joins the Texture's open request work, or reopens it the way an owner revision does. And a turn may not complete that work while a directive is still waiting. The design also found that the store's turn commit cannot consume a directive at all today. So the consumption change and the store fix must land together, or every Texture turn would fail while a directive is pending.
