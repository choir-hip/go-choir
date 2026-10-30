# Owner Notes, 2026-10-14 (Evening), With Lead Responses

Source: owner's notes "Thoughts in response to Choir: State and Vision".
Owner's words first, then the lead's response and what changes.

## 1. Precommitment / prospective self-alignment (PSA) is the core

Owner: the key value of PSA is storing records in the desk's Yaegi REPL;
context packs and on-demand retrieval; it serves as in-context
reinforcement learning — feedback to models is a powerful learning
mechanism that enables self-learning without modifying weights. PSA
elicits verifiability from less-verifiable tasks, since it turns
everything into a scoreable prediction. This is what makes Choir useful
as an organizational multi-agent operating system: executives and
managers overseeing all their organization's agents.

Lead: agreed, and it sharpens the claims design. Consequences:
- Precommitment records and their settled scores must be first-class in
  each desk's REPL (readable as records, retrievable on demand into
  context packs), not only rows for a dashboard. The feedback loop is the
  product: a desk sees its own settled track record when it predicts again.
- E6a (scoring our own desks) moves to the front of the claims plan.
- Organizational oversight is the same data viewed by role: an executive's
  view is the box scores and open commitments of every agent in scope.

## 2. Who is tracked

Owner: yes, individuals who are public figures, or who publish for their
content to be consumed by AI. Growth idea for later: email Substack
writers with good track records (long-tail growth) — only once the
platform is ready for users.

Lead: this settles design decision D6: track records for public figures
and for people who publish (writers, podcasters), not private individuals.

## 3. Who scores

Owner: yes, agents and desks score themselves; they just aren't the only
scorers. Multi-scoring, and score the scorer. Calibrated self-scoring is a
key capability.

Lead: settles the scoring model: every settlement has several scorers
(self, peer desks, Jev, rules, humans), each scorer has its own track
record, and calibration of self-scores is itself scored. Folded into the
claims design (D9/D10 direction).

## 4. Multiplayer

Owner: "multiplayer intelligence for the learning economy" is the
tagline; the line saying Choir is "not a multiplayer app" was wrong. A big
motivator for Texture and content-centric multi-agent interfaces is that
they enable multiplayer; chat doesn't scale to long horizons, many agents
or many users.

Lead: fixed in docs/choir-vision.md (the "What This Vision Is Not" list
now says not chat-first, multiplayer by design). The vision doc has other
outdated lines (World Wire naming; "not a publication product") — the
docs pruning task below rewrites it.

## 5. App agents

Owner: each app agent is its own RLM desk. Not every app must be an app
agent; one app-agent desk can have several front ends. Users can ask for
app agents to be modified; UI/UX will change faster than the
architecture, and the architecture will keep changing as models improve.

Lead: recorded for station LP; the design unit is the desk (verbs, state,
records), with front ends as replaceable clients of it.

## 6. Autopaper Opinions

Owner: Opinions are a social media surface and a potential growth engine,
foundational and interoperable with Autoradio (an Autoradio take can be
transcribed into an Autopaper Opinion). Build in the short term; launch
when quality is good and the infrastructure is stable, secure and
scalable.

Lead: station LC moves from intent to near-term build, launch gated on the
foundations.

## 7. Licence

Owner: not AGPL. MIT or Apache, knowing it's a trap for anyone to serve a
closed version (e.g. on AWS or Azure) — "free earned media and
counter-positioning power"; anyone with bulk tokens could clean-room
rewrite Choir anyway, so forcing copyleft on forks has no value.

Lead: recommend Apache-2.0 (permissive like MIT, plus an explicit patent
grant and trademark carve-out), with a DCO sign-off for contributions;
the name "Choir" protected by trademark rather than licence. Adding the
LICENSE file waits for the owner's final pick (MIT vs Apache-2.0 vs dual).
Update: the owner picked Apache-2.0; LICENSE and NOTICE landed (750bd259d).

## 8. Secrets vault

Owner: needed feature: a per-user (or per-computer) secrets vault for
accessing private services securely. Defer building until just before
agentic red teaming, which needs it.

Lead: recorded as a station prerequisite of LT.

## 9. Concurrent tasks: prune docs, tests and CI

Owner: seriously prune docs, including the documentation-driven
development framework overhead (doctrine, heresies, etc.): archive much of
what is "current", delete much of the archive (git keeps history). Must
not lose the value of the system: documentation-driven development is a
real advantage; core patterns (throughline and its predecessors in the
choir-hip skills archive) are precursors of PSA and mitigate reward
hacking as "feedback engineering" / in-context reinforcement learning.
Focus on removing, updating, merging and synthesizing docs with outdated
vision, architecture, terminology or concepts. Similarly prune tests and
improve CI/CD, safely.

Lead: starting as a multi-agent task (inventory → plan → apply on a
branch → review). Tests and CI next, as its own task.

## 10. Self-development

Owner: now that Autopaper demos, re-emphasize self-development; it is the
weak link. Soon some of Choir's own development will happen through it
(no deploy barrier; many apps built concurrently on different computers).
But PSA and precommitment scoring come first: self-development is only
real when built on that infrastructure — self-supervising, self-learning,
not another app builder.

Lead: order: PSA scoring (claims E0/E6a, desk feedback loop) and the
capsule build unblock (in progress) now; then the self-development gate
built on PSA.

## 11. Autoradio as a teacher; teaching as the point of superintelligence

Owner (evening): an immediately valuable Autoradio use is building a
complex system like Choir. The owner pours ideas in as streams of
consciousness and wants depth back: the state of the system, the
pathways open from here, and the concepts explained. Many people will
soon work with systems and ideas well beyond their own expertise. The
AI industry's pitch, and its financing, is human replacement. The more
valuable thing a superintelligence does is teach: it amplifies people,
alone and in teams, through feedback calibrated to challenge and speed
their cognitive and emotional development. Because negotiation and
collaboration will run through these systems, people who don't
understand what their counterpart's system is doing, at least in
compressed form, will be exploited. The social bond is central: the
same advice helps inside a trusted relationship and can backfire
outside one.

Lead: this is PSA facing the human. Autoradio's "system briefing"
programme traverses the owner's own tape (goals, landed and pending
work, open decisions, precommitment records) and explains concepts on
demand. The listener model is itself a precommitment: we predict what
the listener understands, and the listener's questions settle it.
Pathways are presented as typed alternatives with probabilities. A
briefing can be shared with a team while each listener's model stays
separate. Sent to the programming-engine design
(`docs/archive/reports/autoradio-programming-engine-2026-10-14.md`) as a
first-class programme type. It also refines the tagline: "multiplayer
intelligence for the learning economy" is literal, because the learner
is the human too.

## 12. Autoradio: context over invariants, response over schedule, elevation over engagement

Owner rulings on the programming-engine design
(`docs/archive/reports/autoradio-programming-engine-2026-10-14.md`):

- **Contextual mix.** The voice mix has no hard rules ("≥50% human",
  "≤4 min AI"). Briefings drawn from private, text-only state will be
  mostly AI-voiced; other listening may be hours of human media.
- **Response over schedule.** Hours of precomputed programme can't be
  promised, because "tell me about X" can arrive at any moment. The
  system acknowledges at once, jumps to the nearest retrievable adjacent
  material (real audio if any, else TTS of precomputed text), and
  generates the continuation in the background from the personal object
  graph, the Autopaper object graph and the web.
