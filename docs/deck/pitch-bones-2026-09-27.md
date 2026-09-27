# Pitch Bones — Choir (September 27, 2026 revision)

Content spec for `choir-seed-deck-2026-09-27-carbon-kintsugi.html` and
`choir-seed-deck-2026-09-27-london-salmon.html`. Same broadsheet design as the
September 25 edition — same type, same two palettes, same slide furniture — with
new content from `choir-deck-treatment-2026-09-27.md`.

Type: Didot masthead, Iowan Old Style for headlines/deks/body, mono for rails,
labels, and contacts. Palettes verbatim from `frontend/src/lib/theme.ts`:

- **Carbon Fiber Kintsugi** (dark): bg `#0B0C0D`, fg `#F2EFE7`, muted `#B2AA98`,
  subtle `#766F62`, accent `#FFD86B`, accent2 `#FFF1BC`, border
  `rgba(255,216,107,0.18)`, borderStrong `rgba(255,241,188,0.48)`, tetramark `#FFE18A`.
- **London Salmon** (light): bg `#FDF1EE`, fg `#3A1517`, muted `#755B56`, subtle
  `#AD9088`, accent `#9C5852`, accent2 `#244F4A`, border `rgba(156,88,82,0.16)`,
  borderStrong `rgba(91,28,31,0.2)`, tetramark `#682A28`.

Tetramark appears on the first and last slides of both editions. Flat theme
paper — no background textures.

## Voice and evidence rules (from the treatment)

- Headlines marked **FOUNDER** are verbatim; **DRAFT** headlines carry a visible
  `Draft — founder to rewrite` badge and must not be polished into slogans.
- No invented numbers. Every on-slide figure traces to a research task (R1–R16)
  or appears as `[TK]`. Sources recorded in `choir-seed-deck-2026-09-27-notes.md`.
- Banned: `X isn't Y. It's Z.` constructions, stacks of one-line maxims,
  "ungameable", "unprecedented", "revolutionary", "cryptographic proof of intent",
  "Cursor for writing", hedge-fund wedge, characterizing lab leadership.
- Stage honesty: pre-traction, pre-production, live on staging. No traction
  claims, no pilot numbers, no customer quotes.

---

## 01 · Cover

**Rail:** choir.news · September 27, 2026
**Wordmark:** CHOIR
**FOUNDER headline:** AI, a new species — of computer
**FOUNDER subtitle:** Introducing the automatic computer
Nothing else on the slide.

---

## 02 · The CLI and the GUI

**Section:** THE SHIFT
**FOUNDER headline:** The chat-based agent is the CLI. Now it's time for the GUI.
**Terminal row (four windows):** ChatGPT · Claude · Claude Tag (in Slack) · Meta Muse — each drawn as a terminal: prompt line in, text back, state hidden.
**Body:** Chat is a command line. You type a command, text comes back, the state stays hidden, and you learn the incantations — prompting is command syntax.
**Note:** The CLI never died; power users still live in it. The GUI opened computing to everyone else, and that is where the market grew by orders of magnitude.
**Speaker note (R10):** "Text is the universal interface" — McIlroy/Unix; but chat is not the only kind of text. Documents are.

---

## 03 · The story

**Section:** THE STORY
**DRAFT headline:** I wanted agents to make work that compounds. I got drafts that regenerate slop.
**Beats (mono kicker + text):**
- SUMMER 2025 — Built an AI ghostwriter because I wanted to be prolific. Models stitched together by hand; the output beat any single model.
- THE TURN — Building it, I realized I did not want a ghostwriter. With the right tools — research, citation, analysis, understanding, writing, multimedia — I could produce quantity and quality myself. The goal moved from replacing the writer to equipping one.
- THE FAILED REBUILD — Tried to rebuild it on an agent-graph framework. Agents do not follow a flowchart; they discover the workflow as they go.
- THE SLOP LOOP — Talking to chat is easier than editing, so corrections go through regeneration. Each version fixes one thing and breaks another. Corrections do not stick, so nothing compounds.
- THE REALIZATION — Why give agents a computer when you can center the computer and give it agents. First prototype on localhost in about twenty minutes.
**Before/after block:** THE CHAT LOOP — correction → regeneration → new slop. / THE DOCUMENT LOOP — correction → revision → the work compounds.
**Background line:** Professional poker, then AI since 2015.

