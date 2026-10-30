# Pricing and Economics: Owner Positions and First Analysis

Date: 2026-10-14 (owner in session, about 15:20-15:30 UTC). Status: notes
toward a pricing model; nothing here is built (no billing exists). The
business-model section of docs/archive/reports/choir-state-and-vision-2026-10-14.md
(§8) has the wider vision.

## Owner positions

- **Method:** price from both ends. Bottom-up (cost plus) and top-down
  (value-based) give a spread; the question is pricing power and where in
  the spread to anchor, possibly launching about two-thirds of the way up
  rather than at the bottom.
- **Anchor:** $35 a month at entry. Chatbots are stuck at OpenAI's
  original $20; $35 is clearly above $30 ("more than a dollar a day"),
  and from $35 the price can ratchet toward $50 without losing people,
  whereas $20 → $35 reads as doubling.
- **No inference margin; no competing on inference price.** Paying model
  labs' margins is an accepted disadvantage; Choir succeeds through
  pricing power despite it. Heavy users bring their own inference where
  platforms allow it, or pay per use; higher tiers convert to usage
  pricing instead of subsidized flat plans ("if people want bulk
  high-quality inference, they should go to a Claude Pro plan").
- **Where the value is:** the whole workspace; curated, cleaned,
  organized open-source intelligence; a social network of teammates,
  brilliant people and agents with proprietary data that answer with
  signals without revealing the data; Autoradio as a superset of
  podcasts, audiobooks and YouTube, interactive and two-way ("asynchronous
  Clubhouse" with an algorithm that routes takes to other people's agents).
- **Compute upgrades are not in the $35 tier.** More CPU, RAM and storage
  are sold separately with a healthy margin.
- **Why margin sits on compute, not inference:** "demand for CPU, RAM and
  storage naturally scales with usage and utility, so users will be happy
  to pay extra for these, even at margin. And there are things people
  don't understand the pricing of very well, whereas people do really
  understand the pricing of LLM API inference. You don't want to be
  overpricing a commodity that people understand the market of."
- **Costs ahead:** inference and search will keep falling (Keenable as
  the price leader); CPU, memory and storage will not, because demand for
  agent sandboxes is rising. Autopaper's inference amortizes across users
  ("with 10,000 users we could gladly spend millions a year on Autopaper
  inference").
- **Voice (Autoradio design principle):** text-to-speech will be cheap,
  by API or on device. The AI voice should sound clearly like AI, not
  lifelike; human voices are mixed in, and the contrast is the point.
  Competitors that keep you listening to an AI all day are a worse
  experience.

## First analysis (lead, from today's measurements; estimates)

- Shared Autopaper: one 14-source paper ≈ $109/day at list prices
  (≈ $40k/yr); 28 sources ≈ $60-80k/yr before the caching and search
  optimizations. At 10,000 users that is ≈ $5-8 per user per year; even
  $2M/yr would be ≈ $17 per user per month, inside $35.
- Computers: one EPYC 7371 128 GB ($129/mo) serves ≈ 1,200-2,000
  registered users at ≈ 1.5% busy at peak
  (docs/reports/vm-density-limits-2026-10-14.md): well under $0.15 per
  user per month, before the upgrade margin.
- The per-user variable cost is each user's own agents' inference:
  an included allowance in the $35, then bring-your-own or pay per use at
  zero margin, so heavy users fund their own use.
- Voice: with a deliberately non-human voice (small open TTS model, local
  or on device) the generated parts of Autoradio are negligible in cost;
  most listening is existing audio (publisher-hosted podcasts, clips) and
  recorded human takes.
- Top-down references: consumer news ≈ $15-30/mo; specialist
  subscriptions (e.g. The Information ≈ $33/mo); pro intelligence tools in
  the hundreds to thousands per month. Value-priced pro products (the
  Taiwan vertical for funds, box-score data) cost little extra to serve.
- Long-term margin: compute upgrades now; data products and a fee on
  agent-to-agent consulting, priced by track record, later.
- Risks: bring-your-own inference depends on each platform's terms;
  zero inference margin leaves no buffer if provider prices rise (keep the
  included allowance adjustable); compute prices rising makes CPU
  efficiency (docs/archive/reports/computer-cpu-efficiency-2026-10-14.md, in
  progress) a margin question.

Next: a two-sided model (cost-plus per user under light, typical and heavy
use, with and without Autoradio; value-based references) when the owner
asks for it.

## Owner position: open source and self-hosting are encouraged

"Choir is open source and you'll be able to self-host it. And we even want
to encourage that, because someone who's self-hosting Choir is actually
more valuable for Choir than someone who is paying a lot of money for
managed cloud hosting. Self-hosting is a bigger investment into Choir as a
platform."

Implications (lead): managed hosting is a convenience with a compute
margin, not the business; the value Choir sells is the network that
self-hosted computers join — the Autopaper object graph (publication and
adoption), the agent network and its track records, data products and
Autoradio. Design work this implies: a federation path for self-hosted
computers (identity, signed records and precommitments that verify across
hosts, publication into and adoption from the Autopaper object graph);
self-host packaging (the Nix flake and the Node B module as the starting
point); and a deliberate licence choice (e.g. whether to guard against a
cloud provider reselling hosted Choir), which is open.
