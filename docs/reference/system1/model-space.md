# System 1 model space: every way to get a fast, cheap, calibrated decision

Reference subpage (convention: [../README.md](../README.md)). Sibling pages:
[calibration-and-cascades.md](calibration-and-cascades.md),
[dual-process.md](dual-process.md), [evaluation.md](evaluation.md). Context:
[../JEV.md](../JEV.md) (the cloud System 1 we use today),
[../ON-DEVICE-MODELS.md](../ON-DEVICE-MODELS.md) (the phone candidates).
Researched and fetched 2026-11-10. All dates below are 2025 to 2026.

Tags: `[V]` vendor's own page or figure; `[T]` third party or independent
evaluator; `[P]` peer-reviewed or arXiv paper; `[R]` relayed, unverified
(an aggregator or news item whose figure I could not trace to a primary
page). Fetches went through a tool that summarises pages with a small model,
so figures were read from summaries (papers: from the PDF text) and quotes are
short. Floor device: iPhone 14 Pro Max (A16, 6 GB). **No source found gives any
decision-model figure on an A16 or on any iPhone.**

## Surprises

1. **The "decision model" is now a market, not a product.** In October 2026 at
   least fifteen hosted or open models answer typed questions with
   probabilities in one pass (Jev, Liquid d1, Perplexity Decider, Microsoft,
   OpenAI "Decisions", Cloudflare Clef, AWS Strands Decider, Baseten Mammoth
   and Elephant, GLiDE, Drex, Celeris, SubQ Snap, Mercury, H2O, D3). Most copy
   Jev's wire shape (`state` plus named `questions`; `noul`/`choice`/`score`).
   An aggregator called Strands Decider "the third free Jev clone in a week"
   `[R]`. Our `/v1/systemone`-shaped call is therefore portable.
2. **Jev is no longer the top decision model on the independent index.** On
   Artificial Analysis (AA) Decision Index v2, snapshot 2026-11-10, Jev 1.14 is
   rank 8 of 17 (67.0) behind Drex Max, GLiDE, mammoth-1, Perplexity Decider,
   d1-ultra, SubQ Snap and elephant-1 `[T]`. On v1.0 (2026-10-13) Jev 1.13 was
   first at 58 `[T]`. Rankings changed in under a month, and index versions are
   not comparable (d1-3B is 48.57 on one version and 38.90 on another `[V]`
   `[R]`).
3. **A phone-sized decision model exists on paper and is weak.** The only
   sub-1B decision model, d1-omni-600M (587M), scores 15.95 where d1-3B scores
   48.57 on Liquid's own index `[V]`. The phone-runnable strong one is d1-3B at
   1.67 GB Q4_K_M, served by llama.cpp on the same `/v1/systemone` path `[V]`,
   with latency published only for an M5 Pro Mac, Jetsons and desktop GPUs.
4. **State length, not question count, is the cost on small hardware.** d1-3B:
   1 question 30 ms, 3 questions 41 ms, a 3.4K-token state 640 ms on an M5 Pro
   `[V]`. The phone will pay for our long states (stories lists, transcripts)
   far more than for extra questions.
5. **A 183 MB, seven-head encoder answers 14 ms on four server cores** and
   beats Jev 1.13 on its own narrow task, trained on 39k LLM-labelled synthetic
   requests in about one hour on one RTX 4090 `[V]` (the operator of a hosted
   rival; conflict disclosed). This is the template for the phone System 1.
6. **Trained small beats zero-shot big on stable tasks, and loses when the
   rules change.** Banking77: a 22M encoder plus logistic regression 93.2% at
   8 ms on CPU, Jev zero-shot 80.1%; Yelp stars: Jev 67.2% beat a trained
   classifier at 51.9% `[T]` (a vendor-adjacent blog; recompute before using).
