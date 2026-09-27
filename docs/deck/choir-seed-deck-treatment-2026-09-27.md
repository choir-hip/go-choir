# Choir Seed Deck: Treatment for Revision

*Prepared September 27, 2026. For the deck-making agent. Target: a conference in 4 days.*

---

## 0. Instructions to the deck agent

This treatment replaces the September 25 "carbon kintsugi" deck and its strategic memo as the source of truth. Use earlier decks only as design reference.

**Voice rules (hard constraints).**

- Headlines marked **FOUNDER** are verbatim. Do not rewrite, "improve," or extend them.
- Headlines marked **DRAFT** are placeholders. Keep them short and plain, and flag them for the founder to rewrite in his own words. Do not polish them into slogans.
- Avoid agent tics: repeated "X isn't Y. It's Z." constructions, stacks of one-line maxims, adjectives like "ungameable," "unprecedented," "revolutionary," and phrases like "cryptographic proof of intent."
- No invented numbers. Every figure on a slide must come from the research tasks in Section 3, with a source recorded in the speaker notes. If a figure can't be sourced, leave a visible `[TK]` rather than estimating.

**Positioning rules.**

- Do not use "Cursor for writing." Dropped.
- Do not lead with selling data to hedge funds or finance. Financial data is at most a downstream byproduct of the network, and it does not appear in the main deck.
- Do not lead with "supervising agents." Supervision is how Choir works, not why anyone wants it. It appears once, in the engine slide, for technical investors.
- Do not characterize any AI lab or its leadership's motives. Make structural arguments (open runtime, user-owned state, model neutrality) without naming people.
- Stay honest about stage: pre-traction, pre-production, live on staging. No traction claims, pilot numbers, or customer quotes.

---

## 1. The narrative in one paragraph

AI is a new species of computer, not a new species of being. Every chat product today, from ChatGPT to Claude Tag to Meta Muse, is a command line: you type, text comes back, the state stays hidden, and corrections don't stick. The founder lived this. He set out to build an AI ghostwriter so he could be prolific, and realized he didn't want to be replaced as the writer: with the right tools for research, citation, analysis, understanding, writing, and multimedia production, he could produce both quantity and quality himself. Chat tools couldn't get him there, because agent drafts regenerate slop faster than he can correct it, so the work never compounds. Choir is the GUI moment: an automatic computer where the work itself is the interface, the user owns the accumulated state across every agent and model, and the newspaper (choir.news) and radio (Autoradio) are the surfaces people use every day. Voice is the real platform shift, and Autoradio is expected to be the killer product. The company is pure technology risk: if the computer works, the market follows.

---

## 2. Slide-by-slide treatment

### Slide 1: Cover

- **FOUNDER:** "AI, a new species — of computer"
- **FOUNDER (subtitle):** "Introducing the automatic computer"
- Nothing else on the slide except the wordmark, date, and choir.news.
- *Speaker note:* The line turns a familiar framing (AI as a new kind of being) into a claim about machines. It signals up front that Choir is not selling a companion or a digital employee.

### Slide 2: The CLI and the GUI

- **FOUNDER:** "The chat-based agent is the CLI. Now it's time for the GUI."
- Leads directly into the product demo.
- Supporting content (small, visual): chat logos (ChatGPT, Claude, Claude Tag in Slack, Meta Muse) shown as terminal windows. All chat is CLI: type a command, get text back, state hidden, you learn the incantations (prompting is command syntax).
- Optional one-liner: text is the universal interface, but chat isn't the only kind of text. Documents are. (Research item R10 for the origin of "text is the universal interface" in Unix philosophy.)
- *Speaker note:* The CLI never died. Power users still live in it. The GUI opened computing to everyone else, and that's where the market grew by orders of magnitude.

### Slide 3: The founder's story as the problem

- **DRAFT (founder to rewrite):** "I wanted agents to make work that compounds. I got drafts that regenerate slop."
- This slide replaces the old team slide and the old problem slide. The founder is the problem statement.
- Content, in the founder's voice:
  - Summer 2025: built an AI ghostwriter because he wanted to be prolific. Stitched models together by hand; the output was better than any single model.
  - The turn: in building it, he realized he didn't want a ghostwriter at all. He wanted to, and could, produce both quantity and quality himself, with the right tools for research, citation, analysis, understanding, writing, and multimedia production. The goal shifted from replacing the writer to equipping one.
  - Tried to rebuild it on an agent-graph framework. Failed: agents don't follow a flowchart; they discover the workflow as they go.
  - The slop loop: talking to chat is easier than editing, so corrections go through regeneration. Each new version fixes one thing and breaks another. Corrections don't stick, so nothing compounds.
  - The realization: why give agents a computer when you can center the computer and give it agents. First prototype running on localhost in about twenty minutes.
