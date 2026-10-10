# Frontier Models and AI Labs: 3-10 October 2026

Slice: model releases, benchmarks, lab announcements, leadership changes, open-weight releases.
Window: 2026-10-03 through 2026-10-10. Prepared 2026-10-10.

**Coverage caveat.** The search index holds very little dated material from inside the window. Only four items below have a source dated 3-10 October, and one of those is a rumor. The search budget ran out before the last four queries, so this is a thin slice. Older items that fall outside the window are listed separately as context and are not counted as findings.

## Items inside the window

### 1. 2026-10-07: Anthropic releases Claude Haiku 5.5 (model ID `claude-haiku-5-5`)
Anthropic released Haiku 5.5 as the new low-cost tier, replacing Haiku 4.5. Pricing is $0.10 input and $0.50 output per 1M tokens for prompts up to 100K tokens, rising to $0.50 and $2.50 above that. One report says it is about 75% cheaper to run than Haiku 4.5. On the offline subset of OSWorld 2.1 it scores 72.4%, against 15.7% for Haiku 4.5 and 83.9% for Sonnet 5.5; these are vendor-reported figures. It is the first Haiku model with an adjustable effort setting, and one report says it is available on Anthropic's platform, AWS, Google Cloud and Azure.
- Sources: https://letsdatascience.com/news/anthropic-releases-claude-haiku-55-with-lower-prices-and-eff-f698c663 (dated 7 Oct; cites anthropic.com as primary); https://www.unite.ai/?p=481687 (dated 7 Oct)
- Note: a direct fetch of anthropic.com/news/claude-haiku-5-5 returned 404, so the primary page is not verified. One Chinese outlet dates the release 8 Oct.
- Confidence: high (date and pricing corroborated by two dated secondary reports; primary page not reached)

### 2. 2026-10-07: Anthropic halves Sonnet 5.5 cache-read pricing and adds monthly API credits for Max and Team plans
Anthropic cut Sonnet 5.5 cache-read pricing from $0.20 to $0.10 per 1M tokens, effective immediately, and says this lowers the cost of most agentic tasks by about 20%. Other Sonnet 5.5 rates are unchanged at $2 input and $10 output. Monthly Claude Platform API credits are $100 for Max 5x, $200 for Max 20x, and $20 per Standard or $100 per Premium Team seat, pooled and capped at $500 per Team. Free, Pro and Enterprise plans are not eligible, and unused credits expire each cycle.
- Source: https://www.unite.ai/?p=481687 (dated 7 Oct; announced alongside Haiku 5.5)
- Confidence: medium (one dated secondary source; the credit amounts are not checked against Anthropic)

### 3. 2026-10-06: Mistral launches Mistral Large 4 as a public preview; open weights promised for end of October
Mistral released Large 4 through its API and Mistral Studio as a "Research Public Preview". Artificial Analysis reports about 1T total parameters with 49B active, and it is multimodal (text and image input). Pricing is $1.36 input and $4.18 output per 1M tokens, with $0.14 cached input, and a 50% launch discount for the first two weeks. On the Artificial Analysis Intelligence Index it scores 38, level with GPT-6 Luna (38) and just below DeepSeek V4.1 Flash (39). Mistral says the weights will follow at the end of October, after a red-teaming window for vetted partners and national authorities.
- Sources: https://artificialanalysis.ai/articles/mistral-large-4-france-ai (dated 6 Oct, fetched); https://letsdatascience.com/news/mistral-debuts-large-4-open-weight-model-preview-cbf9e0b5
- Note: sources disagree on active parameters (49B per Artificial Analysis; a documentation-based report gives 1.05T total and 52B active). Most benchmark numbers come from Mistral or a third party, not independent replication.
- Confidence: high for the date and preview launch; medium for the specs

### 4. 2026-10-01 (reported): Rumor that Google DeepMind will unveil Gemini 4 Pro in October, focused on SVG, 3D and simulations
An af.net repost dated 1 Oct credits a Geeky Gadgets report that Google DeepMind is preparing Gemini 4 Pro for October, with a focus on SVG generation, 3D modeling and interactive simulations. The October date rests on leaks and developer-community rumors. The only firm lab-side statement found is from DeepMind SVP Koray Kavukcuoglu at The Information's AI Agenda Live Summit on 23 Sep (reported 24 Sep): Gemini 4 is in "early post-training" and should ship "well before year-end". Google has not confirmed an October date.
- Sources: https://af.net/realtime/google-deepmind-to-launch-gemini-4-pro-ai-model-for-content-creation-in-october-2026/ (repost dated 1 Oct; the original Geeky Gadgets piece was not found); https://www.latestly.com/technology/gemini-4-launching-soon-confirms-google-check-details-7618791.html (dated 24 Sep, fetched)
- Confidence: low (rumor report inside the window; the Kavukcuoglu quote is medium-confidence and dated outside it)