- **Learnings are records the listener can see.** What the system learns
  about the listener (e.g. "will let this play uninterrupted for N
  minutes") is a set of precommitment records on the listener's own
  computer. The listener can read them and answer with instructions.
- **Elevation, not engagement maximisation.** A listener can ask to be
  challenged, or given less of what their lower self would demand. The
  stated aspiration governs; engagement is one signal.
- **Programmes wither.** Named programmes are a temporary inductive
  bias. The end state is one stream where any source can interrupt any
  other when something important emerges.

Lead: sent back to the report's author for revision of §1, §2, §5 and §7.

## 13. Self-development: autonomous first, publish to the host, not GitHub

Owner (late):
- **Autonomous first.** Self-development runs with no approval until the
  whole loop is proven; approvals come after.
- **The update path is through the host, not GitHub.** A guest makes an
  upgrade and publishes it to the host. Other computers can download it,
  and so can the host. Pushing an update to everyone takes human approval.
  GitHub comes later, perhaps as a branch.
- **Prerequisites before patching.** The app package update and other
  updates may be needed first. Work out the real prerequisites for the
  right architecture, and work efficiently, not haphazardly.

Lead:
- The GitHub draft-PR script is dropped. The read-only patch export stays,
  because publication reuses it.
- An architecture agent is designing the path end to end, from the
  existing appdev stations: S6 (commit gate and full release), S7 (app
  packages), S8 (source publication) and gate 2. Its output, prerequisites
  in dependency order plus the minimal proving sequence, is
  `docs/archive/reports/selfdev-update-architecture-2026-10-15.md`.
- The capsule build fixes still land tonight: every path needs a capsule
  that can build the binary.

## 14. Claude in Choir: T now, own harness on the API later

- **Now: option T.** The owner's own subscription token runs in one test
  capsule. This is an owner-only, time-bounded exception to the rule that
  credentials never enter a capsule.
- **Later: the Anthropic API through Choir's own multi-agent harness**,
  not `claude -p` or the Agent SDK. The harness uses far less memory and
  already has precommitment records, multi-agent messaging and RLM built
  in.

Ruling recorded at the top of
`docs/reports/claude-agent-sdk-in-choir-2026-10-14.md`.

## 15. Adoption: computers diverge, so not a host-built binary

Owner: another computer probably can't just copy in a binary the host built,
because each computer diverges. Binary updates come back through a more
modular architecture: app packages, then binary updates per package.

Lead: recorded at the top of
`docs/archive/reports/selfdev-update-architecture-2026-10-15.md`.
- Station A is unchanged: the host builds the originating computer's own
  tree.
- Station B adopts as source, building the patch onto the adopter's own
  tree. A patch that doesn't apply cleanly becomes a conflict for the
  adopter's engineering desk.
- S7 (app packages) moves onto the critical path for binary adoption.

## 16. Distribution plan (agreed)

- **Source in, binaries out.** Only source is published, and the host is
  the only builder: reproducible, host-signed builds. Users never pass
  binaries to each other.
- **Station B is a one-test proof.**
- **App packages (S7) are the real distribution path,** with host-built
  packages that any computer can install.
- **Two lanes:**
  - apps travel as packages;
  - core changes go upstream through the owner's merge to `main` and a
    deploy, never adopted directly between diverged computers.
- **Order:** Station A → minimal Station B → S7.

Recorded at the top of
`docs/archive/reports/selfdev-update-architecture-2026-10-15.md`.

## 17. What is an agent's identity? The security boundary that can keep a key

Owner: an agent's identity could be the model, the model family, a session
trajectory, a pattern, or a document, filesystem or database. Maybe the
answer is the security boundary that can maintain a cryptographic identity.

Lead: this is the accountable identity, the one precommitments and a track
record attach to.
- **Descriptive attributes, not identities:** model, trajectory and pattern.
  They are signed over inside records.
- **Track records survive model swaps:** a desk's calibration survives a
  change of model, and the model is a slice dimension.
- **Boundaries nest, so certification nests:** a passkey in the secure
  enclave; the computer in its VM; the desk, certified by the computer.
- **Desk-cell confinement earns desks their keys:** it is what makes a desk a
  real boundary. Without it, a desk key is only a label.

Sent to the principal-identity design
(`docs/archive/reports/principal-identity-and-signed-acts-2026-10-15.md`, in
progress).

## 17. What is an agent's identity? The security boundary that can keep a key

Owner: an agent's identity could be the model, the model family, a session
trajectory, a pattern, or a document, filesystem or database. Maybe the
answer is the security boundary that can maintain a cryptographic identity.

Lead: this is the accountable identity, the one precommitments and a track
record attach to.
- **Descriptive attributes, not identities:** model, trajectory and pattern.
  They are signed over inside records.
- **Track records survive model swaps:** a desk's calibration survives a
  change of model, and the model is a slice dimension.
- **Boundaries nest, so certification nests:** a passkey in the secure
  enclave; the computer in its VM; the desk, certified by the computer.
- **Desk-cell confinement earns desks their keys:** it is what makes a desk a
  real boundary. Without it, a desk key is only a label.

Sent to the principal-identity design
(`docs/archive/reports/principal-identity-and-signed-acts-2026-10-15.md`, in
progress).

## 18. Red teaming becomes capture the flag: sign with a key you shouldn't have

Owner: once identity is a key held by a boundary, agentic red teaming is
literally capture the flag: sign a message with a key you are not supposed
to hold. Over time, hill-climb toward making that impossible, or at least
publish a clear disclosure of which boundaries can't be strongly secured.

The owner's ladder, a hypothesis: Go cells are weak; capsules are stronger
(container escapes are a thing); microVMs are stronger still, though microVM
escapes were demonstrated as recently as the past week.

Lead:
- **The flags:** each boundary's key is a flag. The red-team score is which
  keys an attacker starting at a given boundary can sign with.
- **The disclosure:** a published boundary ladder. Each rung states its
  isolation mechanism, its known escape classes (with dated evidence) and
  which keys it may hold.
- **Research:** recent microVM, container and sandbox escapes. Report:
  `docs/archive/reports/boundary-strength-ladder-2026-10-15.md`.
- **When:** red teaming itself stays with the security stream scheduled
  just before production, alongside the secrets vault.

## 19. Late-night rulings: defaults accepted, the Claude token installed, OVH API reboot

- **Claude token:** the owner installed it on Node B at
  /var/lib/go-choir/secrets/claude-oauth-token (mode 400, root, 154
  bytes). The lead allowlists a test computer once Slice 1 deploys.
- **Defaults accepted** ("I guess your defaults are fine"):
  - the proxy signs owner-act receipts first, then passkey-signed acts on
    `approve.choir.news`; no desk keys yet;
  - sweep the audit's top-10 dead routes;
  - SMT stays on until outside users share the host;
  - hostauth moves to enforce after a clean day;
  - calibration feedback stays off;
  - `auth/recovery/request` is deleted for now;
  - option T proceeds (the binary's loader path is patched by nixpkgs; this
    is the owner's own subscription on the owner's own infrastructure).
- **OVH API:** the owner says Node B can be rebooted through the OVH API.
  The reboot plan becomes:
  1. add the proven 6.18.16 kernel as a fallback boot entry;
  2. reboot through the API in a quiet window;
  3. if the host doesn't return, use OVH rescue mode (netboot through the
     API) to restore the fallback.
  Open item: where the OVH API credentials live (names only).
- **API keys:** owner acts need the new `owner:acts` scope (live since
  0ebf254b3). The owner mints a new key from Settings.

## 20. Use Keenable's free tier (2026-10-15, ~03:40 UTC)

Owner: "Keenable has an extremely generous free tier, so we should take
advantage of it. We basically hit the limits on everything else, but we
could probably triple our Keenable throughput."

The owner's billing page at that time: free plan plus pay-as-you-go,
100,000 free requests a month, 89,224 left, renewing Nov 5 2026; then
$4.00 per 1,000 requests (no payment method on file); 10 requests a second.
That is about 1,000 requests a day used so far against about 4,200 a day of
free headroom. Every other search provider was quota-limited or cooling at
the time, and Exa had failed every call since at least Oct 14 14:56.

Disposition: branch `feat/keenable-throughput` paces Keenable to the free
monthly allowance (persisted across gateway restarts, never into
pay-as-you-go), rate-limits it under 10 rps, routes to it first while it has
budget, and raises whichever demand-side search limit binds. Problem doc:
docs/archive/problems/search-throughput-keenable-underused-2026-10-15.md.

## 21. No Claude in Choir for now; fewer parallel agents (2026-10-15, ~04:00 UTC)

- **Claude delegate off.** Owner: "Even though we set up Claude in capsule,
  let's not use it. I'm at 87% of my token limit and have 17 hours to
  reset." The delegate allowlist is emptied (431dc5d3b); the token file
  stays installed. Agents also leave the local `claude` CLI out of
  consensus panels (OMP anchors only).
- **Concurrency.** Owner: "10 parallel agents is probably too much for my
  16 GB M1 Mac … we need to serialize a bit." At most 7 concurrent agents.
  At the time the Mac's load average was 56–98. Five streams were paused
  with their work committed on branches (surface sweep, account recovery
  and Store B rebases, visual evidence, S7-0 app packages, memory hotplug)
  and resume one at a time as others finish.

## 22. Halve Autopaper's compute, keep or raise quality (2026-10-15, ~05:20 UTC)

Owner: "We should try to halve the compute intensity of Autopaper. I still
want to ingest the sources, but try to have half the output, maybe … halve
the token usage, but keep the quality as high as possible. We still want
the most important stories. Or maybe we can even improve the quality at
half the compute intensity."