- Background line (small): professional poker, then AI since 2015.
- *Design note:* A before/after visual works well here: a chat thread where a correction is lost across versions, versus a versioned document where the edit persists.

### Slide 4: Product demo, the work itself is the interface

- **DRAFT:** "The work itself is the interface."
- Show Texture: agents update a versioned document; the user edits it directly; the edit is the message and steers what happens next.
- Show Mail on staging: real-world sends gated by owner approval.
- Status labels, honest: Texture (staging), Mail (staging), Newspaper (refactor), Radio (development). Update labels to current state before the conference.

### Slide 5: State, not sandboxes

- **DRAFT:** "Everyone sells compute. Choir owns state."
- Content: dozens of sandbox and agent-computer companies offer essentially the same product, and every agent brings its own sandbox. Only Choir lets the user own accumulated work state and learnings regardless of which agent or model they use.
- The cross-agent demo (the most important proof in the deck): put a Markdown or PDF file into your Choir through MCP, then work on it from ChatGPT or Meta Muse. In Choir it isn't static: it stays live, versioned at the user level and the platform level, and keeps being worked on by the multi-agent system.
- Pre-empt the objection: vendors are adding memory import and export. Memory is a summary of what an assistant learned about you. Choir holds the work itself, versioned and still being worked on by agents from any vendor.
- Research: R7 (sandbox landscape, so the "dozens" claim is accurate; do not say "99" literally unless counted).

### Slide 6: The engine (one slide, for technical investors)

- **DRAFT:** "A computer made of agents."
- Content, compact:
  - Natively multi-agent harness in Go. Desks with bounded capabilities (Texture, Research, Engineering, Management, Mail), each a recursive language model with its own context and imports. No root agent; coordination lives in the state.
  - Precommitment records: agents commit to predictions before acting; outcomes resolve them. The same log is the learning signal, the audit trail, and the reputation record.
  - Model-neutral: cheap open-weight models for volume, frontier models where stakes are highest.
- External validation: Harvey and Baseten's September 2026 research on recursive harnesses for long-document diligence. Use only figures verified in R5.
- *Speaker note:* This is the only slide that mentions supervision.

### Slide 7: The newspaper, public proof and content marketing

- **DRAFT:** "The newspaper is the demo."
- choir.news is an app inside the web desktop, not a separate product. It reads the world continuously, keeps provenance on every claim, and publishes.
- It doubles as content marketing: the founder will stream himself using Choir to cover tech, media, business, politics, and geostrategy. Viewers watch the work get steered by editing, not prompting.
- First reporting series: Taiwan. Reading the Taiwanese internet in its own language and platforms, for Western audiences: silicon supply chains, US-China relations, democratic legitimacy versus the authoritarian growth model.
- Differentiator to highlight: labeling each source's political alignment and flagging coordinated or content-farm material. Taiwan is one of the most heavily targeted information environments in the world, which makes it the hardest possible test of provenance.
- Research: R11.

### Slide 8: Autoradio, the killer product

- **DRAFT:** "The real platform shift is voice."
- Content:
  - The automatic computer is the new computer form. Voice is the new user norm.
  - Earlier voice assistants (Alexa, Siri) were voice CLIs: command, response, no persistent state. Autoradio is a living station you steer by interrupting and recording your own takes.
  - Two-way radio has a proven social form: call-in radio built communities. Autoradio is call-in radio with provenance, where listeners' takes become part of the record.
  - Principle: generate the visuals, preserve the voice.
  - Delivery: native mobile app plus an app inside the web desktop. Mobile puts Choir on the home screen, in the car, and on walks, where the daily habit forms.
- Research: R8, R9.

### Slide 9: Market

- **DRAFT:** "The CLI market is chat. The GUI market is everyone who works in documents."
- Structure, all figures sourced (R1–R4):
  - Top-down: paid seats in office and document suites (the GUI market).
  - Current AI market: paid AI subscribers (the CLI market).
  - First slice, bottom-up: multi-homers, people paying for two or more AI products. Frame with aggregation theory: the labs each want users to single-home; multi-homing shows no lab has won; Choir aggregates the user's work across agents and turns models into interchangeable suppliers.
  - Beachhead: people who write or publish for work (newsletter writers, analysts, podcasters, founders doing their own comms, small organizations with no comms staff).
