# Inbox Delivery Idempotency Proof

Date: 2026-05-13
Mission: `docs/mission-choir-grand-deformation-v0.md`

## Slice

Added pending-delivery idempotency for addressed channel casts:

```text
ChannelCast -> channel audit message -> inbox delivery for target agent
```

The channel log remains append-only and can show repeated model/tool actions. The runtime-owned inbox now dedupes exact still-pending addressed deliveries before the target agent consumes them.

## Reason

The live Playwright worker dogfood showed overproduction:

- multiple worker VM handles from one user prompt;
- duplicate delegate calls;
- multiple equivalent promotion candidates;
- VText edit attempts after the mutation window closed.

Exact duplicate super inbox deliveries are a local recurrence error. They should remain visible in trace, but they should not become multiple independent super obligations.

## Guarantee

For a pending delivery with the same owner, target agent, target loop, source agent, source loop, channel, role, content, and trajectory, `EnqueueInboxDelivery` is idempotent.

This preserves the invariant:

```text
Every durable message is traceable; every pending obligation is idempotent.
```

## Verification

Command run:

```text
CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go test -count=1 ./internal/runtime -run 'TestChannelCastDedupesPendingAddressedDelivery|TestCoagentToolsSupportAddressedCastAcrossProfiles|TestPromptBarToWorkerWorktreePromotionQueueDeterministic|TestQueuePromotionCandidatesForWorkerExportsDedupesExactExport'
```

Result: passed.

Command run:

```text
CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go test -count=1 ./internal/runtime ./internal/store ./internal/promotion
```

Result: passed.

## Boundary

This handles exact pending duplicate addressed deliveries. It does not yet dedupe semantically equivalent objectives, concurrent portfolio workers, or equivalent promotion candidates produced by different worker heads.

## Next Deformation

Add lease/objective fingerprints so the super can tell the difference between:

- one objective accidentally repeated;
- one objective intentionally expanded into a candidate portfolio;
- different objectives that happen to share a channel and target agent.
