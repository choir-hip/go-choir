# Research on hypervisors, sync and storage

I wrote this on Wednesday, October twenty-eighth, in the early afternoon UTC. It covers about fifteen minutes of research by Devin and DeepSeek, which ran in parallel after you asked for more research. It follows this morning's report on the state of the data architecture and does not replace it.

The research supports the plan, and it adds three concrete things: a way to hash SQLite tables cheaply, a relay quorum for durable writes, and a tape that can branch later.

Here is the minute version. Cloud Hypervisor really has no diff snapshots today. It has something close enough for hibernation, and it gains live migration, GPU passthrough, and memory hotplug that probably works on our Node B processor. So we stay on Firecracker and bring Cloud Hypervisor in on the test ring. Apple's File Provider and Windows' Cloud Files API fit refs plus content hashes almost exactly. SQLite can get Dolt's cheap per-table hash, but we have to build it, and there are traps. MinIO is out: its open-source repository is archived. The durable write boundary should be a small quorum of relays off the host, the way Cloudflare and Neon do it. I checked the most surprising claims against GitHub myself, and they held.

## The hypervisor

Devin compared Firecracker, Cloud Hypervisor, QEMU, Google's crosvm, Apple's Virtualization framework, and Hyper-V. On your question: Cloud Hypervisor has an open feature request for diff snapshots, filed in July, and no merged code. Its snapshots are full but sparse on disk, so a hibernation writes only the pages the guest touched. It can also restore lazily, paging memory in as needed. One trap is that its default restore mode copies all of memory up front, so we would have to choose the lazy mode on purpose.

Moving to Cloud Hypervisor would cost us the density numbers we have measured on Firecracker, Firecracker's guaranteed small footprint, and one cold boot per computer at the switch. It would gain us live migration, including a same-host migration of about fifty milliseconds for hypervisor upgrades. It would also gain GPU passthrough and, most interesting for us, memory hotplug. Firecracker's memory hotplug is broken on Node B's processor because of where it places the memory region. Cloud Hypervisor places that region differently, so it probably works there. That is unproven until we try it. Live migration has a price too: guest memory must be shared, which changes how memory is counted and rules out the copy-on-write restore.

On desktops, macOS must use Apple's framework. Linux desktops can run the same hypervisor as the cloud. Windows is the hard case. The plan Devin proposes matches what you said: finish the seam on Firecracker, run Cloud Hypervisor on the test ring, and keep Firecracker for Autoradio's first stage. Cloud Hypervisor then earns more of the fleet when a second host or a GPU gives it a job.

## The drive and desktop sync

DeepSeek found that the big providers converged. Dropbox, Google Drive, OneDrive and Nextcloud all use Apple's File Provider on the Mac, and OneDrive and others use the Cloud Files API on Windows. Linux has no native equivalent, so everyone there uses a FUSE mount. The recommendation is one sync core with three thin adapters.

Our model fits. Each file gets a stable identifier that survives renames. Its content version is its hash. Apple's change anchor can carry our tape position. Two cautions: we need a small change log for enumeration, because scanning the tape every time would be too slow, and many apps download whole files rather than streaming them.

For handing a computer between cloud and desktop, a lease alone is unsafe. A paused machine can wake after its lease expired and keep writing. The fix is a fencing number: every tape write carries the lease's epoch, and the tape refuses any write with an old one. For your later offline feature, DeepSeek recommends making the tape able to branch now: each event names its parents and its branch. Changing a straight-line tape into a branching one later would be a migration, not a switch.

## SQLite's missing hash

Dolt gives us a hash per table almost for free, and SQLite has nothing like it. The construction DeepSeek proposes keeps one running hash per table, updated by triggers in the same transaction as each row change. The hash can add and subtract rows. DeepSeek reproduced the traps on a real SQLite build. Insert-or-replace silently breaks the hash unless one setting is turned on. Vacuum renumbers rows, so we must hash the row contents, never the row number. Adding a column with a default changes every row without firing any trigger. All three are manageable, but they have to be tested on our real schema.

DeepSeek also found DoltLite, DoltHub's SQLite fork built on the same kind of tree as Dolt. It went to beta in August. It may give us cheap table hashes and branching in one engine, so it joins the bake-off as a third candidate.

## Storage and durable writes

For content storage, start with a plain directory that packs small objects into larger files, which also suits a self-hosted single box. Later, SeaweedFS fits millions of small objects at about sixteen bytes of memory each. MinIO is out because its repository is archived and security fixes now go only to the commercial product. Garage struggles at around ten million objects. Ceph is far too heavy for one host.

For durable writes, the boundary should be a quorum: two of three or three of five relays off the host, each writing to disk and returning a signed receipt. Object storage is a slower cold tier behind them. The estimated latency fits easily under our hundred-millisecond target. This raises a new question for you later: a self-hosted single box has no "off-host". Its owner would need a second machine, a remote bucket, or an honest weaker promise.

## What I would do next

The one thing worth remembering: none of this research overturns the plan, and it turns three vague parts into buildable pieces. Next I would add DoltLite and a witness spike to the bake-off, make the tape schema branch-ready before migration, and build a small relay prototype to measure. The census and the crash drill still come first.

## Names and receipts

- Research record: docs/evidence/data-architecture-research-2026-10-28.md
- Raw memos, gitignored: .agentic-consensus/data-arch/research/out-devin and out-deepseek
- Verified on GitHub: Cloud Hypervisor v54.0 of October fourteenth; Cloud Hypervisor issue 8645, open; Firecracker issue 6255, open; the minio/minio repository, archived
- DoltLite beta: dolthub.com/blog/2026-08-31-doltlite-beta
- Prior report: docs/reports/data-architecture-state-and-paths-2026-10-28.md