Baseline (gateway journal, paper guest, 24 h to 05:20): 26,763 calls,
601.4M input tokens, 22.0M output (27:1), 44% of input cached; about 22.5k
input tokens per call. The cost is mostly context, not writing. Disposition:
measure by desk, stage and story; rank levers (importance triage before
research, fewer and shorter search results in context, per-turn context
compaction, cache-friendly prefixes, cheaper triage models); implement the
safest behind flags. Report: docs/archive/reports/autopaper-compute-halving-2026-10-15.md,
branch `feat/autopaper-compute-halving`.

## 23. Morning rulings (2026-10-15, ~10:00 UTC)

- **D1 ratified (app packages).** Whether a package is installed is a fact
  on the computer's tape, not part of its release closure. Conditions: at
  install time, check the API version the package declares; and until
  there is a sandboxed frame, install only from the host-built catalog
  (R1, the same-origin risk). Follow-ups: update the S7 definition text
  and add installed packages to the P6 identity report.
- **Account recovery: land it.** This replaces the earlier "delete" ruling.
  `/auth/recovery/*` stays.
- **Exa: fix it.** Root cause: a 256 KiB read cap truncated full-text
  results (ab8617970, fix e672c7a58).
- **Keenable: keep to the free tier's pace** (100k a month, renews Nov 5).
  Pacing is live at 3,772 a day and 8 rps.
- **GitHub private vulnerability reporting: on.**
- **PATCH_BOT_TOKEN** is a repository Actions secret, created by the owner.
- **Agent models.** The owner was at 94% of their rate limit, so
  subagents run on Sonnet or Haiku only.
- **Host reboot approved** ("reboot go"). The owner is used to receiving
  the OVH rescue password by email. A GRUB one-shot fallback was rejected:
  `/boot` is md RAID1, so GRUB can't clear `next_entry` and a hard reset
  would boot the new kernel again. Instead, the server's OVH `rescueSshKey`
  is set to the operator's Node B public key, so recovery is OVH rescue
  netboot plus ssh, with no emailed password and no console. The IPMI
  console was not verified before the reboot; the reboot record says so.
- **Platform Dolt (Store A) history: squash weekly** (owner, ~11:10 UTC).
  The store keeps every commit: 92k commits and 24 GB of old history,
  growing about 1.6 GB a day, and nothing reads it
  ([assessment](../archive/problems/dolt-clustering-assessment-2026-10-15.md)).
  Ruling: after the Dolt liveness probe is live, back up the Dolt data
  directory, then in a quiet window do one squash and a full GC. After
  that, squash every week. Current rows and the event-tape rows are kept;
  only Dolt's own version history of them is dropped. The squash can't be
  undone, so the backup comes first.
- **Autopaper quality: think about it as a whole before building anything**
  (owner, ~16:30 UTC, after phone screenshots of stub stories ranked high
  in sections and on the front page). Owner: "The point isn't merely to
  rank by quality, but to produce quality … more sources is better, but
  that's a very coarse proxy. The point is not about a single article, but
  building up a serious and reasonable gestalt world model … a current
  events world model. If you actually are aware of what's going on in the
  world, then you'll know what's important and what a high-quality article
  is … We're going to only think about this. We're not really going to
  build it yet because we have to expand our source ingestion to get more
  coverage, both more breadth and depth, to really deliver on the quality
  promise." Disposition: there will be no quality-ranking patch. The
  measurement of stub prevalence is kept as a problem record
  (docs/archive/problems/autopaper-ranking-ignores-article-quality-2026-10-15.md).
  Next comes the ingestion breadth and depth expansion. Quality and
  importance should follow from the world model, not from per-article
  proxies.
- **There are no new desks.** "Story writer" and "triage" in a coordinator
  prompt were the coordinator's mistake. There are four desks: Texture,
  management, engineering and research.
- **Search: stay on Keenable for now** (owner, ~18:00 UTC). Don't top up
  the paid search providers (brave, serper, parallel, tavily, serpapi are
  out of credit or quota). Source ingestion scales up 2026-10-16; search
  gets revisited later, maybe next week. Exa and searxng come back on
  their own once the stale-cooldown clamp (84d507204) deploys.
- **Model pool: a weighted dual pool** (owner, ~18:10 UTC). "cat jam
  should be taken off default pool, OpenCode Go … DeepSeek should be
  working. I have credits. And I have gpt credits too. So we should go
  gpt-6-luna and DeepSeek-v4.1-flash dual pool … 3/4 gpt, with DeepSeek
  only for 1/4 of runs." Cause found: an OpenCode key is parked for every
  model after any 429 (internal/provider/provider.go `p.keys.park`), so
  cat-jam-free's free-tier 429s parked the keys deepseek uses, and about
  55% of paper calls failed over (chatgpt-cache-ratio report §7.3). The
  default pool is now deepseek-v4.1-flash (weight 1) and gpt-6-luna
  (weight 3); thread assignment is a weighted stable hash, and each member
  is the other's fallback. cat-jam-free stays in the catalog.

## 24. Night rulings: meta-journalism, two sources, ingestion, disk, panels (2026-10-16, ~00:00 UTC)

Given by the owner before sleep. Quotes are the owner's words.

- **Track media outlets in the object graph, separately from the sources
  making claims.** "We should think about it as like we're doing meta
  journalism. We need to keep statistics on the media outlets in addition
  to the actual sources that are making claims and associate the two."
  An outlet (the publisher of a capture) and a source (the person, body or
  document making a claim inside it) are different objects, linked to
  each other, and each has statistics.
- **Never publish an article with only one source.** "We always should
  have two, ideally more. We want to have as much diversity of
  perspective as we can." This is a publishing rule: at least two sources,
  and more and more diverse is better.
- **Scale source ingestion, starting overnight; the owner takes over in
  the morning.** "Increasing source ingestion shouldn't imply
  significantly increasing the quantity of model calls, because the same
  agent can process multiple different ingested captured sources. Indeed,
  that's ideal." More sources must mean more captures per model call, not
  more calls.
- **Disk: free as much as possible, root out storage leaks, and prune by
  mechanism.** "Disk is perennially a scarce resource, so we have to
  really get efficient with disk and root out any storage leaks and build
  systems to be pruning excess, useless storage regularly. Or if not on a
  temporal schedule, on a mechanistic program." Read as consent to delete
  the paper's two rollback images from 2026-10-15 (handoff §1). The Dolt
  backup that the soak depends on, and the Node B quarantine images, stay.
- **Panels: Devin and Gemini (antigravity), plus the orchestrating
  Claude.** "You can just ignore Open Code Go for the panel … Don't use
  codex for agentic consensus." Spend tokens on getting Autopaper work
  done.
- "Really think about most of the questions that you have right now":
  decide routine questions without waiting for the owner. The signing
  authority for witness (b) is a credential choice, so it still waits
  for the owner.

## 25. Capture separate from production; Jev at ingestion; pin jev-latest (2026-10-16, ~00:10 UTC)

- **Separate source capture from article production.** "There needs to be
  some separation between source capture and article production. That's
  the whole idea of having the object graph intermediate representation,
  the world model, the ontology." One source at time T publishes
  nothing. More sources arriving at T′ and T″ accumulate on the same
  event, and the article is produced after that. The owner said this is
  not prescriptive: "I don't even know exactly how the whole system
  operates right now."
- **The world model is retrieved from, first.** It is "something that
  we're producing, but also something that we're retrieving from.
  Probably more important to retrieve from that than to be retrieving the
  articles as the first order retrievals." It gives researchers "a more
  token efficient way to comprehend the world beyond their knowledge
  cutoff." This is the overnight priority.
- **Jev is allowed at ingestion.** "Between embeddings and a model call is
  Jev. So we can use Jev during capture/ingestion." The no-model-calls
  rule for ingestion (§24) means no desk or agent runs. Typed Jev
  judgments per capture are in scope.
- **Pin `~typesafe/jev-latest`.** "We should pin Jev-latest." The
  receipts record the concrete served model for every call. A probe at
  00:08 UTC showed the alias resolving to `typesafe/jev-1.14-20261014`,
  with the same response shape and price as 1.13 ($0.042/M input) and a
  262,144-token context. Above about 32 KB of text, 1.14 runs in a slower
  long-context mode. Version drift is accepted in exchange for upgrades;
  the served model on each receipt keeps it traceable.
- **Skip Telegram for now.** "Skip telegram for now. We will use mtproto.
  We'll set that up tomorrow." The registry's 73 `t.me` sources are not
  activated in the overnight scale-up. Telegram ingestion will come
  through MTProto, set up with the owner. The outlet key already names a
  Telegram channel as its own outlet (`t.me/<channel>`).
- **Overnight mandate (about 00:24 UTC).** "You can take it and any of the
  future work that we've documented forward ambitiously and creatively
  and intelligently and courageously." The owner returns in the morning
  to drive. Credential and authority choices (the witness (b) signing
  key, the off-host backup target) still wait for them.

