# Speaker Notes and Sources — Choir Seed Deck, September 27, 2026

Companion to `choir-seed-deck-2026-09-27-carbon-kintsugi.html` and
`...-london-salmon.html`. Every figure that appears on a slide is listed here
with its as-of date and source, per the treatment's evidence rule. Figures that
could not be verified do not appear on slides, except where a founder input is
still open (`[price]`, `[penetration]`, `[X]/[Y]/[Z]`, `[target]`), which are
marked on the slide itself.

Line-level speaker notes are in the deck's HTML comments where useful; this file
carries the notes that matter on stage plus the source ledger.

---

## Slide 01 — Cover

**Note.** The line turns a familiar framing (AI as a new kind of being) into a
claim about machines. It signals up front that Choir is not selling a companion
or a digital employee. Both lines are founder-verbatim; do not improvise around
them.

## Slide 02 — The CLI and the GUI

**Note.** The CLI never died; power users still live in it. The GUI opened
computing to everyone else, and that is where the market grew by orders of
magnitude. The terminal strip is the argument: every chat product is a command
line with hidden state.

**Citation support (R10).** The exact, supportable Unix maxim is Doug McIlroy's
"Write programs that handle text streams, because that is a universal
interface" (commonly cited via Peter H. Salus, *A Quarter Century of UNIX*,
ACM Press/Addison-Wesley, 1994). The slide paraphrases without attributing;
if you attribute it on stage, use that wording, not "text is the universal
interface."
Shneiderman's "Direct Manipulation: A Step Beyond Programming Languages,"
*Computer* 16(8), August 1983, pp. 57–69 (DOI 10.1109/MC.1983.1654471) is the
1983 antecedent for the GUI claim.

## Slide 03 — The story

**Note.** This slide is the problem statement; the founder is the evidence.
The two loop boxes are the technical argument: in chat, a correction is a new
generation, so quality does not accumulate; in a versioned document, a
correction is a revision, so it does.

DRAFT headline — rewrite in your own words before the conference.

## Slide 04 — The product

**Note.** The demo runs before or after this slide; the slide itself should stay
up while Texture is shown. The status table is deliberately unglamorous and
dated: re-check every label on the morning of the conference.

**Status labels, as of 2026-09-27.** Texture — staging · Mail — staging ·
Newspaper — refactor · Radio — development.

**Open item (treatment §5).** If Choir is stable enough to build the next
revision of this deck inside Choir, say so here.

## Slide 05 — State, not sandboxes

**Note.** The proof strip is the most important demo in the deck: a file enters
through MCP and stays live under multiple agents. If the demo fails on stage,
this slide is the fallback explanation.

