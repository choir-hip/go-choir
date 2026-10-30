# Strategy Takeaways After the 10-14 Meeting (Owner)

Date: 2026-10-14, about 17:30 UTC. Owner's brain dump, structured by the
lead; owner's words quoted where they carry the point. Status: direction
for the roadmap; the stations named here are proposals until authored.

## 1. Architecture first

"As I always believe, the software architecture is the key. Building the
platform as we're doing, that's the foundation that lets business and
go-to-market emerge." Autopaper is not yet a business.

## 2. Three businesses, each with its own clock

| line | customer | what makes it valuable | clock |
|---|---|---|---|
| Data products (B2B) | funds, analysts, media | deep verticals (Taiwan first) and precommitment records: box scores and aggregate statistics | at least a month of operation after the architecture works, to build the database and track records |
| Autoradio (prosumer) | individuals | the growth engine (product-led growth): the interactive, two-way experience over Autopaper, podcasts and one's own work | the part that excites people the owner meets |
| Organizational operating system (enterprise) | organizations | hierarchical supervision of every agent the organization runs | probably bigger than the data products |

## 3. New research stream: precommitment records on historical data

Score historical statements from a later vantage point ("take Winston
Churchill in 1941 and score him from the perspective of 1942"). It is both
an evaluation and a product: track records and box scores built from
history, most valuable for the recent past, because those actors are
active today. Expect many iterations. Needs: historical sources, point-in-
time evidence, and back-testing without hindsight leaking into the scoring.
Research: docs/research/historical-precommitment-backtesting-2026-10-14.md
(in progress).

## 4. The organizational operating system

- **App agents first:** a phenomenal suite of apps as app agents, and
  self-development working (about the next month), after the security
  work, agentic red teaming and hardening, which come first.
- **Hierarchical supervision is the value.** One Texture supervises one
  task (which may involve research and engineering); the root Texture
  supervises all Textures in a computer; a further level supervises many
  computers ("Choir in Choir": one computer controlling another).
- **The deployed shape:** at least one computer per employee, plus
  computers for teams and projects, with hierarchical role-based access
  so a CEO, CFO, CTO, COO, CAIO, VP of engineering or a middle manager
  supervises all the agents in their purview, now and historically.
- **Not a dashboard:** an interactive surface to direct agents without
  taking them off track, and to talk to the humans driving them.
- **Autonomous background agents, interoperable:** chat agents (Claude,
  GPT, a personal or business agent) remain interfaces into Choir; Choir
  runs 24/7 in the background; Autoradio is Choir's own interface.
- **Why a third party:** an organization running 80% Anthropic and 20%
  OpenAI agents needs supervision across both, which neither lab is
  likely to build for its competitor.
- **Deployment:** cloud-hosted, self-hosted or colocated, with a
  forward-deployed-engineer approach using whatever the organization
  prefers.
- **The frame:** "the organization is a multi-agent system. Humans are
  one type of agent; language model agents are another," including
  external teams and stakeholders.

## Roadmap consequences (lead's reading)

1. Security and agentic red teaming before the app rebuilds (already the
   owner's order; docs/archive/reports/computer-isolation-security-review-2026-10-14.md
   in progress).
2. Self-development gate, then app agents (LP, S7-0) as the next month's
   main build.
3. Precommitment scoring infrastructure (the claims/events/entities
   design, docs/reports/claims-events-entities-ontology-2026-10-14.md)
   starts its clock early, because the track records need a month of
   operation to be worth anything; historical back-testing can seed them.
4. Multi-computer supervision ("Choir in Choir") builds on S9 forks and
   fleets, S10 org templates and S11 mainline pushes in the app-dev
   metamission, plus hierarchical RBAC (a new station to author).

## Owner correction, about 17:55 UTC: deepen foundations, don't accelerate growth

"Even though Autoradio drives growth, that's not a reason to accelerate
it. [...] Growth is actually not what we need. I don't see Autoradio as a
hypothesis that needs proving. [...] We need to deepen foundations." A
prototype can come when there is free capacity, ahead of production.
Target: Autoputer, Autopaper and Autoradio production-grade in about a
month. "Scaling up should be as simple as I just pay more and the
infrastructure scales." Get the software to diminishing marginal returns
for more human attention first; then product-led growth, go-to-market and
fundraising. The owner is not bothered by code outrunning verification or
decisions piling up (a decision list is coming), and wants maximum
progress in the ~26 hours to the Claude credit reset (Thursday 15:00).
On himself: "I'm working at my limit [...] how can I unbottleneck myself?
How can I accelerate my own learning?" — teaching documents per substrate
(like the isolation security review) and a daily digest.
