# The big picture

Date: 2026-11-27. Status: **draft for the owner to cut down. It is not
authority.** The canonical sequence is still
[roadmap-2026-10-31.md](roadmap-2026-10-31.md), and what exists is still
[current-architecture.md](current-architecture.md). This file gathers a
two-day planning session (2026-11-26 and 27) into one place so the docs
transform can start from it. The owner's own words from the session are in
[inbox.md](inbox.md).

Each fact below says how it is known: **measured** (a command was run),
**read** (from a file, not run) or **not checked**.

## The system in four words

- A **computer** is a person's persistent machine: state, agents, history.
- A **host** is a machine that runs computers and services.
- A **service** is shared work that no single computer owns.
- A **client** is any way in.

| Word | Today | Wanted |
| --- | --- | --- |
| Computer | user computers, all on Node B | an organisation computer; a computer on a person's desktop |
| Host | Node B (production). Node A (a Nix builder, an agents VM, a runner VM and the off-host backup target, all declared in the Mike repository) | Node A as a Choir host; a third machine; a person's desktop |
| Service | Autopaper; podcast work; mail; the model gateway | a vault; pictures; transcription on a server; builders and landers as VMs the platform places |
| Client | the web desktop at choir.news; Autoradio on iPhone (App Store, 2026-11-25); Autoradio on Android (4 of 28 journeys proved, read); the `choir` command; `choir mcp`; the Mac app in `cmd/desktop` (a window and a one-way follower; it runs no VM, read) | Mike, as a chat client; outside agents (Claude Code, Codex) through the command and MCP |

A chatbot is one client among several (owner, this session): good for
debugging a computer and for using it as a tool, and not something to put
inside the web desktop or Autoradio.

## What is offered

The roadmap's direction of 2026-11-25 already states three offers: free
worldwide, paid consumer, organisation. This session adds where the compute
is, and a fourth row.

| Offer | Compute | Cost to us per person |
| --- | --- | --- |
| Free Autoradio | the person's phone | serving shared text and models |
| A computer in the cloud | our hosts | a VM, so it is paid |
| A computer on their desktop | their machine | replication and coordination |
| An organisation's own paper and agents | their platform or ours | paid |

The paid product, stated generally: a standing agent over a body of
material that keeps reports, analysis and signals up to date. Autopaper is
its public instance. Deep research on a podcast is a second instance.

**A gap in the free offer (read, not run):** today a voice turn needs an
account key (`TurnGate.swift`) and gets its answer from the server
(`POST /api/autoradio/turns`, `POST /api/system1/judgments`). A turn that
completes on the phone alone, for a person with no computer, is not built.
The onboarding interview depends on it. With no account, a free
listener's state (the queue, what was heard, what the interview learned)
lives only on the phone.

## What we arbitrage

| Gap | What others pay | What we pay |
| --- | --- | --- |
| Amortised research | every question researched from scratch, then discarded | once per item; the result joins the corpus and cheapens the next |
| Where compute runs | cloud voice and inference, per minute, per person | nothing; the person's device does it |
| Persistence | a sandbox rebuilt each session | a computer that is already set up and still running |
| Flat-rate subscriptions | per-token prices | the person's own subscription, through a harness acting as a desk |

The fourth depends on each provider's terms and is the fragile one
([the 2026-11-17 suspension](problems/antigravity-login-suspended-2026-11-17.md)).
The first three depend on nobody's permission.

## The test for order

**Does this make a whole class of things cheaper, or one thing possible?**
Class work sets the order. Single things are what people see; they ride on
the class work and are scheduled around it.

## Decided in this session

Each is the owner's, with the words in [inbox.md](inbox.md).

1. The docs transform comes before more code.
2. Then the move from Swift to Exact is completed and dead code deleted. No
   app fixes or features until it is. No user state is migrated.
3. No cloud voice models.
4. Android may write Kotlin that is later moved to Exact; parity first.
5. Review keeps agents from different providers, at least two for
   security-relevant changes.
6. Node A becomes a Choir host soon after the transform. Choir hosts run
   arbitrary VMs, not only computers.
7. Mike stays, as a chat client of a Choir computer.
8. Integrations wait; the rare core parts come first.
9. Publishing a voice take is for paid accounts only. Talking to the
   phone is free for everyone.
10. An account is for the Choir computer. A free listener has no account,
    and everything that runs on the device works signed out.
11. Plan for where the product is in a year, not only next week.
12. The Choir name is not dropped. The App Store strategy is to appear
    niche: an automated news app is approvable; an app that is an operating
    system, a store or an app generator is not.
13. Problem records are kept and improved, not scrapped. The docs to
   consolidate are theory, architecture and planning.

## The plan

### Now

