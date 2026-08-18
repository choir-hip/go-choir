# Effects gateway inference EOF after retry repair — 2026-08-18

**Boundary:** execute. Problem documentation first. Not freeze. Not promote. No live send.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Source:** `main@56c01c7298ed1443eb439430fd18111537c2bfa9`
**CI:** `https://github.com/choir-hip/go-choir/actions/runs/32099869536`
**Deploy receipt:** CI job `95600170759`, activation receipt target `56c01c7298ed1443eb439430fd18111537c2bfa9`, selected host service `gateway`.

## Live observation

After `56c01c72` deployed its `GatewayClient` retry change, the retained computer
was owner-refreshed from epoch 300 to epoch 301 at `2026-08-18T05:02:50Z`.
The idempotent operations POST returned HTTP 200 for operation
`selfdev-b090bcd72d300fed17cb3f5a142f8595`.

Super run `b93aa0bb-f192-49e6-bab4-2fca2e021d9b` opened implementation
CoSuper assignment `assignment-ee19e577-42d3-5069-bbad-d9d3960bc5c1` in capsule
`capsule-c2e31ca7-c775-5373-800a-c55d2ebefbb9`. The CoSuper reached 42 tool-loop
iterations; the guest runtime log records iteration 42 at
`2026-08-18T05:06:57Z`.

The CoSuper run then terminated at `2026-08-18T05:11:59.887Z` with:

```
tool loop iteration 42: gateway call failed: gateway client: http call: Post "http://10.200.216.1:8084/provider/v1/inference": EOF
```

The assignment is now `disposition=cancelled`, `capsule_disposition=revoked`,
and its run is `cancelled`. No candidate, bundle, freeze, verification, or
promotion exists. The operation remains `executing` with no bundle; its
`effective_head` and `desired_head` remain
`a3cf16d0d1dbb46e4ebd5841af5007575fb74184d54c2e6fa26f856769b92b44`. All
CoSuper assignments on the trajectory are terminal and no run is active.

## Deployment scope

The deploy job passed and intentionally selected only the `gateway` host
service because the source change is in `internal/gateway`. The deploy log's
activation receipt records gateway `56c01c72`; proxy, auth, vmctl, corpusd,
maild, sourcecycled, and the retained guest image remain on the prior
`3c4f2cec` artifact by the deploy impact classification. The `/health` and guest
observability build fields therefore are not evidence that the gateway package
is stale.

## Belief delta

The capsule toolchain, writable overlay, cancellation reconciliation, and
capability-renewal repairs were sufficient for this attempt to execute 42 tool
iterations. The deployed gateway retry change did not produce a successful
CoSuper completion in this occurrence. The remaining failure is a protected
gateway/provider transport boundary; this receipt does not establish whether
EOF originated from the gateway process, its upstream provider connection, or
an exhausted retry sequence.

## Remaining error and safe next probe

Do not retry the operation blindly. First inspect the exact deployed gateway
retry path and its tests, then obtain evidence that distinguishes a repeated
upstream EOF from a gateway-side connection drop and records attempt outcomes.
No gateway/provider credential, route, model-policy, or live-effect mutation is
authorized by this receipt. Effects remain OFF, mode remains `propose_only`,
`Armed=false`, and the constructed freeze `7122f279` is not promotion
authority.

**Rollback:** this receipt makes no runtime mutation; correct or supersede the
receipt in the Definition if any observed identity is later disproved.
