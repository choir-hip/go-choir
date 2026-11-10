# Node B's sealed tape segments re-packed as 64 KiB blocks: what it would buy (2026-11-10)

Status: measurement and estimate only, mutation class green (docs). Nothing
on Node B was changed and no segment content left Node B. Item X6 of
[the overnight mission](../definitions/choir-autopaper-overnight-2026-11-09.md)
("explorations taken further"), second half. It tests rule A and rule F of
[the small-object design](../design/small-object-storage-2026-10-31.md) with
today's numbers, and feeds the resource-fabric question of what unit the
tape should be stored and synced in
([resource fabric](../design/resource-fabric-2026-10-31.md)). Its companion
is [the paper's tape synced to a Mac](tape-sync-to-mac-shape-2026-11-10.md).

Labels: **[M]** measured by a command in this run (Node B read-only,
metadata only, 2026-11-10 10:46 UTC; or a local throwaway program);
**[C]** read from code; **[E]** estimated, arithmetic shown; **[A]** assumed.
GB means 10^9 bytes, MiB and KiB are binary.

## Summary in plain words

1. **The inode problem the small-object design was written for is gone, and
   the segment store is now a small number of big files.** `tape-seg` is
   7,698 files (3,857 segments and 3,841 indexes) holding 84.3 GB across 99
   computers [M]. The legacy one-file-per-digest tape directories that held
   10.8 M files on 2026-10-31 now hold about 9,000 directory entries [M].
   Re-packing segments as 64 KiB blocks cannot save inodes: there are none
   left to save.
2. **The paper is 80% of the bytes, and its segments are full of data that
   does not compress.** The paper holds 65.7 GB of segments (1,647 files);
   the other 98 computers together hold 16.6 GB. The filesystem is not
   wasting space on them: allocated bytes equal file bytes to 0.02% [M], and
   the recently sealed segments are 1 to 5 extents each [M], so the
   copy-on-write waste of 2026-10-31 (47 GB on disk for 28 GB) is no longer
   visible in these numbers. But btrfs compression, which the seal rewrite
   now asks for, gained nothing on the sealed segments [M, allocated equals
   apparent], where the 2026-10-30 evidence measured 5.4x on the whole
   segments. In a synthetic mix with sealed (random) payloads, 64 KiB block
   compression gives 1.1x on the interleaved stream and 2.6x on events and
   receipts alone [M, synthetic]. The block format's win is therefore capped
   by how much of the stream is ciphertext: roughly 1.3x to 2.0x overall
   (18 to 40 GB of 82 GB saved) [E], not 5.4x.
3. **"Internal fragmentation" is only a cost if blocks are fixed-size slots,
   and then it is ruinous at the record level.** One object per 64 KiB block
   chain pads the paper's 1.7 KB records to 38.8x their size (97.4% padding)
   [M on synthetic sizes whose mean matches the real mean]. Packing records
   into shared blocks without letting a record span two blocks pads 6.4%;
   letting blocks be variable-length (the natural tapestore form) pads
   nothing. At the file level, 64 KiB slots would cost 329 MB of padding
   across the 7,698 files, 20 times today's 4 KiB rounding (16 MB) [M].

## Method

1. **Where the segments live.** `find /var/lib/go-choir -maxdepth 6 -name
   tape-seg` found one place, `/var/lib/go-choir/platform-artifacts/tape-seg/
   <computer-id>/seg-NNNNNNNN.{seg,idx}` [M]. The filesystem is btrfs on an
   md RAID1 pair, 4 KiB blocks, data profile single, metadata DUP
   (`stat -f`, `btrfs filesystem usage -b /`) [M].
2. **Size list.** One `find ... -type f -printf '%s %b %T@ %h %f\n'` over
   the tree (7,698 lines: apparent bytes, 512-byte blocks allocated, mtime,
   directory, name). No file was opened. Only path names, sizes and times
   left Node B.
3. **Record counts from the index.** The format
   ([`internal/tapestore/format.go`](../../internal/tapestore/format.go)) has
   a 32-byte segment header, a 46-byte record header and a sealed index of
   1,064 fixed bytes, 45 bytes per record and a 4-byte trailer [C]. A sealed
   index of size S therefore names (S - 1068) / 45 records [E, exact if the
   format holds], which gives record counts and mean record size per
   segment from metadata alone.
4. **Extent counts.** `filefrag` (FIEMAP, metadata) on 66 paper segments
   spaced every 25th number [M].
5. **Synthetic segment.** A throwaway Go test (in a scratch file of this
   worktree, deleted, not committed; a copy is in the session scratchpad)
   builds 20,000 synthetic events with the repo's own record framing and
   `packRecord` (the writer's per-record zstd rule): each event is one
   1.5 KB JSON envelope with nine random 64-hex digests and ids, one
   payload of random bytes (lognormal, mean 4.5 KB, p95 12.9 KB, from
   [the tape window sizing](tape-window-sizing-2026-10-30.md) section 2.3),
   and two 0.9 KB pin receipts. That is the measured record mix: 4 records
   per event, 8,139 bytes per event. It is then packed into 64 KiB blocks
   with klauspost zstd at the repo's level (SpeedDefault) and timed.
   Compressibility of the synthetic JSON is a guess (its entropy is mostly
   random hex), so every ratio from it is a floor for the JSON part, not a
   forecast.

## Results

### 1. Where, how many, how big [M]

| | Computers | Segment files | Index files | Segment bytes | Index bytes | Allocated / apparent |
|---|---:|---:|---:|---:|---:|---:|
| Paper (`computer-1355b83b...`) | 1 | 1,647 | 1,646 | 65,744,128,994 | 1,763,931,603 | 1.0000 |
| All others | 98 | 2,210 | 2,195 | 16,599,655,331 | 215,953,995 | 1.0000 |
| Total | 99 | 3,857 | 3,841 | 82,343,784,325 | 1,979,885,598 | 1.0002 |

Next largest computers by directory bytes (path names only): 5.9 GB in
1,013 files, 2.6 GB in 669, 1.5 GB in 511, 1.4 GB in 513, 1.3 GB in 441,
1.1 GB in 253, 1.1 GB in 515, 0.4 GB in 181; the remaining 90 are each under
0.16 GB. On 2026-10-31 the design measured 1,093 paper files and 28 GB referenced
across all computers; the paper's directory alone is 3,293 files now.

Size histogram, segment files [M]:

| Bucket | Paper files | Paper bytes | Others files | Others bytes |
|---|---:|---:|---:|---:|
| under 64 KiB | 248 | 2.6 MB | 190 | 1.7 MB |
| 64 KiB to 1 MiB | 20 | 4.3 MB | 227 | 161.8 MB |
| 1 to 4 MiB | 25 | 67.1 MB | 933 | 2,317 MB |
| 4 to 16 MiB | 196 | 2,289 MB | 658 | 5,081 MB |
| 16 to 32 MiB | 205 | 4,732 MB | 95 | 2,205 MB |
| 32 to 64 MiB | 439 | 22,923 MB | 31 | 1,334 MB |
| 64 to 65 MiB (full seal) | 447 | 30,008 MB | 16 | 1,081 MB |
| 65 MiB and over | 67 | 5,719 MB | 60 | 4,418 MB |

Index files: paper 277 under 64 KiB, 526 of 64 KiB to 1 MiB, 843 of 1 to
4 MiB; others 1,779, 341, 75 in the same buckets.

Only 514 of the paper's 1,647 segments (31%) are full. The rest were sealed
short: 76 of the 111 paper segments sealed in the last 48 hours (68%) were
under 64 MiB, a median of 26 minutes between seals against 50 minutes between
full seals [M]. The code seals at 64 MiB or 50,000 records and, per the
design, at every boot [C, small-object design rule A3]; I did not read the
boot log to separate the causes, so "short seals are boots" is [E].

Records, from the index sizes [E from M]: 43.9 M records in all, mean 1,874
bytes including the 46-byte header; paper 39.2 M records, mean 1,679;
others 4.7 M, mean 3,482. A full paper segment holds a median 34,900 records
(p10 31,600, p90 48,050, maximum 50,000). The synthetic mix above has a mean
of 1,689 bytes, within 1% of the paper's measured 1,679, which is the only
check on the synthetic size distribution (its shape is not checked).

Index overhead today: 1.98 GB of indexes for 82.3 GB of segments is 2.4%
(paper 2.7%) [M]. A full paper segment's index is about 2.1 MB.

Extents: the 66 sampled paper segments below number 650 (the old ones) have
120 to 513 extents each, one 128 KiB extent per 128 KiB; segments above
number 650 have 1 to 5 extents [M]. The seal rewrite
(`tapeSealCompactionConfig`, rule F.2) is therefore in effect for new
segments, and the first 650 were never rewritten. I could not measure the
physical bytes of the old ones: `compsize` is not installed on Node B and
`st_blocks` does not see shared or bookended extents. Whether the old
segments still carry the 2026-10-31 copy-on-write waste is **not
determined**.

### 2. The packing arithmetic

Two levels, because the question can mean either.

**(a) The segment file as the object.** 7,698 files rounded up to 64 KiB
blocks [M arithmetic on the size list]:

| Packing | Blocks | Padding |
|---|---:|---:|
| Segments, one chain each (3,857 files) | 1,258,736 | 148.7 MB (0.18%) |
| Indexes, one chain each (3,841 files) | 32,956 | 179.9 MB (9.1%) |
| All files, one chain each | 1,291,692 | 328.7 MB (0.39%) |
| All files packed together | 1,286,678 | 0.06 MB |
| Today, 4 KiB filesystem blocks (allocated minus apparent) | | 16.2 MB (0.02%) |

Packing the files themselves into 64 KiB blocks saves nothing and the
one-chain-per-file form is 20 times worse than the filesystem. The indexes
are the only small objects at this level, and they are 2.4% of bytes.

**(b) The record as the object**, using the synthetic sizes (80,000 records,
135.1 MB) whose mean matches the paper's [M for the synthetic list]:

| Packing of records | 64 KiB blocks | Padding | Stored bytes vs data |
|---|---:|---:|---:|
| (i) every record its own block chain | 80,015 | 97.4% of stored | 38.8x |
| (ii-a) one stream across blocks, records may span | 2,062 | one tail block per pack | 1.00x |
| (ii-b) shared blocks, a record never spans two blocks | 2,203 | 6.4% | 1.07x |

Records over 64 KiB are 15 of 80,000 (0.02%) in the synthetic list. (i) is
the answer to "one object per block chain": fixed 64 KiB slots for 1.7 KB
objects are not a design. Variable-length compressed blocks (the block
record of rule A.2) have no padding by construction.

Index size of a block layout [E]: per record the same 32-byte digest, one
namespace byte and the length, plus a block number and an in-block offset
(about 8 + 2 bytes in place of today's 8-byte file offset): about 47 bytes
against 45 today. A block table of 16 bytes per block adds 16 KiB per full
segment (1,024 blocks of raw 64 KiB). So the index stays at about 2.4% to
2.8% of raw bytes: **block packing does not shrink the index, because the
unit that must be findable by digest is still the record**. Only dropping the
exact per-record index (looking up by digest prefix, then scanning a block)
would, and that trades 45 bytes per record for decompressing more blocks.

### 3. What block compression is worth, with what the stream is made of

Synthetic stream, 80,000 records, 146.9 MB with headers [M, synthetic]:

| Packing | Ratio | Mean stored block |
|---|---:|---:|
| Today's per-record rule (zstd v2 for events and receipts above 200 bytes when it saves 10%) | 1.09x | |
| 64 KiB blocks of the interleaved stream, payloads included | 1.12x | 55.1 KB |
| 64 KiB blocks of events and receipts only | 2.57x | 25.4 KB |
| of which events only | 2.74x | 23.7 KB |
| of which receipts only | 2.49x | 26.2 KB |
| One zstd-3 frame over the whole stream | 1.16x | |

Today's rule gains nothing on receipts in the synthetic list (1.00x): the
synthetic receipts are mostly random hex, which compresses below the 10%
threshold. Real receipts also hold digests and signatures, so the
per-record rule may do the same on the real tape; the 2026-10-31 evidence
measured "1.54x synthetic" for record-at-a-time and "5.4x" for blocks on
real older segments. These are three different corpora; I did not measure
the real current tape's ratio and **cannot without reading segment content**.

Two measured facts bound the answer without reading content:

- **btrfs transparent compression found nothing** on the paper's 514 full
  segments: allocated bytes are 1.000036 of apparent [M], although the seal
  rewrite sets `Compress: true` and the files carry the btrfs compress
  attribute (`lsattr` shows `c`) [M]. btrfs compresses per 128 KiB extent
  and bails out when a sample looks incompressible, so a stream that mixes
  ciphertext and JSON in the same extent can defeat it. Block compression
  that separates namespaces is the thing btrfs cannot do.
- **The paper's records got smaller** than the 2026-10-30 mean: 1,412 to
  1,453 bytes per record in segments sealed in the last 24 hours (about
  5.7 KB per 4-record event), against 2,035 on 2026-10-30 (8.1 KB per
  event) [E from M index and size sums]. A smaller payload mean (the
  2026-10-30 mean was 4,657 and the median 2,150) is consistent with the
  guest compressing private payloads before sealing
  (`compressPrivateWrites`, in flight in the design). **This is an
  inference from sizes, not a read of content.**

Bounding the saving [E]: let p be the share of segment bytes that is sealed
payload (incompressible), and r the block ratio on the rest.

| p (sealed payload share) | r = 2.6 (synthetic floor) | r = 5.4 (2026-10-30 measurement) |
|---|---:|---:|
| 0.65 (synthetic mix, payload uncompressed) | 1.28x | 1.40x |
| 0.40 (payload compressed by the guest first) | 1.59x | 1.96x |
| 0.25 | 1.86x | 2.57x |

Over the range p = 0.40 to 0.65 and both ratios, the 82.3 GB becomes 42 to
64 GB, a saving of 18 to 40 GB; for the paper's 65.7 GB alone, 34 to 51 GB,
a saving of 14 to 32 GB [E]. The number that would settle it is
one count: bytes of payload records against the rest in a sealed segment,
which needs the index's namespace bytes (the index holds them; reading it
copies a name-free, content-free 2 MB, but I did not do it here, because the
rule was metadata only).

### 4. Cost

| Item | Number | Basis |
|---|---|---|
| Encode one 64 KiB block | 364 us | [M] synthetic, one core, local Mac, zstd default |
| Decode one 64 KiB block | 16 us | [M] same |
| Re-pack of all 82.3 GB: blocks | 1.26 M | [E] 82.3 GB / 64 KiB |
| Re-pack: CPU | 7.6 core-minutes on this Mac; 15 to 25 on Node B | [E], the Node B factor is [A] 2 to 3x slower per core |
| Re-pack: IO at the existing seal-rewrite pace of 32 MiB/s | about 41 minutes of transfer per direction | [E] 82.3 GB / 33.5 MB/s |
| Steady-state write | 2.4 to 2.5 GB/day of segment bytes now | [M] seals of the last 24 h: 2.53 GB |
| Block fill time at the paper's rate | about 1.9 s | [E] 5.2 events/s x 4 records x 1,679 B = 35 KB/s, so 64 KiB every 1.9 s |
| Read of one record | decode one block: about 16 us and a read of the stored block (about 25 KB compressed for events, up to 64 KiB) instead of about 1.7 KB | [M/E] |

Rewrite amplification. Today every byte is written twice, once by the
fsynced append and once by the seal rewrite (rule F.2). A block seal rewrite
is the same second write, of fewer bytes (by the ratio above), so **block
packing at seal adds no write amplification beyond what rule F.2 already
pays** [E]. If blocks were instead formed at append time, a block would be
sealed with each group commit (every 1 to 2 seconds the paper fills one, so
the open segment would hold near-full blocks), but a cold computer with a few
events a day would write one tiny block per commit; its stored block can be
shorter than 64 KiB without padding. Deleting records one at a time inside a
sealed block would need a block rewrite; the design drops whole segments by
time, so this does not arise for the tape.

Read path. The newest-first probe by digest reads the index fanout, finds an
entry, reads the record. Blocks add one decompression (16 us) and 6 to 40
times the bytes read per record (stored block size against record size). On
a page-cache hit that is invisible; on a cold read of the 82 GB it multiplies
IO for random digest reads, though the tape is read in sequence for restore
and follow, where block reads are a gain (fewer syscalls, sequential
decompress at several GB/s).

### 5. What it would buy and cost, summary

| | Today | Block-packed | Change | Basis |
|---|---:|---:|---:|---|
| Files (tape-seg) | 7,698 | 7,698 | 0 | [M] no loose files left |
| Legacy loose tape entries | about 9,000 | | | [M] `ls -f` of three directories |
| Inodes saved | | | 0 | |
| Segment bytes | 82.3 GB | 42 to 64 GB | 18 to 40 GB saved | [E] table in section 3 |
| Index bytes | 1.98 GB | about 2.0 GB | none | [E] |
| Filesystem padding | 16 MB | 0 to 330 MB | worse if slots are fixed | [M] |
| Sync granularity | a sealed 64 MiB file, or whole events | a 64 KiB block, about 2 s of paper tape | finer | [E] |
| Dedup across segments | none (content-addressed records are unique) | none | 0 | [A] |
| Write amplification | 2x (append plus seal rewrite) | 2x of a smaller second write | below 2x | [E] |
| Read cost per record | 1.7 KB | 16 us and 6x to 40x the bytes | worse on cold random reads | [M/E] |

Dedup deserves one line: records are keyed by SHA-256 of their content, one
copy per segment (`Append` of a digest already in the segment is a no-op),
and the legacy loose files were already one copy per digest. I did not look
for the same digest in two segments (that needs the index contents); chunk
dedup across nightly restic runs is already complete for sealed, immutable
segments [C, E], so the nightly job's re-send is the open segment only.

## What is measured, estimated, assumed

- Measured on Node B, metadata only: file counts, sizes, allocated blocks,
  mtimes, extent counts of 66 files, three directory entry counts, btrfs
  usage, mount type.
- Measured locally, synthetic: record framing, per-record and per-block
  compression, block timing.
- Estimated: record counts from index sizes (assumes the format in the code is
  what is on disk; `(size - 1068) / 45` came out an exact integer on the two
  indexes I checked, 19,236 and 47,024 records), payload share, ratios on real
  content, Node B CPU factor.
- Assumed: that the synthetic JSON is no more compressible than random hex
  digests plus text, which makes the JSON ratio a floor; that record counts
  reflect distinct records.

## Limits

- No segment or index content was read, so the real compression ratio and the
  real namespace split of the current tape are unknown. They are one
  ~2 MB index read and one 64 MiB sample away, if the owner permits reading
  content on Node B; running them off-host would copy private data.
- `compsize` is absent, so physical bytes of the 650 oldest, fragmented
  segments, and the 47 GB of 2026-10-31, are not re-measured. If the
  bookends are still there, the old-segment rewrite (rule F.1, defragment
  once) is worth more than any block format.
- The synthetic mix has 15 records over 64 KiB in 80,000. The real maximum
  payload is 767 KB, so a few real blocks hold one record.
- Boot-time seals were inferred from sizes, not from boot logs.
- The Mac has different cores and memory bandwidth from Node B.

## What this suggests

- **For the small-object design**: the inode half of the problem is solved
  by the legacy-to-segment move and does not argue for 64 KiB blocks. The
  byte half is about what the tape is made of. Ask the sealed payload share
  first (one index read). If it is above 0.6, block compression is worth at
  most 1.3x and the better move is compressing payloads before sealing, which
  has already begun; if it is 0.3 or below, blocks are worth about 2x
  (18 to 40 GB).
- **Keep the pack format record-addressed with variable-length blocks.** A
  fixed 64 KiB slot is ruinous at 1.7 KB records and pointless at the file
  level. 64 KiB is a good compression window (it fills every 2 s on the
  paper and decodes in 16 us) and a good sync window; it should not be a
  storage slot.
- **Fix the segment count before the format.** 68% of recent paper segments
  are short seals, 85% of all segments are under 64 MiB, and the `.idx` of a
  short segment is as costly in files as a full one. Rule A3 (reopen instead
  of seal at boot) is the cheapest change in this document and the only one
  that reduces file count.
- **For the resource fabric**: the unit that syncs should be a sealed block
  range, not a sealed segment. A 64 KiB block is about 2 seconds of paper
  tape; a segment is 25 to 50 minutes of it. See
  [the Mac sync shape](tape-sync-to-mac-shape-2026-11-10.md) for what that
  difference is worth in lag.
- **Next measurements, cheapest first**: namespace split from one sealed
  index (read 2 MB, copy nothing); `compsize` or `btrfs filesystem du` on the
  old segments; whether short seals are boots (boot log counts against seal
  times).
