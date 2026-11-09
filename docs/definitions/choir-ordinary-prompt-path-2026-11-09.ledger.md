# Ledger: the ordinary prompt path (2026-11-09)

Append-only. One line per move; newest last.

- 2026-11-09 Investigation landed: docs/problems/ordinary-prompt-path-2026-11-09.md (7c209d975). Prompt to v2 202 s on d@e; research's first citable source +18 s, first packet +126 s; 12 reads, 3 errors unseen; no loop cap or repeated-call detector; the 498-call runaway did not reproduce on d@e.
- 2026-11-09 L1 landed: pool routing by availability, crouton-alpha leads (ead8873b6; problem doc 06f9f6df6; Devin ACCEPT, non-blocking: the last member is never demoted, strike inflation under bursts). (red; rollback: revert)
- 2026-11-09 L2 built: prompt bar, Sources while revising, search.served event and search providers in Sources (1adebd1e2, epoch 115); Devin review running.
- 2026-11-09 L3 built: layered image on every computer ("*"), every tool layer attached, delegate granted to d@e and the owner's computer (e6efe56c5); tier 2 review (Devin plus a Sonnet seat) running.
- 2026-11-09 Promoted to executable by the owner; tempo: expedite everything until Autoradio is solid (CLAUDE.md updated).
- 2026-11-09 L3.5 added by the owner: tools attach on demand, not at boot. Physics: an attached but unread layer already costs no page cache and its bytes are stored once on the host, so L3.5 buys modularity (no tool in a computer's store until it is used, adding a tool without a cold boot), not memory. Design next.
- 2026-11-09 L3.5 widened by the owner to load and unload live, secure: a tool enters memory only when loaded and leaves it on unload, without a restart. Moved up: design now, build after L2 and L3 land.
