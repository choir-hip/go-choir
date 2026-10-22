# Node B's platform artifact store is 122 GB; five stores outside the tape have no working retention (2026-10-22)

Date: 2026-10-22. Written for FP0, the launch first-pass orchestrator. Status: documented, evidence and plan only. Docs only: no code changed, nothing deleted, no host read by the author. Mutation class of this doc: green.

Out of scope: the three legacy tape namespaces (`computer-event`, `computer-event-payload`, `pin-receipts`; 45 GiB, 9.8M files) and `tape-seg`. Another lane is building their repack.

Evidence labels: **[V]** code or doc read at file:line. **[M]** measured by FP0, 2026-10-22 23:05 to 23:15 UTC (the table FP0 handed over). **[H]** hypothesis, with the host read that settles it. Sizes are KiB from `du`, so GiB here and GB in FP0's table differ by 7%.

Related records, not repeated here:
[artifact-store-grows-12gb-a-day-2026-10-14](artifact-store-grows-12gb-a-day-2026-10-14.md) (the GC fix),
[checkpointd-scratch-leaks-superseded-generations-2026-10-15](checkpointd-scratch-leaks-superseded-generations-2026-10-15.md),
[platform-artifact-small-file-growth-2026-10-19](platform-artifact-small-file-growth-2026-10-19.md),
[tape-sweeps-destroyed-and-orphans-2026-10-20](tape-sweeps-destroyed-and-orphans-2026-10-20.md),
[no-off-host-backup-2026-10-20](no-off-host-backup-2026-10-20.md),
[node-b-disk-fills-from-the-paper-data-image-2026-10-22](node-b-disk-fills-from-the-paper-data-image-2026-10-22.md).

## Summary

- The artifact GC works. It is active on Node B with a 168 h grace, hourly, and its default namespace list includes `projection-base`, `file-cas-roots` and `file-cas-chunks` [V `nix/node-b.nix:1194-1199`, `internal/platform/artifact_gc.go:106-118`]. Today's log shows deletes only in `platform-update` [M]. That means every remaining object in the other swept namespaces is **live by rule or inside the grace**. The leak is in the rules, not in a stopped sweeper.
- The 10-08 and 10-09 bases (16.6 GiB) are almost certainly the owner computer's latest two bases, kept by the "latest two per computer" rule. That is a 2-base retention floor, not a leak. They are also its only restore points. Nothing here may touch them without the owner's yes. [H, strong; settled by one SQL read, item 2.]
- Three stores have no collector at all: `capture-bodies` (6.0 GB, growing about 0.6 GB a day), `artifact-gc-log` (1.7 GB, no rotation), and `file-cas-roots` (every root row is live forever, 2.7 GiB).
- `checkpoint-scratch` has a partial collector. It did not shrink after the 10-15 fix (6.6 GB then, 6.4 GB now [M]).
- Dead computers hold bases, chunks and roots forever. The GC never asks whether a computer still exists.

Biggest safe wins first: scratch sweep (up to 6.4 GB), a measured purge of dead computers' bases and chunks (up to 4 GiB of bases plus an unmeasured share of 25.6 GiB of chunks), file-cas-roots retirement (about 2.5 GiB), GC-log compression (about 1.5 GB). The `capture-bodies` rule is the largest forward saving but needs the owner's yes.

## 1. What each store is

### projection-base (20.65 GiB, 246 files)

