# The data architecture, state and paths

I wrote this on Wednesday, October twenty-eighth, in the early afternoon UTC. It covers a three-round review panel that ran for about an hour and a half today, with Devin and DeepSeek on the panel. It also covers where the data architecture stands after this week.

The panel agreed: keep small pointers in a small database, keep everything else as content-addressed objects, and make the tape the only thing that must be durable.

Here is where things stand. Choir's memory and disk trouble comes mostly from the shape of the data, not from Dolt the engine. We copy immutable content into a versioned, mutable database, and then we copy the copies. The panel's verdict is to change the data model first. It only takes small steps, and each one leaves a working system. The engine question comes second, and it gets settled by measurement. SQLite is the likely winner for each computer's small state. Postgres is the likely home for shared platform state. A content-addressed object store holds the bulk. Dolt stays in the contest as the control, and it could still win. You have one real question to answer: what zero downtime means for Autoradio. Everything else is a path I can walk without you.

## What I asked, and how the rounds went

You asked for three rounds: divergent, then lateral, then convergent. You wanted me to review between rounds and both panelists to research prior art. I gave each of them the goal file, the design notes and your binding answers from this morning.

The divergent round came back wide. Between them they offered more than a hundred options. Some were ordinary, like SQLite per computer with Postgres for shared state. Some were radical. One was to make the virtual machine itself the database, using Firecracker snapshots. Another was to delete the per-computer store and treat everything as a projection of the tape. Another was to move each computer's store out of the guest onto the host. They cited real systems: Cloudflare's Durable Objects, which run one SQLite database per object at enormous scale; Neon's split of compute from storage; and Fly's Corrosion, a gossip-replicated SQLite. Fly had its worst outage when one global state domain amplified a single deadlock.

I grouped the options into ten families. Then I looked for what the round had missed. Nobody had asked what actually fills a computer's memory. Everyone treated the Autopaper computer as an ordinary computer. Nobody had defined zero downtime. I put those gaps into the second round.

## The lateral round changed the question

The second round asked both panelists to break the frame instead of adding options. Working separately, they reached the same reframe: authority is refs, content is objects.

Git is the analogy. It keeps every piece of content as an immutable object named by its hash. The only mutable things are refs, which are tiny pointers. Maildir is the other analogy. A mail server keeps one file per message, and its indexes are caches it can delete and rebuild. Our Texture documents already work this way. A document's history is a chain of revisions, each named by its content, and the document row just points at the latest one. Dolt's version history plays no part in it.

Seen this way, most of what Dolt holds for us is not authority state at all. It is immutable content stored in a mutable engine. The biggest win is deleting that duplication, and the win is the same whichever engine survives.

The second reframe is about memory. Both panelists read our own measurements and found that the engine is not what fills memory. On the paper, the host sees about eight gibibytes. Nearly five of that is the guest's file cache, and over four of it is cold Dolt files that nobody will read again. The Go process holds most of the rest. Part of that is Dolt's table indexes, which cost about twenty-five mebibytes of heap for every gibibyte of live store, so they grow forever with history.

I checked one claim between rounds and corrected it. Devin said that guest page cache reclaim was proven, bringing a computer down to about one and a third gibibytes. Our own notes call that a single uncontrolled reading. Another test returned nothing at all. So we cannot yet make a guest give its file cache back to the host. That makes deleting the bytes the only lever we know works.

The third reframe was about the paper. A newspaper has a wire room and a reader's porch. The wire room takes in a firehose and is public. The porch is small and private. The paper's twenty-six gigabytes are a wire room problem. They belong in the corpus tier. They should not decide which engine a hundred thousand small personal computers use.

The fourth reframe was about zero downtime. Telephone networks and WebRTC services never migrate a live call. They drain old calls on the old version and start new calls on the new one. So zero downtime belongs to where a session is anchored, not to the database.

## The verdict, tier by tier

The convergent round produced two verdicts that agree on almost everything.

The tape is the only truth and the only thing that must be durable. A write counts as accepted when it is appended to the tape and a receipt comes back from off the host. Shipping a database's write-ahead log off the host, the way Litestream does, only speeds up restores. It is asynchronous and can lose the last few seconds, so it can never be the acknowledgement.

Each computer keeps a small authority store inside its own virtual machine. It holds runs, leases, lifecycle, routes and the head pointers for Texture documents. The projected engine is SQLite in write-ahead-log mode. It is not new to us: it already runs in every guest as the actor log. It has no garbage collector, its page cache has a fixed size, and its one writer matches our one writer per computer. The store stays in the guest because SQLite's write-ahead log cannot cross a virtual machine boundary without a server in between.

Immutable content goes to a content-addressed store on the host, encrypted per owner. Dedup happens only within one owner, and across owners only for public objects. That way nobody learns what someone else holds. Deleting an owner's data means destroying their key. It starts as a plain directory. Later it moves to a self-hosted object store, and both panelists prefer SeaweedFS to MinIO.