1. **The docs transform**, in this order: the landing process; this plan
   and the inbox, replacing the roadmap, `ACTIVE.md`, the standing
   metamission, the mission graph and the standing questions; the
   architecture docs into one; the theory docs into one; a few small
   failing checks in place of `doccheck`.
2. **Android parity** continues in its own lanes.

### The unit of planning

The unit is the next App Store submission, about one a week. Each
submission is a real feature step, not a bug-fix release (owner,
2026-11-27).

**Autoradio 1, version 1.1:** the app made nice with the features it has,
deeper use of on-device models, and the onboarding voice interview. The
move to Exact finishes first, on the latest Exact. The docs transform is
its prerequisite and counts as part of 1.1.

**Version 1.2:** takes and social.

### Next, in order

1. **Finish the move to Exact.** About a day of agent work (owner). Update
   the Exact pin to the latest first. Design the on-phone turn and the
   onboarding flow into the voice turn code as it moves.
2. **Takes: recording and distributing them.** The owner's focus for the
   month, and the paid step: publishing a voice take is for paid accounts
   only, because voice needs moderation and moderation has a cost. The
   server side is deployed and proven on the drill account (2026-11-18,
   read); a recording made by a phone is not proven, and the only
   moderation built is an operator's removal.
3. **Onboarding.** Autoradio introduces itself, interviews the person and
   sets up the models for their device and account while it talks.
4. **Two vaults** (owner, 2026-11-27: "we need two vaults").
   - *The platform's secrets:* provider keys and host credentials, managed
     by the owner alone. Design exists (2026-10-22, not ratified). The owner
     counts the platform as staging, not production, until this is done.
   - *People's credentials,* ported from Mike to Go as a host service. A VM
     asks for its own owner's secrets; approval of a write comes from the
     owner's signed-in session, never from the VM. Token refresh is needed
     for OAuth sign-ins.
5. **Node A as a Choir host.** Before it: the backup target moves to a
   third location, Node A's system config moves out of the Mike repository,
   and the two workstations that hold account data are decided.
6. **Service VMs.** One more kind beside computers and leased leaf VMs:
   long-running, no tape. Builders, landers, transcription, MCP servers.
7. **Driving a computer from outside:** run a command, read and write
   files, snapshot. The command, MCP, Mike and Choir in Choir all need it.
8. **Landing.** One batched lander that runs the existing `scripts/land`
   logic on rented compute, with our own host as the exercised backup.

Items 4 to 8 are host-side Go and can run beside items 1 to 3, which are
the phone app. The shared limit is the owner's review attention.

### Later, unordered

- Pictures: one service, found (openly licensed, credit stored) and made
  (generated stills); text laid over the picture.
- Deep research on podcasts and any item, amortised into the corpus.
- Permalinks for public Autopaper articles (small).
- Search over Autopaper in Autoradio.
- An Autoradio explore feed.
- Podcast explore and recommendations; new episodes across many shows.
- An MCP client inside the computer, so official connectors (Google's
  hosted servers among them) work without a per-vendor integration.
- Import from Google Takeout.
- Any agent harness as a desk, through one adapter.
- A Mike host VM that uses each person's computer as its workstation; later
  the per-person parts of Mike inside the computer.
- A triage of Mike's upstream (362 commits behind as of 2026-10-30,
  measured from stale refs).
- The organisation computer.
- Social: every story, podcast and published piece branches into
  listeners' responses. The half-built op-ed feature is revisited here.
- An agentic recommender: the Autoradio desk in a paid person's computer,
  and a smaller version that runs on the phone alone.
- Moderation of published takes beyond an operator's removal.
- Deeper use of on-device models.
- More clients, on a one-year view: media production, live streaming, and
  hardware (an e-ink tablet running Autoputer with on-device models).
- Monitoring as a desk, replacing the present monitoring app, readable in
  Autoradio. Publish as much usage and performance data as is safe; decide
  what is the owner's and what is public.
- Zero-downtime deploys: two hosts for planned moves, three votes for
  automatic failover.
- The desktop as a host: VMs, large local models, computer use.
- Autoradio as a desktop app.

## Where this sits in the roadmap

The session did not contradict the roadmap; it filled in later stages.
Stage 4 (voice and Autoradio) reached the App Store. "More servers,
zero-downtime deploys" is stage 5. The offers are stage 7. "Images in
articles" is in stage 3. What is new is the order: the docs transform and
the move to Exact come first, and onboarding is named as the revenue step.

## Designs that already cover parts of this

Read these before writing anything new. Finding the first one late is what
showed the size of the docs problem.

- [design/resource-fabric-2026-10-31.md](design/resource-fabric-2026-10-31.md):
  sites, placement, a home per computer, the Mac as a writer, Choir in
  Choir, the native capability model, the organisation. The fabric model
  was accepted by the owner on 2026-10-31. It does not cover service VMs,
  a vault for people's credentials, rolling a platform release, or the
  free offer.