- Show one bottom-up calculation: [beachhead count] × [price] × [defensible penetration]. Every input sourced; arithmetic checked.
- Do not use the installed-base "10⁹ automatic computers" chart as the market proof. It can appear as a small vision line at most.

### Slide 10: Business model

- **DRAFT:** "Choir owns the relationship by letting you own your state."
- Content:
  - Open-source runtime, self-hostable. Most won't self-host, but the ability to leave is what earns the trust.
  - Hosted Choir for convenience. Premium surfaces (Radio, research, teams) built on the user's state, never a claim on it.
  - The network: choir.news, Autoradio, and citations across computers, where reach grows with each publisher.
- Analogy: WordPress and Automattic. Open source, self-hostable, most people use the hosted service, and it powers a large share of the web partly because no one fears lock-in. Research: R12.
- Pricing: include only prices the founder has decided. If undecided, omit.

### Slide 11: Why now

- **DRAFT:** "Agents are everywhere. Their work belongs to no one."
- Content:
  - The personal-agent wave of mid-2026: Meta Muse, Instinct, Grok Bot, Town, and OpenAI and Anthropic entries. Every one of them keeps the user's work in the vendor's silo. (R6)
  - Chat is buckling under long-horizon work: public reporting of tens of thousands of problematic agent incidents across top labs, and OpenAI pausing training on its most capable models. (R13)
  - Open-weight models are now cheap and good enough for volume work, which makes a model-neutral computer economically viable. (R14)
- Update after OpenAI DevDay on September 29: if OpenAI announces a managed-agents platform or a consumer agent, add one line on how Choir relates to it (another agent that can work on Choir state).

### Slide 12: The ask

- **DRAFT:** "Building the system that needs to exist."
- **Framing: spending ramps with revenue.** A low base burn keeps the company default alive. The raise is capital to move fast when demand shows up, not money to go looking for it. Hiring, salaries, inference, and administrative spend each step up only when revenue milestones are hit.
- Money ask: approximately **$1.0–1.25M** (founder to confirm the final number), structured as a runway floor plus a growth reserve.
- Milestone: production launch, then evidence of repeat use from a first cohort of publishers.

**Base plan (pre-revenue), roughly $20K/month:**

| Line | Monthly |
|---|---|
| Founder salary | $10K |
| Inference (open-weight volume plus frontier security bursts) | ~$3–5K |
| Infrastructure and miscellaneous | ~$3–4K |
| Contractors, amortized (pre-launch security audit, legal) | ~$3K |

At that burn, the raise alone covers several years, so the company does not need revenue to survive. That is the point.

**Growth reserve, unlocked by revenue milestones (founder to set thresholds):**

| Trigger (example, `[founder sets]`) | Unlocks |
|---|---|
| Production launch plus first paying cohort | Inference ramp for volume work |
| MRR reaches `[X]` | First generalist builder ($10K/month cash plus equity) |
| MRR reaches `[Y]` | Second builder; larger frontier inference budget |
| MRR reaches `[Z]` | Administrative and operations spend (bookkeeping, support tooling) |

- Discipline stated on the slide: keep the burn multiple (net burn divided by net new ARR) under `[target, e.g. 1–1.5]` once revenue starts. Name the metric; investors know it.
- Team philosophy (one line): inference before headcount; hire only generalist builders with range beyond engineering; specialists as contractors.
- Option pool reserved at seed to keep flexibility on equity hires.
- Non-money ask, one per conversation: introductions to writers, podcasters, and analysts who publish weekly and would try it. **Not** advisors.
- *Speaker note:* Every spending step is gated on evidence, the same way Choir's own goal files gate progress on evidence. An AI-native company that measures return on inference.

---|---|
| Founder salary ($10K/month) | ~$180K |
| Open-weight inference for volume (ramping with stability) | ~$180–270K |
| Frontier inference for security hardening (bursts) | ~$180K |
| 1–2 generalist builders from ~month 6 ($10K/month cash each, plus equity) | ~$120–240K |
| Specialist contractors (security audit, legal, design) | ~$80K |
| Infrastructure and miscellaneous | ~$70K |
| Buffer (~20%) | remainder |

- Team philosophy (one line): inference before headcount; hire only generalist builders with range beyond engineering; specialists as contractors.
- Option pool reserved at seed to keep flexibility on equity hires.
- Non-money ask, one per conversation: introductions to writers, podcasters, and analysts who publish weekly and would try it. **Not** advisors.
- Inference-spend increases tied to milestones rather than calendar dates. This is itself part of the pitch: an AI-native company that measures return on inference.