7. **Billing differs by up to 60x and the token price is not the driver.**
   Some APIs bill the shared state once per request (Jev); GLiDE and
   Decider-Flash bill it per question (63k vs 9k input tokens for one AA task)
   and d1-ultra is described as billing each question separately `[T]` `[R]`.
8. **Apple's on-device model exposes no token probabilities** (Apple Q&A and a
   June 2026 developer-forum question `[R]`). "Apple's model first" for
   decisions therefore means uncalibrated self-reported confidence.
9. **Decision models are overconfident exactly where it matters.** AA: across
   almost all probability levels, accuracy falls below stated probability on
   hard knowledge items; they are mostly calibrated on tool-calling and safety
   `[T]`. Detail in [calibration-and-cascades.md](calibration-and-cascades.md).
10. **Option order still moves answers.** Moving the gold option to D cost
    gpt-3.5-turbo 6.3 points (67.2 to 60.9); most of it is a prior over option
    ID tokens, correctable by estimating the prior on about 5% of samples `[P]`.

## 1. Taxonomy

| Code | Family | Answers by | Examples (this page) |
|---|---|---|---|
| A | Hosted decision models | one forward pass, typed answers with probabilities, no output tokens | Jev, d1-ultra, Microsoft-Decision-1, OpenAI Decisions, Clef, Mammoth, Elephant, GLiDE, Drex, Celeris |
| B | Open-weight decision models | same, self-hosted | d1-3B, d1-omni-600M, Strands Decider 2B, Perplexity Decider 27B/4B, Clef open weights |
| C | Verdict, guard and router models | short label or class (generative or encoder) | Arch-Router 1.5B, Prompt Guard 2 (22M/86M), Qwen3Guard (0.6B to 8B), Granite Guardian 4.1 8B |
| D | Encoders with answer heads | one pass of an encoder, one head per question | fixed-schema router (mmBERT-small), GLiClass, SetFit, zero-shot NLI, cross-encoder scorer |
| E | Small generative model used as a scorer | one constrained token, read logits | Qwen3-0.6B, Gemma 4 E2B, LFM2.5/LFM3, Apple Foundation Models |
| F | Small hosted LLM, reasoning off | one short generated answer | Claude Haiku 5.5 low-effort, GPT-6 Luna |

### 1.1 Decision models (A and B)

What they are: the answer is read from a readout of the final hidden states,
not generated. Liquid: "do not produce tokens" `[V]`. AWS Strands Decider: the
language-model head is removed and replaced by a pointer head of about 1M
parameters plus a rank-16 LoRA on Qwen3.5-2B (1.9B total) `[R]`. Perplexity:
fine-tune of a 27B base with a "separate readout head" and no LM head `[R]`.
AA's definition: "choose from a fixed set of options, with a probability for
each, in one call"; language models by contrast return a structured output
naming one option with no per-option probability `[T]`.

AA Decision Index v2 snapshot, 2026-11-10 (17 decision models; v2 headline is
the Decision Score, 50 x (1 + accuracy - Brier), so confident wrong answers
are punished) `[T]`:

| Rank | Model | Index | $ per 1M tasks | Median time per task |
|---|---|---|---|---|
| 1 | Drex Max v1.1 | 73.4 | 102 | 275 ms |
| 2 | GLiDE v1.2 | 72.8 | 780 | 289 ms |
| 3 | mammoth-1 | 72.4 | 262 | 96 ms |
| 4 | Perplexity Decider v1.2 27B | 70.5 | 68 | 222 ms |
| 5 | d1-ultra | 70.4 | 132 | 295 ms |
| 6 | SubQ Snap 1.1 | 69.2 | 136 | 138 ms |
| 7 | elephant-1 (9B) | 67.5 | 92 | 70 ms |
| 8 | Jev 1.14 | 67.0 | 156 | 93 ms |
| 9 | Celeris-1.1 Decision | 63.5 | 14 | 56 ms |
| 12 | Microsoft-Decision-1 | 62.8 | 142 | 261 ms |
| 13 | GPT-6 Luna (Decisions) | 61.7 | 337 | 209 ms |
| 14 | d1 (older) | 61.6 | 63 | 141 ms |
| 15 | Perplexity Decider Flash 4B | 61.6 | 196 | 124 ms |
| 16-17 | Clef, Clef-Omni | 60.2, 59.3 | 859, 527 | 335, 183 ms |

