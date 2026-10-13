# Cut 2 breakage log, 2026-10-13

One entry per breakage found in the Cut 2 window (decisions §12.4). Each
entry names the evidence; the fix commit follows it.

## B10. A trajectory settles under unread records

Found by the research scenario c2-judge-1 on 49a766fc (bundle
~/.config/choir-qa/traces/c2-judge-1/bundle-a1.json). Texture's first cell
wrote a plan and called Done, which settled the only work item
(work_settled at 10:49:37). Research's final Report reached texture:<doc>
at 10:57:25. Research's run then ended, and
`ReconcileLifecycleSettlementForTerminalRun` (internal/store/store.go:1026)
settled the trajectory at 10:57:32: `lifecycleSettlementReady`
(internal/store/lifecycle.go:2718) checks work items, subject refs and
packets, but not records. The Texture wake for the Report then found a
settled trajectory and was refused (classifyTextureLifecycleActivationSnapshot),
so the Report was never read and no cited revision followed (I4 read
nothing; no support pairs). In c2-write-2 the same shape happened in a
different order (the Texture turn read the Report before the research
run's end), so it passed.

On the records route the records are the work (Cut 1), and settlement
must agree: a trajectory is not ready to settle while its Texture's
inbox has unread records or one of its conversations has an open round.
This matters most for Autopaper, whose stories receive records long
after their first turn; a living story must not settle at all (the
living-stories work decides how a publish-auto document is marked).

Fix: `lifecycleSettlementReady` also requires texture:<doc> to be read
through its head and no open round on texture:<doc>'s conversations.
Mutation class red (lifecycle settlement).

## B11. A slice literal that indexes choir.Records() panics the cell compiler

Found while writing the living-stories management test (unit 2b), on a
local desk worker built from this branch. In a management cell,
`rs := choir.Records()` followed by `fmt.Println([]string{rs[0].Kind})`,
`x := []string{rs[0].Kind}` or `[]string{rs[0].Captures.Items[0].CaptureID}`
fails with `yaegi compile panic: runtime error: invalid memory address or
nil pointer dereference`; `r := rs[0]` fails with `constant definition
loop`. A map literal (`map[string]any{"k": rs[0].Kind}`), a plain
expression (`fmt.Println(rs[0].Kind)`) and a range loop that appends
(`for _, c := range rs[0].Captures.Items { ids = append(ids, c.CaptureID) }`)
work. It is not specific to the new Captures body: `rs[0].Kind`, a plain
string field, trips it too. Hypothesis (not confirmed in the trace or the
interpreter): the cell normalizer hoists top-level `:=` into package-level
declarations, and yaegi's global initialization order mishandles a slice
composite whose element indexes another global's call result.

Not fixed here. The management prompt shows the loop pattern for
collecting capture ids, and the living-stories test uses it. A model that
writes the slice literal gets a panic it can read and a fresh session.
The fix belongs in the cell normalizer or upstream yaegi.

## B12. Stories never publish; guest writes to corpusd 403 intermittently

Found by the paper scenario c2-paper-3 on 244e97b1 (test computer
computer-2da6… for c2-paper-2, and the c2-paper-3 computer; guest log
read through /api/runtime/observability). One owner act started
schedules for rss:bbc_world, rss:guardian_world and rss:npr_world at
12:35:08. The first BBC poll wrote 5 captures and its arrival record
reached management, which opened 5 stories by Casts to texture:new
(publish auto, 12:35:46). Texture wrote each story (one reached revision
5, 8,448 characters) and read every record. Two failures:

1. No story published. The front list stayed empty for 30 minutes, and
   the guest log has no "story publish:" line at all: not published, not
   a gap, not "no platform to publish to". The auto-publish path
   (internal/agentcore/story_publish.go, AutoPublishRevision from
   commitTextureWrite) never reached its logging, so it either is not
   called or returns before deciding.
2. Guest writes through the proxy to corpusd get 403 "internal caller
   required", intermittently: rss:npr_world's first poll (12:35:08) failed
   while rss:bbc_world's succeeded in the same second; later polls of all
   three feeds failed; every support-verdict push (/internal/platform/
   support-verdicts) failed (1, 1, 14 and 10 verdicts). The proxy
   (internal/proxy/guest_authority.go trustedInternalProxyTransport)
   treats a caller at 127.0.0.1, ::1 or 192.0.2.* as trusted internal
   transport and then requires X-Internal-Caller; corpusd
   (internal/platform/authority.go trustedInternalCaller) requires both
   too. Which side refused is not yet known (the services do not log
   per-request).

Both block autopaper_live: (1) means nothing reaches readers; (2) loses
captures and leaves every citation pending in the public overlay.