- **What.** A full snapshot of one computer's projected state at a tape sequence, plus a `<digest>.descriptor.json` sidecar (computer id, sequence, canonical head, blob size, vocabulary version). A new base installs on a guest as the starting point of a restore, so replay covers only the tail after it [V `internal/projectionbase/types.go:136-147`, `publisher.go:35`].
- **Writer.** `checkpointd`, one base per cadence tick (about every 2,500 events) per computer; `PublishCheckpointResult` records the new base and moves the watermark [V `internal/platform/projection_jobs.go:343-395`, `cmd/checkpointd/worker.go:61-224`]. Also `choir-rebuild-base` offline.
- **Readers.** Restore and recovery admission install a base from the watermark or from a pin; the next checkpoint job uses the prior base as its seed [V `internal/agentcore/restore_base.go:122-150`, `internal/vmctl/recovery_admission.go`, `worker.go:134-174`]. Without a base, a computer past `MaxRecoveryTailEvents` is refused until an operator runs a genesis repair [V `worker.go:129-131`].
- **Holders (the live set).** One DB union [V `projection_jobs.go:315-317`, `artifact_gc.go:344-362`]:
  1. every computer's advertised watermark `base_ref`;
  2. every row in `computer_projection_base_pins` (restore points; written by `pinCheckpointBase` at checkpoint time, `agentcore/restore_base.go:154-171`, `checkpoint_restore_bindings.go:101`);
  3. each computer's **latest two** rows in `computer_projection_bases`, for every computer that ever had a row, live or dead;
  4. the seed of any queued, running or retrying job.
  The sidecar of a live blob stays live.
- **Retention rule.** GC deletes a base only when none of those hold **and** its mtime is older than 168 h [V `artifact_gc.go:538-551`]. Pins have no TTL. The only caller of the pin release endpoint (`DELETE .../pins`) is the lifecycle API; no code path calls it [V grep: `HandleProjectionBasePins`, `projection_jobs_http.go:117-123`, has no in-repo caller of DELETE]. So pins never expire.

### checkpoint-scratch (6.4 GB)

- **What.** The offline rebuilder's working Dolt workspace for one checkpoint job, one directory per `<computer>-<generation>`. It holds a seeded copy of the prior base plus replayed state, so each is about the size of that computer's base (the 10-15 record saw up to 660 MB) [V `internal/projectionbase/rebuilder.go:159-265`; the 10-15 record].
- **Writer/reader.** `checkpointd` only. A retry of the same job resumes from it.
- **Retention rule.** It is removed after a successful publish [V `worker.go:218`]. A failed job keeps it on purpose. When the same computer's next job is claimed, older generations of that computer are pruned [V `worker.go:162`, `cmd/checkpointd/scratch.go:18-43`]. **Nothing else deletes it** [V: `cmd/checkpointd/main.go:61` calls `runCheckpointWorker` once per unit run and has no boot sweep].
- **Nothing references it.** No DB row, pin or checkpoint names a scratch path. It is derived data: seed base plus the tape.

### capture-bodies (6.0 GB)

- **What.** The byte store behind Autopaper `choir.web_capture` objects, at `capture-bodies/<hh>/<sha256>` [V `internal/platform/captures.go:78-140`]. Each new capture writes **three** blobs: the raw bytes as fetched (up to 16 MiB: a page or a PDF), the block JSON, and the block text projection [V `internal/sourcegraph/captures.go:195-216`].
- **Writer.** corpusd, from the paper computer's polls and `choir.Read` page reads [V `researchtools/read.go:159`, `poll_source.go:239`].
- **Readers.** The block JSON feeds publication versions and citation checks [V `internal/platform/publication_versions.go:462-465`, `captures.go:295-303`, `researchtools/texture_captures.go:90`]. The text blob feeds capture reads [V `captures.go:279`]. The **raw blob has no reader in the repo**: `ContentBlobID` is only returned as a ref (`captures.go:292`), and the by-hash route `HandleCaptureBody` (`captures.go:214`) has no in-repo caller that passes a raw hash [V grep]. [H: confirm on the host that no guest or script fetches raw bodies.]
- **Holders.** Store B `og_objects` rows of kind `choir.web_capture`, fields `content_blob_id`, `extracted_text_blob_id`, `blocks_blob_id`. Externalized rows hold no copy: "the file is the only copy" [V `no-off-host-backup-2026-10-20.md:43`].
- **Retention rule.** None. `capture-bodies` is not in `artifactGCNamespaces` [V `artifact_gc.go:106-112`], and no other code removes from it [V grep of `capture-bodies`, `CaptureBodiesRoot`].
- **Growth.** 1.4 GB on 10-14 [V `search-is-duplicated-and-discarded-2026-10-14.md:137`], 6.0 GB on 10-22 [M]. That is about 0.57 GB a day [derived from two records, not a rate measurement]. The HN point-count churn that minted a capture every poll is fixed (`hn-point-counts-mint-a-capture-every-poll-2026-10-17.md`).

