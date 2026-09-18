# Memo: Build Horizontal, Sell Diagonal — the Media Strategy

**Date:** 2026-09-17
**Status:** owner strategy direction, recorded. Not doctrine; the vision
doc remains apex. This memo carries the business layer the repo has not
had: what the technology is *for* commercially.
**Mutation class:** green (documentation only)

## The three phases

Owner framing, 2026-09-17: the product story is three phases, and the
automatic computer is not the product — it is phase one, the technology.

**Phase 1 — the automatic computer.** A different take on what an AI
system should be. The industry is clustered on one idea: an agent *has*
a computer it uses — the computer is the agent's fundamental tool, and
the chat thread is the agent's interface to the user. Choir inverts it:
the multi-agent system *is* the computer. AI framed not as replacing a
worker, but as replacing the computer.

**Phase 2 — the automatic newspaper.** The application that proves the
computer's value. A computer that updates its own code and works
continuously is cool — but what is it good for? Tracking what is
happening in the world. Two interfaces: a standard news UI (a news
article is just another document in the computer interface), and an
API/skill/MCP that other agents — Claude Code, ChatGPT — use to get
up-to-date information. Then the platform: anyone white-labels it,
combining our curated public data with their own private sources to
produce data products for internal teams, paywalled distribution, or
public dissemination — or any combination.

**Phase 3 — the automatic radio.** The upshot of the whole stack. A
voice layer on top of the newspaper: a more advanced podcast player you
can talk to. An AI DJ mixes clips and full-length podcast episodes with
text-to-speech of public and private documents; users record their own
takes with voice and distribute them to their audience — internal to an
organization or public. In the organization-internal form it creates a
new kind of multiplayer AI: teams work together in a meeting — or an
asynchronous meeting — that is simultaneously prompting a multi-agent
system working continuously in the background.



## The four layers

**Technology, strategy, platform, product** — separate things, kept
separate here.

- **Technology:** the automatic computer. Built, mostly — the durable
  substrate is proven on staging; the RLM runtime is mid-cutover.
  Standing alone it is either a solution in search of a problem or an
  open-source general-purpose technology. Neither valence implies
  business value. Its commercial role: the moat, and the open-source
  community that accelerates go-to-market for the data products.
- **Strategy:** build horizontal, sell *diagonal*. Media — like code and
  legal — is a diagonal that cuts across every endeavor of life, unlike
  verticals such as real estate or agriculture. Media is the attractive
  diagonal because it is *not* competitive the way legal and coding
  tooling are: every organization needs to know what is happening and to
  say what it knows; few see a media engine as a threat.
- **Platform:** enable others to build AI-enabled and AI-native media.
  Producers integrate public and private data and produce their own
  automatic newspapers, newsletters, and data products — exposed as
  APIs/MCPs/skills into existing agents, and as standalone agents that
  encapsulate data products: charts, graphs, reports, voice on demand.
- **Product:** the things people buy before the platform exists. Our own
  data products first — the automatic newspaper with an AI-focused
  edition — then the white-label version sold to orgs (finance and other
  industries) that combine their private data with the Choir object
  graph.

## The dependency chain

```text
autoputer  (the automatic computer — exists, mid-cutover)
  → autonews  (the automatic newspaper — the World Wire)
    → autoradio  (the voice projection — a projection OF autonews)
```

Autoradio is not a third product to build. It is the second surface of
the second product: the wire's object graph populated with structured,
provenance-linked content, plus a DJ desk plus audio plumbing that is
~80% open-source for MVP. The newspaper can launch *as* a station — "the
newsroom that broadcasts" — which is a more visceral demo of the same
substrate than another web page.

## Autoradio

Every producer of data products — including internal teams — will want to
encapsulate them in a voice layer. Not a turn-based chatbot: something
that *speaks* — an AI DJ mixing together human voice recordings — and
that can record human takes for distribution. Both a viable public
consumer product and an internal B2B organizational multiplayer AI
system.