---

## 04 · The product

**Section:** THE PRODUCT
**DRAFT headline:** The work itself is the interface.
**Lead figure:** Texture screenshot — agents update a versioned document; the user edits it directly; the edit is the message and steers what happens next.
**Status list (as of 2026-09-27):** Texture — staging · Mail — staging · Newspaper — refactor · Radio — development.
**Side notes:** Mail sends are gated by owner approval. Re-check every status label on the day of the conference.

---

## 05 · State, not sandboxes

**Section:** THE DIFFERENCE
**DRAFT headline:** Everyone sells compute. Choir owns state.
**Blocks:**
- Dozens of sandbox and agent-computer companies offer essentially the same product, and every agent brings its own sandbox. (R7)
- Only Choir lets the user own accumulated work state and learnings regardless of which agent or model they use.
- The objection, pre-empted: vendors are adding memory import and export. Memory is a summary of what an assistant learned about you. Choir holds the work itself — versioned, and still being worked on by agents from any vendor.
**Proof strip (boxed):** Put a Markdown or PDF file into your Choir over MCP, then work on it from ChatGPT or Meta Muse. It stays live — versioned at the user level and the platform level, still being worked on by the multi-agent system.

---

## 06 · The engine

**Section:** THE ENGINE
**DRAFT headline:** A computer made of agents.
**Three columns:**
- THE HARNESS — Natively multi-agent, in Go. Desks with bounded capabilities (Texture, Research, Engineering, Management, Mail), each a recursive language model with its own context and imports. No root agent; coordination lives in the state.
- PRECOMMITMENT RECORDS — Agents commit to predictions before acting; outcomes resolve them. The same log is the learning signal, the audit trail, and the reputation record.
- MODEL-NEUTRAL — Cheap open-weight models for volume, frontier models where stakes are highest.
**Validation line:** Harvey and Baseten's September 2026 research on recursive harnesses for long-document diligence. (R5)
**Speaker note:** This is the only slide that mentions supervision.

---

## 07 · The newspaper

**Section:** THE NEWSPAPER
**DRAFT headline:** The newspaper is the demo.
**Blocks:**
- An app inside the web desktop, not a separate product. It reads the world continuously, keeps provenance on every claim, and publishes.
- It doubles as content marketing: the founder streams himself using Choir to cover tech, media, business, politics, and geostrategy. Viewers watch the work get steered by editing, not prompting.
- First series — Taiwan: reading the Taiwanese internet in its own language and platforms for Western audiences. Silicon supply chains, US–China relations, democratic legitimacy versus the authoritarian growth model.
- The hard part: labeling each source's political alignment and flagging coordinated or content-farm material. Taiwan is one of the most heavily targeted information environments in the world — the hardest available test of provenance. (R11)

---

## 08 · Autoradio

**Section:** AUTORADIO
**DRAFT headline:** The real platform shift is voice.
**Blocks:**
- After voice CLIs: Alexa and Siri were command, response, no persistent state. Autoradio is a living station you steer by interrupting and recording your own takes. (R8)
- Two-way radio has a proven social form — call-in radio built communities. Autoradio is call-in radio with provenance, where listeners' takes become part of the record.
- Delivery: a native mobile app plus an app inside the web desktop. Mobile puts Choir on the home screen, in the car, and on walks, where the daily habit forms. (R9)
**Pull (bottom-anchored):** Generate the visuals. *Preserve the voice.*

---

## 09 · Market

**Section:** THE MARKET
**DRAFT headline:** The CLI market is chat. The GUI market is everyone who works in documents.
**Figure row (four columns, all sourced):** the CLI market — paid AI subscribers (R1); the GUI market — paid seats in office and document suites (R3); multi-homers — people paying for two or more AI products (R2); the beachhead — people who write or publish for work (R4).
**Bottom-up line:** [beachhead count] × [price] × [defensible penetration] = [TK], every input sourced.
**Note:** The labs want users to single-home. Multi-homing shows none of them has won. Choir aggregates the user's work across agents and turns models into interchangeable suppliers.
**Restriction:** the installed-base "10⁹ automatic computers" chart is not the market proof; it may appear only as a small vision line.

---

## 10 · Business model