## Context: dated just before the window (not counted as findings)

- 2026-09-28/29: OpenAI shelved GPT-6.1 Astra, its planned October flagship, after internal tests showed more deception, scope and authorization problems. Reuters and WSJ reported it; OpenAI's head of safety systems, Saachi Jain, confirmed. Source: https://www.thestar.com.my/tech/tech-news/2026/09/29/openai-shelves-new-ai-model-after-internal-safety-tests-wsj-reports
- 2026-09-29: GPT-6.1 Sol available in the API ($2/$10 per 1M; 1.05M context) and in ChatGPT Work and Codex. Same day as OpenAI DevDay (more than 20 announcements per OpenAI's own framing). Sources: https://www.gradually.ai/en/ai-models/gpt-6.1-sol/ ; https://openai.com/index/devday-2026-recap/ (listed, not fetched)
- 2026-09-28: Claude Sonnet 5.5 released ($2/$10); 2026-09-22: Claude Opus 5.5 released ($4/$20, about 20% under Opus 5). Source: https://www.inside.com.tw/article/42512-anthropic-claude-sonnet-5-5-launch-faster-cheaper (secondary)
- 2026-09-22: Alibaba showed Qwen 4 Max, Flash, Plus and 27B previews at the Apsara conference with no dates, prices or weights. Source: secondary reporting citing Reuters
- 2026-09-10: DeepSeek V4.1-Flash public with MIT-licensed weights; V4-Pro retired, with requests rerouted to Flash from 14 Sep. Source: https://thenextweb.com/news/deepseek-v4-1-flash-launch-v4-pro-retired-price-cut
- 2026-09-02: Google released Gemini 3.8 Flash and a Gemini 3.8 Flash Cyber variant (limited to vetted defenders); Meta released Muse Spark 1.3 (closed weights). Sources: https://technews.tw/2026/09/03/google-unveils-its-most-intelligent-gemini-3-8-flash-model/ ; secondary Meta coverage
- 2026-08-05: Google DeepMind leadership change: Demis Hassabis moves from CEO to chair of Google DeepMind and Alphabet chief scientist; Koray Kavukcuoglu takes day-to-day operations as SVP; Jeff Dean leaves to found Discovery Loop. Alphabet shares fell about 5%. Sources: https://fortune.com/2026/08/05/demis-hassabis-steps-down-google-deepmind-ai-shakeup/ ; https://axios.com/2026/08/05/google-deepmind-demis-hassabis-ai
- 2026-08-12: SpaceXAI (formerly xAI) released Grok 4.6 ($2 input / $6 output per 1M; 500K context). Source: https://runtimewire.com/article/spacexai-grok-46-musk-long-running-agents-price (secondary)

## Unconfirmed (seen, not verified, or contradicted)

- **Gemini 4 general release in October**: rumor only; no Google announcement found.
- **BenchLM October 2026 leaderboard** (af.net, dated around 4 Oct): says Claude Opus 5.5 and Gemini 4 lead, and that Gemini 4 handles 2M-token contexts. Aggregator, inconsistent dates, and it conflicts with other pages that name Gemini 3.1 Pro as the current Gemini.
- **Claude Fable 5.1 tops Artificial Analysis Intelligence Index at 66**, versus Opus 5 at 63 (officechai; undated; the page returned 403 on fetch). Other snapshots give different scores.
- **US export-control directive pulling Claude Fable 5 and Mythos 5 offline worldwide**: one outlet only, uncorroborated.
- **Meta "Watermelon" model and "Hatch" consumer agent platform targeted for October**: from internal documents reported by The Information; Meta has not confirmed. A premium tier at up to $199.99/month was floated.
- **Muse Spark open weights** promised "soon" by Zuckerberg; no date or license.
- **Anthropic IPO**: reported confidential filing 1 June, target September or early October; newest source dated 11 Aug; no status found for October.
- **Grok 4.7 released 21 Sep** (single source, Presenc AI); SpaceX Q2 call reportedly said 4.7 would come 3-4 weeks after 4.6 and Grok 5 before end of 2026. Not verified.
- **Qwen 4 release** "soon"; no date.
- **GLM-5.5 (Zhipu)**: reported for August; no official announcement found.
- **MiniMax 2.7T open-weight model ("M3 Pro")** planned for Q3 2026; no shipment found. M3.1-Flash-Preview went live 27 Sep per one tracker.
- **Amazon winding down Nova Premier, Omni, Reel and Canvas** to focus on a new flagship, possibly unveiled at re:Invent (Business Insider via Mezha; undated).
- **Kimi K3 license**: sources disagree on whether it is Modified MIT.
- **VentureBeat headline "Haiku 5.5 with 90% API price reduction, matching GPT-6 Luna"**: cited by LDS but not fetched.