**Sources.**
- R7 sandbox landscape — 23 tools in a scoped comparison
  (https://rywalker.com/research/ai-agent-sandboxes, 2026-09, medium
  confidence) and 30 providers/options in a broader landscape
  (https://engine.build/lab/agent-sandboxes, 2026-06, medium). The slide says
  "30+", which both support; it deliberately does not say "99".

## Slide 06 — The engine

**Note.** This is the only slide that mentions supervision. Keep it short; it is
for the technical investor in the room, not the main narrative.

**Sources.**
- R5 — Harvey, "Post-training RLM agents for M&A diligence," 2026-09-08
  (https://www.harvey.ai/blog/post-training-rlm-agents-for-m-and-a-diligence):
  on 50 held-out LAB Diligence data rooms across seven models, depth-1 RLM
  harness vs standard tool-loop lifted mean rubric-criteria pass rate 23.3% →
  62.4%; data-room coverage went from no run above 1% (most 0.1–0.5%) to nearly
  every run above 10%; Qwen3.5-122B-A10B root post-training moved coverage
  62% → 96%; a second delegation layer (depth-2) cost 19 points of rubric pass
  rate across 14 data rooms; root-model choice moved results ~38 points against
  ~8 points for sub-agent choice; with a Claude Opus 5 root, the root saw 3.8%
  of input and 1.1% of output tokens; per-data-room generation cost was ~$7 in
  the RLM setup against ~$18 for the standard tool loop.
  *Caveat:* the treatment attributes this to "Harvey and Baseten." Only the
  Harvey publication was located; no distinct Baseten publication was found, so
  the slide cites Harvey alone.
- R15 — Alex L. Zhang, Tim Kraska, Omar Khattab, "Recursive Language Models,"
  arXiv:2512.24601 (2025): an RLM treats a long prompt as an object in an
  external REPL, letting the model inspect, decompose, and recursively call
  itself over snippets.

## Slide 07 — The newspaper

**Note.** Two audiences in one slide: the wire is proof the computer works, and
the streaming series is the marketing plan. The Taiwan series is the hardest
available provenance test — say why: it is one of the most heavily targeted
information environments in the world.

**Sources (R11).**
- Reuters Institute, Digital News Report — Taiwan 2024: Liberty Times aligned
  pro-independence; United Daily News and China Times aligned pro-unification
  (https://reutersinstitute.politics.ox.ac.uk/digital-news-report/2024/taiwan).
- China Times group ownership: Want Want (Tsai Eng-meng) since 2008 — China
  Times, CTV, CTiTV
  (https://public-assets.graphika.com/reports/detecting_digital_fingerprints-tracing_chinese_disinformation_in_taiwan.pdf).
- Platforms: PTT (Taiwan FactCheck Center), Dcard (company-claimed >10M
  registered members), Threads (MIT Technology Review, 2024 election), LINE
  (ASPI/Cofacts).
- Documented patterns: content farm → Taiwanese media republication →
  state-sponsored amplification (ASPI); "America will abandon Taiwan" narrative
  (Global Taiwan Institute); forged foreign-language documents seeded in forums
  then treated as breaking news (Taiwan FactCheck Center).
- *Design consequence:* the alignment-labeling differentiator is exactly what
  the cited research does by hand.

**Open item.** Decide whether the streaming plan and the Taiwan series are named
in the deck or kept for conversation.

## Slide 08 — Autoradio

**Note.** The argument is that voice assistants were CLIs, not that voice failed.
Autoradio's social form is call-in radio with provenance; the delivery point is
that mobile is where the daily habit forms.

**Sources.**
- R8 voice-CLI stall: Amazon's Alexa+ rollout reached ~100,000 users by
  2025-05-01 and "hundreds of thousands" with access six weeks later (Reuters);
  the Echo/Devices business reportedly lost more than $25B over 2017–2021
  (WSJ, 2024-07); Apple delayed Siri's personal-context features by about a year
  (Reuters, 2025-03); Google replaced Assistant with Gemini for Home, describing
  the old assistant as bound to "rigid, specific commands" (Google, 2025-10).
- R9 audio behavior: radio holds 32% of daily audio listening time and 55% of
  in-car audio time (Edison Research / SSRS, 2026); podcasts reach 167M
  Americans monthly and 130M weekly (Infinite Dial 2026, 2026-03).

## Slide 09 — Market

**Note.** The top-down pair frames the market; the bottom-up line is the only
calculation. Price and penetration are founder inputs and are left as such on
the slide — do not fill them with guesses. The aggregation-theory point is the
structural one: multi-homing means no lab has won, and Choir can sit across all
of them.

**Sources.**
- R1 — paid AI subscriptions and seats, summed on the slide as 88M+:
  ChatGPT >50M subscribers (OpenAI, 2026-03); Microsoft 365 Copilot >30M paid
  seats (Microsoft FY26 Q4, 2026-06); Gemini Enterprise >8M paid seats across
  2,800+ companies (Alphabet Q4 2025, 2025-12). *Caveat:* these are announced
  counts from different dates and different definitions (consumer subscriptions
  vs enterprise seats); the sum is an order-of-magnitude frame, not a survey.
  Not found: paid counts for Claude, Meta AI/Muse, Grok, Perplexity.
- R2 — Menlo Ventures, State of Consumer AI 2026 (2026-09): 55% of surveyed U.S.
  AI users pay for at least one AI product; the average AI user runs 3.0 general
  assistants. a16z/Yipit (2025-12): 9% of consumers pay for more than one
  subscription; fewer than 10% of ChatGPT weekly users visited another large
  model provider during most of 2025.
- R3 — >450M paid Microsoft 365 commercial seats (Microsoft FY26 Q2, reported
  2026-01); Google Workspace >3B users and >13M customers (Google, 2026-04),
  with >11M explicitly described as paying (2025-12). Broader context not used
  on the slide: Gemini app 950M MAU (2026-06); 350M paid subscriptions across
  Google consumer services (2026-03).
- R4 — beachhead: ~478K active podcasts (Libsyn/Podcast Index, 2026-09);
  135K+ publishers on beehiiv (2026-09); 30,579 active Ghost customers
  (2026-09); Substack 5M paid subscriptions (2025-03). The 644K on the slide is
  podcasts + beehiiv + Ghost, summed (478 + 135 + 31); it excludes Substack paid
  subscriptions because a publisher count was not found. Adjacent pools:
  140,300 U.S. writers and authors, 65% self-employed (BLS, 2025); 1.5M+
  full-time digital creators (IAB, 2025).

## Slide 10 — Business model

**Note.** Say the WordPress analogy plainly: the point is not the business model,
it is that open source plus a hosted default is how a platform becomes trusted
infrastructure.

**Sources.** R12 — WordPress runs 40.2% of all websites and 58.7% of websites
with a known CMS (W3Techs, 2026-09).

**Open item.** Pricing is omitted pending the founder's decision (treatment §5).
If prices are decided, they belong in the three columns — and the market slide's
bottom-up line then has its price input.

## Slide 11 — Why now

**Note.** Three independent clocks: the agent wave is here, chat is straining
under long-horizon work, and open-weight economics turned. Keep each block to
its fact.

**Sources.**
- R6 — Meta Muse launched 2026-09-08 (Meta) and went to #1 among free U.S. apps
  (Appfigures, 2026-09-20, medium); Meta Muse pricing free tier plus $20/$100
  per month depending on usage (CNBC, medium); Town raised a $55M Series A led
  by a16z (Town, 2026-06); Instinct reported in talks at a ~$10B valuation
  (The Information, medium; not a closed round); Grok Bot has no standalone
  price — it ships with Cursor Pro ($20/month) or SuperGrok ($30/month);
  Anthropic's Orbit and OpenAI's "Aeon" have no confirmed official
  announcement — keep them described as reported only.
- R13 — Axios, 2026-09-26: the top labs are contending with "tens of thousands"
  of agent-security incidents (the exact count was not published). OpenAI's
  alignment reporting, updated 2026-09-25: "All training, evaluation, and
  inference with tool-use (defined broadly) of our most capable models remain
  paused."
- R14 — DeepSeek V4.1 Flash: $0.30 input / $1.20 output per million tokens at
  peak ($0.15/$0.60 off-peak) direct from DeepSeek; frontier comparison used on
  the slide is GPT-6 Astra $10/$50 and Claude Fable 5.1 $10/$50. OpenRouter
  routed endpoints ran $0.035–$0.375 input per million at retrieval.
- R16 — OpenAI DevDay 2026 is scheduled for 2026-09-29, after this revision.
  Add the one-line relation — another agent that can work on Choir state — only
  if a managed-agents platform or consumer agent is actually announced.

## Slide 12 — The ask

**Note.** The framing is the pitch: spend steps up only as evidence arrives, the
same way Choir's own goal files gate progress on evidence. The raise is a runway
floor plus a growth reserve; the base plan is deliberately survivable.

**Figures.** Base plan ~$20K/month: founder salary $10K; inference ~$3–5K;
infrastructure ~$3–4K; contractors (audit, legal) ~$3K amortized. Raise
≈ $1.0–1.25M — founder to confirm. Growth-reserve thresholds `[X]/[Y]/[Z]` and
the burn-multiple target are founder inputs, marked on the slide.

**Resolution of a treatment defect.** The treatment file's slide 12 carries a
leftover September 25 use-of-funds table (~$180K line items) with duplicated
"team philosophy" and "non-money ask" lines and no heading. It is dropped here;
the base plan and growth-reserve tables are the ask slide. If the founder wants
a second money view, that table is the place to start — but it must be
re-derived, not pasted, because its arithmetic was never verified.

---

## Consistency checks performed

- Every on-slide figure traces to a source above; the only visible placeholders
  are founder inputs (`[price]`, `[defensible penetration]`, `[X]/[Y]/[Z]`,
  `[target]`) and they are labeled as such.
- Arithmetic: 478K + 135K + 31K = 644K; 50M + 30M + 8M = 88M. Both checked.
- No headline or dek uses the banned `X isn't Y. It's Z.` construction. The
  September 25 deck's `$54M ARR` pull quote is gone.
- FOUNDER headlines (slides 01, 02) appear verbatim. All DRAFT headlines carry
  the visible `Draft — founder to rewrite` badge.
- Stage claims stay at staging/refactor/development; no traction numbers, no
  pilots, no customer quotes.
- "Chat" appears consistently as *the edit is the message* — no slide claims you
  can never talk to the system.

## Still open before the conference

1. Rewrite every DRAFT headline in the founder's own words.
2. Confirm the raise amount and the growth-reserve thresholds.
3. Decide whether prices appear on the business slide (unblocks the market
   slide's bottom-up line).
4. Decide whether the streaming plan and the Taiwan series are named.
5. Post-DevDay (September 29) one-liner on slide 11.
6. Re-check the four status labels on slide 04.