For reference, on the same v2 article: Claude Haiku 5.5 (Max) 77 at $6,240 and
32.65 s per task; GPT-6 Luna (Max) 76 at $1,530 and 11.34 s `[T]`. The v1.0
article (2026-10-13): Jev 1.13 58 at $42 and 0.13 s; Haiku 5.5 (Low) 61 at $184
and 1.4 s; "Haiku is about 11 times slower and 4.4 times more expensive for 3
more points" `[T]`. The $/task figures depend on the benchmark's task size
(Jev 1.13 was $42 on v1 and Jev 1.14 is $156 on v2); do not compare across
versions, and do not transfer to our prompts (see section 4).

Notable properties:

| Model | Size, licence | Price and shape | Evidence | Tag |
|---|---|---|---|---|
| Jev 1.13/1.14 | closed, hosted | $0.042/M input, state billed once; `POST /v1/systemone` | see JEV.md | `[V]` |
| Liquid d1-ultra | closed, hosted; text and image; 44 languages; 32K context | $0.042/M; `POST /decisions/v1/systemone`; each question billed separately | AA v1.0: 65, first, 85% non-hallucination rate on AA-Omniscience-Decision; says trained so "90% is right at least 90%" | `[R]` aggregator of vendor |
| Liquid d1-3B | 3.12B, LFM Open License v1.0 (commercial use only under $10M revenue) | open; GGUF Q4_K_M 1.67 GB, Q8_0 2.87 GB, BF16 5.4 GB | Decision Index v0.2.1 48.57; knowledge 23.8 weakest; 83.3 vs Decider 4B 76.0 on SQuAD 2.0 | `[V]` |
| Liquid d1-omni-600M | 587M, same licence | text plus image or audio; encoder base (LFM2.5-Encoder-350M) | 15.95; mean 78.4 vs d1-3B 82.9 on seven public sets; best on Civil Comments (95.8) and PAWS-X (79.5) | `[V]` |
| Microsoft-Decision-1 | closed; Qwen3.5-9B base | $0.042/M; Foundry and OpenRouter; vendor median 85 ms, a third party measured 0.46 s | AA v1.0: 54, highest non-hallucination (40%) | `[R]` |
| OpenAI Decisions | closed; runs `gpt-6-luna` only | $0.10/M, output and cache free; 1 to 200 questions; `predicate` for yes/no | JevBench calibration 90; most expensive hosted API on JevBench ($0.052 per 1,000 decisions) | `[R]` |
| Perplexity Decider v1.2 | 27B (and 4B Flash), Apache-2.0, needs about 49 to 52 GiB GPU memory | hosted $0.02 or $0.04/M (sources disagree); up to 128 questions | vendor ECE 0.014 on its own dev set; vendor panel 85.71% vs Jev 84.51% | `[R]` |
| AWS Strands Decider 2B | 1.9B, Apache-2.0, self-host | `strands-decider serve`, same shape as Jev | 153 ms on M3 Pro (vendor, early checkpoint), 280 ms warm median measured by a reviewer; Decision Index 0.3.2 21.34 (rank 135); a reviewer saw 0.209 vs 0.912 for two phrasings of one yes/no on the same case | `[R]` |
| Cloudflare Clef / Clef-flash | 27B / 9B, Apache-2.0 | $0.24 / $0.038 per M input; vendor medians 209 ms / 39 ms | Decision Index 0.3.2 53.08 / 47.61 | `[R]` |
| Baseten Mammoth-1, Elephant-1 | 27B MIT; 9B | about $0.05 / $0.02 per M (estimated) | AA-Symmetry 96 and 93; same answer across variations on 99% of questions | `[T]` for AA-Symmetry, `[R]` rest |
| H2O-Lightning | 31B; H2O non-commercial licence | about $0.09/M (estimated); 63 ms | restricts commercial use | `[R]` |
| Celeris-1.1 Decision | n/s | $14 per 1M tasks, 56 ms | Paperclip pass rate 24% but Brier 0.15 vs Clef 0.27 | `[T]` |