The paper keeps a small, ordinary authority store like any computer. Its bulk moves to the corpus tier: the tape as the log, content bodies in the object store, metadata and indexes in Postgres, with corpusd in front.

Shared platform state moves from the fifty-gigabyte Dolt server to Postgres. It records an audit log instead of committing every write. The hundred and twenty thousand commits of history get archived read-only, never converted.

Search starts in Postgres, with full text and the entity graph. Vectors come later through pgvector, which matches your Qdrant decision. Private search stays inside each computer.

A checkpoint becomes small: the tape head, a snapshot of the authority store, the projector version and a root hash. Firecracker snapshots stay what they are today, a fast way to hibernate, and never the checkpoint. Resuming a memory snapshot would resume work in flight, and our crash rule forbids that.

The panel also listed what it would not do. No distributed database yet. No custom storage engine. No message bus until fan-out is measured. No page deduplication between guests. No swap. And no leaving two homes for the same data after a migration.

## The path, and where the futures branch

You asked about the path integral, meaning the spread of likely futures rather than one plan. Here is how I see it.

Every path starts with the same steps, so they carry no risk of regret:

1. Finish the census, now with real memory shape: how much is cache and how much is heap, per file.
2. Turn on the mechanisms already built: run the crash drill for the debounced checkpoint, then widen it ring by ring.
3. Measure the platform squash phase by phase.
4. Finish moving trace lines out of the store.
5. Delete the duplicated event bodies from the tape index.
6. Move content-hash kinds into the object store.

Those steps should cut the paper's store by roughly half, and the gain does not depend on the engine.

Then the paths branch on measurement. The panel's main experiment is two by two: Dolt against SQLite, crossed with the old data model against the new one. It answers a question we have never been able to ask, which is whether the engine or the data model deserves the credit.

Three futures carry most of the weight. In the most likely one, SQLite wins once authority state is small, and the swap is cheap because there is little left to move. In a real second one, Dolt-small passes every gate, the swap waits, and we keep Dolt this quarter. That is a valid outcome, not a failure. In the third, the census shows authority state is not small after all. Then the authority model gets redesigned before any engine is chosen.

The panel named kill criteria that would move us between those futures. If authority state is over a gigabyte per computer, or over a fifth of the bytes, we redesign the authority model. If the engine accounts for less than a fifth of the win, we defer the swap. If a computer still sits above two gigabytes after deletion, the next lever is a thinner guest, not storage.

The two panelists disagreed on one thing: the order. Devin wanted to delete bytes in production before the experiment. DeepSeek wanted the experiment first. I sided with deleting first. Deletion does not depend on the engine, it is also the disk win, and the experiment can still measure the old model from a frozen copy of the tape.

## Risks worth holding

Both panelists named the same top danger in different words. The first is a migration that stalls halfway and leaves two homes for the data, which is the exact disease we are curing. The cure is to delete the old path in the same mission that adds the new one.

The second is a third kind of state that cannot be rebuilt from the tape. We know two already. The cure is to keep the comparison harness as a permanent test.

The third is narrower but real. SQLite has nothing like Dolt's per-table hash, so the cheap witness will have to be built, as a root maintained alongside each write.

The fourth is the paper's tape. Replaying it from the beginning would take about a day and a half, so a restore must always start from the newest base.

## Your one question

What does zero downtime mean for Autoradio? If it means a call never drops and a deploy causes at most a quarter-second gap and one invisible reconnect, then Firecracker is enough. We would anchor the audio relay at the edge of the host and drain sessions before a deploy. If it means a virtual machine never restarts at all, we would need live migration, and that means changing hypervisors to Cloud Hypervisor. The panel assumed the first. So would I.

The one thing worth remembering is this: the cure is deleting duplicated bytes, and that work can start now on the engine we already have. If I were holding the phone, I would answer the zero-downtime question with a single sentence. Then tomorrow I would start the census with memory shape, and the crash drill for the debounced checkpoint.

## Names and receipts

- Goal file: docs/definitions/choir-data-architecture-selection-2026-10-28.md
- Consensus record: docs/evidence/data-architecture-consensus-2026-10-28.md
- Raw panel output, gitignored: .agentic-consensus/data-arch/r1, r2 and r3, with FP0 reviews in r1-synthesis.md and r2-synthesis.md
- Memory evidence:
  - docs/problems/start-refused-for-memory-no-reclaim-2026-10-26.md
  - docs/problems/daemon-memory-res18-2026-10-22.md
  - docs/evidence/guest-memory-split-balloon-2026-10-14.md
- Texture parent chain: internal/store/texture.go line 1235
- SQLite already in each guest: internal/actorruntime/adapter.go line 105
- Off-switched mechanisms: store.debounced_checkpoint, store.trace_log, store.event_table, telemetry.batch
- Prior art the panel cited: Cloudflare SQLite in Durable Objects, Litestream tips, Fly Corrosion, Firecracker snapshot support and versioning docs, Cloud Hypervisor live migration