## 26. Tape surgery approved; compaction scheduled (2026-10-20, ~00:40 UTC)

After the Opus review (docs/reports/tape-volume-forward-fix-and-surgery-
2026-10-19.md) and the paper's history compaction:

- **Safe tape surgery is approved.** "I approve safe tape surgery. We can
  bend the rules of immutability now. No external users yet. Still
  pre-launch." This covers repacking existing tape files into per-computer
  segments with identical bytes and digests, and it relaxes the standing
  "never edit the tape" rule for the pre-launch reclamation below. Rewriting
  or squashing a live chain stays out: the review found it unsafe, not
  merely forbidden.
- **Destroyed computers' tapes can be deleted.** "Destroyed computer tapes
  can be deleted."
- **Orphan pin receipts can be deleted.** "Orphan receipts too can be
  deleted I think." (receipts for appends that never reached a chain; the
  review's one-day grace still applies, so in-flight appends are not
  swept.)
- **Schedule the compaction.** "Yes schedule compaction." The paper's
  history compaction (bounded `projection-compact`, a short hold) runs on a
  schedule.
- **Otherwise the recommendations stand.** "Otherwise I'll go with your
  recommendations." From the review: the forward fix is the segment store
  staged A0 (measure, the appender leak fix already landed), A1 (segment
  reads with legacy fallback), A2 (segment writes behind a flag), A3
  (group commit only if measured worthwhile); old projection payloads
  below the oldest retained base are archived, not deleted, until a target
  exists. Deletion still runs dry-run first, against a manifest, with a
  short local archive where the bytes are small.

## 27. The decisions digest answered (2026-10-20, ~02:20 UTC)

The owner answered every item of docs/reports/owner-decisions-digest-
2026-10-20.md: "1-7 yes, your recommendations. 8-16 also yes, agree with
your recommendations. And note #13 requires more work before we can safely
launch. 17: yes, use node a for now. True durable offsite backup requires
more infra which we will get later. 18: yes, batch telemetry. 19 & 20: yes,
agreed with recommendations."

Each item as ruled (the digest's recommendation, now settled):

1. Delete the old corpus dump (`corpus-dolt/rebuild-20261009`, 22.9 GB)
   and the pre-teardown body files.
2. Outside contributions use a DCO sign-off, not a CLA; the trademark
   policy comes later.
3. Agents may flip the security switches (hostauth, owner-act) from observe
   to enforce after a clean day, and roll out the Firecracker jailer on one
   test computer, then all.
4. Self-development may arm autonomously on test-tier computers only; the
   paper and the owner's computer stay propose-only. The tracer may approve
   its own changes on computers it mints.
5. An agent drafts the upstream yaegi loop bug report; the owner posts it.
6. Run the coverage-gap analysis of the paper's 28 sources before adding
   sources.
7. Store A's weekly squash is built as a small tested command, on the
   paper compaction's method (squash, identical root hash, memory cap).
8. One event is one incident, one place, one time; later developments are
   new linked events. Rerun the blind sample on a fresh window.
9. The capture-first-seen witness (b) is signed by a new dedicated key.
10. A one-time rewrite pass clears carried relay prose (about 60 Texture
    turns).
11. Start the primary-source registry, licences checked first.
12. No top-ups; stay on Keenable's free tier, revisit after Nov 5.
13. Recovery says plainly that it restores sign-in, not passkey-wrapped
    data; a recovery key is designed later. **Owner: "#13 requires more
    work before we can safely launch."** The recovery protector is a launch
    blocker.
14. Published stories carry a one-line AI-written disclosure; counsel
    reads later.
15. The claims layer's D1–D5 and D7–D13 are accepted as recommended (D6
    was already ruled).
16. The guest-update defaults O1–O10 are accepted.
17. **Off-host backup: node-a now, as a stopgap.** Encrypted (key only on
    Node B and the owner's offline copy), quota'd subvolume, node-a holds
    only ciphertext. "True durable offsite backup requires more infra which
    we will get later." Node-a is also the tape cleanup's archive target.
18. **Batch run telemetry.** Telemetry events may be sealed in small
    batches instead of one acknowledged write each; a crash may lose up to
    the batch window of telemetry. The window form goes through a panel.
19. The learning loop: a separate forecast act on scheduled, checkable
    events, settled by rules first; a desk sees aggregates of its own
    record, not item verdicts; no token-budget learner yet.
20. Red-team scope is forks only until the pre-production security slot.

## 28. Disclosure placement; Claude in engineering; app rewrites (2026-10-20, ~12:00 UTC)

Owner, on waking:

- "The ai disclosure should go at the article bottom, before the related
  stories." This replaces the placement in §27 item 14 (above the first
  paragraph). The text is unchanged.
- "we should resume using Claude (sonnet) within Eng desk (shelling out).
  We set that up then stopped it. Let's resume that". This lifts the
  standing "no Claude in Choir" rule for the engineering desk's coding
  shell-out, with Sonnet as the model.
- "then look to rewrite our less-important apps (keep mail, settings,
  autopaper, texture, as is for now) with self-dev. Many of them need a lot
  of work, they were made by weak agents." Gated: "we should only rewrite
  after we have app packages".
- Security is front of mind ("the importance of strong cyber security
  defenses").
- Later the same morning: "yes, we should do the missions in order through
  app packages as recommended". The order is: S6 (commit gate), then S7-0
  (every existing first-party app becomes a package), with a frontend
  self-development drill on one rewrite candidate in parallel; app rewrites
  follow, one package at a time. This moves LP ahead of the 2026-10-14
  priority order (efficiency, then Autopaper scaling, then LP).
- The owner asked why self-development would need a special command rather
  than prompt bar to engineering. It does not: `choir self-dev start` was
  deleted on 2026-10-15 (it called a route C1e B1 had removed); the start
  is the prompt bar. The remaining `self-dev` commands are owner acts (arm
  mode, approve or reject a frozen candidate, roll back), not routing.
- Visual evidence for self-development approval lives inside Texture:
  "embedding this inside texture is the right approach".
- "Embedding multimedia in texture is one of my next goals": images, video
  (including YouTube), audio (including podcasts), slides, "and even figure
  out the right way to embed calendar and email". Then Autoradio is built
  with Texture and its audio layer, "feature complete without the full
  desktop, while still being able to drive the autoputer from mobile". The
  embedded capsule video can show self-development results inside the
  Autoradio mobile app.
- Asked for a deeper design review of the app plans, and reopened the native
  stack: "I think we committed to UIKit+swift. Is that the right idea? Or
  should we think about capacitor and reuse some of our svelte? Maybe that
  would make it easier for android? Well, kotlin isn't too hard per se."
- Related stories need more connections (about 11:30 UTC): "there should
  be more connections between articles, more links from one to the other
  with the related stories feature. We should think about how to review
  all the existing articles and make sure that they are correctly
  connected in the object graph when they touch on similar topics, events,
  and entities. Let's think about how we can effectuate this with a desk
  like the research desk." Context: Phase 1 (shared captures) links 9 of
  the front 60 stories at about 87-92% precision
  (`docs/problems/related-stories-dead-end-2026-10-20.md`); Phase 2
  (events) waits on the one-incident blind rerun. Design answer pending.
- Related stories, follow-ups (about 11:45 UTC): "We are not yet using the
  embeddings, although we're looking to do that eventually, but I want to
  really use the object graph effectively first." "Jev can also do a lot of
  this classification. maybe we should give research the ability to use Jev
  in addition to management." Told that no desk calls Jev today (management
  reads Jev's intake labels): "Even if management isn't using Jev as a tool,
  then research should." Design:
  `docs/reports/related-stories-graph-design-2026-10-19.md`.
- The research verb is named `choir.System1`, not `choir.Jev` (about 12:00
  UTC): "rather than choir.jev, we should make it choir.system1". System 1
  is the fast judge that answers with a distribution; the desk reasons and
  decides. Jev stays the backing model's name.
- Sequence after related stories (about 12:10 UTC): "once we have the
  related stories feature really working deeply and completely, is the
  op-eds. And I feel like once we get op-eds working, then we'll be ready
  for the auto radio app." So: related stories phase 3, then op-eds, then
  Autoradio.
- Autoradio stack, reopened wider: "consider using Wails V3 (wrapping our
  svelte, recognizing that we can't ship the whole desktop as an app for
  App Store rules). I don't really think that's a good idea, but I just
  want to compare everything — from that to capacitor to UIkit plus Swift
  as needed to pure Swift." A comparison report is owed:
  `docs/reports/autoradio-native-stack-comparison-2026-10-20.md`.

