# Pitch Bones — Choir

The textual skeleton. The deck (`choir-pitch-2026-09-18.html`) is a
function of this document — edit here, re-render there. One section per
slide; keep each to the words that belong on the slide.

Reframe (2026-09-18): Choir is **multiplayer AI** — a self-supervised
multi-agent system for complex, long-horizon, continuous work. The
automatic newspaper is the proof point; the automatic radio is the UX
that forces adoption. The through-line: the document surface is both
the supervision interface and the product interface — same surface.

## 1 · Title

CHOIR
Multiplayer AI.
A self-supervised multi-agent system for complex, long-horizon,
continuous work — proven by a newspaper, adopted through a radio.

## 2 · The inversion

AI that replaces the worker — or AI that replaces the computer.

The industry: AI replaces the worker. An agent has a computer it uses;
a chat thread is its interface to you.

Choir: AI replaces the computer. The multi-agent system *is* the
computer.

## 3 · The stack

General-purpose technology, sold as media.

We build a general-purpose computer but market it as media — not a
coding agent — for differential impact. All agents are coding agents;
the dev market is crowded, segmented, and sticky.

- **Technology** — the automatic computer. Open source, secure,
  auditable — orgs learn from their data.
- **Strategy** — build horizontal, sell ~~vertical~~ diagonal. Code,
  legal, media cut across every endeavor — unlike true verticals (real
  estate, agriculture).
- **Platform** — the white-label media engine.
- **Products** — the newspaper (proof), the radio (adoption), the data
  products.

## 4 · Phase 1 — the automatic computer

A persistent computer that works continuously and develops itself. An
RLM (recursive language model, cf. Zhang et al., MIT) coordinates many
communicating agents — persistent and ephemeral — over long horizons.
Every change a typed event; every surface a deterministic projection.

Open source because unownable by design — orgs won't accept a
proprietary layer between them and their private IP and learning.

## 5 · Self-supervision

Chat breaks at this scale.

Long-horizon, multi-agent work creates a complexity that breaks the
affordances of chat as UX — you can't supervise a continuously running
system through a scrollback.

The answer: agents supervise agents, and documents show the current
work state. The document is the control surface — and the same surface
the product is made of. What an agent writes, a human or another agent
reads, checks, and corrects.


## 6 · The learning loop (PICL)

Own your own learning loop.

PICL — predictive in-context learning. Before acting, an agent commits
a prediction; after acting, the outcome resolves it. Every committed
prediction becomes a provenance-linked record — improving the quality
of outcomes through learning, and safety through logging and
monitoring.

Four abstractions: **Security** · **Auditability** · **Customizability**
(vibecode your own apps — multiagent workflows and APIs inside the
autoputer) · **Recovery** (if anything goes wrong you can always
recover — your data is safe).

## 7 · Phase 2 — the automatic newspaper (the proof point)

The same document surface that supervises the system, published
outward. An article is a work-state document — sourced, written,
verified, corrected, published around the clock.

Three interfaces: for people, the article is a document on the
computer; for orgs, API access to the object graph — a curated,
provenance-linked record of what is happening, plus your private
sources; for agents, a skill/MCP for up-to-date information.

## 8 · Phase 3 — the automatic radio (the adoption UX)

The same surface, in audio — and doubly custom. An AI DJ learns from
your usage and stays grounded in the provenance-linked record; you can
interrupt it, talk to it, record your own takes.

Interruptible (talk to it, steer it — not a one-way broadcast) and
grounded (learns from usage but stays anchored to the verified record —
personalization without losing the facts).

## 9 · Multiplayer

The opportunity after autonomy is coordination.

Most AI systems are designed to reduce the amount of human
intervention required. Choir increases the amount of human input that
can be productively absorbed — a meeting becomes an input to a
persistent multi-agent system already doing the work. Decisions update
priorities; disagreements are preserved; commitments propagate.

AI not as a replacement for humans, but as leverage — an organization
with higher coordination bandwidth, not higher overhead.


## 10 · The box score

Of course you keep score.

A provenance-linked object graph already knows who said what — the
obvious next layer tracks whether they were right. Public and private:
the public system tracks the discourse; the private system tracks
internal decision contributions. Every statement becomes a forecast
with a resolution contract; figures accumulate track records. Matures
into the station's editorial judgment — and its own data product and
API.

## 11 · The privacy answer

Your data stays home. Managed under zero-data-retention, or self-hosted
open source ± consulting. The gamut: hobbyist free → data products
(API access to the object graph, the box score) → managed →
consulting. PICL makes open source a community-driven gym.

## 12 · Where we are

Prototype to production. The prototype autoputer and autonews needed
refactors for secure continuous operation — that work is landing.

## 13 · Team

**Yusef Mosiah Nathanson** — founder. Self-taught software and AI
engineer — came to AI in 2015 from professional poker, when Noam
Brown's Claudico and the poker AIs started beating top players.
Building Choir in the open, aggressively using many agents.
github.com/yusefmosiah · linkedin.com/in/y-m-nathanson · mosiah.org

## 14 · Close

Your computer, made of agents.
Tune in to what it says.
Open source. Live on staging. Phase one is landing; the newspaper is
next; the radio is the payoff.
The ask: your read — on the business and on this pitch. Where does it
break? What would you push on?
github.com/choir-hip/go-choir · choir.news
