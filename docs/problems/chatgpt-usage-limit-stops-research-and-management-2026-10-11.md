# One provider's usage limit stops research and management

Found 2026-10-11 during the P1 deployed demo (staging 0f1c86c7, reader test
computer demo-arxiv-2026-10-10c). Status: documented, not fixed. The fix is a
provider-routing change (red), which waits for the owner.

## What happened

After Texture's second run opened research at 17:16:00Z, every research run
failed within a minute:

```
17:17:20 run d9cf5084 → failed: tool loop iteration 3: gateway call failed:
         gateway client: provider chatgpt: circuit open (upstream unhealthy)
17:18:16 run 89eca672 → failed: (same)
17:19:17 run 71958fbf → failed: (same)
17:20:11 run 6d72b5bd → failed: (same)
```

The gateway on Node B logged `chatgpt: status 429 Too Many Requests` eleven
times between 17:16:04 and 17:20:10, with no 429s in the six hours before. The circuit
breaker then opened. The four retries stopped by themselves. Texture runs on
a different provider (opencode-go, DeepSeek), so its revision still landed,
but the brief got no research.

## What is known

- The default model policy (internal/modelpolicy/model_policy.go, both the
  default policy text and fallbackPolicy) routes conductor, management,
  research and both verifier roles to the ChatGPT provider. Only Texture
  and engineering use another provider.
- There is no per-role provider fallback. The search path has rotation and
  fallback (internal/gateway/search.go); the inference path does not.
- The same afternoon, three review panelists that use OpenAI models failed
  locally with a usage-limit message (P1-review2, 16:36Z). Whether that is the
  same account as the gateway's ChatGPT credential is not established.
  The local Codex CLI reports the limit resets at 18:59Z; if the accounts
  are the same, staging research returns then.

## Why it matters

One account's limit stops four of the six roles, including research and
management, which every Autopaper station depends on. The failure is visible
in the guest console, but the owner sees only a brief that stops growing.
Under the protocol, a failed research run should come back to Texture as an
observation record; today it does not.

## Options (owner decision)

1. Per-role fallback in the model policy: a second provider for each role,
   used while the first one's circuit is open. This is a red change to
   provider routing.
2. Move research to a provider with more headroom by default.
3. Accept the dependency and treat a ChatGPT limit as an outage.

Until the owner decides, the work continues on whatever does not need
research inference, and the next research demo waits for the limit to
reset.

## Owner note (2026-10-11, about 20:30Z)

The owner's local Codex still had ChatGPT credits while staging was at the
limit, and it is the same account as Node B's. Copying the local
credentials would add no quota (limits and credits are per account) and
would share one rotating refresh token between two clients, so nothing was
moved. Open question (hypothesis): credits may apply to Codex CLI requests
but not to the gateway's direct calls on the same account. The next time
staging hits the limit, check whether the gateway's refusal names credits,
and whether a Codex CLI call on the same account still succeeds at that
moment.
