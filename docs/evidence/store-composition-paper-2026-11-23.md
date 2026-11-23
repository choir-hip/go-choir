# What the paper's store is made of, 2026-11-23

The store composition reading (`internal/storecomposition`, station S0 of the storage mission) ran on the paper for the first time: started 2026-11-23T21:45:11Z through the read-only runtime route, finished 22:02:37Z (17 minutes, not the 8 hours allowed), all 30 tables complete, 14.6 million rows walked. Sizes and counts only. It is a reading over a live store, close and not exact. Bytes are the stored column values in the main file; indexes, page overhead and free pages are not walked.

**The file is 51.6 GiB and the stored values are 30.2 GiB.** The other 21.4 GiB (41%) is indexes, page overhead and free pages, which this reading does not divide. That is the largest single thing still unattributed.

## By table

| Table | Rows | GiB |
| --- | ---: | ---: |
| og_bodies | 8,181,676 | 16.04 |
| run_memory_entries | 3,667,816 | 7.46 |
| og_objects | 8,181,676 | 5.72 |
| og_key_index | 2,171,255 | 0.68 |
| og_edges | 324,800 | 0.13 |
| computer_event_index | 120,379 | 0.11 |
| texture_agent_mutations | 134,176 | 0.03 |
| agent_evidence | 292 | 0.00 |

## Objects by kind (header plus body)

| Kind | Objects | GiB | Largest body |
| --- | ---: | ---: | ---: |
| choir.event | 2,991,558 | 5.51 | 147,118 |
| choir.source_ref | 884,781 | 2.60 | 2,083 |
| choir.texture_revision | 136,614 | 2.02 | 841,425 |
| choir.commitment_record | 578,713 | 1.90 | 1,178,372 |
| choir.lifecycle_command | 171,958 | 1.82 | 467,253 |
| choir.run | 210,316 | 1.42 | 384,735 |
| choir.inbox_command | 376,489 | 1.06 | 326,310 |
| choir.source_entity | 397,779 | 1.03 | 4,048 |
| choir.citation_check | 352,722 | 0.83 | 8,085 |
| choir.capture_label | 99,690 | 0.81 | 23,407 |
| choir.support_verdict | 335,335 | 0.81 | 8,086 |
| choir.actor_wake_outbox | 496,994 | 0.62 | 912 |
| choir.lifecycle_event | 171,970 | 0.23 | 15,288 |
| choir.event_membership | 132,192 | 0.16 | 4,465 |
| choir.capture_disposition | 118,785 | 0.14 | 2,919 |
| choir.conversation_head | 107,173 | 0.10 | 3,733 |
| choir.world_event | 109,710 | 0.09 | 864 |
| choir.agent | 87,808 | 0.08 | 450 |
| choir.management_task | 39,061 | 0.08 | 5,929 |
| choir.trajectory | 75,593 | 0.08 | 1,000 |
| 27 other kinds | 306,435 | 0.27 | |

Every object is current: 0 superseded, 0 tombstoned. Nothing in the object tables is ever marked replaced or removed. Duplicate content is negligible (0.26 GiB at most). No orphan bodies.

## Run memory

| Kind | Rows | GiB |
| --- | ---: | ---: |
| message | 3,017,137 | 6.91 |
| compaction | 22,553 | 0.22 |
| cell_fate | 383,587 | 0.21 |
| rlm_inbox_cursor | 170,336 | 0.07 |
| texture_cell_author_receipt | 74,203 | 0.06 |

| Age | Rows | GiB |
| --- | ---: | ---: |
| week | 1,798,074 | 4.03 |
| older | 1,333,969 | 2.19 |
| day | 535,773 | 1.24 |

## What it says

1. Two fifths of the file is not stored values. Which indexes hold it is the next reading (SQLite's `dbstat` by index, or page counts per index): a write, an index dropped or a table rebuilt may return more than any deletion.
2. The event layer's objects (`choir.event`, about 3.0 million) are the largest kind. The design draft of 2026-11-23 replaces that layer; what happens to its objects is part of that design.
3. Run memory is 7.5 GiB, nearly all messages; 2.3 GiB is older than a week.
4. Texture revisions are 2.0 GiB in 137 thousand objects, each a full copy and each current: the store has no notion of a superseded revision.
5. Nothing is ever superseded, tombstoned or deleted, in any kind. The store grows about 2 GiB a day because every record of every run is kept for ever.

The raw report is not kept in the repository; it can be read again with `?store_composition=1` until the next reading replaces it.