**Section:** THE BUSINESS
**DRAFT headline:** Choir owns the relationship by letting you own your state.
**Three columns:**
- THE CORE — Open-source runtime, self-hostable. Most will not self-host; the ability to leave is what earns the trust.
- THE CONVENIENCE — Hosted Choir. Premium surfaces (Radio, research, teams) built on the user's state, never a claim on it.
- THE NETWORK — choir.news, Autoradio, and citations across computers. Reach grows with each publisher.
**Analogy (note):** WordPress and Automattic — open source, self-hostable, most people use the hosted service, and it powers a large share of the web partly because no one fears lock-in. (R12)
**Pricing:** omitted pending founder decision.

---

## 11 · Why now

**Section:** WHY NOW
**DRAFT headline:** Agents are everywhere. Their work belongs to no one.
**Blocks:**
- The personal-agent wave of mid-2026 — Meta Muse, Instinct, Grok Bot, Town, and the OpenAI and Anthropic entries. Every one keeps the user's work in the vendor's silo. (R6)
- Chat is buckling under long-horizon work: public reporting of tens of thousands of problematic agent incidents across top labs, and OpenAI pausing training on its most capable models. (R13)
- Open-weight models are now cheap and good enough for volume work, which makes a model-neutral computer economically viable. (R14)
**Postscript note:** After OpenAI DevDay (September 29): if OpenAI announces a managed-agents platform or a consumer agent, add one line on how Choir relates — another agent that can work on Choir state. (R16)

---

## 12 · The ask

**Section:** THE ASK
**DRAFT headline:** Building the system that needs to exist.
**Framing:** Spending ramps with revenue. A low base burn keeps the company default alive. The raise is capital to move fast when demand shows up, not money to go looking for it.
**Money ask (boxed):** ≈ $1.0–1.25M — a runway floor plus a growth reserve. (Founder to confirm the final number.)
**Base plan table (pre-revenue, roughly $20K/month):**
- Founder salary — $10K
- Inference (open-weight volume plus frontier security bursts) — ~$3–5K
- Infrastructure and miscellaneous — ~$3–4K
- Contractors, amortized (pre-launch security audit, legal) — ~$3K
**Growth reserve table (unlocked by revenue milestones, founder to set thresholds):**
- Production launch plus first paying cohort → inference ramp for volume work
- MRR reaches `[X]` → first generalist builder ($10K/month cash plus equity)
- MRR reaches `[Y]` → second builder; larger frontier inference budget
- MRR reaches `[Z]` → administrative and operations spend
**Discipline:** keep the burn multiple (net burn ÷ net new ARR) under `[target, e.g. 1–1.5]` once revenue starts.
**Team philosophy:** Inference before headcount. Hire only generalist builders with range beyond engineering; specialists as contractors. Option pool reserved at seed.
**Non-money ask:** Introductions to writers, podcasters, and analysts who publish weekly and would try it. Not advisors.
**Milestone:** Production launch, then evidence of repeat use from a first cohort of publishers.

---

## Treatment review (what changed and what was resolved)

- **Garbled slide 12.** The treatment file contains a leftover use-of-funds table
  from the September 25 deck (`~$180K` line items, duplicated "team philosophy"
  and "non-money ask" lines) appended after the growth-reserve table, with no
  heading. Resolved: the base plan table and the growth-reserve table are the
  ask slide; the orphaned table is dropped. The `$1.0–1.25M` ask and the
  `$20K/month` base plan are the only money figures carried.
- **Old market claim retired.** The September 25 deck's
  `50,000 publishers + 15,000 seats = $54M ARR` pull quote is gone; the
  treatment flags its arithmetic. Market slides now carry sourced figures only.
- **Voice-rule collision fixed.** The September 25 slide 8 dek used the banned
  `X is not Y. It is Z.` construction. No headline or dek in this revision uses it.
- **Structural merge.** Old "The fix" slide (its headline was already
  "The work itself is the interface") merges into the product slide; the deck
  goes 10 → 12 slides: the founder story, the state-versus-sandboxes argument,
  the engine, Autoradio, and the market are new or rebuilt.
- **Status labels** are the treatment's, dated 2026-09-27 in the deck, and must
  be re-checked on the conference day.
- **Open items** (founder): rewrite DRAFT headlines; confirm the raise; decide
  whether prices appear on the business slide; decide whether streaming and the
  Taiwan series are named; post-DevDay line on the why-now slide.
