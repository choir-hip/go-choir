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

## Finding for (1), from d@e's recent_log (2026-11-09, read-only)

The hypothesis above was wrong: neither the claude route nor attestation is
involved. The launcher only forks /run/capsule/broker. The **computer
store snapshot** took the whole 90 s spawn budget: `spawn
phase=computer_store took=1m28.759s` (04:36:00), `1m27.952s` (04:38:35),
`1m28.531s` (04:41:36), each `store=false` with `scan
run_memory_entries.summary: context deadline exceeded` or `copy: context
deadline exceeded`; the launcher then got an expired context, hence
"capsule start broker launcher: context deadline exceeded". The other
spawn phases took 23 ms, 741 ms and 55 ms. The snapshot is `VACUUM INTO`
plus a secret scan of every text column (internal/store/snapshot_to.go
~110-122, scanColumnForSecrets :321) over a 95 MB state.db. In the same
windows the guest ceiling reclaim cut used memory from about 2.7 GiB to
1.3-1.6 GiB against `ceiling_mib=1024` (04:35:05, 04:38:06, 04:41:05) with
refault rates near 340/s, and vmctl saw `pressure=true signals=memory,io`.
Physics: copying and scanning 95 MB should take a few seconds; 88 s means
the copy ran while the reclaim evicted its pages (thrash). Since
computer-state-for-claude-mode (2026-11-03) claude capsules take the store
copy too.

So (1) is two defects: the snapshot is charged to the launch deadline (a
slow but live copy kills the spawn), and the memory ceiling reclaims a
guest's working set while it is building a capsule.

## Belief and next

The L7 acceptance cannot pass until both are fixed. Design space for (2),
cheapest first: an artifacts channel beside screens (the capsule writes
deliverables under one output directory; on completion the host copies them
into Files/Reports/<assignment> through ownerfiles, bounded in count and
bytes, with types refused outside an allowlist), and the completion names
the paths so Texture can embed them with the L8 {{image}} and {{file}}
tokens; or a host-side render tool for Texture. The first keeps engineering
the maker of artifacts and reuses the screens path.

## Owner correction (2026-11-09, late)

Owner: "No, we can't be wasting time like this. No big startup rituals."
The first fix brief (give the snapshot its own budget phase) treated the
copy as legitimate and only moved its cost. That is the wrong frame. The
copy is the whole store (`VACUUM INTO`), then deletes of the kinds the
lease does not name, then a scan of every text column: its cost grows
with the computer, not with what the capsule needs. The redirected fix:
a spawn copies no store by default (an ordinary assignment such as a PDF
report names no store kinds); an assignment that does need state gets a
copy built from only the named kinds' rows, so the copy and its secret
scan cost what the capsule uses. The launch keeps its own deadline.

Owner, same night: "There's no secrets in the computers. We have a gateway,
and soon we will have a general purpose vault ... for all user secrets
outside of essential api keys for models and search." So secret scans over
computer-held data (the store copy's text-column scan, a scan of each
engineering deliverable) guard against a threat the architecture already
excludes: model and search keys stay in the host gateway, and user secrets
go to the vault (docs/design/secrets-vault-design-2026-10-28.md, accepted,
not built). The kind filter that remains is custody (Texture bodies, the
mailbox), not secrecy. The deliverables channel drops its secret scan; a
false positive would only refuse a legitimate report.
