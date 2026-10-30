# Roadmap after Autopaper live (DRAFT for review, 2026-10-13)

Status: draft for an agentic-consensus review requested by the owner. Not
yet a metamission. Written by the orchestrator from the owner's direction in
session on 2026-10-13.

## Where we are

- The re-engineering metamission
  (docs/definitions/choir-reeng-metamission-2026-10-12.md) has landed C0
  (eyes) and C1-core (records as the only work state on Texture, research
  and management; crash closes work). C2 reached Autopaper live on
  2026-10-13 (docs/evidence/c2-autopaper-live-2026-10-13.json): one
  designated paper computer, 14 sources, 26 stories and 76 versions in the
  watched hour, crash proof passed. Open in that metamission: C1e
  (engineering and self-development on records, wholesale deletion of
  packets and work items), three Cut 2 receipts (support-failure correction
  loop, cites-newer-capture check, model comparison), the final convergent
  review.
- Deploys now refresh active computers (including the owner's always-on,
  constructed computer) through a guest-written planned-restart marker;
  busy computers are deferred (commit bcc556de, deploying).
- Owner context: the owner prototyped the automatic newspaper in June and
  considers this the first real integration into Choir. "The next 20% of
  the work takes 80% of the time. So we're just starting."

## Owner goals (2026-10-13, in session)

1. **Precommitment records are the throughline** ("the scientific core, the
   theoretical essence ... what turns it from a product to a platform to a
   network with defensibility"). See docs/Precommitment Records — Engineering
   Memo.md, docs/precommitment-records.md,
   docs/precommitment-records.md,
   docs/precommitment-records.md,
   docs/precommitment-records.md. Apply to:
   (a) the world's news sources: statistics and analytics of sources;
   entity extraction; attributing claims to sources within captures;
   updating the ledger as events change; (b) our own agents and computers in
   self-development.
2. **Cost and performance first.** "My goal is to cover all global OSINT,
   but I really can't afford to do that now." A general basis, then
   verticals one by one.
3. **Verticals** (owner interest order): Taiwan and East Asia (US-China
   relations, Taiwanese democracy, the computer hardware industry, broader
   East Asia); the Global South and development (e.g. urbanization in
   Nairobi; India); cybersecurity.
4. **Sources, much broader than today:** RSS (137), GDELT, Telegram (73),
   Hacker News exist in the registry; tiered rollout in progress (14
   running). The owner wants maximal long-tail multilingual OSINT: AT
   Protocol (Bluesky), Mastodon/ActivityPub, RSS bridges such as RSSHub for
   Asian social platforms (Weibo, Bilibili, Zhihu, PTT, Naver ...), and
   multilingual media broadly. A deep-research pass on how to get there
   cheaply runs before this review:
   docs/reports/multilingual-osint-sourcing-research-2026-10-13.md.
5. **Security:** agentic red teaming (including reporting on breaking out of
   our own VM sandbox), the auth rebuild, the deferred hardening pass.
6. **Research retrieval:** search and retrieval over our own corpus and
   object graph first, the web second; later expose the news system as an
   API.
7. **Self-development velocity:** move as much of the orchestrator's current
   work as possible into the self-development loop to skip CI time.
8. **Updates:** revisit how VMs update (the owner's computer was silently
   pinned for days).
9. **Apps:** rebuild slides, ebook, podcast, PDF viewer and others to
   current standards and integrate deeply (PDF opens in a viewer; YouTube,
   image, podcast embeds in Texture); "a really nice operating system in the
   next couple of weeks".
10. **Writing:** tune Texture's writing; style guide Textures.
11. **Autoradio** about a month away, after the apps.

## Deferred work being brought back (from the gate metamission)

From docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md,
deferred 2026-10-09 ("prove the system for human QA before I invest more
money into it, and before we make the infrastructure more complex"):

- SA agent density (docs/definitions/choir-appdev-sa-agent-density-2026-10-05.md):
  2-4 GiB VMs, memory budget, the embedded-Dolt engine bottleneck,
  operational versus versioned store split.
- S3 fast resume (snapshot secrecy, one-shot invariants).
- SP production infrastructure
  (docs/definitions/choir-appdev-sp-production-infrastructure-2026-10-09.md):
  second host, logs/metrics/alerts over WireGuard, zero-downtime deploys,
  multi-host placement. Node B today: one OVH host, 12 cores, 31 GiB.
- S7 app packages, S8 source publication, S9 forks and fleets, S10 org
  templates, S11 mainline and security push.
- Gate 2 self-development: S4 open-world capsule, S5 live preview
  supervision, S6 commit gate and full release.
- The deferred security hardening pass: per-tool-call Landlock; encryption
  at rest for snapshots and exports; a review of every host-internal
  endpoint's authority model; signer and credential lifecycle; supply-chain
  review of pinned inputs; an external review before public launch. Plus
  today's: a guest can reach corpusd internal routes
  (docs/problems/s0-guest-reaches-host-internal-authority-2026-10-04.md),
  the auth rebuild, sandbox-escape red teaming.

## New evidence since the draft (C2 close receipts, 2026-10-13)

- Published versions are immutable (30 of 30 pinned versions byte-identical
  hours later, 8 of them after later versions published). Every committed
  revision published. Every multi-version story has later versions citing
  captures that arrived after the previous version. 64 of 68 unsupported
  verdicts were followed by a revision rewriting the failing sentence.
- But management left 46 of 118 arrival items unanswered (no story, feed or
  decline), 40 of them from one 8-record burst when tier 1 started
  (docs/problems/autopaper-arrivals-unanswered-2026-10-13.md). One desk
  answering every capture does not scale as-is; this bears directly on
  sources growth and cost.

## Proposed order (draft)

0. **Finish the current metamission**: C1e, the three Cut 2 receipts, the
   final convergent review.
1. **Cost and performance baseline**: per-story and per-desk model cost and
   tokens from traces; search and fetch counts; management's clustering
   cost; SA's memory and density measurement on the paper and owner
   computers; the Dolt engine bottleneck. Cheap wins: corpus-first research,
   near-duplicate capture dedupe before the model, per-desk model routing,
   sampling vs judging every citation. Output: a cost model that sets the
   source and vertical budget.
2. **Precommitment ledger v1 on Autopaper** (design station first: a lateral
   and convergent panel on what counts as a claim and its resolution for
   news): claim and entity extraction on captures; attribution to sources;
   resolution as later captures and events arrive; resolution of
   management's story hypotheses; a per-source ledger page.
3. **Corpus retrieval** (with 1-2): research searches our own captures and
   object graph before the web; the substrate for resolution and the news
   API.
4. **Self-development, scored**: C1e then Gate 2 (S4-S6) with
   precommit-before-material-action; the faster loop for computer-local
   work (apps, prompts).
5. **Security**: the hardening pass plus agentic red teaming, run as
   precommitted operations; publish findings in the cybersecurity vertical.
6. **Verticals one by one**, each a source set + standing management
   instructions + a style guide Texture + its ledger, budgeted against 1:
   Taiwan and East Asia first, then Global South/India, then cybersecurity.
7. **S7 app packages and the apps rebuild with Texture embeds**, then
   Autoradio.
8. **SP production infrastructure** when cost and scale require it (likely
   before global OSINT, after the first verticals); S3 with SA.

## Open questions for the review

- Is the order right? What should move earlier or later, and what is
  missing?
- How should precommitment records concretely enter each station (not as a
  slogan)?
- Can the solo owner afford this, and what is the cheapest path to the
  defensible network asset?
- What should the next metamission's first three stations be, and what
  belongs in later metamissions?
- What will break first as sources and verticals grow?