---

## 3. Research tasks

For each task: find the figure or fact, record the source and date in speaker notes, and mark `[TK]` on the slide if not found. Prefer primary sources (company filings, official blogs, the original research) over aggregators. Record the as-of date for every number, since these move fast.

| ID | Question | Where to look | Used on |
|---|---|---|---|
| R1 | Paid AI subscriber counts for major consumer AI products, as of 2026 | Company announcements, earnings calls, credible reporting | Slide 9 |
| R2 | Share of AI users who pay for or regularly use two or more AI products (multi-homing) | Consumer AI surveys (e.g., Menlo Ventures, a16z consumer reports), other survey data | Slide 9 |
| R3 | Paid seats or paying customers for Microsoft 365 and Google Workspace | Microsoft and Alphabet filings and earnings calls | Slide 9 |
| R4 | Size of the writing and publishing beachhead: paid newsletter publishers and subscribers (Substack, Beehiiv, Ghost), active podcasts | Company announcements, Podcast Index or similar | Slide 9 |
| R5 | Verify the Harvey and Baseten September 2026 recursive-harness research: rubric pass-rate gain, coverage figures, the depth-2 penalty, root-versus-sub-agent effect sizes, root share of input tokens, and the cost comparison (the sentence in the founding document about Opus cost appears garbled; get the exact figures) | The original Harvey/Baseten publication | Slide 6 |
| R6 | Verify personal-agent competitor facts: Meta Muse launch date, App Store rank, pricing; Instinct valuation; Grok Bot pricing; Town funding; Anthropic Orbit status; OpenAI "Aeon" status | Official announcements, TechCrunch, Axios, The Information | Slide 11 |
| R7 | Landscape of agent sandbox and cloud-computer companies, to support "dozens of near-identical products" | Company sites, funding databases, recent coverage | Slide 5 |
| R8 | Voice-assistant history: evidence that command-style voice assistants stalled (e.g., reported losses or usage plateaus) | Credible reporting | Slide 8 |
| R9 | Audio listening behavior: time spent with audio, in-car listening share, podcast reach | Edison Research (Infinite Dial and similar) | Slide 8 |
| R10 | Origin of "text is the universal interface" in Unix philosophy (Doug McIlroy), and Ben Shneiderman's "direct manipulation" (1983) | Primary sources | Slide 2 (speaker notes) |
| R11 | Taiwan media landscape: partisan alignment of major outlets (Liberty Times, United Daily News, China Times and its ownership), major platforms (PTT, Dcard, Threads, LINE), documented PRC information-operation patterns | Academic and think-tank research, Taiwan FactCheck Center, credible reporting | Slide 7 |
| R12 | WordPress share of websites; hosted versus self-hosted split if available | W3Techs, Automattic statements | Slide 10 |
| R13 | Axios report (September 2026) on tens of thousands of problematic frontier-model incidents; OpenAI's training pause statement | Axios, OpenAI statements | Slide 11 |
| R14 | Current API pricing for DeepSeek v4.1 flash across providers, and frontier API rates, to sanity-check the inference budget | Provider pricing pages | Slide 12 (notes) |
| R15 | Recursive language models (Zhang et al., MIT) citation | Original paper | Slide 6 (notes) |
| R16 | Results of OpenAI DevDay (September 29) relevant to agents | Official announcements | Slide 11 |

---

## 4. Consistency checks before handoff

- Every number on a slide has a source in the notes, or a visible `[TK]`.
- All arithmetic re-checked (the September 25 deck's $54M did not add up; do not repeat that).
- Status labels (staging, refactor, development) match reality on the day of the conference.
- No slide contradicts another about chat: the position is "no chat thread; the edit is the message," not "you can never talk to the system."
- The deck contains no hedge-fund wedge, no "Cursor for writing," no characterization of lab leadership, no "ungameable."
- Founder headlines (Slides 1 and 2) appear verbatim.
- All DRAFT headlines are visibly flagged for founder rewrite in the version handed back.

---

## 5. Open items for the founder

- Rewrite every DRAFT headline in your own words.
- Confirm the raise amount and use-of-funds split.
- Decide which prices, if any, appear on Slide 10.
- Decide whether the streaming plan and the Taiwan series are named in the deck or kept for conversation.
- If Choir is stable enough, build the next revision inside Choir and say so on Slide 4.