### file-cas-chunks (25.62 GiB) and file-cas-roots (2.70 GiB)

- **What.** The owner-files store. A guest syncs its Files tree to the platform as encrypted chunks (`file-cas-chunks/<computer>/<digest>`) plus one manifest per sync (`file-cas-roots/<computer>/<root>.json`) naming the chunk digests [V `internal/platform/file_cas_blobs.go:20-27`, `internal/filecas/filecas.go:38-44`]. These are owner files; the S0 incident record says the CAS file "was the sole copy" [V `no-off-host-backup-2026-10-20.md`].
- **Writer.** The guest file sync (`autoputer/file_sync.go`), via the platform immutable PUT.
- **Readers.** Hydration of a computer's Files from its **latest** root only [V the 10-14 record, "HydrateIfNeeded"]; the GC itself, which parses manifests. Only the platform reads manifests [V grep `ManifestRef`, `file-cas-roots`: `file_cas.go`, `file_cas_blobs.go`, `artifact_gc.go`].
- **Holders.**
  - Roots: **every** row in `computer_file_roots` is live, and rows are never deleted [V `artifact_gc.go:335-342`; `file_cas.go:43-47` is insert-only]. So `file-cas-roots` can never shrink.
  - Chunks: per computer, the chunks named by its latest root plus every root recorded inside the grace window (168 h) [V `artifact_gc.go:571-690`, `fileCASRootsForGC` at `:788-820`]. A computer with **no recorded root** keeps all its chunks (reported as a warning). A computer with any unreadable or invalid manifest keeps all its chunks.
- **Retention rule.** Chunks: latest root + 7 days of roots. Roots: forever. The deterministic chunk nonce landed (`filecas.go:142-163`), so a reboot no longer re-stores the tree. That fix explains the drop from 46 GB (10-14) to 25.6 GiB now.
- **Dead computers.** The chunk sweep walks every `file-cas-chunks/<computer>` directory, with no check that the computer still exists. A destroyed computer's latest root keeps its chunks live forever.

### artifact-gc-log (1.7 GB, 17 daily files)

- **What.** The GC's deletion log: one `intent` line per candidate (fsynced before the unlink) and one outcome line (`deleted`, `kept_refreshed`, `gone`, `error`) [V `artifact_gc.go:690-770`], in `artifact-gc-log/<UTC day>.jsonl`.
- **Writer.** `deleteArtifactCandidates`. The tape sweep writes its own `tape-sweep-<run>.manifest.jsonl` into the same directory [V `tape_sweep.go:846-852`].
- **Reader.** None in code. It is an audit trail for a human [V grep `artifactGCLogDir`: only writers].
- **Retention rule.** None. No rotation, no compression, no logrotate entry [V grep `nix/`, `scripts/`: the only mention is a comment at `nix/node-b.nix:1188`].
- **Growth.** About 460 bytes per deleted object (two lines). The 570,600 `og` deletes (about 1.1M lines) and file-CAS cleanups account for the size [H; settle with the jq counts in section 6].

### Outside the asked list, in one line each

- `platform-update` (1.29 GiB, 12 files): deploy payloads. Live set is every route slot and every transition receipt's old/new ref, so rollback refs survive. The GC deletes here today, correctly [V `artifact_gc.go:363-381`].
- `og` (0.28 GiB, 12,098 files): the pre-teardown corpus bodies, draining at up to 50,000 deletes an hour since `og` joined the active list on 10-20. About 5,000 of them are probably still eligible [H; the journal report settles it].
- `tape-seg` (8.0 GB): the repack lane's. Out of scope.
- `tape-sweep-archive` (0.35 GB): the tape sweep's archive of deleted destroyed-computer files; pruned by `prune-archives` with a 7-day floor [V `tape_sweep.go:47-58`]. Leave it.
- `computer-inputs` (0.7 GB): `VMCTL_BASE_BLOB_ROOT`, vmctl's base blobs [V `nix/node-b.nix:1442`]. I did not trace its retention. [H: vmctl owns it; check with the `find` in section 6.]

