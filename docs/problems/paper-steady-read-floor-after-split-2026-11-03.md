# The paper's median read minute rose after the store split (2026-11-03)

Status: open. Found by the store split's S6 judgment
(`docs/definitions/choir-store-header-body-split-2026-11-02.md`, receipt S6).

## Evidence

From the memory meter (`/var/lib/go-choir/growth-meter/mem-2026-11.jsonl`,
the paper's VM), excluding 30 minutes after each boot:

- Before the split (2026-11-01T19:32Z to 2026-11-02T04:00Z): median 3.1,
  mean 75.8, p90 132.7 MiB read per minute.
- The meter day after it (2026-11-02T05:00Z to 2026-11-03T05:00Z): median
  38.7, mean 49.4, p90 104.5.
- Work was similar: 28.3 against 32.5 inference requests a minute. Reads
  per request fell from 6.7 to 2.1 MiB.

So the big reads are gone (the retirement pass and boot bursts fell), but
the typical minute now reads about 40 MiB where it read about 4. The hourly
means over the day sit between 30 and 115 MiB/min, except the hours when
the paper was wedged.

Second residual: the finalize freed 22.4 GB inside the paper's disk and
0.47 GB on the owner's computer, but the host's free space did not change
(143G before and after). The guest images are sparse and the guest has not
discarded the freed blocks.

## Belief

HYPOTHESIS: a steady re-read of the hot set (the header table and its
indexes, about 8 GB, and run_memory_entries, 4.7 GB) against a guest page
cache bounded by the memory ceiling; the old layout's typical minute was
served from a warmer cache. Not yet separated from new steady work
(intake, story expansion, tonight's feature landings).

## Next probe

Correlate the paper's per-minute read_bytes with its statement mix for one
hour (the guest store's slow-statement log or a sampled statement counter),
and read the guest's page cache size against the hot set; separately, check
whether the guest mounts its data disk with discard or runs fstrim, and run
one fstrim on d@e to see whether the host reclaims.
