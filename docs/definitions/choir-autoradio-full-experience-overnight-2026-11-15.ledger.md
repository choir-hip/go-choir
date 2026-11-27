# Ledger: Autoradio full experience, overnight 2026-11-15

Standing: built | kind: happened | scope: in part | settled-by: the ledger of choir-autoradio-full-experience-overnight-2026-11-15.md: K landed a99e07d3b, S landed 53747dcca, both on origin/main, and C107 and C109 true | note: goal file now.status is working (line 170); ACTIVE.md:215 heads it Superseded

- 23:50Z. Goal authored on the owner's statement of 23:36Z. Seven builders started 23:40Z to 23:58Z (Q, P, R, T, D, V, S). V pushed the shared components (527df404b); S pushed the design note with the clip shapes (20a73eb14), forwarded to P and D.
- 00:05Z. Owner rulings: the station starts on app open; the queue is a structure the DJ edits from its cell and the reader reorders by talking; the books app is named Reader; voice turns on at open and off after silence. Station K started (server). Q, D and T were given the op list and the snapshot shape.
- 02:03Z to 02:15Z. The Jev dial fix and the inline-body reader landed and deployed (5841da812). C104 and C105 settled true at 08:15Z.
- 01:50Z. S: three review rounds without convergence, every blocker in the account-reported feed intake. Structural assessment written; that intake was deleted from the series, not repaired. Round four accepted the reduced change.
- 03:17Z to 03:33Z. K landed (a99e07d3b), deployed 03:29Z, proven on d@e: "put the long read next" moved the right entry three of three; "what's coming up?" sent no edit three of three. C107 true.
- 04:30Z to 05:01Z. S landed and CI refused it: the match job is a new direct Jev call, which the Jev entry gate forbids, and scripts/land did not run that gate. Backed out (af31e4fd0). C106 false. The series was cut again without the automatic match job, with an operator hand path and the gate added to scripts/gates.
- 06:27Z to 06:40Z. S landed again (53747dcca), CI green, deployed 06:39Z.
- 06:40Z to 06:55Z. The worker could not fetch from the build VM (proxy-only egress). Moved to the lead's Mac; the operator key stays in a loopback proxy there. 183 episodes of 61 feeds recorded.
- 07:43Z. An agent judged 240 ranked candidates; 14 clips written, 0 refused; 13 served over 5 front stories. The phone shows them under "Hear it" on a live story (simulator, signed out). C109 true.
- 00:28Z to 07:57Z. Phone: eleven installs. Design sweeps one to seven (no pills, then no filled blocks outside flows, then no boxed rows, then the live paper's prompts). Queue fixes from the staging proof (e8446a5bb).
- 08:10Z. Found and recorded, not fixed: the paper publishes stories whose headline is a failed search; the hourly script's search line misled (fixed, 59eec98ea).
- 08:31Z. D proven on the server: a stretch on d@e with four clip-bearing candidates returned 15 segments, 3 of them podcast_clip, each after its story with a lead-in (DeepSeek V4.1 Flash on River, 47.7 s). Second matching pass: 26 clips over 11 stories from 9 shows. Report published.
- 08:40Z. Final install: the Reader's demo sample book merged (3f11437d3); Kit suite 862 tests, 0 failures on the final tree; the Reader seen holding a document in the simulator.
