# The paper runs without Jev and with one search provider; Synthetic and Haiku carry more of its calls

Date: 2026-11-11. Class: green (record only; nothing changed). Related:
[paper-model-and-search-plane-cluster-2026-11-09](paper-model-and-search-plane-cluster-2026-11-09.md).

## Problem

Found on the hourly Autopaper tick at 20:36Z, from Node B gateway logs (read
only):

1. **Jev has no credits.** Every Jev judgment since 16:05Z fails with
   `OpenRouter returned 402 Payment Required: Insufficient credits`. Today
   there are 3,907 such failures: 3,904 from the paper and 3 from d@e. By
   hour (UTC): 16h 777, 17h 966, 18h 885, 19h 720, 20h 559 to 20:44Z. The
   gateway logs a successful Jev call 12,670 times today, all before the
   credits ran out.
2. **Search has one provider left.** In the last hour every paid search
   provider (Tavily, Brave, Parallel, Serper, SerpApi) is skipped as
   `quota_limited`. SearXNG answers, often with nothing
   (`success_empty`), and 146 searches had zero providers to ask.
3. **The paper's paid and self-development share rose.** In the hour to
   20:36Z the paper's 2,480 calls were 81.3% Devin swe-2-flash, 11.7%
   Bedrock Haiku 5.5 and 6.5% Synthetic (161 calls, $1.00). In the hour
   before, Synthetic had served 15 calls. The Synthetic calls carry the
   same three system prompts (23,464, 15,337 and 11,628 characters, one
   tool, reasoning low) as the paper's Devin and Haiku calls, so Synthetic
   is a step in the same fallback order, not a separate role. Synthetic is
   reserved for self-development.
4. **The tick's Devin health count missed failures.** The gateway logged 8
   `devin: read stream frame: context deadline exceeded` failures for the
   paper in the hour, but those lines do not contain `provider=devin`, and
   the tick's grep required it. The free models iceman and beam were
   `circuit open (model unhealthy)`.

## Belief

Jev and search exhaustion are account money, not code: topping up
OpenRouter and the search providers is the owner's act. Eight Devin
deadlines do not explain 161 Synthetic calls, so the fallback choice is made
somewhere the gateway log does not show (the paper's own router). That is
unconfirmed until the paper's run ledger shows why a turn left Devin.

## Remaining error

- Whether the paper's output quality changed without Jev's judgments is
  not measured.
- Why the paper chose Synthetic is not traced. The next read-only step is
  the paper's turn ledger for one Synthetic call.
- The tick's Devin failure count now matches `provider call failed for
  autoputer vm-1669020a…: devin:` as well as `provider=devin`.
