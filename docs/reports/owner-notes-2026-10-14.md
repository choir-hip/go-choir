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
(`docs/reports/autoradio-programming-engine-2026-10-14.md`) as a
first-class programme type. It also refines the tagline: "multiplayer
intelligence for the learning economy" is literal, because the learner
is the human too.

## 12. Autoradio: context over invariants, response over schedule, elevation over engagement

Owner rulings on the programming-engine design
(`docs/reports/autoradio-programming-engine-2026-10-14.md`):

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
  `docs/reports/selfdev-update-architecture-2026-10-15.md`.
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
`docs/reports/selfdev-update-architecture-2026-10-15.md`.
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
`docs/reports/selfdev-update-architecture-2026-10-15.md`.

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
(`docs/reports/principal-identity-and-signed-acts-2026-10-15.md`, in
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
(`docs/reports/principal-identity-and-signed-acts-2026-10-15.md`, in
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
  `docs/reports/boundary-strength-ladder-2026-10-15.md`.
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
docs/problems/search-throughput-keenable-underused-2026-10-15.md.

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
safest behind flags. Report: docs/reports/autopaper-compute-halving-2026-10-15.md,
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
  ([assessment](../problems/dolt-clustering-assessment-2026-10-15.md)).
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
  (docs/problems/autopaper-ranking-ignores-article-quality-2026-10-15.md).
  Next comes the ingestion breadth and depth expansion. Quality and
  importance should follow from the world model, not from per-article
  proxies.
- **There are no new desks.** "Story writer" and "triage" in a coordinator
  prompt were the coordinator's mistake. There are four desks: Texture,
  management, engineering and research.
