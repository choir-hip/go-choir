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

## 8. Build plan for F1, F4 and F6 (lane `lane/fp10-artifact-retention-fixes`, 2026-10-22)

Owner, 2026-10-22: "Anything which improves resource utilization on a forward and ongoing basis should be prioritized over stability, availability, or uptime in the short term." FP0 asked for ranked items 1, 3 and 4 only (F1 scratch, F4 roots, F6 log). Nothing else in section 5 is touched. Documented before any code, per Problem Documentation First.

### F1: checkpoint-scratch sweep (orange)

- Where: `cmd/checkpointd` (a `scratch_sweep.go` beside `scratch.go`), called from `runCheckpointWorker` at start (before the claim) and from the existing 45 s worker tick at most every 15 minutes. One new read-only store method, `Store.ListProjectionJobs` (a plain SELECT of `computer_projection_jobs`; no schema change).
- What it removes: a direct child of the scratch root named exactly `<computer>-<digits>` (the S1 to S3 shape rule of the 10-15 record is kept) that no job holds.
- Held (never removed): the computer's job row has the same generation and status `queued`, `running` or `retry`; a generation newer than the row's (out of order, fail closed); any name not of the exact shape; anything not a directory.
- Genesis-repair guard (kept, logged as `scratch_kept`): a directory is genesis-repair scratch when the job row has `genesis_repair` and the same generation, or when the directory holds a `.genesis-repair` marker. The worker now writes that marker when it creates the directory of a genesis-repair job, so the guard survives the row moving to a later generation. The guard does not try to narrow itself to "computers that ever had an owner account": checkpointd has no ownership authority (that is vmctl's registry, used only by the tape sweep), so it fails closed and keeps every genesis-repair directory of every computer. Cost: a genesis-repair scratch of a throwaway computer stays until a human removes it. That is the safe side of the record's "needs the owner's yes".
- Age gate: the record's gate (c), directory mtime older than 48 h, is kept as a second guard.
- Order of reads: list the directories first, read the job rows second. A job claimed after the listing creates a directory that was not listed; a job that existed at the read is held.
- Crash safety: a candidate is first renamed to `.sweeping-<name>` (atomic), then removed. The held name never names a half-deleted tree. A `.sweeping-*` leftover from a crash is finished at the next sweep. Removal errors are logged and never fail the job.
- Log: each removal (`scratch_removed`, bytes) and each kept genesis directory (`scratch_kept`, reason) is appended as a JSONL record to `artifact-gc-log/checkpoint-scratch-<UTC day>.jsonl`. A separate file, not the corpusd day file: corpusd's buffered writer can split a line across two writes, and a second process appending to the same file could land inside it. The name does not match the log rotation of F6, so it stays plain (a few lines a day).
- Failure modes written before the code: a held directory is removed; a genesis-repair directory is removed; a non-shaped name is removed; a crash leaves a half-deleted held name; an orphan is never removed; a sweep error fails the checkpoint job.
- Rollback: `git revert`. The content is derived data (a rebuild from the seed base, minutes; hours only for a genesis repair, which is exactly what is kept).

### F4: retire old file-cas roots (orange)

- Where: `internal/platform/artifact_gc.go`, inside `RunArtifactGC`, before the live sets are read, only in `active` mode and only when `file-cas-roots` is in the active namespaces (it is by default).
- Rule: for each computer, keep its latest root (by `created_at`, then `root`, as `LatestFileRoots` orders) and every root newer than the retention window. Delete the other rows of `computer_file_roots` with a plain `DELETE` (no schema or migration change; the rows stay in Dolt history). The window is `max(grace, 7 days)`: a short test grace cannot shorten it.
- Owner-data guard: the rule is the same for every computer, owner computers included: never the latest root, never a root inside 7 days. There is no weaker rule for computers without an owner account, so no ownership lookup is needed and none can misclassify.
- Effect on the sweep: once a row is gone its manifest is no longer in the `file-cas-roots` live set, and the existing sweep deletes the manifest file when it is past grace. The chunk sweep already ignores old roots (`fileCASRootsForGC` keeps the latest root plus roots inside the window), so retiring rows does not change which chunks are live. The record's wording "so the chunk GC can collect chunks no live root references" is already true today; what retirement adds is the manifests and the rows.
- Guards in the SQL: `created_at` older than the window and `root` not the computer's latest, evaluated per computer, capped at `MaxDeletes` rows per run. The report gains `retired` on the namespace (dry-run reports what it would retire).
- Log: each retired row is appended to the day's artifact GC log as `root_retired` (computer, root, manifest ref).
- Failure modes written before the code: the latest root is retired; a root inside the window is retired; dry-run retires; a computer with one old root loses it; an old root is retired while another computer's row is touched; a retired row's manifest is not collected afterwards.
- Rollback: `git revert`; deleted rows can be read back from Dolt history; deleted manifest files do not come back, but they name chunks that are already past the window.
- Stop condition: if retiring needed a schema or migration change, this item would be deferred here instead. It does not.

### F6: compress the artifact GC log (green/yellow in intent; the rotation runs in-process, so orange for deploy)

- Where: `internal/platform` (`artifact_gc_log.go`), called at the end of `RunArtifactGC` in `active` mode, so it runs hourly with the GC and once at the first pass after a deploy.
- Rule: a file named exactly `YYYY-MM-DD.jsonl` whose UTC day is more than 2 days before today is written to `YYYY-MM-DD.jsonl.zst.tmp`, read back and compared byte for byte (sha256) with the original, renamed to `.zst`, and only then is the original removed. A mismatch, error or crash keeps the original and removes the temporary file. If `.jsonl` and `.jsonl.zst` both exist (a crash after the rename), the `.zst` is verified again before the original goes.
- Skipped: everything not matching that exact name, so the `tape-sweep-<run>.manifest.jsonl` files (a tape-sweep apply names a manifest by sha256) and the F1 scratch log are never touched.
- Readers: no code reads the log (the 10-22 grep). The new `openArtifactGCLog` reads both forms (plain, or `.zst`) and is what the round-trip check and the tests use, so any future reader has one entry point.
- Rollback: `git revert`, then `zstd -d` the `.zst` files (the original bytes are exactly reproduced; that is the round-trip check).

### Test plan

Focused, with `-race`, `GOMAXPROCS=2 GOFLAGS=-p=1`: `go test ./cmd/checkpointd` and `go test ./internal/platform -run 'FileRoot|GCLog|ProjectionJobs|ArtifactGC'`. No host, no staging, no SQL against a production store. Test commit first; it fails on this lane's base `d381440ce` (F1: does not build, `sweepOrphanScratch` undefined; F4 and F6: the sweep neither retires rows nor compresses logs).

Heresy delta of this plan: discovered none; introduced none; repaired none (docs only).

## 9. Built: F1, F4 and F6 (lane `lane/fp10-artifact-retention-fixes`, 2026-10-22)

Status: built and tested locally; not deployed. Nothing was read from or run against Node B, staging or any store. F2, F3, F5, F7, F8 and F9 are untouched. Commits, in order: plan (green) `03a718a43`; tests (yellow); F1 (orange); F4 (orange); F6 (orange: it removes files in-process after the check; the plan said green/yellow only for a script).

Tests: `GOMAXPROCS=2 GOFLAGS=-p=1 go test -race ./cmd/checkpointd` and `go test -race ./internal/platform -run 'ArtifactGC|FileCAS|FileRoot|TapeSweep|GCLog|ProjectionJob|Checkpoint|ProjectionBasePin'`; all pass. The test commit does not build on the plan commit (the new functions are undefined), which is how it fails there.

### F1 as built

- `cmd/checkpointd/scratch_sweep.go`; called from `runCheckpointWorker` before the claim (the checkpointd timer fires every minute, so that is every minute) and from the 45 s tick at most every 15 minutes during a long job.
- Held and so never removed: the row's generation with `queued`, `running` or `retry`; a generation newer than the row's; any name not shaped `<computer>-<digits>`; anything modified in the last 48 h; plain files.
- Genesis-repair guard, as planned: a sibling `<dir>.genesis-repair` marker (written by the worker for a genesis-repair job, removed with the directory on a successful publish) or a row with `genesis_repair` at that generation keeps the directory, logged once a day as `scratch_kept`. The marker is a sibling because the scratch directory is packed into the published base blob. A marker whose directory is gone is deleted. Because checkpointd has no ownership authority, no genesis-repair directory is removed for any computer; the owner's yes is needed for all of them.
- Crash safety: rename to `.sweeping-<name>`, then remove; the next sweep finishes a leftover. The removal log must open before anything is removed.
- Declined: scoping the keep rule to owner computers (see the plan; fail closed). A directory of a computer with a `failed` job at the current generation is removed, since the next admission starts a new generation and never resumes it.

### F4 as built

- `internal/platform/file_cas_retire.go`, called from `RunArtifactGC`. A plain `DELETE` on `computer_file_roots`: no schema, DDL or migration change, so nothing was deferred.
- Owner-data guard, named: for every computer, owner or not, a root is retired only if it is older than `max(grace, 7 days)` **and** is not the computer's latest root. There is no weaker rule for any class of computer. A computer with one root loses nothing.
- `MaxDeletes` (10,000) caps rows retired per pass, so the first clean-up may take more than one hourly pass. The namespace report carries `retired`.
- A correction to the record's F4 text: the chunk sweep already keeps only the latest root plus roots inside the window (`fileCASRootsForGC`), so retiring rows does not free chunks; it frees the rows and the manifest files, whose sweep was blocked only by the all-rows-live rule.

### F6 as built

- `internal/platform/artifact_gc_log.go`. FP0 asked for "days older than 2", not the record's 7: a `YYYY-MM-DD.jsonl` whose day is before today minus 2 days is compressed, so today, yesterday and 2 days ago stay plain.
- `rotateArtifactGCLogs` runs after each active pass, outside `artifactGCMu` (which also serializes projection-base pins), under its own mutex. Verification is the sha256 and length of the decompressed file against the original, done before the rename and again after it.
- Only the exact daily-file name is touched. `tape-sweep-<run>.manifest.jsonl` and `checkpoint-scratch-<day>.jsonl` never are.
- No code reads the log; `openArtifactGCLog` reads both forms and is the one entry point the tests and any future reader use.
- The expected 8 to 12 times ratio is a hypothesis from the record; the report's `log_rotation` records the real before and after.

### Expected bytes on Node B (estimates from section 1 measurements, not measured)

| Fix | One time | Per day |
|---|---|---|
| F1 scratch | up to 6.4 GB, minus directories held by a live job, less than 48 h old, or genesis-repair | keeps the store near 0; before the 10-15 fix it grew about 1 GB a day, the residual is not measured |
| F4 roots | up to about 2.5 GiB of manifests (several hourly passes at 10,000 deletes per pass) | about 0.1 GB |
| F6 log | about 1.4 GB (1.7 GB less the last 3 days, at 8 to 12 times) | the daily file shrinks by about 90 percent three days after it is written |

### What FP0 reads after deploy

All read-only.

- F1: `journalctl -u go-choir-checkpointd --since <deploy> | grep 'checkpoint scratch sweep'` (removed count, bytes, kept genesis count); `du -sk $A/checkpoint-scratch`; `jq -c . $A/artifact-gc-log/checkpoint-scratch-*.jsonl` for each `scratch_removed` (path, bytes) and `scratch_kept` (reason); `ls $A/checkpoint-scratch/*.genesis-repair`. Proof of the guard: no directory with a marker or a `genesis_repair` job row is gone, and every held `<computer>-<generation>` of a queued, running or retry job in `computer_projection_jobs` still exists.
- F4: the corpusd journal line `artifact gc:` with `.namespaces["file-cas-roots"].retired` and `.deleted`; `du -sk $A/sha256/file-cas-roots`; `jq 'select(.op=="root_retired")' $A/artifact-gc-log/<day>.jsonl`; `SELECT computer_id, COUNT(*), MAX(created_at) FROM computer_file_roots GROUP BY computer_id` shows every computer that had a root still has at least one, and no row younger than 7 days is missing from the retired list. The first pass is about 10 minutes after the corpusd restart.
- F6: the same `artifact gc:` report's `.log_rotation` (`logs_compressed`, `log_bytes_before`, `log_bytes_after`); `ls -l $A/artifact-gc-log` shows `<day>.jsonl.zst` for days before today minus 2 and no `.jsonl` beside them; `zstd -dc <day>.jsonl.zst | head` is the original JSONL; the tape-sweep manifests are unchanged.

### Deferred

- Nothing in the three items was deferred for a schema or migration change.
- Not done, by instruction: F2/F3 dead computers, F5 file-cas window, F7 capture-bodies, F8 owner checkpoint, F9.
- Open: the genesis-repair keep rule leaves a throwaway computer's genesis scratch in place until a human removes it. If the host read finds many, a narrower rule needs an ownership source inside checkpointd (the vmctl registry), which is its own change.

Heresy delta of this build: discovered none new; introduced none; repaired: the scratch sweep missing terminal and dead computers (section 8's F1 class), `file-cas-roots` rows with no retention, and `artifact-gc-log` with no rotation. Mutation class: orange (checkpointd housekeeping, GC live set and a row delete on `computer_file_roots`, GC-log file rotation); protected surfaces touched: none of the red list (no Texture canonical write, no checkpoint publication or route change).

## 10. Round 2 review findings (bunch 131, tip `c4c2c4191`), before any fix

Three reviews: `omp-deepseek-v41-flash` ("do not ship as-is, two required fixes"), `omp-gemini38` ("revise before ship"), `devin` ("approve, two medium-low issues"). Findings are numbered N1 to N11 here; dispositions are in section 11 after the fixes.

| # | Where | Finding | Rated by | Severity |
|---|---|---|---|---|
| N1 | F1 `scratch_sweep.go` list, job read, rename, remove | A job admitted between the job-row read and the rename (a row-less `<computer>-1` directory, generation 1 is the first value `EnqueueProjectionJob` assigns) can have its directory renamed and deleted under it. The held-set invariant is checked once, then acted on later. The coordinator asks for the hold check and the rename to be atomic with claiming, or a re-check after the rename with a rename back. | all three | high (deepseek), high (gemini, "overstated" in dissent), medium (devin) |
| N2 | F4 `artifact_gc.go` order | Root rows are retired before the live set is loaded. A live-set failure aborts the pass with rows already deleted and an empty report. Required: compute and verify the live set before any DELETE; one transaction scoped to that set. | deepseek, gemini | medium / high |
| N3 | F4 `file_cas_retire.go` | The old-row query materializes every row older than the window before the limit is applied, and `created_at` is not the leading index column. | deepseek, gemini | medium |
| N4 | F1 `scratch.go` | `pruneSupersededScratch` (the claim-time prune) removes older generations without checking the genesis-repair marker, so the marker's guarantee does not hold on that path. | devin | low-medium |
| N5 | F1 and F4 logs | `scratch_removed` and `root_retired` are written after the mutation, so a crash drops the audit line. The GC's own discipline is intent first. | all three | low |
| N6 | F1 dangling-marker cleanup | It runs before the job snapshot with no age gate; a marker can be listed while its directory (created first) is not yet in a chunked `ReadDir`. | deepseek | low |
| N7 | F6 | Rotation runs when the GC pass failed (`err != nil`, for example a live-set failure). | deepseek | low |
| N8 | F6 | A backward clock step makes today's open file "old". | deepseek | low |
| N9 | F6 | A well-formed `.zst` whose bytes differ from a still-present plain file is removed as corrupt; it could be the only archived copy of earlier records if the plain file was appended after rotation. | devin | low |
| N10 | F4 | The retire budget is additive to `MaxDeletes` (up to 2 times the documented cap). | devin | nit |
| N11 | F1 / F4 | F1 has no off or dry-run mode; retiring rows truncates the Files snapshot listing (`HandleFileCASRoots`) at 7 days; an index on `created_at` would remove the scan. | deepseek, gemini | info |

Evidence for N1, from the code: `ClaimProjectionJob` never changes a generation; `EnqueueProjectionJob` writes `old.Generation+1`, and no row means 0, so a first job is generation 1. No code deletes `computer_projection_jobs` rows, so N1 needs a restored or hand-edited store; the fix is still required because the sweep's contract is absolute. Evidence for N2: `artifact_gc.go` calls `retireFileCASRoots` before `artifactGCLiveSets`, and the live-set failure returns with `Namespaces` empty.

Heresy delta: discovered N1 to N9 (new code only); introduced them in round 1; repaired none yet.