## 2. Why the 10-08 and 10-09 bases still exist

**Finding [H, strong].** They are the latest two bases of one computer, `computer-03335285269bdba4f94377e56879f9e6`, which is the owner's staging computer. No newer base has been published for it, so both stay live under rule 3 of the live set, and the GC is right to keep them.

Evidence:

1. [V `2026-10-14` record] The four giant bases all belong to `computer-03335285...`: 09-09 seq 148,431 (15.9 GB), 10-07 seq 572,151 (8.7 GB), 10-08 seq 574,653 (8.7 GB), 10-09 seq 577,162 (8.8 GB). The same record says no base had been published for that computer since 10-09.
2. [M] Today the two big groups are 10-08 (8.4 GiB) and 10-09 (8.2 GiB), and **no** base from 10-10 onward is larger than 1.1 GiB. A base for a computer with 577k events cannot be 1 GiB, so none has been published since 10-09.
3. [M, consistent] The 09-09 and 10-07 giants are gone. Both fell out of "latest two" once 10-08 and 10-09 existed, and both were older than 168 h by 10-16. The GC deleted them. That is the retention rule working.
4. [V] The GC would delete 10-08 and 10-09 only if they were not a watermark, not pinned, not within the latest two for the computer, and not a job seed, and older than 168 h. For a computer with no newer base, they fail the "latest two" test.

Referenced by (to settle, rows not read): `computer_replay_watermarks.base_ref` (likely 10-09), `computer_projection_bases` (both), and possibly a pin. Settled by the SQL reads Q1 to Q3 in section 6.

**Would GC ever delete them?** Only after a newer base lands. A fresh base makes 10-09 the second-latest and 10-08 third. 10-08 is then eligible once it is unpinned. So the existing rule frees 10-08 (8.4 GiB) automatically, and adds a new base of about the same size or larger. **Net saving is zero, or slightly negative.** The 2-base floor costs one extra base per computer. For this computer that is 8.4 GiB.

**Why no new base [H].** The checkpoint for this computer may be failing. Known causes of record:

- `og migrate ... has migrated identity fields but no verified identity formula` blocked replay for other computers on 10-14/15 (`checkpointd-replay-upcast-missing-identity-formula-2026-10-15.md`);
- recovery admission refuses `projection_base_missing` [V `recovery_admission.go:49`];
- a blocked job needs an explicit operator genesis repair [V `worker.go:129-131`].
Settle with Q4 (job status and `error_text`) and the checkpointd journal.

**Risk the other way.** The descriptors carry `vocabulary_version`; "a missing or unknown vocabulary version refuses: pre-seam bases are uninstallable" [V `types.go:149-151`]. If 10-08/10-09 predate the current vocabulary, they may not install, and the owner computer would be one failed checkpoint away from a genesis replay of 577k+ events. [H: read `.vocabulary_version` in both descriptors.] This is the real danger of the 10-08/10-09 bases, not their size.

**Do not delete them.** They may be the only restorable state of the owner's computer. This needs the owner's yes, and in practice a replacement base first.

## 3. checkpoint-scratch and capture-bodies

**Does scratch ever get cleaned?** Partly [V]. Successful publish removes its own directory. The next job claim for the same computer removes older generations. The remaining leak classes are:

1. the last failed or blocked generation of a computer that is never enqueued again (stopped, destroyed, or `blocked` for good);
2. a directory left by a checkpointd crash or kill mid-job, for a computer that then stays quiet;
3. a directory whose computer has since been destroyed.