**How it works (ontology-correct):** the AI DJ is a desk — an
encapsulation boundary. Internally it may be one agent or a thousand
(queue planner, mixer, take editor, music programmer); other desks see
one addressable actor and do not know or care. The DJ traverses the
**object graph** — the live projection of current state — to build a
queue: audio objects (podcasts, embedded YouTube clips) play natively;
text objects are read aloud via TTS. The tape records what the DJ did —
queued, played, mixed, superseded — because the tape records everything.
Content flows OG → queue → output; accountability flows action → tape.
The tape is backwards-looking; the DJ never "plays the tape."

**Legal shape:** playing podcasts and clips is a podcast player — clear.
TTS on text is clear. The only rights-sensitive surface is recorded human
takes, and that is the internal/B2B case where the org owns them anyway.

**Multiplayer:** humans record takes; the DJ mixes them in; a human take
can supersede the AI's segment — correction as an ordinary write, in
audio. Not a podcast tool, not a chatbot: a persistent organizational
radio station.

**Serving surface:** continuous audio out is the first product surface
that is not a request-response render — an infrastructure question, not
an ontology question, and the open-source audio stack covers MVP.

## The box score

A higher-level data product: **a box score for all public statements,
institutions, speakers, and figures.** A track-record ledger — every
public statement becomes a forecast with a resolution contract; the wire
scores it continuously; figures accumulate track records. This is PICL
made consumer-facing, and it is the thing impossible without the tape: a
wrapper can summarize the news, but it cannot maintain a persistent,
provenance-linked, resolvable record of who said what and whether they
were right.

**Sequencing:** the box score has a data cold-start problem — it needs
months of resolved predictions before it is interesting. Autoradio ships
first with a simple programming algorithm (recency, topics, prefs); the
box score accretes underneath as resolutions accumulate, then *upgrades
the algorithm* — airtime allocated by track record. Autoradio gives the
box score its distribution surface; the box score becomes the editorial
judgment layer of the station. It is also its own product and API.

## The funnel and the network

- Own data products first: the automatic newspaper, with an edition
  focused on AI — the owner's domain: tracking the happenings, covering
  the reports and aggregators, sensing what others are sensing.
- Readers/consumers → producers: support internal use, paywalls, and
  optionally publishing individual articles into the choir.news
  aggregator — the network-effect seed.
- Stronger B2B funnel: data-product consumers → API/MCP customers →
  platform producers. Analysts and agents consume the data products;
  some want them on their own data; the platform serves them.
- Pitch to orgs in finance and other industries: combine private data
  with the Choir object graph to produce their own automatic newspapers.

## Honest risks

1. **Distribution is still distribution.** The substrate gives a
   production advantage and a provenance moat; it does not give
   attention. The box score's virality (arguing from track records is
   inherently social) is a better distribution mechanism than a news
   site.
2. **The AI edition is the most crowded beat in existence** — unless it
   leads with accountability (track records) rather than coverage.
3. **Box score cold start** — no data yet; autoradio-first sequencing
   exists precisely because of this.
4. **Voice rights** — only for recorded takes; design provenance in from
   day one.
5. **Platform before product is the classic failure** — hence: sellable
   data products first, platform second.

## What this memo is not

Not a re-sequencing of the vision. The order stands: supervised
self-development proven first, then the wire. This memo says what the
wire is *for* — and that its second surface (autoradio) and its judgment
layer (box score) are the business, with the platform as the endgame.

## Sources

- `docs/choir-vision.md` — the order: computer first, wire downstream.
- `docs/memo-autopaper-world-wire-generalization-codesign-2026-08-09.md` —
  the wire as generalization codesign.
- `docs/designs/choir-event-driven-rlm-ontology-minimal-2026-09-15.md` —
  desks, casts, OG, tape.
- `docs/reviews/picl-consensus-synthesis-2026-09-17.md` — the learning
  record schema the box score consumes.
- `docs/rlms-ontology-brief-2026-09-10.md` — forecast/adjudication/
  assessment objects the box score is built from.
- `docs/reports/choir-status-rlm-picl-worldwire-2026-09-17.md` — current
  build state.