Sources disagree on facts as basic as the d1-3B scores and the Perplexity
price. Treat every `[R]` row as a lead to confirm on the vendor page before
spending money.

### 1.2 Verdict, guard and router models (C)

| Model | What it is | Size, licence | Evidence |
|---|---|---|---|
| Arch-Router | generative router: maps a query to a user-defined "domain-action" policy name; new routes without retraining | 1.5B (Qwen 2.5 base), fine-tuned on 43,000 examples | paper `[P]` claims state of the art but its abstract gives no numbers; "over 28x faster than its closest competitor" and "51 ms" are secondary `[R]`; co-author on HN: under 100 ms end to end, 50 ms on an L40S `[R]` |
| RouteLLM | trained routers (matrix factorisation, BERT, causal LLM) choosing strong or weak model from preference data | routers small | over 2x cost reduction without quality loss on public benchmarks `[P]` |
| Llama Prompt Guard 2 | BERT-style classifier; labels `benign` or `malicious`; needs no prompt template | 86M (mDeBERTa-base, multilingual) and 22M (DeBERTa-xsmall, English); 512-token window; Llama 4 Community License | Meta: the 22M "reduces latency and compute costs by 75%"; mirror lists 92.4 ms and 19.3 ms, recall at 1% FPR 97.5% and 88.7% `[R]` |
| Qwen3Guard | Gen (generative, labels safe, controversial, unsafe) and Stream (per-token heads on the last layer) | 0.6B, 4B, 8B; Apache-2.0 `[R]`; 119 languages | the 0.6B Gen "rivals or exceeds" much larger guards on English benchmarks; Stream trails Gen by about 2 points `[P]` |
| Granite Guardian 4.1 | yes/no judge for a user-supplied criterion; 3.0 scored by softmax over the safe and unsafe token log-probs | 8B (4.1), Apache-2.0 `[R]` | no latency figure found; 3.0 paper AUC 0.871 guardrail `[R]` |

Pattern: guards are decision models in disguise (a fixed label set, one pass,
a probability read from a head or two logits). Their value for us is the
**shape of the head** (two-class, per-token) and the trained-on-adversarial-
data lineage, not a ready answer to our questions.

### 1.3 Encoders with answer heads (D)

| System | Size | How it answers | Evidence |
|---|---|---|---|
| Fixed-schema router (`s1-llm-auto-router`) | mmBERT-small pruned to 11 layers; 183 MB int8 ONNX; MIT | seven heads: `category` (9), `difficulty` (5), `stakes` (4), four yes/no flags; per-head temperature in `meta.json` | 14 ms p50, 47 ms p99 for all seven on 4 pinned cores of an EPYC 9555; 26 ms p50 on 4 threads of a Xeon E5-2680 v4; n=2,175: category 0.898, flags mean 0.974, all-seven exact 0.819, routing agreement 0.742; calibration error 0.018 category, 0.008 to 0.028 flags; 100-item Opus-relabelled subset routing 0.88 (about +/- 8 points) `[V]` |
| mmBERT | small 140M total (42M non-embedding), base 307M | encoder; fine-tune heads | GLUE 84.7 small vs 78.3 multilingual MiniLM; XNLI 73.6 vs 71.2 `[P]` |
| GLiClass v3 | edge 32.7M (131 MB), base 187M, large 439M | labels are concatenated with the text and encoded once (uni-encoder), then scored | zero-shot average F1: edge 0.4900, base 0.6764, large 0.7193; throughput 97 to 25 examples per second on an A6000; 1 to 128 labels costs 7 to 20% throughput while a DeBERTa NLI cross-encoder fell 24.55 to 0.47 ex/s (52x); 8 examples per label add +0.19 (edge) to +0.11 (large) F1 `[P]` |
| SetFit | sentence-transformer 110M to 355M | contrastive fine-tune of the encoder on a few pairs, then a logistic head | 8 labelled examples per class; SetFit RoBERTa beat GPT-3 by 8.6 points and PET by 1.7; training about 30 s (MPNet) to about 700 s (RoBERTa-large, $0.7) `[P]` |
| Zero-shot NLI cross-encoder | 100M to 400M | one forward pass per (text, label) pair | Banking77 48.8% classic, 66.7% updated `[T]` |
| Vela-1.0 encoder | 307M, 32K context, MIT | mmBERT-base continued pre-training; a base for routing heads | masked-token NLL 1.387 vs 1.441 at 512 tokens `[V]` |

