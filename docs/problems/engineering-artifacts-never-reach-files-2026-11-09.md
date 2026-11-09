# Engineering's artifacts never reach Files, and capsules time out at start on d@e (2026-11-09)

Station L7 of [the ordinary prompt path](../definitions/choir-ordinary-prompt-path-2026-11-09.md).

## Evidence (d@e, trajectory 0ddef9e4-badd-515a-93a3-06773856a669, doc 6e7d1bba)

Prompt: fetch FRED UNRATE and CPIAUCSL for ten years, analyse, make a PDF
report with a chart, put it in Files, open it in Reader, embed the chart.

- Research fetched the series; Texture wrote a sound analysis (correlations
  -0.36 level, -0.51 on 12-month changes).
- Texture cast engineering three times (assignments c9966373 at 04:34:29,
  a5ac9729 at 04:37:05, 4a175702 at 04:40:05). Each was cancelled about 90 s
  after its durable open with `capsule_disposition: unbound`: "spawn assigned
  capsule after durable open: capsule bind broker: capsule start broker
  launcher: context deadline exceeded". The 90 s deadline is
  internal/agentcore/engineering_assignment_runtime.go:866; the error wraps
  internal/capsule/executor.go:991 and start_command.go:28. A launcher
  timeout is cancelled outright: no retry, no admission queue (only host
  memory refusals are deferred, engineeringHostMemoryWait).
- Texture revision 5 reported the cancellations truthfully and waited.
- No PDF in Files.

## Two problems

1. **Capsules do not start on d@e** (three in a row, network_mode claude).
   Hypothesis, unconfirmed: d@e joined the Claude delegate grant in 82e4e7727,
   so its engineering capsules now take the claude network mode, and the
   start path waits on something (the gateway address attestation, the
   claude route) past the broker's 10 s start timeout and the 90 s outer
   deadline. To confirm from the broker and vmctl logs before any fix.
2. **Engineering has no path to put a file in Files, by design.** The
   capsule sees Files as a read-only point-in-time snapshot at
   /computer/files, its writes there are scratch
   (docs/design/capsule-overlay-computer-state-2026-11-03.md, lines 108-131),
   and only the source patch and screens return (Freeze; screens are written
   host-side through ownerfiles, internal/agentcore/selfdev_screens.go:134).
   An engineering deliverable that is a document (a PDF, a chart, a CSV)
   cannot reach the owner even when the capsule runs. Nor does any desk verb
   open a Files path in Reader.

## Belief and next

The L7 acceptance cannot pass until both are fixed. Design space for (2),
cheapest first: an artifacts channel beside screens (the capsule writes
deliverables under one output directory; on completion the host copies them
into Files/Reports/<assignment> through ownerfiles, bounded in count and
bytes, with types refused outside an allowlist), and the completion names
the paths so Texture can embed them with the L8 {{image}} and {{file}}
tokens; or a host-side render tool for Texture. The first keeps engineering
the maker of artifacts and reuses the screens path.
