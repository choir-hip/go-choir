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

### Root causes (from code at 3f018e55 and Node B, read-only)

1. **Every story revision inherits the seed's role.** The opening's seed
   revision carries `revision_role: story_seed`
   (internal/store/story_open.go:126). `revision_role` is a durable
   metadata key (internal/textureowner/texture_task_types.go:50), so
   `buildAppagentRevisionMetadata`
   (internal/textureowner/texture_revision_metadata.go:26 and :38) copies it
   from the parent onto every Texture revision. `AutoPublishRevision`
   (internal/agentcore/story_publish.go:149) then sees `story_seed` on
   every revision and returns without a log line. The existing tests
   missed it because their story lived on a computer other than the
   runtime's (`computer-story` vs `autoputer-test`); the parent read
   (`getTextureRevision`, handler.go:123, keyed by the runtime's
   computer id) failed there, so nothing was carried. Deployed, the story
   lives on the runtime's own computer and the parent read succeeds.
2. **Captures 403 at random: vmctl binds a guest address to a stale
   computer.** A guest's platform URL is corpusd itself
   (`choir.platform_url=http://<host>:8086`, internal/vmmanager/manager.go:1597;
   confirmed in the live fc-config.json), and guest packets keep their
   tap source address after the DNAT to 127.0.0.1:8086 (locally
   delivered packets never pass POSTROUTING). corpusd binds the caller
   by vmctl's lookup-guest, which walked every ownership in map order and
   answered with the first whose computer_url host matched, whatever its
   state (internal/vmctl/handlers.go:212-228). Stopped computers keep
   their old computer_url and tap addresses are reused: on Node B,
   10.200.44.2 is claimed by computer-695920e (stopped 05:01) and by the
   c2-paper-3 computer computer-0d1dcde (booted 12:35:08); 10.200.43.2
   (c2-paper-2's computer-2da63a1) by six ownerships. About half (or six
   in seven) of the lookups named another computer, and corpusd refused
   the write as "internal caller required". The proxy and maild bind
   guests through the same lookup. The same address-to-ownership walk in
   `bindRequestToGuestComputer`, `bindRequestToRouteSlot` and
   `callerBoundToAnyOwnership` lets a guest at a reused address pass as
   the stale computer (heresy discovered).
3. **Verdict pushes always 403: corpusd's verdict route admits no
   guest.** `HandleSupportVerdicts` (internal/platform/publication_checks.go:151)
   required `trustedInternalCaller` only, written for a proxy hop the
   guest never takes (the publisher posted to the platform URL, corpusd).
4. **Found on the way: a story version would 404.** The guest's
   publisher posted `types.StoryPublishPath` to the same platform URL,
   but only the proxy serves that route (it reads the revision back from
   the computer); corpusd has none. Once (1) is fixed every version would
   be a gap with a 404.

Fix (one rule for a bound guest's write): vmctl's registry binds an
address to the single live ownership claiming it (running before
booting; stale ownerships never; two claims bind none) and every vmctl
binding uses it; corpusd's capture and verdict routes share one
internal-or-bound-guest rule; the guest publishes versions to a new
`choir.proxy_url` and verdicts to its platform URL; the seed role is
never carried forward; every early return of the publish path logs.
Mutation class red (vmctl guest binding, corpusd authority, VM boot
args).

## B13. A desk's first look at its records consumes them

Found by c2-paper-4 and c2-paper-5 on 3e8e8417, after B12's fix (captures
now arrive from every feed: three arrival records, 15 captures). No story
opened in 35 minutes. Trace of c2-paper-5's management run (2dcd2533):
cell 1 was a bare `choir.Records()`; it staged no act, yet the cell's
commit advanced management's cursor over the range it read (cursor 2 =
head 2). The printed records showed `Captures:0x1daabebf`, a pointer, not
the capture items. Cells 2 and 4 ranged over `choir.Records()` and printed
nothing; cell 6 marshalled it to `null`. Management then sent Notes to the
two sources declining, "the capture-item payload was not retained in this
turn", and the run completed. Every capture of the window was lost to the
paper.

Cause: (1) choir.Records() shows records past the cursor, and any
committed cell advances the cursor to the head it read, so a read-only
cell consumes the records for the rest of the turn (the C1 close review's
deepseek F4, "the read-batch contract"); (2) InboxRecord.Captures is a
pointer, so the default print shows an address. c2-paper-3 passed only
because its model opened stories in its first cell.

Fix: choir.Records() shows every record past the cursor the run started
at, for the whole run, so a desk can read its records in as many cells as
it likes; the cursor still advances with each committed cell, and the
next run starts past it. Capture and other bodies print as values.

## B12b. The guest's proxy URL is read and then dropped

Found by c2-paper-6 on 215d8f6f. B13's fix held: management opened 12
stories within seconds of the first arrivals, Texture wrote them, and
support verdicts published (403s gone). Every version publish failed:
"no proxy URL (choir.proxy_url) to publish through; this computer booted
without one". On Node B the VM's boot args carry
choir.proxy_url=http://10.200.43.1:8082, and the guest image (built from
215d8f6f) writes CHOIR_PROXY_URL into /run/go-choir-autoputer.env, which
the runtime unit loads. provideriface.LoadConfig reads it; but
internal/autoputer/run.go buildRuntimeConfig copies the runtime config
field by field and omits ProxyURL, so the runtime never sees it. B12's
tests built the runtime config directly, past this copy.

Fix: buildRuntimeConfig carries ProxyURL; a test pins that every
provideriface.Config field the guest sets from its environment survives
the copy.