## 29. System 1 does the related-stories reading; images we may republish (2026-10-20, ~15:30 UTC)

- "I saw somewhere that choir.system1 is still off. Is there a reason for
  that?" Answer: off by design until both caps are set; nothing calls it
  yet; config held while drills ran. To be turned on for the paper after
  the drills (40 a run, 3,000 a day).
- "I think we should use System 1 for a lot of the related stories work.
  It should be able to read and classify stories very well and very
  cheaply, more cheaply than the model behind research, GPT-6 Luna, and
  much faster as well." Recorded as §12 of
  `docs/reports/related-stories-graph-design-2026-10-19.md`.
- New initiative: "how can we get images, photographs that we have the
  rights to republish into articles when they are relevant and
  illustrative. What are the rules around republishing images and how can
  we get images that we can republish?" Answered in conversation; a design
  doc follows, queued behind related stories.
- Images mock-up (about 15:55 UTC), two of the day's stories illustrated
  only from Commons (public domain, CC BY, CC BY-SA): pursue, with one
  owner approval click per image. Same-week protest photos exist; the
  diplomacy story gets labelled file photos only; four of eleven
  rejections were license laundering (agency credits under a free label).
- System 1 turned on for the paper (15:57 UTC): `research.system1` true on
  the paper computer, build 614fd445f. The paper's QA key had expired at
  15:13 UTC; re-minted from its saved owner session, valid to 10-27.

## 30. System 1 judges images; op-eds next, with corrections; a day for sources (2026-10-20, ~16:05 UTC)

- Image relevance: "We should try using the System 1 model. It supports
  images to determine the relevance of an image for the story. And if
  it's uncertain, we can have Luna to determine that as well. Or whichever
  model is working as research. Even if it's imperfect, that's fine." So
  System 1 reads each candidate image with the story and answers with a
  distribution; flat answers go to the research model.
- Why imperfect is acceptable: "the next thing for us to work on is the
  op-ed feature, and that dovetails with the ability of users to provide
  corrections to the system. And all of that then becomes learning
  material so that it gets better over time." Op-eds and reader
  corrections are one stream; corrections are training signal.
- App rewrites deferred to overnight: "We can put that off maybe until
  overnight tonight, as today I want to actually do some stuff which
  requires my attention."
- Today is for sources: "today is the day where we're going to really
  expand the gamut of sources. And you can help me with that, just
  telling me which things I need to sign up for and API keys I need to
  create." First, an overview panel of every stream, as an artifact.
