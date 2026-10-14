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