- [provider-vault-and-model-policy-design-2026-10-22.md](provider-vault-and-model-policy-design-2026-10-22.md):
  a vault for the platform's own provider keys (a proposal, not ratified).
  Mike's vault is for people's credentials. They are two vaults (owner,
  2026-11-27).
- `reference/AUTORADIO-TAKES.md` and the takes row of
  `current-architecture.md`: what is built for takes.
- `design/coding-agent-desk-over-mcp-2026-11-09.md`, with
  `internal/claudedelegate` and `cmd/choir-desk-mcp`: a harness as a desk,
  partly built.
- `reports/desktop-sync-and-local-vms-2026-10-14.md`: the desktop design
  (not read in this session).
- In the Mike repository (`~/Dev/mike`): `docs/vault.md` and `infra/vault`
  (about 1,960 lines of TypeScript, no dependencies), `infra/workstation`
  and `infra/node-a`, `computer-helper` and `computer-helper-rs`.

## Measured in this session

- **Size:** about 7,700 tracked files. Go 1,098,000 lines (567,000 product,
  531,000 test). Markdown 1,641 files and 4.4 million words. Swift 35,000
  lines (21,400 source, 13,100 test). 14,449 commits since 2026-04-10,
  3,699 of them in the last 30 days.
- **Docs by folder, in words:** archive 1.21 million, problems 0.71 million
  (490 files; 296 have no status line), design 0.70 million (145 files),
  evidence 0.43 million, definitions 0.38 million.
- **Swift that must stay native,** judged against Exact at its head on
  2026-11-26 (read, not run): about 5,000 to 6,000 lines. About 7,000 to
  8,000 more could be shared code. About 1,300 are dead or development
  only.
- **Exact gaps that keep code native:** no raw audio in or out of shared
  code; no reading a streamed POST response; no device memory or disk
  facts; downloads capped at 1 GiB with no resume.
- **Landing:** `scripts/land` is a hand-built serial queue tied to a VM on
  Node A; about 25 minutes per lane by its own comment. `main` is not
  protected.
- **CI:** of the last 40 runs on `main`, 25 passed (median 12 minutes) and
  15 failed, 10 of those in Vocabulary Gates.
- **`doccheck`:** 1,918 lines; its default mode reports and always exits
  0, so it gates nothing.
- **VM control:** `internal/vmctl` 24,100 lines and `internal/vmmanager`
  15,300, tests excluded; 53 lines in them mention Node B or a single
  host.

## Read by agents on 2026-11-27 (not run; counts approximate)

- **A second host is a rearchitecture of control, not of the VM.** One
  control process holds every VM in memory with one local file behind it; a
  computer has no host recorded; about 20 local state paths, none
  replicated; nothing stops two hosts booting one computer; the deploy is
  one 1,186-line script for one host. The host record, writer lease and
  host key of the fabric design are not in the code.
- **Service VMs are an extension.** A VM kind field exists with one value.
  The obstacle is boot, which requires a computer id, an owner, a
  credential envelope and a route record. They can come first, on Node B.
- **Landing.** Push to staging serving is 12.9 minutes at the median (8.1
  to 24.9, measured over 21 runs): 6.0 for CI and 6.8 for the deploy. The
  wait between queued lanes is a convention; the deploy already lets a
  newer commit supersede a queued one. Time before the push is not
  recorded anywhere; three failed runs show 10 to 22 minutes.
- **Vocabulary Gates** is about fifteen structural lints. Its ten failures
  on 2026-11-26 were one lint failing on an expiry date, not on a change.
- **Designs.** About 18 designs carry an owner ruling and still govern; 12
  are linked from no always-read doc. In `docs/design`,
  `docs/definitions` and the top level: 51 files govern (266,000 words),
  126 are built, 30 superseded, 27 undecided, 47 unsorted. There are three
  vault designs, a dozen storage designs and four SwiftUI-era app designs.

## Not known

- How long a landing spends before its push, step by step.

- What a turn costs the server for an account with no computer.
- Barge-in latency if the turn policy moves to shared code.
- Whether shared code keeps running on a locked phone. Read at Exact
  `8552d20f9` (2026-11-27): Exact does not freeze the page while the app
  holds a playback or speech claim, and nothing in that rule changed since
  our pin. Whether iOS keeps the process running is not measured by us or
  by Exact. A ten-minute locked-phone log settles it; the native deck's
  fallback rules stay until it is run.
- Whether `choir mcp` can run a command in a computer today.
- What memory a Choir VM allows, which decides whether Mike's backend fits
  inside one.
- Whether the tests hold usable end-to-end checks.
- Rights for showing another show's full transcript.