Observations that change the plan:

- **GLiClass edge scored 0.0272 on Banking77** (77 labels) in zero-shot, and
  0.2217 on 20 newsgroups `[P]`. Zero-shot "any label" encoders are a lottery on
  fine-grained tasks. The data-heavy fixed-schema router is the reliable form.
- **Multi-head training is cheap and multi-question by construction.** One
  encoder pass feeds all heads, so adding a question costs a linear layer;
  the vLLM Semantic Router trains LoRA per task on a shared base instead of
  separate fine-tunes `[R]`.
- **On-device export evidence is old and good.** Apple's own DistilBERT port
  ran 3.47 ms at 0.454 W (or 9.44 ms at 0.072 W) on an iPhone 13 Neural
  Engine at sequence 128, batch 1, with 14x lower peak memory and "up to 10x"
  speed-up `[V]`; the iPhone 13 is an A15, one generation before the A16. Core ML
  tooling needs `ane_transformers` and A14 or newer `[V]`. No source found
  measures a ModernBERT or mmBERT on an iPhone `[gap]`.
- **Data needs (numbers found).** Few-shot heads: 8 examples per label
  (SetFit, GLiClass) buy large gains on unambiguous tasks. Synthetic
  distillation scale that worked in practice: 39k LLM-labelled requests for
  seven heads `[V]`; an 8B student reached 90.5% agreement with its GPT-5
  teacher after 73K teacher annotations and 80.5% with the human majority `[R]`
  (the figure comes from a search summary, not a page I read). A reasoning-
  free rule: if gold labels are plentiful, train directly; a distilled student
  inherits the teacher's mistakes (tinydistill README `[R]`). I found **no
  published curve of examples versus agreement with an LLM judge**; run your own
  (see [evaluation.md](evaluation.md)).
- **Distillation recipe that is published.** Teacher mmBERT-base fine-tuned on
  soft labels, student mmBERT-small layers {0,2,...,18,21}, alpha 0.9, 12
  epochs, lr 1.5e-4, one hour on one RTX 4090, temperature fitted per head on
  a dev set `[V]`. Soft labels from several LLM judges (a plurality of five open
  LLMs, at least three votes) were the gold label.

### 1.4 Small generative models as scorers (E) and small hosted LLMs (F)

