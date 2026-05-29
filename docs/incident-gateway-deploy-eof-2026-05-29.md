# Gateway Deploy EOF Incident - 2026-05-29

## Summary

During a live demo on staging, a prompt-bar trajectory produced this channel
error:

```text
tool loop iteration 0: gateway call failed: gateway client: http call: Post "http://10.200.60.1:8084/provider/v1/inference": EOF
```

The failing trajectory was `24469b07-6300-43a9-b78b-558a54c5d978`; the failed
child run was `de7066b4-71c6-4fe6-b118-ab6ba981c710`; the channel id was
`76073139-4d8e-47c6-95d9-1d4448e4b532`.

## Evidence

Staging was deploying commit `4956f4cdf40645c5249cbdff9e558e61d5edaa31`
at the same time as the demo run.

Node B gateway logs show the request that later failed:

```text
2026/05/29 17:15:03 gateway: inference request from sandbox vm-5b0c1bef1e2b6d7f8dad7d0e8473ed19 (provider=fireworks model=accounts/fireworks/models/deepseek-v4-flash messages=1 tools=16 tool_choice= system_chars=8853 max_tokens=0 reasoning=medium stream=false)
2026/05/29 17:15:03 provider: fireworks call model=accounts/fireworks/models/deepseek-v4-flash
```

There is no matching provider completion or gateway success/error line for
that request. Instead, the deploy activated a new NixOS configuration and
stopped the gateway:

```text
2026-05-29T17:16:57Z activating NixOS config
2026-05-29T17:16:57Z stopping the following units: go-choir-auth.service, go-choir-gateway.service, go-choir-maild.service, go-choir-platformd.service, go-choir-proxy.service, go-choir-sandbox.service, go-choir-vmctl.service
2026/05/29 17:16:57 gateway: received terminated, shutting down gracefully
```

The VM runtime logged the user-visible failure immediately afterward:

```text
2026/05/29 17:16:57 runtime: run de7066b4-71c6-4fe6-b118-ab6ba981c710 -> failed: tool loop iteration 0: gateway call failed: gateway client: http call: Post "http://10.200.60.1:8084/provider/v1/inference": EOF
```

GitHub Actions run `26651286252` selected a full host OS, ordinary guest, and
Playwright guest deploy because the pushed range from
`ceace0cf823b23f2a3ea3a206f4d905138df7985` to `4956f4cdf40645c5249cbdff9e558e61d5edaa31`
included `flake.nix` from earlier commits in the stack. The visible tip commit
was frontend-only, but the pushed range was not.

## Current Belief State

This was not a Fireworks provider outage and not a gateway credential failure.
The provider call was in flight when staging deploy sent SIGTERM to the gateway.

The likely code root cause is in the shared server shutdown path. `Server.Start`
starts shutdown in a goroutine. When `http.Server.Serve` returns
`http.ErrServerClosed`, `Start` returns immediately, allowing `main` to exit
while `Shutdown` is still waiting for active HTTP handlers. That turns intended
graceful shutdown into an active connection cut for long provider calls.

The remaining error field is whether the fixed shutdown path alone is enough for
long model calls during host deploys, or whether deploy orchestration also needs
an explicit active-run drain before host service restart and VM refresh.

## Next Fix

Fix the shared server lifecycle so process exit waits for shutdown completion
and active handlers get the configured grace window. Then prove with a
regression test that a SIGTERM during an active request does not return from
`Start` before the handler finishes.

Follow-up hardening should make deploy output distinguish "tip commit changed
frontend" from "pushed range included host/guest changes" so demos are not
surprised by full staging restarts.