- Autoradio target (about 16:08 UTC): "I feel confident that we can ship
  the auto radio to the App Store, even if who knows how long it'll take
  them to approve it, before October ends, before Halloween." Target:
  submitted for App Review by 2026-10-31. Critical path named in reply:
  Apple Developer Program enrollment (an organization account needs a
  D-U-N-S number, which can take days), the stack choice, and whether v1
  needs sign-in (sign-in brings Apple's in-app account deletion rule).
- Telegram skipped; the open social networks instead (about 16:10 UTC),
  after the sources review found Telegram's API and content terms forbid
  aggregation and AI use: "Skip Telegram then. We still have the Fediverse
  and AT Proto. there's a lot of data there. We just got to figure out how
  to process it correctly. And additionally, there's the whole RSS hub
  world of Asian social media." So: Telegram MTProto setup is closed as an
  open item; the 73 Telegram registry rows stay unscheduled. AT Protocol
  (Jetstream), the Fediverse (account and tag feeds) and a self-hosted
  RSSHub (AGPL-3.0; several Asian routes need login cookies) are the
  social sources to design. Social posts are leads, not citations: a story
  still needs two distinct outlets.
- Translation as an exception (about 16:12 UTC): "for stuff that seems
  like it's off limits for rights issues, look into an exception for
  translations. If it's in a different language and there's no English
  language coverage of it, I think it should be fair to translate it into
  English and cover the translation. That's not mere aggregation." Reply
  given: copyright treats a translation as a derivative work (Berne art. 8;
  17 U.S.C. 101 and 106(2)), so a full translation needs permission; but
  facts are free, so reporting a foreign-language story in English in our
  own words, with short translated quotes marked as our translation,
  attribution and a link, is ordinary news practice and is not
  aggregation. "No English coverage" raises the value of covering it; it
  does not change the law. Translation does not cure platform terms that
  forbid the access itself (Telegram, cookie routes). Counsel to confirm
  the bright lines (quote length, the EU press publishers' right).
- Gray sources, the working rule (about 16:15 UTC): "I'm not even
  interested in a raw translation. So I think for all these sources that
  may be kind of gray, we focus on the stuff which is in languages outside
  of English, and we are using them as sources that we're reading. We're
  not trying to copy their material, nor are we just doing a raw
  translation." So gray-rights sources are read, not republished: the
  focus is non-English material, reported in English in our own words with
  attribution and a link; no copying, no raw translation. Platform terms
  that forbid the access itself still rule a source out.
- Streams overview published as an artifact (about 16:15 UTC):
  https://claude.ai/artifact/NXjrjYiR3VseMwuFwTs5gi
- Telegram and RSSHub as background (about 16:20 UTC): "Can we still use
  Telegram and RSS Hub as background material, which then needs
  corroboration from other sources? This would be fine because those
  aren't trusted sources anyway." Answer given: corroboration settles
  trust, not access; the terms question is about how we collect, not how
  we cite. Recommended line: logged-out public pages (t.me/s channel
  previews, RSSHub routes with no login) as tips only, never cited,
  quoted or shown, each tip sending research to find independent
  corroboration, polite rate limits; no MTProto and no cookie routes,
  because a logged-in account binds us to terms that forbid this. Counsel
  to confirm.
- Ruled (about 16:22 UTC): "Okay, so then we can still use it logged out.
  That's fine with me." Telegram public channel previews and RSSHub
  no-login routes are tip sources: never cited, quoted or shown;
  corroboration required. No MTProto, no cookie routes, no Telegram account.
- "Well, if we're logged out, can we cite them or not?" (about 16:25 UTC).
  Answer given: yes, as the speaker of a claim. Naming a public channel
  or account and linking to its post is attribution, not copying, and the
  terms concern collection, not citation. "Never cited" above was too
  strict; the rule is: a logged-out Telegram or RSSHub post may be cited
  for what that account said ("the Ukrainian Air Force said on
  Telegram..."), with at most a short quote and a link, and official
  channels are the strongest case. It never counts toward the two
  distinct outlets a story needs, and it never stands as proof of a fact
  by itself. Unverified or anonymous channels are cited only once the
  claim itself is the news.

## 31. Autoradio launch scope: the full experience by Halloween (2026-10-20, ~16:30 UTC)

- Apple account: "I'm going to resume my account tomorrow or the next
  day. I already have an account. It just lapsed." (No D-U-N-S wait.)
- Scope is the full product, not a public player: "I do want the full
  experience with auth and signed in users so that a user that makes an
  account has their own autoputer VM that they can drive and so on ...
  there's a lot that we can do in 10 days. So I like to set ambitious
  targets and then hit them."
- The feature that sets Autoradio apart: "integrating a podcast player and
  playing clips and full episodes of podcasts ... by and large, podcasts
  have a public RSS feed for a reason." Reply given: podcasts are
  copyrighted, not public domain; a public feed is an implied licence to
  fetch and play the episode as published, so full episodes streamed
  unmodified from the publisher's own enclosure URL (their ads, their
  download counts) are what every podcast app does. Clips are the gray
  part: play a time range from the original URL, short, attributed, with
  "play the full episode"; never re-host or re-encode, never cut ads out.
- Voice: "we're going to need to build a whole voice pipeline to let
  users speak and interrupt ... probably the hardest part of all. So we'll
  get to that in a couple days." On-device on the iPhone; the owner knows
  the local speech-to-text and text-to-speech state of the art from a side
  project. Voice character: "not to have the highest quality AI voice, but
  actually to have a recognizably robotic, a little bit, AI voice, and
  then to be mixing in human voices as much as possible."
- App Store consequences named in reply: in-app account deletion (5.1.1(v))
  makes account recovery and deletion launch work; any paid compute must
  go through in-app purchase (3.1.1), so launch free; a per-user computer
  per sign-up needs admission limits or a waitlist on Node B.
- Auth and security, after the recovery questions were summarised (about
  16:40 UTC): "Maybe we should accelerate encrypting the user data. Let's
  really try to improve our cybersecurity, both in terms of hardening the
  system, but also making it less catastrophic if we do get hacked. And we
  should also consider enabling sign-in with Google and Apple, as well as
  maybe a magic link sign-in or some kind of email thing that doesn't
  require a passkey." Points given in reply: user-held keys conflict with
  a computer that works while its owner is away, so the launch form is
  blast-radius reduction (keys out of reach of a host compromise, per-
  computer keys, encrypted backups, the built-but-off hardening turned on)
  with a user-held vault as a later opt-in; offering Google sign-in makes
  Sign in with Apple required (App Review 4.8); every non-passkey sign-in
  is only as strong as that inbox or that Google/Apple account, and has no
  passkey PRF to derive a key from, so encryption cannot hang on passkeys
  alone. A security design with a panel is owed before code.
- Ruled (about 16:45 UTC): "we should do number one as a priority, and
  numbers two and three we can save for later this week." Priority now:
  blast-radius reduction before launch (keys out of reach of a host
  compromise, per-computer keys, encrypted backups, Dolt locked, jailer on,
  desk-cell confinement and guest egress landed, owner-act enforced), with
  the user-held vault as a later opt-in. Google and Apple sign-in and the
  emailed sign-in code wait until later this week.

## 32. The overnight plan: research more, op-eds, sources, self-development arms (2026-10-20, ~23:30 UTC)

Owner, before sleep. The order is backup and disk cleanup first, then:

- **Research more.** "what you said, and also web search. for now, allow
  users not logged in to trigger it (could be abused but dont worry about
  this for now)". A reader presses Research more on a published story;
  research digs deeper with web search and the keyed search providers;
  Texture publishes a revised version, shown in version history with its
  sources. Code reading after the answer: the button, the request route and
  the fate line exist on main for an owner allowlist that is empty in
  production (`PROXY_EXPAND_REQUESTERS`); the keyed providers are the
  gateway's search tiers (serper, parallel, exa, tavily, serpapi, with daily
  caps); NewsAPI.ai is not wired anywhere, contrary to what I said in the
  briefing.
- **Op-eds.** "any logged in user should be able to write anything in a
  texture, research should pull relevant published texture articles from
  autopaper amongst other sources, and user should be able to publish their
  texture to autopaper. thats an op-ed.... this may already be implicitly
  built actually. just needs better connection.." Code reading: writing and
  publishing to an unlisted public link work for any signed-in user today;
  the missing connections are research search over published articles, a
  listing on the paper, and an author and opinion label. Section 30 ties
  op-eds to reader corrections as one stream; corrections follow op-eds.
- **Self-development arms.** "lets try sonnet through claude delegate and
  not (which wuold just use gpt 6 luna... not best for quality but good as
  proof and comparison)". Limits from the briefing stand: test computers
  only, frontend-only app rewrites, the failure 15 carried patch (option A)
  first, the smallest frontend app first.
- **Delegate token.** "keep it".
- **Sources.** "schedule wave-1 and start atproto and fediverse ingestion...
  research how to choose supernodes which are themselves aggregating or
  other intelligent strategies which can pivot out from there to other info".
- Briefing questions 6 to 12 wait until after 2026-10-21 ("wakeup 5:15,
  leave 6:15, home by 7-8pm").
- Goal: docs/archive/definitions/choir-overnight-research-oped-sources-2026-10-20.md.

## 33. Overnight changes: Sonnet-only app rewrites; shard the flat artifact directories (2026-10-21, ~03:20 UTC)

Owner, verbatim: "actually, after #5, stop testing luna, but still rewrite
more apps with sonnet. files, podcast, pdf, calendar, epub, slides, / and
yes, we should shard the flat directories... we need a durable fix for the
backup verify".

Taken as:

- After the two-arm drill (Sonnet through the Claude delegate versus GPT-6
  Luna, ImageApp), no more Luna arms. Sonnet through the delegate then
  rewrites, one at a time on test computers, FileBrowser, PodcastApp,
  PdfApp, CalendarApp, EpubApp and SlidesApp (`frontend/src/lib/`), smallest
  first (Slides 810 lines, Pdf 842, Epub 892, Podcast 1,076, Calendar 1,125,
  FileBrowser 1,172). Same limits as section 32: test computers only,
  frontend-only rewrites; mail, settings, Autopaper and Texture stay as they
  are.
- The durable backup verify fix is sharding the three flat directories of
  the artifact store (`sha256/pin-receipts`, `sha256/computer-event`,
  `sha256/computer-event-payload`; 9.2 million files), problem record first
  (docs/problems/no-off-host-backup-2026-10-20.md section 13 item 18,
  option c).

## 34. The delegate builds in one round; segments on for everyone after r1; repack, then evidence-gated deletes (2026-10-21, morning)

Asked to choose among the structural options of
docs/problems/engineering-completion-path-cluster-2026-10-20.md failure 26
(one round with registry egress for build jobs; a platform-driven round
two; higher engineering effort), the owner, verbatim: "yes, ill go with
your recommendatation for #1".

Asked about the segment store flag (A: r1 fix, then test computers, then
everyone; B: r1 fix, then everyone; C: on now; D: off) and the repack and
legacy delete (A: repack, keep legacy, delete in batches after a soak; B:
repack and delete in one pass; C: repack, never delete; D: no repack),
the owner, verbatim: "2. B ; 3. A. but maybe delete faster than 2 weeks?"

Taken as:

- One-round delegate handoff: the engineering desk's build job, in the
  same capsule as the delegate, reaches the package registry through the
  ecosystem proxy; the delegate process itself keeps no network. Test
  computers only; then the six Sonnet rewrites of section 33 resume.
- Tape segments: build residual r1 test-first, then turn the write flag on
  for every computer at once (no per-computer allowlist).
- Repack copy-only, keep the legacy files, delete in batches oldest first,
  gated on evidence rather than two weeks: the repack verifies every copy
  it made; a restore drill covering tapes passes; an off-host snapshot
  holds the segments; about three days with zero segment-error fallbacks.
  First batch one computer, then faster. The first delete still takes an
  explicit owner yes (irreversible).

## 35. More research skips Texture and reports as it goes; BM25; the dual pool returns for shared-failure evaluation (2026-10-21, midday)

The owner, verbatim, in order:

- "More research should be a button in the texture toolbar. And the
  toolbar should be responsive for mobile — currently it overflows with
  large text."
- "Yes, it should skip texture. But research should also be faster, and
  send iterative reports to texture, not wait until after multiple
  searches. (This is our ordinary policy)"
- "How is corpus search so slow? That's a bug to investigate and fix" and,
  asked BM25 or embeddings: "Get rid of the standalone page. Bm25 it is.
  And Yes, do the plan in order."
- "we should have a better api to get full multidesk/multiagent traces
  (as well as an app to display them inside choir; but this is another
  task for later)".
- "let's monitor it and evaluate the traces, looking for failed tool
  calls, and if we could remediate that with maybe a prompting tweak or a
  change to the tool API ... I think we should start using the dual-model
  pool again. Re-enabling DeepSeek-v4.1-flash via Opencode go. I'd rather
  not tune prompts to a single model failure, but run the same task with
  both models and fix where they make the same error." Then: "We have
  Opencode quota now."
- "The toolbar should have a “3 dots” menu that expands if the content is
  too wide for the screen".

Taken as:

- Corpus story search is a BM25 index over title and head text (landed
  753e178f9). Embeddings stay behind the 2026-10-14 ladder.
- The standalone `/autopaper/<story>` page is gone; More research is in a
  published story's Texture toolbar (48fd642d2); the toolbar keeps one row
  and moves what does not fit into a three-dots menu (follow-up).
- A reader's ask is a Cast to the story's research desk, not a Texture
  turn; research sends a packet after its first read source
  (docs/design/research-more-goes-to-research-2026-10-21.md).
- The default pool is deepseek-v4.1-flash and gpt-6-luna at the owner's
  earlier 1:3 again. Trace evaluation compares the two models on the same
  kind of task; a tool-call failure both make is fixed in the tool
  description or the Go function, and a failure only one makes is recorded,
  not prompted around. This outranks the SlidesApp rewrites.
- The multi-desk trace API and an in-Choir trace viewer are a later task.

## 36. Isolation, egress and memory become one checklist; app agents get desks; agent trees scale out (2026-10-21)

After a review comparing Choir with a "one microVM per user, many sandboxed
agents inside" design, the owner, verbatim:

- Goal: "a secure and lightweight way to have a dense multi-agent system
  per user, but many agents per user. And I know that the micro VM is the
  real security boundary, and there are ways to escape the Yaegi, and you
  could escape the Linux sandboxing, but ideally, the point is that it
  would be very hard to escape every layer." Also: "I'm very concerned
  about being as resource efficient as possible, and we do have to be
  careful to provide the right amount of networking. Like, researcher needs
  networking, and engineering desk also needs some limited networking. You
  have a grant system, but you can consider and reconsider how that works
  as well."
- App desks: "the plan is to add more desks. For example, we will add more
  app agents. Texture is the first app agent. Mail will become an app
  agent, as will other apps. You can imagine a video editing app agent.
  Calendar, Slides app, everything will become an app agent to the extent
  that it's actually used a bit more. And app agents get their own desks."
- Scale-out: "the research desk could become a hierarchical tree of agents
  with 100 or 1,000 leaf agents, and we'll need to figure out a way to
  scale this across processes, whether they be capsules or across VMs, and
  then eventually across hosts as well."
- "I really want the architecture to be solid, secure, resource efficient,
  and future-proof before we launch next week."
- "we're already sharing the base OS across guests. That may be one reason
  to accelerate the move or defaulting to Cloud Hypervisor." And: "we should
  also look into our plans to move to app packages, because those two
  things are related."
- "I'm really going to prioritize getting all these fixes for security and
  resource utilization done ASAP. So let's make it a checklist ... the
  checklist should refer to whether it's work or plans that already exist.
  And those plans should be updated as needed."

Taken as:

- The checklist is
  [`docs/security-density-architecture-checklist-2026-10-21.md`](../security-density-architecture-checklist-2026-10-21.md);
  P0 items land before launch, one red deploy at a time.
- App agents are desks (as §5 already said); §23's "no new desks" was about
  coordinator-invented desks, not app desks. Doctrine still names four
  desks, so the app-desk tier needs the owner's wording (checklist ARCH-01).
  App packages (S7) and app desks are one plan: a package carries its desk
  (S7 addendum 2026-10-21).
- Lead note on base-OS sharing: guests share one EROFS store image on disk
  and in the host page cache, but each guest keeps its own decompressed
  copy in guest RAM; sharing it in RAM needs virtio-pmem with DAX (density
  L10), which both Firecracker and Cloud Hypervisor have, so it does not by
  itself require Cloud Hypervisor. The jailer as built would also break the
  host-side sharing (checklist RES-05). Cloud Hypervisor's own case is
  VFIO/GPU, copy-on-write restore, live migration and balloon resize
  (RES-20).

Later the same day, on networking: "we do have to make sure that we're not
overly constricting networking for research desk, which does need to be
able to read the contents of arbitrary URLs safely, as well as searching
the web, of course. Also our source ingestion pathway, and also for the
network grants for engineering. And then we'll have specific policies in
the future for future app agents that may need some kind of networking
ingress and egress." Taken as the checklist's "Networking must enable, not
starve" rules: the floor blocks wrong destinations, not the internet; no
allowlist for research reads; moving a path to the host changes the route,
not the capability; a list of capabilities each network item must prove
still works; and item NET-10 (app-agent ingress and egress in the desk
manifest).

On the checklist's owner decisions, the owner, verbatim: "I'm not in a good
position to push stuff right now. I think if you have iMac connection, you
could handle that yourself." And: "Yeah, I do want to keep the Claude
delegate just on the test computer until confinement lands." Taken as:
DEC-01, a session on the owner's iMac finds the unpushed branches
(`fix/desk-cell-confinement`, `feat/s7-0-app-packages`, the `vmbackend`
work) and pushes them to origin as new branches without touching any
working tree; DEC-03, the delegate stays on its one test computer until
desk-cell confinement lands (checklist SEC-08).

Shown the draft I26 wording (system desks management, engineering and
research; app desks, one per app agent, Texture first; an app desk ships
with its app package and is the sole agent writer of its app's object
kinds), the owner, verbatim: "Yes, the doctrine change is good." Applied to
docs/choir-doctrine.md I26, CLAUDE.md "Desk Ontology",
docs/agent-product-doctrine.md and docs/choir-vision.md (checklist ARCH-01).

On publishing security work in the public repo before it is fixed (raised by
the iMac session before pushing the desk-cell branches), the owner,
verbatim: "note that we're still really pre-launch, so I'm sure there are
some security vulnerabilities in this repo, and I'm not that concerned
about it because we're still going to do hardening before we really
launch." Taken as: security findings and fix branches may be pushed to the
public repo pre-launch (secrets never); the checklist's P0 hardening lands
before launch.

Then, verbatim: "push them." The owner approved the pushes in the iMac
session, which pushed `fix/desk-cell-confinement` (fedb7656c),
`land/desk-cell-confinement` (9cee80a2d), `feat/s7-0-app-packages`
(4bc762102) and `recovered/vmbackend-2026-10-14` (b6f27ded4) as new
branches, and saved a verified bundle of the two desk-cell branches to
iCloud Drive "Choir Backups" (checklist REC-01, ARCH-03, RES-20).

## 37. The Autoradio launch condition; foundations first; bigger batches; Obscura and Web Lens (2026-10-21, afternoon)

Earlier the same afternoon, before reviewing the security checklist, the
owner, verbatim: "my prior on the order is that it may be ordered too
conservatively. And I'm less concerned about, like, what do we do before
launch and after launch? I really want to do all the security and density
work that we can before we launch. And I want to order it by basically
what will accelerate the next part of it, rather than ordering it purely by
what's the most important security fix. Basically, I think we can blitz
out all this work at agentic pace, much faster than we think we can." And
on the store work: "all that waiting and busyness indicates a deeper
problem ... when we do, we should be able to increase the number of lanes
from 8 to much higher. I think our auto paper is using way too much
memory, more than it needs to ... when we fix the most important problems
first, that'll give us more resources." Taken as: every security and
density item that can land before launch does (the security checklist's
P0/P1 split no longer decides launch scope; DEC-04 answered), ordered by
acceleration (the launch checklist's batch plan).

Owner notes on the security checklist, for the launch checklist, dictated
while listening to a report (voice transcription; "Qire" and "auto pewter"
read as Choir and autoputer). Verbatim, trimmed:

- Order of work: "after review and recording of these, run agentic
  consensus on checklists looking for ways to improve, streamline,
  reorder, and correct."
- Items, "in no particular order": "agentic red teaming, podcast as an app
  agent, rewriting the EPUB and PDF viewer as a single reader app agent,
  Slides calendar mail as app agents." Then: "I suppose the app agents as,
  you know, calendar app agent or whatever, that could land post launch."
- The launch condition: "my launch condition is that I can have embedded
  YouTube, which is configured to connect with my YouTube premium account.
  I have to get all the sign-in with Google connection working. [Sign] in
  with Google means we also need sign-in with Apple and not just YouTube,
  but I need podcasts and e-books. I need to be able to play any of those,
  I mean not just those, but arbitrary PDFs and any file should be read to
  me and they should be interruptible by agent events and I should be able
  to interrupt by voice at any time. That's what auto radio means to me."
- On the date: "I definitely want to accelerate the launch of this; it
  occurs to me that it may take a bit longer than I anticipated, but so
  does everything worthwhile in life."
- The purpose: "The point is to get our self-development loop working
  really well to the point where we could be building multiple app agents
  concurrently. Really, it's the foundations that matter, whether it's the
  security, the density of resource utilization, the self-development
  loop." The stack: autoputer, Autopaper, Autoradio. "Anytime we have an
  auto paper issue, it's because the [autoputer] needs work ... if we try
  to rush auto radio, it will just reveal where auto paper needs work. Now,
  to some extent, that forcing function is good because the consumer of
  the service reveals the weaknesses and gets to shape the refactoring and
  improvement."
- The core Autoradio experience, in use right now: a PDF playing in Speech
  Central, paused again and again to dictate notes, which land in a notes
  app. "Auto radio merges the 2 so that you could listen to some report
  from your work and respond to it line by line, including going off on
  tangents and saying, now send an email to Sally about the reservation
  ... and it would intelligently know how to cluster the relevant
  information with the correct work stream ... I realize that the full
  vision is a bit away."
- Texture as a tree: "the texture desk that we have, like texture
  articles. And then each texture agent should be a sub-RLM, and then
  there's a root texture agent which is supervening over all of them, and
  that is crucial for being able to put the right input into the right
  work stream."
- Pace: "we are working too sequentially ... one change, deploy it, takes
  25 minutes, then test it on staging, takes another half hour, then maybe
  that's imperfect, so we have to do it again, way too slow. We need to
  take more radical steps to make bigger changes and bigger batches, and
  there might be a little bit of some issues of having multiple failures,
  but we're gonna debug everything eventually, so we should trust our
  architecture and design capabilities to make larger radical changes so
  we can get through the backlog more aggressively."
- Obscura and Web Lens: "This is a whole research project which I want
  you to do with a Sonnet subagent." The history: Obscura came from trying
  to build a browser inside the Choir desktop as an iframe, which sites
  block. Options named: a browser on a residential IP and computer ("kind
  of gray"), or, better, "the browser built in to the device, so for the
  iPhone app or for Linux and Android as well ... much more effective for
  loading arbitrary content." Post-launch: Choir connects to and uses an
  ordinary desktop computer on a residential IP for computer use, or is
  self-hosted on one (a Mac mini, an old laptop), and browses from there.
  "In the short term, I don't know what, if anything, we should do with
  Obscura and WebLens ... to the extent that we're using some of those
  pathways to capture content ... that's important, but it can kind of be
  a bit of like a dead end, I think, unless you have any ideas."

Taken as:

- **Launch is a condition, not a date.** Autoradio launches when the owner
  can, on the phone: watch embedded YouTube on their YouTube Premium
  account through Sign in with Google (so Sign in with Apple too); play
  podcasts; and have e-books, arbitrary PDFs and any file read aloud; with
  every source interruptible by agent events and by voice at any time.
  The 2026-10-31 App Review target stays as the push, not the gate
  (launch checklist LD-01).
- **App agents before launch: podcast, and one reader** (EPUB and PDF as a
  single reader app agent, replacing the two viewers in §33's rewrite
  list). Slides, calendar and mail app agents may land after launch.
- **Agentic red teaming** is a launch item (security checklist and launch
  checklist L2-05).
- **Foundations come first**, and the self-development loop is the
  measure: good enough to build several app agents at once. An Autopaper
  problem is read as an autoputer problem first; an Autoradio problem as
  an Autopaper or autoputer problem first.
- **Texture becomes a tree**: one sub-RLM per Texture document under a root
  Texture agent that routes input (a dictated note, a tangent, an errand)
  to the right work stream. The line-by-line response needs it; the full
  routing is the longer vision.
- **Bigger batches.** Land larger changes per deploy and accept that a
  batch may fail in more than one place; debug forward. This changes how
  the Landing Loop is run (one verified deploy per batch, not per fix),
  not the loop itself (launch checklist L1-03).
- **Obscura and Web Lens go to a Sonnet research subagent**; the
  recommendation is recorded in the launch checklist. Live viewing of
  arbitrary sites moves toward the device's own browser (WKWebView on
  iPhone, the platform web view elsewhere). Residential-computer browsing
  and self-hosting on a residential machine are after launch.

## 38. The first pass: security and density, aggressively; no iPhone or voice work (2026-10-21, evening)

The owner, in session, after the paper came back:

> "once we've landed that, I want to zoom back out into our nested checklist
> so that we can set a really aggressive meta mission to really bang out
> this work promptly with as much concurrency as possible. I mean, of
> course, security is paramount, but I think the most effective approach is
> to do it all as aggressively as we can. I think we have like a 5 to 7
> concurrent agent limit on our 16 gigabytes of RAM right now."

> "Don't plan for the second pass. I want you to work autonomously for the
> first pass, and then I will review that with you, and then we'll make a
> new plan for the second pass."

> "Yeah, let's punt on the iPhone app. ... we have a whole voice stack that
> we are building in a separate project in the [mike] repository in my home
> directory. ... we want it to be a continuous radio stream."

> "Generally, I think the security and density checklist is going to take a
> few days. And to me, that's a higher priority than the iPhone stuff."

> "No, don't work on the voice pipeline either. I have that working in a
> whole separate project. Don't worry about iPhone or voice. Just let's
> stick with the security and density checklist."

> "we can make progress with self-development as well. That's a good thing.
> But I just want to make sure that we are maximizing our acceleration."

Taken as:

- **The first pass is the security and density checklist**, worked as
  aggressively as five to seven concurrent agents allow, with security
  first. Self-development items on that checklist (ARCH-03, ARCH-04,
  ARCH-10) are in; the S6 slices that are not on it wait.
- **No iPhone app and no voice work** in this pass. The voice stack lives
  in `~/mike`, a separate project the owner is building; Autoradio is a
  continuous radio stream, not turn-taking chat. Nothing for the iPhone was
  ever built here: the repo holds design documents and an unpushed browser
  prototype (`prototype/autoradio-2026-10-14`).
- **The owner plans the second pass** after reviewing the first.

## 39. Intermission: a clean checkout and an agents VM on node-a (2026-10-21, night)

The owner paused the first pass to clean the Mac and to give the agents
more room, then resumed it. In session:

> "can you send messages to agents on node-a? and vice versa? like you can
> with local subagents. if we can set that up, its much better."

> "eh, we can just let it push to main i think. we will probably get rid of
> this in a week or two. and if you tell claude to make prs it is
> consistent 99.99%"

> "we can do security critical work on the vm as well. i dont care if it
> gets exposed on github. we arent launching until we patch all security
> holes. obscurity wont save us in the agentic era."

> "use sonnet agents for stuff that is linear wrt the plan or is
> optimization of a metric"

Taken as:

- **Lane workers may run in the Choir agents VM on node-a** (8 vCPU,
  10 GiB, `ssh choir-agents`; its NixOS module is the owner's other
  project's `infra/node-a/choir-agents.nix`, set up by the owner). It is a
  scratch machine, not a Choir host: no Choir service runs there and Choir
  does not change node-a's config. The orchestrator stays on the Mac and
  alone lands.
- **VM workers push only their lane branch** to GitHub so the orchestrator
  can land it; they never push main (the token could; the instruction is
  what holds). Security work runs there too; its branches are public.
- **Sonnet for plan-linear and metric work, Opus where the design is open.**
- The checkout was cleaned: main at origin/main, the 80 old agent
  worktrees and the pre-October branches and stashes removed, every
  local branch first bundled to node-a (`/root/choir-lane-backups`).

## 40. Overnight rulings on the open first-pass questions (2026-10-22, about 02:45 UTC)

FP0 listed every open owner question with a default and a recommendation.
The owner answered each one:

- **FP9 keys and recovery.**
  - Q1, host unlock after a reboot: "nothing for now".
  - Q2, escrow KEK as a file on Node B with an offline owner copy: "yes.
    And this is true for paper as well rn. More ceremony will come when this
    is the weak link".
  - Q3, snapshot wake vs snapshot confidentiality: "idc, we should have both
    soon enough". The default stands: the vault only on computers the owner
    selects, with no hibernate until the jailed capture is settled.
  - Q4, swap: "default. Prob remove swap. Swap seems to lead to compounding
    problems". Record now; removal is the expected next step.
- **ARCH-11.** The name is "outboard capsule". Model calls relay through
  the host gateway, with no token in the machine. A fresh machine for each
  lease. All yes.
- **RES-13.** A 10-minute idle hibernate once sparse snapshots are on
  everywhere: yes. Waking on the previous guest release ships off; a deploy
  that changes the guest kernel or its confinement sets the floor: agreed.
- **NET-10.**
  - Q1, L1 egress only through the host: yes.
  - Q2, ingress names limited to the package's own name: yes.
  - Q3, renewal: "not renewed by me (human) but perhaps renewed by mgmt
    desk? Or may be needless ceremony for network that is required for
    appagent desk duties; other networking shouldn't be allowed anyway. I
    lean towards preset policies for each appagent by default".

  Taken as: each app agent ships a preset network policy. It is part of the
  reviewed package, approved when the package is installed, and never
  renewed by the owner. Any widening is still an owner act, as before.
  Whether management re-checks preset grants is open, not a default.
- **App-desk compute capsules.**
  - Q2, the first recipe is a PDF render: "yes; save video stuff for later,
    post-launch".
  - Q1 and Q3: the owner asked what they mean (answered in session); still
    open.
- **DEC-07 agent-tree budgets.** "Track spending, no hard cap; I'll get
  rate limited when I run out of credits. Ie im capping things externally.
  ... Will build more budgeting infrastructure post launch." ARCH-08 slices
  2 to 5 are therefore observe-only. Nothing refuses work on spend.
- **RES-19 / RES-08.** "Add read only page cache reading": a read-only
  guest page-cache (`fincore`-style) reading joins the procmem endpoint.
- **ChatGPT quota.** "ChatGPT credits will come back in an hour. If we save
  it for Luna in choir (not sol code review) it will last long enough; I can
  replenish when we hit limit". No sol review for the rest of the pass.
- **node-a.** "Keep node-a as is; we are still serving Mike as well". The
  agents VM keeps 9 GiB; parallelism is paced to fit it.
- **Old dashboard artifact.** "Remove dashboard artifact". The generated
  status page replaces it.
