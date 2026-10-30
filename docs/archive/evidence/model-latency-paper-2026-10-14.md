# Model Call Latency on the Paper Computer, 2026-10-14

Measured about 04:25 UTC from the live paper computer's last 100 runs
(computer-1355b83bced1613eb048ee28d66688a8, build 66fe0917). One call is
the gap between a `loop.progress` event that opens an iteration and the
event that reports that iteration's tokens. 760 calls in all.

| model | calls | p50 | p90 | max | median input tokens | p50 by desk |
|---|---|---|---|---|---|---|
| gpt-6-luna (chatgpt) | 103 | 2.5 s | 10.0 s | 34 s | 10,767 | texture 5.2, research 2.1, management 2.1 |
| cat-jam-free (opencode-go) | 232 | 1.9 s | 13.4 s | 199 s | 11,897 | texture 2.8, management 1.2, research 2.1 |
| deepseek-v4.1-flash (opencode-go) | 205 | 8.4 s | 54.0 s | 318 s | 15,159 | management 3.9, research 5.7, texture 12.9 |
| **longcat-2.5-preview-free (opencode-go)** | 220 | **21.8 s** | **115.4 s** | **932 s** | 47,745 | research 22.0, management 18.2 |

LongCat by input size:

| input tokens | calls | p50 | p90 |
|---|---|---|---|
| under 20k | 60 | 6.3 s | 66.7 s |
| 20k to 50k | 52 | 20.2 s | 91.1 s |
| 50k and over | 108 | 35.5 s | 157.8 s |

Reading:
- LongCat is about 9 times slower than gpt-6-luna at the median and 11
  times at p90. It is slow even on small inputs, so this is the model, not
  only the context.
- It was the model on the 38-minute management turn that stopped the
  paper (docs/archive/problems/management-serial-turn-blocks-paper-2026-10-14.md),
  and on the 5-minute support judgement in the prompt-bar reproduction
  (docs/archive/problems/prompt-bar-research-slow-unsynthesized-mangled-2026-10-14.md).
- The owner added the free models to the pool on 2026-10-13 "to measure
  how much they carry and whether they meet the bar". On latency, LongCat
  does not. Cat Jam does, at the median and p90; its max (199 s) is worth
  watching.

Decision (2026-10-14): LongCat leaves the default pool and stays in the
catalog, like step-5-preview-free. Rollback: put the pool line back. Cost
effect: LongCat was free; its threads go to deepseek (Go allowance), Cat
Jam (free) and Luna (subscription). At list rates, the last window's
LongCat traffic was about $2.37
(docs/evidence/model-prices-2026-10-14.md).