| Technique | Evidence |
|---|---|
| Read the first-token logits over the answer set | 4-bit quantisation costs about 1 to 3% quality `[T]`; llama.cpp supports grammars (Liquid's documented route for phones, via ON-DEVICE-MODELS.md) |
| Constrained or JSON-mode decoding | hurts reasoning (stricter formats degrade it more) but on classification tasks JSON-mode is competitive and sometimes best (Gemini 1.5 Flash on DDXPlus) `[P]` |
| Verbalised confidence vs logit confidence | for RLHF-tuned models verbalised confidences are typically better calibrated than conditional probabilities, often cutting ECE by a relative 50% (TriviaQA, SciQ, TruthfulQA) `[P]`; raw probabilities were "consistently over-confident" for gpt-3.5-turbo `[P]`. Result is for chat models; decision models are trained for calibrated readouts instead |
| Prefix (KV) caching | Prompt Cache: time-to-first-token 8x lower on GPU and 60x on CPU, on long document prompts, with no accuracy change `[P]`. Anthropic: cache reads cost $0.01/M against $0.10/M input on Haiku 5.5 `[V]` |
| Batching questions into one prompt | batch prompting: token and time cost fell roughly inversely with batch size `[P]`; zero-shot multi-problem prompts: classification accuracy 75.5% single vs 72.3% batched across 13 LLMs and 6 classification sets (a small drop, but consistent declines when the task is turned into "choose an index") `[P]`; 50 questions per prompt on transcripts: GPT-4o about 0.95 at group sizes 5 and 10, and fine-tuned 7 to 8B models can beat it `[P]`. Decision models differ: Jev answers are independent by construction (JEV.md) |
| Position and option-ID bias | Llama-30B picks A/B/C/D 34.6/27.3/22.3/15.8% of the time on 1,000 balanced MMLU items; gpt-3.5-turbo 22.5/25.6/... `[P]`. PriDe estimates the prior with permutations on about 5% of samples and removes it `[P]`. Calibrating with content-free inputs ("N/A") lifted GPT-3 and GPT-2 accuracy by up to 30 points absolute `[P]` |
| Apple Foundation Models | typed (`@Generable`) output; **no token probabilities exposed** `[R]`; independent benchmark answered 60% of questions and ran 70 to 86 tokens/s (ON-DEVICE-MODELS.md, relayed) |
| Phone numbers (iPhone 17 Pro, A19 Pro) | Qwen3-0.6B Core AI prefill 5,807 tokens/s, decode 115 tok/s, 0.22 GB (synthetic 512/1024 protocol); Gemma 4 E2B LiteRT-LM decode 61 tok/s, prefill 3,513 tok/s at p=1024, 497 MB; same 0.6B model in a later beta re-export fell to 1,519 prefill tok/s `[T]`. **No A16 data exists in that benchmark** (it asks for older devices) |

A prefill-bound estimate (mine, not sourced): at about 5,800 tok/s a 2K-token
context costs roughly 0.35 s on an A19 Pro for the 0.6B model, before any
A16 slowdown. Time to the answer on a phone is prompt length divided by
prefill speed; use a cached prefix.

## 2. Comparison across families

Latency is the best figure found with its hardware; none is for the A16.
"Calibration" is what the evidence says, not what is promised.

| Family | Latency (hardware) | Marginal $ per 1,000 decisions (our 6-question, 800-token-state example) | Accuracy evidence | Calibration | Data needed | Runs where |
|---|---|---|---|---|---|---|
| A Jev | 93 ms median (AA, hosted); 83 to 150 ms upstream in our log | $0.0081 (state billed once) | AA v2 67.0 | over-confident on hard knowledge; thresholds to tune (JEV.md) | none | cloud |
| A d1-ultra | 295 ms (AA v2) | $0.0361 if the state is billed per question | AA v2 70.4 | trained for calibration (vendor) | none | cloud |
| B d1-3B | 30 ms (M5 Pro), 8 ms (RTX 4090); 640 ms with a 3.4K state | $0 plus device | 48.57 own index; weak on knowledge | not stated | none to fine-tune | Mac, GPU, maybe phone (1.67 GB) |
| B d1-omni-600M | no figure | $0 | 15.95 | not stated | fine-tune | phone-sized |
| C guards (22M to 8B) | 19 to 92 ms (22M, 86M mirror) | $0 | vendor tables | label probability | none | CPU or phone for 22M |
| D fixed-schema encoder | 14 ms for 7 heads (4 server cores); 3.47 ms DistilBERT on an iPhone 13 ANE | $0 | 0.74 to 0.97 vs LLM-ensemble labels on its task | 0.008 to 0.028 per head after temperature | about 39k synthetic or labelled; 8 per label to start | phone, CPU |
| D GLiClass edge | 97 ex/s on an A6000 | $0 | F1 0.49 zero-shot, +0.19 with 8 per label | not reported | 0 to 8 per label | phone-sized (131 MB) |
| E 0.6B scorer | prefill 5.8K tok/s (A19 Pro) | $0 | task dependent, unmeasured by us | logits, bias-prone | prompt only | phone |
| E Apple model | 70 to 86 tok/s | $0 | answered 60% in one benchmark | none exposed | prompt only | iPhone 15 Pro or later `[R]` |
| F Haiku 5.5 low-effort | 1.4 s (AA v1.0 Low) | $0.0885 separate prompts, $0.0165 with cached state, $0.0243 batched | AA v1.0 61 | verbalised, decent | prompt only | cloud |

Cost arithmetic is in [evaluation.md](evaluation.md) section 4.

## 3. Decision guide: which family for which question

| Question kind | Use | Why |
|---|---|---|
| Open vocabulary, new every week, no labels | A (Jev), or B d1-3B on a Mac/server | zero-shot; adapts to rule changes without retraining `[T]` |
| Stable schema we ask thousands of times (acknowledge or not, genre, urgency, needs-tools) | D fixed-schema encoder distilled from Jev's record | 14 ms server, phone-sized, per-head temperature; wins on stable tasks `[V]` `[T]` |
| Safety, injection, harm | C guard (Prompt Guard 2 22M, Qwen3Guard 0.6B) beside any other | trained on adversarial data; cheap |
| Which model or tool to use | C router (Arch-Router) or D heads | the label is the model name |
| Needs world knowledge or a long state | A on the server; escalate to F | knowledge is where small decision models fail (d1-3B knowledge 23.8; AA overconfidence) |
| Needs a probability we can threshold | A, B, D | F needs verbalised confidence; Apple's model gives none |
| Offline on the phone, mid-quality | E (Qwen3-0.6B, Gemma 4 E2B) with a cached prefix, or D | no decision model of the right size is proven on an iPhone |
| Answer depends on another answer | two requests, code in between | questions are blind to each other (JEV.md) |

Proposed phone interface (ours; keep the cloud wire shape so one record format
scores every System 1):

```json
{"state": {"turn": "...", "playing": "..."},
 "questions": {"ack_now": {"type": "noul", "instructions": "Does the listener expect a spoken reply now?"}},
 "route": "auto"}
```

`route` tries D, then E, then the cloud; the response carries `model`,
`probabilities` and `latency_ms` so the Jev record (internal/jevrecord) can
score all of them on the same questions. A phone model earns a question only
when its agreement with Jev on that question passes the bar in evaluation.md.

## Could not verify / open questions

- Any iPhone figure (A16 or newer) for d1-3B, d1-omni-600M, an mmBERT-class
  encoder or Strands Decider; the d1-3B GGUF page has no mobile guidance.
- d1-3B scores differ by source (48.57, 38.90, and two per-dataset tables that
  disagree: SQuAD 2.0 85.3 vs 83.3); the blog and HF post were both fetched and
  disagree with each other.
- d1-ultra billing "each question separately" is an aggregator claim; the
  Liquid API page was not fetched. Perplexity hosted price: $0.02 vs $0.04.
- Whether Apple has since exposed logprobs (nothing newer than June 2026 found).
- ModernBERT, NeoBERT, EuroBERT: no phone or latency evidence fetched.
  Llama Guard and ShieldGemma lineage: not fetched.
- Cloudflare Clef third price; Celeris parameters; AA index methodology page.
- No labelled-examples-versus-LLM-judge-agreement learning curve found.
- The `mindstudio` Banking77 numbers come from a blog; not reproduced.

## Sources (all fetched 2026-11-10)

1. Liquid, Open d1 blog, https://www.liquid.ai/blog/d1-open `[V]`
2. Liquid, HF blog, https://huggingface.co/blog/LiquidAI/open-d1 `[V]`
3. HF model card, https://huggingface.co/LiquidAI/d1-3B `[V]`; GGUF card, https://huggingface.co/LiquidAI/d1-3B-GGUF `[V]`
4. ModelFit, https://modelfit.io/blog/liquid-open-d1-decision-models-mac/ `[T]` (relays vendor numbers)
5. AA Decision Index v1.0, https://artificialanalysis.ai/articles/artificial-analysis-decision-index `[T]`
6. AA Decision Index v2, https://artificialanalysis.ai/articles/decision-index-v2 `[T]`
7. BenchLM snapshot, https://benchlm.ai/benchmarks/artificial-analysis-decision-index `[T]`
8. HF leaderboard dataset, https://huggingface.co/datasets/multimodalart/decision-index-leaderboard `[T]`
9. Pinggy, https://pinggy.io/amp/blog/best_ai_decision_models/ `[R]`; DataCamp, https://www.datacamp.com/blog/top-ai-decision-models `[R]`; AICoder d1-ultra, https://aicoder.com/news/news-20261017-liquid-ai-d1-ultra-decision-model-aa-index-1 `[R]`
10. AI Weekly (Perplexity), https://aiweekly.co/alerts/perplexity-open-sources-27b-decider-edges-jev-on-11-test-panel `[R]`; Strands search results (thenewstack, sqmagazine) `[R]`
11. System1models router card, https://huggingface.co/system1models/s1-llm-auto-router `[V]`; Vela, https://huggingface.co/llm-semantic-router/Vela-1.0-Encoder-307M `[V]`
12. mmBERT, https://arxiv.org/abs/2509.06888 `[P]`; GLiClass, https://arxiv.org/abs/2508.07662 `[P]`; SetFit, https://arxiv.org/abs/2209.11055 `[P]`
13. Arch-Router, https://arxiv.org/abs/2506.16655 `[P]`; RouteLLM, https://arxiv.org/abs/2406.18665 `[P]`; Qwen3Guard, https://arxiv.org/abs/2510.14276 `[P]`
14. Meta Prompt Guard 2, https://developer.meta.com/ai/docs/model-cards-and-prompt-formats/prompt-guard/ `[V]` (latency from a mirror `[R]`)
15. Apple ANE transformers, https://machinelearning.apple.com/research/neural-engine-transformers `[V]`
16. iPhone 17 Pro runtime benchmark, https://github.com/john-rocky/apple-silicon-llm-bench `[T]`
17. Large Language Models Are Not Robust Multiple Choice Selectors, https://arxiv.org/abs/2309.03882 `[P]`; Calibrate Before Use, https://arxiv.org/abs/2102.09690 `[P]`
18. Just Ask for Calibration, https://arxiv.org/abs/2305.14975 `[P]`; Let Me Speak Freely, https://arxiv.org/abs/2408.02442 `[P]`; Prompt Cache, https://arxiv.org/abs/2311.04934 `[P]`
19. Batch Prompting, https://arxiv.org/abs/2301.08721 `[P]`; Evaluating LLMs with Multiple Problems at once, https://arxiv.org/abs/2406.10786 `[P]`; Multi-question QA on transcripts, https://arxiv.org/abs/2509.21732 `[P]`
20. Anthropic pricing, https://platform.claude.com/docs/en/about-claude/pricing `[V]`
21. MindStudio Jev vs BERT, https://www.mindstudio.ai/blog/jev-vs-classic-classifiers-benchmark `[T]`
22. Apple Foundation Models logprobs search results (WWDC25 301, Apple developer forum thread 832158) `[R]`
