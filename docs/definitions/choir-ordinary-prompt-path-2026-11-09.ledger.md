# Ledger: the ordinary prompt path (2026-11-09)

Append-only. One line per move; newest last.

- 2026-11-09 Investigation landed: docs/problems/ordinary-prompt-path-2026-11-09.md (7c209d975). Prompt to v2 202 s on d@e; research's first citable source +18 s, first packet +126 s; 12 reads, 3 errors unseen; no loop cap or repeated-call detector; the 498-call runaway did not reproduce on d@e.
- 2026-11-09 L1 landed: pool routing by availability, crouton-alpha leads (ead8873b6; problem doc 06f9f6df6; Devin ACCEPT, non-blocking: the last member is never demoted, strike inflation under bursts). (red; rollback: revert)
- 2026-11-09 L2 built: prompt bar, Sources while revising, search.served event and search providers in Sources (1adebd1e2, epoch 115); Devin review running.
- 2026-11-09 L3 built: layered image on every computer ("*"), every tool layer attached, delegate granted to d@e and the owner's computer (e6efe56c5); tier 2 review (Devin plus a Sonnet seat) running.
- 2026-11-09 Promoted to executable by the owner; tempo: expedite everything until Autoradio is solid (CLAUDE.md updated).
- 2026-11-09 L3.5 added by the owner: tools attach on demand, not at boot. Physics: an attached but unread layer already costs no page cache and its bytes are stored once on the host, so L3.5 buys modularity (no tool in a computer's store until it is used, adding a tool without a cold boot), not memory. Design next.
- 2026-11-09 L3.5 widened by the owner to load and unload live, secure: a tool enters memory only when loaded and leaves it on unload, without a restart. Moved up: design now, build after L2 and L3 land.
- 2026-11-09 L3.5 design scope widened by the owner: desks and Library packages that need heavy tools (video, Blender, CAD). Tools become packages, desks lease them, slots are generic.
- 2026-11-09 L2 and L3 deployed: 83a206203 live (CI 42559736851; L2 e5cfeeb7b, L3 82e4e7727 + 6a59cdc5b; the test count fix 83a206203 after CI 42557693886 failed TestSemanticKindsNeverEnterTheQueue). d@e claude_delegate available (Sonnet, until 2026-11-17, 50M a day): L3's delegate acceptance holds on d@e; b@c available, 98,708 tokens used today, still on e8d666e4a (its ring has not opened). Lesson: a docs push restarts CI and cancelled the L2 run; hold docs pushes while a deploy is in flight.
- 2026-11-09 L3.5 design revision 2 (tier 2 r1: Devin ACCEPT_WITH_CHANGES, Sonnet SEND_BACK; both: mount namespaces are the gate, and the memory outcome needs no drive swap). Phase 1 eviction built (88898613e), Devin reviewing.
- 2026-11-09 L6 r1 SEND_BACK (fingerprint hashed the per-call duration; the flint loop could never match), fixed 3a32dcc34, r2 running. L5 r1 SEND_BACK (ledger never reset between runs on a persistent desk session), fixed d8c06d081, r2 running.
- 2026-11-09 Finding (read-only, d@e): /api/trust froze at 2026-11-07T12:50Z because the desk-cast judge's provider calls fail since 2026-11-08T00:02Z, so no new desk_cast settlements exist; the trust code is right. Recheck after the routing fix (L1).
