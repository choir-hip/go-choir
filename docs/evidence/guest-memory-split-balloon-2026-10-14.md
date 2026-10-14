# Guest Memory Split and the Balloon's First Read, 2026-10-14

Computer `computer-c926a9f6b49cc542667b3cafa21d86e9` (VM
`vm-0f24d3b8f19a07764b562a7447fb7a06`), build 5e4b6fa7, 8 GiB, booted
06:12 UTC with the virtio balloon (free page reporting) and the soft Go
memory limits. Burst: the paper tracer scenario (rss:hn_frontpage),
06:12-06:14. Host RSS sampled every minute from the Firecracker process;
guest split read at 06:27 from `/internal/debug/pprof/procmem` (host-only).

## Host RSS (Firecracker process)

| time | VmRSS |
|---|---|
| 06:13:51 | 1.67 GiB |
| 06:17:53 | 2.03 GiB (peak) |
| 06:18:53 to 06:25:57 | 2.03 GiB, flat |

## Guest (06:27)

- MemTotal 7.76 GiB, **MemFree 5.77 GiB**, MemAvailable 6.45 GiB,
  AnonPages 0.87 GiB, Cached 0.86 GiB.
- Host RSS (2.03 GiB) equals the guest's used memory (about 2.0 GiB): the
  free 5.8 GiB was never touched, so the host never allocated it.

| process | count | RSS | private (anon) | file-backed |
|---|---|---|---|---|
| autoputer daemon | 1 | 776 MiB | **690 MiB** | 85 MiB |
| desk-session workers (autoputer re-exec) | 9 | about 80 MiB each | **about 17 MiB each** | about 63 MiB each |
| all other processes | | under 200 MiB | | |

## Reading

- **A desk worker costs about 17 MiB of private memory.** Its other
  63 MiB are the autoputer binary's pages, shared through the page cache.
  Nine warm workers cost about 150 MiB, not 720. The 12-idle-session
  cache (`deskSessionMaxIdle`) is not where memory goes; the daemon is
  (690 MiB private, mostly the store; see the provisional cost read).
- **The balloon had nothing to return in this window.** Free page
  reporting returns pages the guest frees after touching them. Here the
  guest never touched its free memory, and nothing it used was freed: the
  workers live 30 idle minutes, and the daemon keeps its heap. The lever
  shows only after a burst that frees memory (workers exiting, the daemon
  shrinking under its soft limit, page cache dropped). That read is
  still owed: host RSS before and after the workers' 30-minute
  retention ends.
- So the next density lever is the daemon's store memory (Dolt node
  cache and live trees; the `CHOIR_DOLT_CACHE_NODE_MIB` knob is wired,
  default off), not the worker cap.
