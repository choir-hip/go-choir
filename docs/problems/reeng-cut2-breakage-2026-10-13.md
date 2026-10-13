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
