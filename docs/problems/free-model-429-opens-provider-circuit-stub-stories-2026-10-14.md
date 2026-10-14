# A Free Model's Rate Limit Opens the Whole Provider's Circuit; Stub Stories Publish

Date: 2026-10-14, 09:30 UTC. Found by the owner on the live paper ("some
(not all) of the articles published are stubs"; phone screenshots at
09:21-09:23 UTC). Computer: the live paper
(computer-1355b83bced1613eb048ee28d66688a8, build 9ace75e5, VM
vm-1669020adcece51c2f5ff9b6df399ef5). Status: fixed on a branch (see
"Fix" below); not pushed, not deployed, no staging proof yet.

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

## Mitigation on the paper (09:36 UTC; no deploy)

The owner's OpenCode console (09:21-09:25 UTC) shows the same thing from
the provider side: Cat Jam requests answered `429 Rejected
inference_failed`, while DeepSeek V4.1 requests answered 200. So
DeepSeek was healthy upstream; our gateway's provider-wide circuit kept
it from being used.

A code fix will not reach the paper soon: the paper is never idle, so
deploys do not refresh it. The paper reads its computer-owned model
policy (`System/model-policy.toml`) at each run start, so the policy was
changed through the owner files route (`PUT /api/files/System/model-policy.toml`),
marked `# owner-edited` so platform defaults leave it alone:
texture, conductor, verifier and defaults use `chatgpt/gpt-6-luna`;
management, research and engineering use `opencode-go/deepseek-v4.1-flash`;
nothing routes to cat-jam-free. The previous file is saved at
`~/.config/choir-qa/traces/paper-model-policy-before-0935.toml`.
Trade-off: single models have no fallback until the pool returns. New
runs after the change: texture on gpt-6-luna, research on
deepseek-v4.1-flash (one texture run still on cat-jam, a thread that
started earlier). Restore `pool:default` once the per-model circuit and
failover fix is live on the paper.

## Confirmed from code (2026-10-14, fix branch)

Code read at 2036b6f0. The trace claims above were not re-read; these are
the code paths that produce them.

1. **One breaker per provider.** `cmd/gateway/main.go` wraps each provider
   once (`gateway.WrapMultiProvider`), and `CircuitBreakingProvider`
   (`internal/gateway/circuit_breaker.go`) holds one
   `health.CircuitBreaker` for every model it serves. `providerFailureCounts`
   counts 429 as an upstream failure (deliberately, 2026-10-10). Five
   consecutive counted failures open it for 30 s, and the half-open state
   lets one probe through. Each runtime call is up to three gateway calls
   (`gatewayruntime.Provider.call` retries a 502), and the tool loop
   retries a "429" text three more times (5, 20, 60 s,
   `providerRateLimitRetryDelays`). So a rate-limited cat-jam-free thread
   alone produces the five failures. Whatever call takes the probe slot
   after 30 s decides for every model, and on a busy paper that is often
   another cat-jam call that gets 429 and reopens the breaker. Nothing in
   the code tells a provider-wide failure apart from a model-specific one.
2. **Failover goes one step, to the first fallback only.**
   `modelcatalog.Pools()` orders the default pool [deepseek-v4.1-flash,
   cat-jam-free, gpt-6-luna], and `modelpolicy.Assign` rotates it to the
   thread's member. So the fallbacks are: for deepseek, [cat-jam, luna];
   for cat-jam, [luna, deepseek]; for luna, [deepseek, cat-jam].
   `RunToolLoop` (`internal/toolregistry/toolloop.go`, "Once per run")
   fails over only once, to `fallbackLLMConfigs[0]`. A deepseek thread
   fails over to cat-jam, which is on the same provider behind the same
   open breaker. It then spends its three transient retries (15 s each)
   and ends with `circuit open`. A cat-jam thread fails over to luna and
   succeeds. This is why some calls failed over to luna and others did
   not. The design (docs/model-pools-design-2026-10-12.md §5, "Failover
   loops") assumed a pool of two. With three members, "at most once" stops
   at the second member.
3. **Auto publish does not look at the revision.**
   `Runtime.AutoPublishRevision` (`internal/agentcore/story_publish.go`),
   called after every committed Write by
   `textureowner.CommitCellTextureAuthor`, publishes every non-seed
   revision of a publish-auto story. It checks only the trajectory kind,
   the publish mode and the seed role. A revision whose body cites no
   captured source publishes like any other. The proxy's guest route
   (`HandleGuestTexturePublication`) checks nothing about content either.
   An owner-initiated publish (`PublishModeOwner`) is a different route.
4. **The prompt has no rule for a turn with no new evidence.** The
   Texture core forbids narrating process ("desks, rounds") but never
   says what to do when the records bring no evidence. A failed research
   record became a Write that said so ("no execution or verification work
   was performed").

The load shift (cause 3 above) is policy, not a defect. It is a residual
for pool composition, not part of this fix.

### Failure modes the fix must not have

- A 429 from one model still blocks another model on the same provider.
- A 429 opens nothing, so every run keeps hitting a rate-limited free
  model and fails over only after 85 s of backoff.
- Connection failures to a provider no longer open anything shared, and
  each model has to find out for itself.
- Failover visits a member twice, or bounces between members, so a run
  never ends.
- Failover stops at a member whose breaker is open while a healthy member
  is left, or the run fails before every member has failed.
- Our own cancellation, or a malformed request, triggers failover.
- An uncited revision publishes, or a cited one does not.
- The rule reaches owner-initiated publishes, or retroactively unpublishes
  stubs already published.
- A held revision leaves no visible reason.

## Fix (branch; not pushed or deployed)

- 8f074cb0 (gateway): one breaker per (provider, model) plus one for the
  provider. A model's 429, 5xx and timeouts open only its own breaker.
  The provider breaker counts only transport failures that are not
  deadlines. A model's open circuit says "provider P model M: circuit
  open" and carries no status code. 429 keeps counting, now per model:
  the open model breaker is the short backoff, and Retry-After is not
  plumbed.
- 733ded02 (tool loop, gateway client): failover walks the remaining
  pool members forward, each at most once, and the run fails only when
  the last member fails. The gateway client no longer retries a 502
  that says "circuit open".
- ef5d8e7e (story publish): a publish-auto story's revision that cites
  no source is held. It stays committed, its version is a gap, and the
  next cited revision publishes as its own number. The hold is logged,
  and the turn receipt carries `publish_held`. Owner publishes are
  unchanged, and stubs already published stay published.

Residuals:

- Staging proof is owed for all three. The breaker change needs a
  loaded paper: a cat-jam 429 burst with deepseek still answering, and
  research runs that end completed through luna.
- The prompt line "a turn whose records bring no new evidence writes
  nothing; Wait" was not added. The Texture core is at 3988 of its
  4000-character budget, so adding it means trimming another rule. The
  publish hold covers the reader-facing harm.
- The proxy's guest publish route still accepts any revision. The
  evidence rule is enforced on the guest only.
- The pool's load shift (two of three members on opencode-go) is
  unchanged.
- A Retry-After from the upstream is still dropped by the provider's
  sanitized errors.
