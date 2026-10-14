# A Free Model's Rate Limit Opens the Whole Provider's Circuit; Stub Stories Publish

Date: 2026-10-14, 09:30 UTC. Found by the owner on the live paper ("some
(not all) of the articles published are stubs"; phone screenshots at
09:21-09:23 UTC). Computer: the live paper
(computer-1355b83bced1613eb048ee28d66688a8, build 9ace75e5, VM
vm-1669020adcece51c2f5ff9b6df399ef5). Status: documented; not fixed.

## What the owner saw

Published v4s of four stories ("OpenAI lays out principles for
third-party AI assessments", "Apple adds Indigenous languages to iPhone
keyboards", "US agents used fake cell towers under warrants in Don Lemon
p...", and others) are one paragraph saying the claims are unverified:
"the official page could not be retrieved", "no execution or verification
work was performed for this account", "the previous research attempt
failed without returning evidence". Others are full stories.

## Trace

- Paper runs 09:04-09:24 UTC: research 29 failed, 7 completed, 4 running,
  1 blocked; texture 49 completed, 3 failed; management 2 completed,
  1 failed.
- Every sampled research failure: `tool loop iteration N: gateway call
  failed: gateway client: provider opencode-go: circuit open (upstream
  unhealthy)`.
- Gateway journal: `provider: opencode-go rejected request status=429
  reason="server_error Upstream request failed: Rate limit exceeded."`
  for `model=cat-jam-free` (09:22-09:24, repeatedly); the circuit for the
  provider `opencode-go` opens and every opencode-go call fails at once,
  deepseek-v4.1-flash included. Calls routed to `chatgpt` / gpt-6-luna
  succeed (for example 09:24:09, 09:24:11, 09:24:14). The circuit first
  opened at 04:25:41 and again at 07:49:23.

## Causes (from the trace and code reading; to confirm in the fix)

1. **The circuit breaker is keyed by provider, not by model.** A free
   preview model's upstream rate limit (429) takes down the paid model
   on the same provider (deepseek-v4.1-flash, the Go plan).
2. **Failover is incomplete.** Some calls fail over to gpt-6-luna and
   succeed; many runs still end failed on "circuit open", so the run's
   fallback chain (pool order: deepseek, cat-jam, luna) does not reliably
   skip an open circuit and reach the next provider.
3. **Load shift.** LongCat left the default pool at 05:27 (d5139fd6,
   reached the paper at its 06:03/06:37 refreshes), so its threads moved
   to deepseek, cat-jam and luna; two of the three remaining members are
   on opencode-go, and cat-jam's free tier now rate-limits.
4. **Stubs publish.** With research failed, Texture writes a revision
   that says the claims are unverified and narrates the process ("no
   execution or verification work was performed"); auto publish
   publishes it. A version with no cited source should not publish
   automatically.