[M] 6.6 GB on 10-15 and 6.4 GB on 10-22. The 10-15 fix did not bring it near zero, which fits classes 1 to 3. [H] Settle with the `find`/`du` and the job-status query in section 6.

**Can capture-bodies be retained less?** Yes, in three layers, from safest:

1. **Raw blob only.** Nothing reads it [V, above]. The raw bytes are probably most of the 6.0 GB (PDFs and pages; the text and block blobs are small) [H; settle with the size-by-age census]. A rule "raw blob older than N days and not named by a published citation" frees most of it. The capture row would keep `content_blob_id` pointing at a deleted blob. That row field is `required` by validation [V `objectgraph/web_capture.go:192-193`], so the rule must keep the field and mark the blob as aged out, not null it. Class: red (evidence, Store B reference surface).
2. **Whole capture past N days, never cited.** Needs a citation index from publications to captures. Larger saving, larger risk.
3. **Do nothing and add a disk-pressure rule.** Not recommended.

The capture is the paper's source evidence. The 10-20 backup record counts it among things where "the file is the only copy". Any deletion is the owner's yes.

## 4. artifact-gc-log retention

None [V]. 17 files, 1.7 GB, the 10-14 file 74 MB [M]. It is audit evidence, so the fix is **compress, do not delete**. gzip (or zstd) of repetitive JSONL is typically 8 to 12 times smaller [H; test on the 74 MB file]. Keep the newest 7 days plain. Never touch `tape-sweep-*.manifest.jsonl`: a tape-sweep apply names a manifest by sha256, so rewriting one would break a pending apply [V `handlers.go:526-530`; check `tape_sweep.go` apply before any change].

Forward growth is small once the `og` backlog is done: roughly 460 bytes per deleted object. The worst case is a mass delete, which is exactly the case where the log must survive.

## 5. Per-store fix table

Savings are upper bounds from [M], unless marked [H].

| # | Store | One-time saving | Forward saving | Fix | Mutation class |
|---|---|---|---|---|---|
| F1 | checkpoint-scratch | up to 6.4 GB | keeps it near 0 (was about 1 GB/day before the 10-15 fix; the residual rate is not measured) | Boot sweep in checkpointd; plus a one-time manual remove after the gate | orange (scratch is derived data); red only if a genesis-repair scratch of the owner computer is among the dirs |
| F2 | projection-base, dead computers | up to 4.0 GiB (20.65 minus the 16.6 GiB giants) [H] | their last-two floor, about 0.3 GiB each at the next deploy | Drop from the live set any base of a computer with no ownership record and no head row (or after the tape sweep destroyed it) | red (checkpoint class), with a dead-computer proof |
| F3 | file-cas-chunks, dead and rootless computers | unmeasured; up to a share of 25.6 GiB [H] | their whole latest-root set | Same dead-computer rule; report rootless computers first | red-leaning (owner files); owner's yes unless the computer is a test or fleet computer |
| F4 | file-cas-roots | about 2.5 GiB [H: all roots older than 168 h except each computer's latest] | about 0.1 GB/day | Live set becomes: latest root + roots inside the grace window. Old manifests name chunks that are already deleted, so they restore nothing | orange (GC live set); data class: manifests of owner files |
| F5 | file-cas-chunks, window | unmeasured [H]; the 7-day roots window keeps churn | proportional to file churn | A separate shorter window (for example 48 h) for file CAS only, not the global grace | orange; do after F3 and F4 are measured |
| F6 | artifact-gc-log | about 1.5 GB [H] | small | Compress files older than 7 days after hash-verified round trip; never the tape-sweep manifests | green/yellow if a script; orange if a timer; the file is audit evidence, so keep originals until verified |
| F7 | capture-bodies | unknown; probably most of 6.0 GB if raw dominates [H] | about 0.6 GB/day | Retire raw blobs past N days not cited by a publication | red (evidence); owner's yes |
| F8 | projection-base, owner computer | 8.4 GiB (10-08) | 0 | Unblock the owner computer's checkpoint, so the existing rule frees 10-08 after 7 days | red (checkpointd for the owner computer); net saving about zero because a new base replaces it |
| F9 | projection-base "latest two" | one base per computer | one base per computer | Change the floor to watermark + pins + seeds | red; **not recommended**: the predecessor is the failure fallback |

### Evidence gate and rollback, per fix

**F1 scratch.** Gate: (a) `go-choir-checkpointd` is not running; (b) the directory's `<computer>-<generation>` has no job in `queued`, `running` or `retry` with that generation; (c) mtime older than 48 h; (d) name matches `^<computer>-<digits>$` exactly (the 10-15 record's S1 to S3). The sweep logs each removal. Rollback: none needed, because the content is derived. The cost of a wrong delete is a rebuild from the seed (minutes), and for a genesis-repair scratch of a 577k-event computer, hours. Exclude any directory whose job is `blocked` with `genesis_repair=true` unless the owner agrees.

**F2/F3 dead computers.** Gate: all of (a) no vmctl ownership record; (b) no `computer_event_heads` row, or a tape-sweep manifest naming the computer as destroyed and quiet for 7 days; (c) a dry-run report listing the computer, its bytes and its last sequence; (d) the report reviewed. The safest first cut is computers already in a reviewed tape-sweep manifest. Rollback: none for a deleted base or chunk (the destroyed computer's tape is already gone by owner rule: "Destroyed computer tapes can be deleted", owner notes §26). So the delete is irreversible, and the gate is the only protection. A test or fleet computer needs no owner yes. **Any computer that ever held an owner account needs the owner's yes**; file chunks are owner data.

**F4 roots.** Gate: root older than the grace window and not the computer's latest; the GC's `fileCASReachableChunks` for the same window still verifies. Rollback: the root rows stay in Dolt, with history. The manifest bytes would be gone, but the chunks they name are already past the window, so nothing restorable is lost. Revert is a config/code revert; deleted manifests do not come back.

**F6 log.** Gate: `zcat file.gz | sha256sum` equals `sha256sum file` before the original is removed. Rollback: `gunzip`. Do not delete without owner approval; compression is enough.

**F7 capture raw.** Gate: no reader fetches raw (host grep of corpusd/proxy access logs for `/captures/bodies/`), a citation index shows the capture is uncited, and the owner approves. Rollback: none after the delete. A refetch of the URL is not the same bytes.

**F8 owner checkpoint.** Gate: root-cause the failure from the job's `error_text` and the checkpointd journal first. The fix is separate work (a problem doc first, per Problem Documentation First).

### Things that could be owner data or the only copy of a checkpoint

These need the owner's yes. Do not delete without it:

1. The 10-08 and 10-09 bases of `computer-03335285...` (owner computer; probably its only restore points) and every pinned base.
2. Any file-cas chunk or root of a computer that ever had an owner ownership record, including stopped ones.
3. `capture-bodies` raw blobs: the paper's source evidence.
4. Any scratch dir for a genesis repair of the owner computer.
5. The platform-update payloads (rollback refs). The GC handles these correctly; leave them.

## 6. Host reads for FP0 (read-only)

Set `A=/var/lib/go-choir/platform-artifacts`. All of these only read. The SQL reads need the platform Dolt (127.0.0.1:13306), SELECT only. Use FP0's usual read-only client.

### projection-base (items 2, F2, F8)

```sh
# the big files, with their sidecar descriptors
find $A/sha256/projection-base -type f -not -name '*.descriptor.json' -size +100M \
  -printf '%TY-%Tm-%Td %TH:%TM %s %f\n' | sort
for f in $(find $A/sha256/projection-base -name '*.descriptor.json' -size -1M -newermt 2026-10-07 ! -newermt 2026-10-10); do
  jq -c '{computer_id,sequence,blob_size_bytes,vocabulary_version,created_at}' "$f"; done
# bytes per computer, from all descriptors
for f in $A/sha256/projection-base/*.descriptor.json; do jq -r '[.computer_id,.blob_size_bytes]|@tsv' "$f"; done \
  | awk '{s[$1]+=$2;n[$1]++} END{for(c in s) printf "%s\t%d\t%.2f GiB\n",c,n[c],s[c]/2^30}' | sort -k3 -nr
```

```sql
-- Q1 the latest-two rows
SELECT computer_id, base_ref, sequence, created_at FROM computer_projection_bases ORDER BY computer_id, sequence;
-- Q2 pins
SELECT computer_id, base_ref, reference_id, created_at FROM computer_projection_base_pins;
-- Q3 watermarks
SELECT computer_id, watermark_sequence, base_ref, updated_at FROM computer_replay_watermarks;
-- Q4 job state (why no new base)
SELECT computer_id, generation, status, reason, target_sequence, genesis_repair, failures, LEFT(error_text,300), updated_at
  FROM computer_projection_jobs ORDER BY updated_at;
```

Settles: whether the 10-08/10-09 base_refs are watermark, latest-two or pinned; whether the owner computer's job is blocked and why; which computers' bases belong to dead computers (join `computer_id` against the vmctl list below).

```sh
journalctl -u go-choir-checkpointd --since '2026-10-10' --no-pager | grep -i -e 03335285 -e 'checkpoint published' -e blocked | tail -40
```

### vmctl ownership (dead-computer test for F2, F3, F5)

```sh
curl -s http://127.0.0.1:8083/internal/vmctl/list | jq -r '.. | .computer_id? // empty' | sort -u > /tmp/live_computers
ls $A/sha256/file-cas-chunks | sort > /tmp/cas_computers
comm -13 /tmp/live_computers /tmp/cas_computers | wc -l      # computers with chunks and no vmctl record
```

(`/internal/vmctl/list` and `/internal/vmctl/lookup?computer_id=` are the routes [V `internal/vmctl/handlers.go:1273,1277`]; port 8083 is from the 09-11 deployed proof.)

### file-cas (F3, F4, F5)

```sh
# bytes per computer dir, largest first, flagged live/dead
du -sk $A/sha256/file-cas-chunks/* | sort -nr | head -30
# rootless computers: have a chunk dir, no root dir
comm -23 /tmp/cas_computers <(ls $A/sha256/file-cas-roots | sort)
# count and bytes of roots per computer
for d in $A/sha256/file-cas-roots/*/; do echo "$(basename $d) $(ls $d | wc -l) $(du -sk $d | cut -f1)"; done | sort -k3 -nr | head -20
# the GC's own view, latest report: eligible, unproven, warnings
journalctl -u go-choir-corpusd --no-pager | grep 'artifact gc:' | tail -1 | sed 's/.*report=//' | jq '.namespaces, .warnings'
```

```sql
SELECT computer_id, COUNT(*), MIN(created_at), MAX(created_at) FROM computer_file_roots GROUP BY computer_id ORDER BY 2 DESC;
```

The journal report settles: `Unproven` and the rootless warning for chunks; `Eligible` for `og`; the empty-live-set guard.

### checkpoint-scratch (F1)

```sh
du -sk $A/checkpoint-scratch/* | sort -nr | head -50
find $A/checkpoint-scratch -maxdepth 1 -mindepth 1 -type d -printf '%TY-%Tm-%Td %f\n' | sort | uniq -c -w10
systemctl is-active go-choir-checkpointd; systemctl list-timers | grep -i checkpoint
```

```sql
-- the dir <computer>-<generation> against jobs
SELECT computer_id, generation, status, genesis_repair, updated_at FROM computer_projection_jobs;
```

Settles: which dirs are older than their computer's current job, and which belong to terminal or dead computers.

### capture-bodies (F7)

```sh
# size distribution: raw vs small blobs
find $A/capture-bodies -type f -printf '%s\n' | awk '{n++; s+=$1; if($1>262144){b+=$1;bn++}} END{printf "files=%d total=%.2f GB; >256KiB files=%d bytes=%.2f GB\n",n,s/1e9,bn,b/1e9}'
# growth by day
find $A/capture-bodies -type f -printf '%TY-%Tm-%Td %s\n' | awk '{s[$1]+=$2;n[$1]++} END{for(d in s) printf "%s %d files %.2f GB\n",d,n[d],s[d]/1e9}' | sort | tail -14
# does anything fetch bodies by hash? (access logs; adjust unit names)
journalctl -u go-choir-proxy -u go-choir-corpusd --since '2026-10-15' --no-pager | grep -c '/captures/bodies/'
```

Store B read (corpus Dolt 127.0.0.1:13307): count of `choir.web_capture` rows by age, and how many publications cite a capture (the citation index for F7 does not exist yet; this is the first look).

### artifact-gc-log (F6)

```sh
ls -l --time-style=long-iso $A/artifact-gc-log | awk '{print $5, $6, $8}' | sort -k2
for f in $A/artifact-gc-log/20*.jsonl; do echo "$f"; jq -r '[.op,.ns]|@tsv' "$f" | sort | uniq -c | sort -nr | head -6; done
zstd -19 -c $A/artifact-gc-log/2026-10-14.jsonl | wc -c     # test ratio on the 74 MB file; writes nothing to disk
ls $A/artifact-gc-log | grep -c manifest                     # tape-sweep manifests share the directory
```

### outside the list

```sh
du -sk $A/computer-inputs/blobs; find $A/computer-inputs/blobs -type f -printf '%TY-%Tm-%Td\n' | sort | uniq -c | tail
du -sk $A/tape-sweep-archive/*
```

## 7. Ranked fix list

By GB reachable per unit of risk. "Owner yes" means a blocking ask is justified (red or black, owner data or only copy).

1. **F1: checkpoint-scratch boot sweep.** Up to **6.4 GB** one-time, keeps it near 0. Orange. No owner yes unless a genesis-repair dir of the owner computer is present. First step is the read in section 6, then a problem commit, then the sweep code with the 10-15 gates. A manual `rm` of old dirs is safe once the gate passes, if FP0 needs space now.
2. **Measure F2/F3 (dead computers).** Up to **4.0 GiB of bases** and an unmeasured share of 25.6 GiB of chunks. Red. The measurement is free (section 6). Owner yes for any computer that ever had an owner account; test and fleet computers only need the reviewed dry-run.
3. **F4: retire old file-cas roots.** About **2.5 GiB** one-time, 0.1 GB/day forward. Orange. No data loss, since the chunks are already gone. Do it after F3 so the GC rule change is one review.
4. **F6: compress artifact-gc-log.** About **1.5 GB** one-time. Green/yellow. Cheapest, lowest risk. Hash-verify, keep recent 7 days plain, skip tape-sweep manifests.
5. **F7: capture-bodies raw-blob retention.** Possibly **most of 6.0 GB**, **0.6 GB/day** forward. Red; **owner yes**. Largest forward saving, so decide the rule soon. Needs the size census and an access-log check first. The block JSON and the text blob stay.
6. **F5: shorter file-cas window.** Unmeasured. Orange. After F3/F4.
7. **F8: unblock the owner computer's checkpoint.** About zero net bytes; the point is restore safety, since 10-08/10-09 are likely its only bases and may sit on an old vocabulary. Red; its own problem doc. **Owner yes** for any step that touches that computer.
8. **Not recommended: F9** (cut "latest two" to one). Saves one base per computer (8.4 GiB for the owner) but removes the fallback.

Total reachable from 1 to 6 without touching the owner computer: roughly **15 to 20 GB**, depending on the F2/F3 and F7 measurements. This will not by itself rescue the Node B root disk; the paper's data image (about 250 MiB a minute, see the 10-22 disk record) is the larger fill.

## Heresy delta

- discovered: `file_cas_roots` rows and `file-cas-roots` files have no retention; pins have no release path; `capture-bodies` and `artifact-gc-log` have no collector; the GC live set never asks whether a computer still exists; the scratch sweep misses terminal and dead computers.
- introduced: none.
- repaired: none (docs only).
