# Survey 06 (verified): distributed and personal operating systems, whole-system precedents (2026-10-31)

Scope: prior art for "one computer, many machines", read against the brief's Q1-Q10 and the design docs
`resource-fabric-2026-10-31.md` (RF) and `branching-tape-2026-10-31.md` (BT). No final choice is recommended.

## Verification log

Tags used below: **[V url]** confirmed against a primary or official source (or a faithful mirror of it);
**[V-vendor url]** confirmed only by vendor/third-party documentation; **[CORRECTED]** original claim was wrong or
incomplete, fix and source given; **[UNVERIFIED]** I tried and could not confirm (what I tried is stated);
**[ADDED]** new entry. Untagged lines are interpretation/mapping to Choir (not factual claims about the system).
The original tags [F]/[M]/[?] are replaced. Counts are my tally of tagged claims, approximate.

| Outcome | Count |
|---|---|
| Confirmed [V] / [V-vendor] | ~38 |
| [CORRECTED] | 7 |
| [UNVERIFIED] | ~16 |
| [ADDED] entries | 4 (Condor, local-first software, Spritely/OCapN, Apple Private Cloud Compute) |

Most consequential corrections:
1. **Sprite (A3):** the survey said migration was "little used" and abandoned because of owner objections. The primary sources say the opposite: Douglis's thesis reports ~2 years of regular service, 20+ daily users, ~31% of all Sprite processing done by migration, 71% of hosts available by day; the real pain was implementation cost and kernel changes silently breaking a rarely-exercised feature. The "owners objecting" claim is not supported by what I could read.
2. **Amoeba (A2):** attenuation was described as always "a service call". In the sparse-capability scheme as published (Tanenbaum/Mullender 1986) a holder of an *owner* capability can derive a restricted capability **client-side** using a public one-way function, f(check XOR rights), without contacting the server; the server-side restriction is one of several variants described. The revocation weakness (cannot revoke one holder) is confirmed.
3. **Oz/Mozart (A7):** the fault model is `ok / tempFail / localFail / permFail` (four states, Mozart 1.4.0), reported through per-entity *fault streams*, with exceptions and handlers removed; synchronous operations on a failed entity *block*. `permFail` is only announced when the system can guarantee no site can use the entity. The survey's "fault-state exceptions" wording was wrong.
4. **LOCUS (A5):** merge is automatic for directories (name-conflict check) and mailboxes (version-vector merge); other files are *reported to the user*. The survey's "works only for few types" is directionally right but the mechanism is more specific.
5. **Urbit (A9):** the duplicate-ship warning is confirmed in the docs ("networking is stateful, you can't run two copies"), and persist-before-effects ordering is documented; the survey's "a crash can duplicate or lose effects" is **not confirmed** by any source I found (see A9, Q5).
6. **Microsoft 365 30-day (A21):** the 30-day default is real but applies to OneDrive access for a *manager/secondary owner when access delegation is configured*; it is not unconditional; deleted OneDrive then sits 93 days in the site-collection recycle bin.
7. **macOS PPPC (A20):** confirmed that Screen Recording (and Camera/Microphone) cannot be granted by MDM; Accessibility can be pre-approved. The "since 10.15/11" version claim is unverified.

Fetch caveats this pass: Emerald and Sprite PDFs were scanned/compressed (unreadable by the fetch tool); docs.urbit.org Ames/escape pages not reachable directly (used mirrors of official docs); Tailscale/Apple Platform Security pages partly reached via search summaries; Apple Platform Deployment pages not opened directly.

What RF/BT commit to, for reference: sites with signed site records and a trust-class relation between site principal and work owner (RF 1.1-1.2, default deny across people); one writer per tape branch, host not VM, lease with no expiry, home holds lease row, fencing before grant, no merge (publication/adoption instead) (RF 2, BT 0-1); a home per computer and a workstation outbound session (RF 1.3); controller grant for Choir in Choir (RF 4.2); native capability manifest with three gates (OS, owner, ring) and the tape as only audit (RF 5.2-5.3); org lifecycle largely unspecified, proposal 9 "employee's computer goes with the employee" (RF 7.3, 10). (Taken from the original survey; the design docs were not re-read in this pass.)

---

## Part A. Per-system notes

### A1. Plan 9 and Inferno (Bell Labs)
- **Attempted:** a network of machines feeling like one timesharing system: every resource a file, every process its own view. [V https://9p.io/sys/doc/names.html]
- **Core abstraction:** per-process namespaces built with `bind`/`mount`, union directories, 9P (17 message types, kernel mount driver); `cpu` re-creates the terminal's namespace on the CPU server and imports terminal-resident devices (screen, keyboard, mouse). Authors' stated limits: process creation, network addressing, shared memory are not in the namespace. [V names.html, as in original]
- Inferno: Limbo compiled to Dis bytecode; Styx (9P2000 relative); each application has its own hierarchical namespace, clonable independently, and namespace operations can restrict a process's access. [V https://doc.cat-v.org/inferno/historical_documents/website/namespace via search summary; Styx/9P2000 kinship per search result]
- **Factotum:** user-level file system acting as the user's authentication agent, holding keys as attribute=value sets; protocols p9any, p9sk1, dp9ik; consults `secstore` on the auth server at start. [V https://swtch.com/plan9port/man/man4/factotum.html]
- **Why not won:** no ecosystem; Unix/Linux free and good enough; rich APIs strained "everything is a file"; security via namespace + auth server, not a delegation algebra; 9P assumes a live connection. [UNVERIFIED: I found no source for the reasons-for-failure claims; these are interpretation.]
- **Mechanism vs policy:** bind/mount + one protocol = mechanism; user `profile` = policy. [interpretation]
- **Map:**
  - Q7: strongest precedent for native capability mediation (a device is a file served by its owning machine and imported by reference). Plan 9 has no per-use audit; RF uses the tape.
  - Q3: namespace-as-authority is attenuation by omission; revocation = unmount/drop fid.
  - Q2: LAN assumption, not addressed.
  - Q6: `cpu` leaves state on the file server and the remote CPU does write the user's files, the opposite of RF 4.2.

### A2. Amoeba (Vrije Universiteit)
- **Attempted:** transparent distributed OS: processor pool, one system image. [V https://users.soe.ucsc.edu/~sbrandt/221/Papers/Dist/mullender-computer90.pdf (title/abstract level via search); pool processors dynamically allocated: V per search summary]
- **Core abstraction:** sparse capabilities (port, object, rights, check field), user-space managed and cryptographically protected. The check field is a per-object random number stored by the server; a restricted capability's check is a one-way function of (stored check XOR rights); the server recomputes and compares; the owner capability (all rights) carries the raw check number. [V Tanenbaum & Mullender, "Using Sparse Capabilities in a Distributed Operating System" (1986), via https://homepage.divms.uiowa.edu/~jones/opsys/spring97/notes/37.html and typeset.io copy]
- **[CORRECTED]** Attenuation: in that scheme the one-way function is public, so an owner-capability holder can fabricate a reduced capability **without** asking the server; the paper describes four ways of handling rights, so "server-side restriction" is one variant, not the rule.
- **Worked:** unforgeable bearer tokens, user-space, small kernel; Bullet server = immutable-file service; pool placement of processes. [V Bullet/pool: https://fsd-amoeba.sourceforge.net/amoeba.html via search; "run server decides placement" UNVERIFIED]
- **Why not won / revocation:** a single holder cannot be revoked without changing the object's check number (revokes everyone). [V same Iowa notes: "Clients cannot easily be retracted"]. Custom-hardware/research-only reasons: UNVERIFIED.
- **Citation:** the survey's "Tanenbaum et al., Experiences with Amoeba, CACM 1990" could not be confirmed in search; the retrievable paper is Mullender, van Rossum, Tanenbaum, van Renesse, van Staveren, "Amoeba: A Distributed Operating System for the 1990s," IEEE Computer, May 1990. [CORRECTED citation]
- **Map:** Q3 (bearer capabilities, attenuation, revoke-all-or-none), Q4 (pool allocation).

### A3. Sprite (Berkeley)
- **Attempted:** network OS for workstations with a shared file system and transparent process migration to idle hosts. [V https://www2.eecs.berkeley.edu/Pubs/TechRpts/1990/6382.html]
- **Core abstraction:** a *home machine* per process; location-dependent kernel calls are forwarded to the home machine; migration moves running processes "at any time"; daemons detect idle hosts and trigger eviction when owners return; remote exec up to a few hundred ms, eviction typically a few seconds. [V same abstract; home-machine forwarding V via Sprite OS paper per search summary]
- Stateful servers with recovery by reconstructing state from clients: [UNVERIFIED in this pass: I did not retrieve Nelson/Welch/Ousterhout or Welch & Ousterhout; widely attributed but not opened.]
- **Worked:** parallel `pmake` speedups of ~3-6x (limited by file-server contention), CPU-bound simulations up to ~8x; regular service ~2 years, 20+ daily users; 31% of Sprite processing via migration; 71% of hosts available by day; evictions mostly on hosts that had only just become idle. [V Berkeley TR 6382]
- **[CORRECTED]** Original claim "migration was little used... owners objected" is not supported. The authors' own retrospective (Sprite status report, via search summary) says migration cost much more to implement than anticipated, and the greatest problem was kernel changes breaking migration because it was not used regularly enough to catch breakage; the greatest success was fully transparent preemptable remote execution in general use. [V-search-summary https://discmaster.textfiles.com/file/16861/Sprite 1984 - 1993.iso/papers/proc-mig-status.ps/proc-mig-status.pdf; full 1991 SP&E paper text NOT read (PDF unreadable to the tool): https://web.stanford.edu/~ouster/cgi-bin/papers/migration-spe.pdf]
- **Mechanism vs policy:** daemon-driven idle detection/eviction vs migration mechanism. The name `migd` for the central selection daemon is UNVERIFIED.
- **Map:** Q4 (idle harvesting, eviction on owner return: RF 1.2 same problem), Q2 (home machine as forwarding point for location-dependent state; residual dependency), Q1 (state reconstruction vs lease: UNVERIFIED above).

### A4. The V System (Stanford)
- Paper: Cheriton, "The V Distributed System," CACM 31(3):314-333, March 1988. [V https://cacm.acm.org/?p=10995 / scilit record]
- **Remote execution:** Theimer, Lantz, Cheriton, "Preemptable Remote Execution Facilities for the V-System," SOSP 1985: network transparency, preemption and migration of remotely executed programs, avoiding residual dependencies on the original host. [V https://synapsesocial.com/papers/W2050765519; https://none.cs.umass.edu/~shenoy/courses/fall04/readings/Theimer_vsystem.pdf]
- Pre-copy migration, multicast host selection, decentralised context-prefix naming, process groups ("Distributed process groups in the V Kernel," 1985): the specific mechanisms are [UNVERIFIED: citations exist, search returned no mechanism text; the paper PDF was not opened].
- **Map:** Q4 (decentralised host selection vs RF's management desk), Q2 (naming without a central server).

### A5. LOCUS (UCLA)
- **Walker, Popek, English, Kline, Thiel, SOSP 1983, pp. 49-70.** Each partition may update objects independently; inconsistency detected with version vectors; reconciliation by type: directories merged by name-conflict check, mailboxes merged by version-vector rule, **other files reported to users**; a three-stage repartition/merge/recover protocol. [V-search-summary https://pages.cs.wisc.edu/~zuyu/summaries/cs739/locus and https://carlstrom.com/stanford/quals/mirror/www-db.stanford.edu/~daswani/quals/Walker83%20-%20LOCUS.htm; full paper not opened]
- Parker et al., "Detection of Mutual Inconsistency in Distributed Systems," IEEE TSE, May 1983: version vectors; contrasts with voting/token schemes which keep consistency at the cost of availability; states the approach is used in LOCUS. [V https://pages.cs.wisc.edu/~remzi/Classes/739/Fall2016/Papers/parker83detection.pdf (abstract via search)]
- **[CORRECTED]** merge detail as above. "Shaped Coda/Ficus/Bayou" is interpretation, UNVERIFIED here.
- **Map:** Q1: the merge side of the argument; BT's no-merge is a deliberate contrast (interpretation).

### A6. Emerald (Jul, Levy, Hutchinson, Black; ACM TOCS 6(1), 1988)
- Fine-grained object mobility; primitives locate, move, fix, unfix, refix (refix = unfix, move, fix at a node), attach (bind objects so they move together); objects addressed indirectly by object ID. [V via course review pages https://courses.cs.washington.edu/courses/csep551/04wi/Messages/paper19/0016.html and slides https://www.cs.ubc.ca/~bestchai/teaching/cs538b_2020w1/slides/Emerald.pdf (search summaries)]
- Forwarding addresses then broadcast for location: [UNVERIFIED: search summaries only mention a location message-passing algorithm; the paper PDF (https://pages.cs.wisc.edu/~remzi/Classes/736/Spring2008/Papers/emerald.pdf) is a scanned bitmap the tool could not read]. Distributed GC / node-crash semantics: UNVERIFIED.
- **Map:** Q1/Q4 (move transfers a single object's ownership), Q2 (location-hint scheme: unconfirmed).

### A7. Oz / Mozart
- **[CORRECTED]** Failure model: entity fault states are `ok, tempFail, localFail, permFail` (Mozart 1.4.0). Each site keeps a **fault stream** per entity recording local fault history; transitions notified by dataflow; watchers built over streams. `tempFail` may last forever; the system announces `permFail` only with a guarantee that all sites are unable to use the entity. Synchronous operations on a failed entity simply block; exceptions/handlers were discarded, so an application that ignores failure keeps running. The older `Fault` module is obsolete (replaced by `DP`). [V https://www.mozart-oz.org/documentation/system/node52/ , https://www.mozart-oz.org/?p=130, UCL papers https://webperso.info.ucl.ac.be/~pvr/failure2.pdf]
- Per-entity distribution protocols (stationary, mobile via token passing, replicated), "network transparency with network awareness": [UNVERIFIED in this pass beyond the failure-model pages; consistent with the white paper https://mozart.github.io/mozart-v1/doc-1.4.0/ds_white_paper/node5.html which I did not open.]
- **Map:** Q1: `permFail` only after a guarantee is closer to a *fenced* death declaration than to a lease timeout; `tempFail` forever is the "holder unreachable, undecided" state. Q9 (per-type protocol choice).

### A8. Smalltalk images; Croquet / TeaTime
- Smalltalk image as single persistent artifact: [UNVERIFIED, from memory; not looked up.]
- Croquet: replicated deterministic computation; replicas deterministic so identical inputs give identical results; the **reflector** is a stateless message-passing service mirroring each user's events to all session members; claimed bit-identical across Linux/Windows/Mac; TeaTime described as coordinated two-phase commit controlling progress at multiple sites. [V-search-summary https://lively-kernel.org/lively4/aexpr/demos/swd21/croquet/presentation/index.md ; https://oopsla.org/2005/ShowEvent2455.html ; https://en.wikipedia.org/wiki/Croquet_Project]
- Original claim "reflector orders and timestamps events" is plausible but the sources found say "mirrors"; timestamping detail UNVERIFIED. Citation "Smith, Kay, Raab, Reed, Croquet: A Collaboration System Architecture, C5 2003": search could not find it. [UNVERIFIED]
- Successor Multisynq/Croquet.io: https://croquet.io/origin (existence only).
- **Map:** Q1 (sequencer as single writer for ordering), Q5 (external effects outside the deterministic core: interpretation), Q9 (deterministic replay).

### A9. Urbit
- **Life and rift:** `$life` = count of networking-key changes; `$rift` = continuity number = count of breaches; a breach ("broken continuity") usually also changes keys so both increment. [V https://docs.urbit.org/urbit-id/life-and-rift.md]
- **Two-copies hazard:** docs state Urbit networking is stateful and you cannot run two copies of the same ship in two places; a "continuity breach" results and the breached pier keeps working but won't connect to the network; ship-troubleshooting says "maybe you booted your ship twice, or ran it using old files"; fix = personal breach (the ship tells the network to treat it as new). [V-mirror https://urbit-org-iorxds0k4-urbit.vercel.app/using/os/getting-started ; https://urbit-org-3oir6rgul-urbit.vercel.app/using/os/ship-troubleshooting ; /using/id/guide-to-breaches]. A GitHub issue proposes a boot-time "have you started this ship elsewhere" confirmation, i.e. no enforcement mechanism exists (proposal, https://github.com/urbit/urbit/issues/2963).
- **Sponsors:** sponsorship is a voluntary arrangement either side can sever; stars provide peer discovery; when a sponsor cannot serve its children they *escape* to another sponsor; routing falls back ship IP -> sponsor IP -> sponsor's sponsor (2016 essay; may not match current code); Layer-2 sponsorship rules apply. [V-mirror https://operators-urbit-org-git-m-dns-deg-urbit.vercel.app/guides/running-a-star ; https://ustj-urbit-org.vercel.app/ustj/v03-i01/mss2.html ; https://urbit-qp5qoxu57-urbit.vercel.app/docs/azimuth/l2/layer2]
- **Event log:** the event must be persisted before its effects run; on restart the ship replays the log, with snapshots shortening replay; full replay from event 0 can break on jet mismatches and Vere proposed log *epochs*. [V https://docs-urbit-942n67ai3-urbit.vercel.app/courses/core-academy/ca06 ; https://github.com/urbit/vere/issues/316]
- **[CORRECTED]** The survey's "outputs after persistence, so a crash can duplicate or lose" is not confirmed: sources say persist-then-effects; what happens to effects of a persisted event after a crash before release was NOT found. Treat as open.
- Key-role separation (ownership/management/transfer/voting proxies), Azimuth on Ethereum, galaxy>star>planet: [UNVERIFIED in this pass; docs.urbit.org Azimuth pages not opened.]
- **Map:** Q1 (rift bump vs BT equivocation proof: related not identical), Q2 (sponsor routing + escape), Q3, Q5 (persist-then-effects ordering is the tape-then-outbox shape), Q8.

### A10. Solid pods
- **WAC:** subjects `acl:agent`, `acl:agentGroup` (vcard:Group), `acl:agentClass` (foaf:Agent, acl:AuthenticatedAgent), `acl:origin` ("the origin of an HTTP request that is being given the access permission"; only applies when an Origin header exists and doesn't grant everyone alone); modes Read, Write, Append (subclass of Write), Control; inheritance via `acl:default` from nearest container. [V https://solid.github.io/web-access-control-spec/]
- **ACP:** policies combine matchers with allOf/anyOf/noneOf; spec version 0.9.0, editor's draft at solid.github.io/authorization-panel/acp-specification. [V-search-summary https://docs.inrupt.com/guides/access-control-policies]
- Reasons not won, no offline/multi-device writer: [UNVERIFIED, interpretation.]
- **Map:** Q3 (resource ACLs; no delegation chains or attenuation in WAC itself; `acl:origin` is a carrier check alongside user), Q2, Q8.

### A11. Upspin [V as originally fetched: https://upspin.io/doc/overview.md]
Unchanged from original: key / directory / store servers; names `user@domain/path`; nothing shared by default; `Access` files; suffixed users as sub-identities. E2EE per-file key wrapping detail: [UNVERIFIED, not re-read]. "Never left Google side-project status": interpretation.

### A12. Perkeep [V as originally fetched: https://perkeep.org/doc/overview]
Unchanged: immutable content-addressed blobs; mutable objects as signed timestamped claims applied in time order; index derived; sync without conflict. "No access control beyond single owner's server auth": [UNVERIFIED.]

### A13. Apple Continuity, Handoff, iCloud Keychain
- **Circle of trust:** a new device generates its own key pair and is admitted by approval from a device already in the circle; the approved public key is signed and added to the circle on each device and in iCloud. [V-secondary https://tidbits.com/2014/03/01/how-to-protect-your-icloud-keychain-from-the-nsa/ ; Apple Platform Security guide https://support.apple.com/guide/security/sec3e341e75d/web (search summary)]
- **Escrow recovery:** HSM clusters guard escrow records; recovery needs iCloud account + password, an SMS to the registered phone, then the iCloud security code; if a majority of the cluster agrees it unwraps the record; **10 attempts**, after which the HSM cluster destroys the escrow record and the keychain is lost forever; policies are in HSM firmware. [V Apple Platform Security guide via search summary] (Original said "attempt limits": confirmed, with the destroy-on-10th consequence.)
- Cloud Key Vault, Advanced Data Protection, recovery contacts, Handoff mechanics (advertised activity + payload), BLE/AWDL discovery, TCC: [UNVERIFIED in this pass.]
- **Map:** Q2/Q3/Q7/Q8 as original.

### A14. Android and ChromeOS
- **Android permissions:** if an app targets Android 11 (API 30)+ and is unused "for a few months", the system auto-resets its sensitive runtime permissions; **one-time permissions** ("Only this time") for location, microphone, camera since Android 11; revoking a one-time permission terminates the app's process. [V https://developer.android.com/training/permissions/requesting]
- **Work profile:** controlled by an IT admin with functionality set separately from the primary profile; admin chooses allowed apps and device features; by default most intents don't cross profiles and the admin sets the policy; separate storage (file URIs don't cross); NotificationListenerService not usable from the work profile; personal-profile apps can listen to work notifications only if the admin allowlists them. [V https://developer.android.com/work/managed-profiles]
- Fully managed, COPE, dedicated devices, "admin cannot read personal profile; wipe only the work profile": [UNVERIFIED: the page fetched does not cover them; general knowledge only.]
- **ChromeOS** (OU policy, stateless devices, forced re-enrollment, verified boot): [UNVERIFIED, not looked up.]

### A15. Tailscale
- **ACLs/grants:** default-deny, rules distributed to every node which enforces incoming connections; `tagOwners` define which users may assign each tag; **grants** are the newer, recommended mechanism over ACLs. [V https://tailscale.com/kb/1018/acls ; https://tailscale.com/kb/1337/policy-syntax]
- **Policy tests:** the policy file supports `tests` (accept/deny assertions for grants and ACLs) and `sshTests`; both run when the policy changes and a failing assertion makes Tailscale **reject the updated policy**. [V https://tailscale.com/kb/1337/policy-syntax] (Confirms and strengthens the original [M].)
- **Tailnet Lock:** a new node's public node key requires a signature from a Tailnet Lock key; peers verify before connecting; even a compromised coordination server cannot add nodes that can send/receive; trust starts from the initial state each node receives (verify with `tailscale lock status`); disabling needs a disablement secret; lost secrets can make the tailnet unrecoverable. [V https://tailscale.com/kb/1226/tailnet-lock]
- **DERP:** "Designated Encrypted Relay for Packets": helps devices find each other and traverse NAT, and is the last-resort relay; "blindly forwards already-encrypted traffic"; most connections use DERP only to establish a direct connection; a **peer relay**, if available, is tried before DERP. [V https://tailscale.com/kb/1232/derp-servers] (peer relays are newer than the original survey.)
- Key expiry, MagicDNS, SSH policy, Taildrop, "coordination server is SPOF for changes": [UNVERIFIED in this pass.]
- **Map:** unchanged: Q2 primary, Q3, Q8, Q9.

### A16. Fuchsia component framework
- Capability routing via manifests (`capabilities`, `use`, `offer`, `expose`), absent capability if no valid route, parents define children's sandboxes, `weak` for cycles. [V https://fuchsia.dev/fuchsia-src/concepts/components/v2/capabilities (as originally fetched)]
- **Build-time verification:** the `verify_routes` GN template checks every `use` of protocols/directories has a valid expose-offer-use chain and that directories get correct rights, via `ffx scrutiny verify routes`, with allowlist/exceptions files; component_manager policy restricts which monikers a listed protocol may route to; scrutiny checks policy enforcement at build time for build types with the verifier enabled. Caveat: some snapshots describe the older v1 (appmgr) policy, and v1 allowlists lack compile-time verification. [V-search-summary https://fuchsia.googlesource.com/fuchsia/+/57ef8eaaff96ca15cebfbd2e5605521ab24317e6/build/security/verifier/verify_routes.gni ; https://fuchsia.googlesource.com/fuchsia/+/63df7289f45f648c62c2fd43a5092a321fce404b/src/security/policy ; current tree behaviour NOT checked]
- `zx_handle_duplicate` with reduced rights as attenuation: [UNVERIFIED: the handle reference page URL 404'd.]
- "Shipped on Nest Hub only": [UNVERIFIED.]

### A17. Unikernels [UNVERIFIED entirely]
Not looked up; retained as background from the original survey (MirageOS/IncludeOS/Unikraft/OSv; Madhavapeddy et al., ASPLOS 2013 citation unchecked). Low relevance to Q1-Q3.

### A18. Qubes OS
- Qube = Xen VM; templates; disposables; service qubes; colored borders; Split GPG; "confine, control, contain". [V https://doc.qubes-os.org/en/latest/introduction/intro.html (as originally fetched)]
- **qrexec policy:** each rule line has service, optional argument (`+ARG` or `*`), source VM, destination VM, action; **lines are checked in order, first match decides**; actions `allow`, `deny`, `ask` (prompts the user in dom0); **"If no policy rule is matched, the action is denied."** [V https://doc.qubes-os.org/en/latest/developer/services/qrexec.html] (Original said "per service, default deny": confirmed, plus first-match semantics.)
- dom0 has no network access: [UNVERIFIED in this pass.]

### A19. Nix / NixOS and Guix
- **NixOS module priorities:** a definition can override others by priority, lowest value wins and others are discarded; `mkForce` = `mkOverride 50`, `mkDefault` = `mkOverride 1000`; default priorities differ by release (older text: option definitions 1000; newer: definitions 100, option defaults 1500); `mkIf` pushes the condition down to individual definitions (avoids infinite recursion); `mkMerge` merges several definition sets; `mkBefore`/`mkAfter` order list options. [V-mirror https://alpha.tangled.sh/pyrox.dev/nixpkgs/blob/master/nixos/doc/manual/development/option-def.section.md ; check manual of your release]
- **Guix channel authentication:** authorized keys listed in `.guix-authorizations` in the repo; a channel *introduction* (commit that first added the file + OpenPGP fingerprint of its signer) bootstraps trust, verified on first `guix pull`/`time-machine`; merge commits authentic iff signed by a key in `.guix-authorizations` of both branches; keys supplied as `.key` files, default branch `keyring`; `guix git authenticate` for non-Guix repos. [V https://guix.gnu.org/manual/en/html_node/Specifying-Channel-Authorizations.html ; https://guix.gnu.org/blog/2020/securing-updates ; https://guix.gnu.org/de/blog/2024/authenticate-your-git-checkouts]
- NixOps/Colmena/deploy-rs/morph, `nixos-rebuild build-vm`, NixOS multi-VM tests, generations as GC roots: [UNVERIFIED, from memory.]

### A20. Corporate device management (MDM, Intune, Jamf, BYOD)
- **Apple User Enrollment:** creates a separate managed APFS volume with separate cryptographic keys destroyed with the volume at unenrollment; admin cannot wipe the whole device or set/clear passcode; managed-space data only (managed apps, enterprise Notes/iCloud Drive/Keychain, managed mail, calendar attachments). Contrast: supervised (DEP/Automated Device Enrollment) lets the admin wipe/lock. [V-vendor https://simplemdm.com/apple-user-enrollment/ , https://help.ivanti.com/mi/help/en_US/CORE/11.x/dmgi/DMGfiles/User_Enrollment_Apple_Biz_Manager.htm ; Apple's own page not opened; some vendor text describes iOS 13-era behavior]
- **Declarative Device Management:** device acts autonomously on its own state changes; sends status reports for subscribed items (initial then changes); status of declarations always reported; status items usable in activation predicates; reports are per channel. [V WWDC22 10046 https://developer.apple.com/videos/play/wwdc2022/10046 and Apple Platform Deployment https://support.apple.com/guide/deployment/depb1bab77f8 (search summaries)]
- **macOS PPPC:** Screen Recording, Camera and Microphone cannot be granted by MDM (restrict or leave user-controlled); Accessibility can be pre-approved; `AllowStandardUserToSetSystemService` lets standard users approve ListenEvent/ScreenCapture. One Intune thread claims a custom `.mobileconfig` set Screen Recording to Allow: anecdotal, conflicts with vendor docs. [V-vendor https://www.manageengine.com/mobile-device-management/help/profile_management/mac/mdm_mac_pppc.html ; https://docs.iru.com/en/endpoint/library/library-items-profiles/configure-the-privacy-preferences-policy-control-pppc-library-item ; https://help.swif.ai/en/articles/11103442-macos-privacy-preferences-pppc-policy ] The OS-version cutoff (10.15/11) is [UNVERIFIED]; Apple's PPPC payload reference not opened.
- Android Enterprise modes, Windows Intune MDM/MAM/Autopilot, Jamf smart groups, conditional access (Entra, Google context-aware, Okta Device Trust): [UNVERIFIED from memory; not looked up.]
- APNs/FCM/WNS as wake path: [UNVERIFIED]; DDM sources do confirm the status-report architecture.
- **Map:** Q8 (container as unit of wipe is supported by the User Enrollment volume and Android work profile evidence above), Q3, Q4/Q7, Q2.

### A21. Enterprise identity lifecycle: SCIM, JML
- **SCIM RFC 7644** [V https://datatracker.ietf.org/doc/html/rfc7644 as originally fetched]: unchanged.
- **`active` attribute (RFC 7643 §4.1.1):** a Boolean "indicating the user's administrative status"; "The definitive meaning of this attribute is determined by the service provider" (typically true = can sign in, false = suspended). [V https://www.rfc-editor.org/rfc/rfc7643.html] So deprovisioning semantics are provider-defined, as the original suspected.
- **Microsoft 365 / OneDrive:** a manager or secondary owner can be given access to a deleted user's OneDrive for 30 days by default (configurable in SharePoint admin center); the default retention period is also 30 days; reminder email 7 days before expiry; then recycle bin for 93 days; whether the manager gets automatic access depends on the delegation setting; Microsoft 365 retention policies/labels take precedence. [V https://learn.microsoft.com/en-us/OneDrive/retention-and-deletion] **[CORRECTED]** from unconditional "manager gets access for 30 days".
- Google Workspace Drive ownership transfer, GitHub org ownership, JML/access creep description: [UNVERIFIED, from general knowledge.]

---

## Part A2. Added prior art

### A22. Condor (Litzkow, Livny, Mutka, ICDCS 1988) [ADDED]
- "Condor - A Hunter of Idle Workstations," 8th IEEE ICDCS, pp. 104-111: scheduling system for a workstation environment, aim of maximizing workstation utilization with as little interference as possible with owners' activities; guarantees the job eventually completes and little if any work is performed more than once; performance data from 23 stations over one month; workstation owners have complete priority over their machines; when the owner resumes, the remote job is checkpointed and moved. (The checkpoint-on-owner-return detail is from a secondary abstract.) [V-secondary https://minds.wisc.edu/items/c30eb149-ee84-425e-9139-e34759a11f6d ; https://research.cs.wisc.edu/htcondor/doc/tech.text ; ICDCS High Impact Paper Award 2020 https://www.cs.wisc.edu/2020/07/24/miron-livny-awarded-2020-ieee-award-for-outstanding-technical-achievement-and-2020-high-impact-paper-award/]
- **Map:** Q4 directly: owner priority + eviction + checkpoint is the long-lived (still in production as HTCondor) counterexample to the idea that idle-harvesting failed. Q5 adjacent ("little work performed more than once" is at-least-once with bounded redo, for pure computation).

### A23. Local-first software (Kleppmann, Wiggins, van Hardenberg, McGranaghan; Ink & Switch 2019) [ADDED]
- Seven ideals: no spinners; work not trapped on one device; network optional; seamless collaboration; the Long Now; security and privacy by default; you retain ultimate ownership and control. CRDTs presented as a possible foundation (sync over any channel incl. servers, P2P, USB; "multi-user from the ground up while also being fundamentally local and private"); servers as optional "cloud peers" rather than source of truth; network communication noted as still unsolved in practice. [V https://www.inkandswitch.com/essay/local-first/]
- **Map:** Q1 (the explicit merge/CRDT pole against which BT's branching-and-adoption is a position), Q2 (relay as cloud peer), Q8 (ownership by the user).

### A24. Spritely Goblins / OCapN / CapTP [ADDED]
- OCapN is a suite with CapTP (Capability Transport Protocol) at its core, runnable over different netlayers (Tor, I2P, libp2p); Spritely's Goblins has the most complete CapTP pieces (distributed acyclic GC, handoffs); draft specs warn they are likely to change; revocation is described as a programmable pattern over references, not a protocol primitive; a sturdyref nonce/signature scheme was proposed on cap-talk but status unconfirmed. [V-secondary https://github.com/ocapn/ocapn ; https://groups.google.com/g/cap-talk/c/zGu5th2Lsxc ; official draft specs NOT opened]
- **Map:** Q3 (distributed object capabilities with third-party handoff; delegation by reference passing), Q6.

### A25. Apple Private Cloud Compute [ADDED]
- Node root of trust is Apple silicon (Secure Enclave, Secure Boot); a device sends nothing to a PCC node whose certificates it can't validate and encrypts requests only to nodes whose attested measurements match a release in the public transparency log; request data held only until the response is sent, no retention; volume keys randomized each reboot and not persisted; no remote shell, no general-purpose logging, only structured audited logs; "target diffusion" (OHTTP relay hides device IP, load balancer without user info, device encrypts to a subset of nodes) so requests can't be routed to targeted nodes; publishes images for production builds in an append-only transparency log. [V https://security.apple.com/blog/private-cloud-compute/]
- **Map:** Q4 (admission by *client-side verification of the executor's attested software*, a reverse of the usual "host vets the workload"), Q7, Q9 (transparency log for what runs).

---

## Part B. Cross-cutting findings (revised)

Pattern 1 — Transparency lost to partial failure (Oz, Sprite, Emerald, LOCUS, Plan 9). Oz is the explicit case: failures are named states on per-entity streams, and operations block rather than throw. [V for Oz; others interpretation]

Pattern 2 — Single-writer token + explicit death/handoff vs merge. LOCUS/Parker (merge by type, user-reported otherwise), local-first/CRDT (merge always), Emerald/Oz (exclusive object/token ownership), Urbit (no two live copies; breach), Perkeep (never mutate). Choir's branches + primary-ref sits near Urbit rift + Perkeep claims + Emerald single-owner (interpretation). Urbit shows the cost of *not enforcing* single-copy: documented warning and a proposed prompt only.

Pattern 3 — A home/sponsor/coordination server as replaceable control plane: Sprite home machine, Urbit sponsor (escape), Upspin key server, Tailscale coordination + DERP (+ peer relays), MDM + APNs, iCloud. [V for Sprite, Urbit, Tailscale, Upspin; MDM/APNs unverified]

Pattern 4 — Authority as a graph assembled by the parent vs bearer tokens vs lists: Plan 9, Fuchsia, Qubes (first-match rules, default deny, `ask` in dom0), Android profile intents; Amoeba (bearer, revoke-all-or-none), Solid (lists/matchers), OCapN (reference capabilities).

Pattern 5 — Wipeable unit = separately keyed container: Apple User Enrollment volume (keys destroyed with volume) [V-vendor], Android work profile (separate storage and admin policy) [V], Tailscale key expiry [UNVERIFIED].

Pattern 6 — Capability mediation by a broker on the machine that owns the physical thing: Plan 9 `cpu`, Qubes service qubes, Android one-time permissions and auto-reset [V], macOS TCC (MDM cannot grant Screen Recording [V-vendor]).

Pattern 7 — Configuration as data with static checks: Nix module system [V], Guix channel authorization [V], Fuchsia verify_routes/scrutiny [V-search-summary], Tailscale policy tests that reject a failing policy [V]. Runtime verification family: MDM compliance, DDM status reports [V], Apple PCC transparency log [V].

Pattern 8 [ADDED] — Admission by verifying the *executor*: Apple PCC (client checks attested measurements against a transparency log), Tailnet Lock (peers check signatures the control plane cannot forge), Guix (client checks signed history). Contrasts with host-side vetting of workloads.

Why most did not win: reasons are interpretation except where evidence above contradicts them; **Sprite is a counterexample to "idle-harvest migration failed"**, as is Condor.

Mechanism vs policy summary (as original, Sprite row corrected):
| System | Mechanism (small) | Policy (data) |
|---|---|---|
| Plan 9 | bind/mount, 9P, rfork | per-user profile |
| Sprite | migration, home-machine forwarding, eviction | idle-host selection daemon |
| Amoeba | sparse capability + RPC | rights issuing |
| Tailscale | WireGuard keys, DERP, netmap, tailnet lock | grants/ACL file with tests |
| Fuchsia | handles + component routing | manifests, policy allowlist |
| Qubes | Xen VMs, qrexec | ordered qrexec rules |
| Nix/Guix | store, derivations, module system | the configuration |
| MDM | enrolment, attest, container, DDM | profiles, declarations |
| Urbit | event-log VM, Ames, Azimuth | sponsor choice, apps |

---

## Part C. Mapping to the brief's questions (no recommendation)

**Q1 Writer authority.**
- Exclusive ownership plus explicit breach/death: Oz token/`permFail` (announced only on guarantee), Urbit rift/breach, Emerald object ownership, Sprite home authority. Merge side: LOCUS (merge by type), local-first/CRDTs, Perkeep (claims).
- Urbit: duplicate-instance hazard handled by *public epoch bump* after the fact; no prevention (docs warn; boot-prompt proposed). BT handles by signed equivocation proof (BT §3.3, per original).
- Croquet: the reflector is a stateless sequencer; replicas do not write.
- Sprite state reconstruction at the server: [UNVERIFIED].

**Q2 Reachability.** (1) Home as rendezvous + relay: Tailscale DERP (relay sees only ciphertext [V]), Urbit sponsor, Sprite home forwarding. RF 1.3's home terminating HTTPS is the open trust point. (2) Direct path with NAT traversal and relay fallback: Tailscale [V]. (3) Vendor push wake path: [UNVERIFIED]. (4) Home failover: Urbit escape [V]; Upspin/Solid pointer update [unverified]. (5) Identity-not-address: Tailscale, Urbit, Upspin.

**Q3 Access.** Spectrum: resource ACL/matcher (Solid WAC/ACP) -> identity/tag default-deny enforced at edge (Tailscale) -> parent-assembled capability graph (Plan 9, Fuchsia, Qubes) -> bearer (Amoeba) / reference capabilities (OCapN). Attenuation mechanisms: Amoeba (public one-way function; client-side from owner cap), Plan 9 bind subset, Upspin child Access replaces parent [original V], Android cross-profile allowlist [V], Fuchsia (unverified detail). Carrier checks: Solid `acl:origin` [V] and Android cross-profile [V] check user and carrier, RF's site-principal x work-owner shape. Delegation chains with caveats (macaroons, UCAN, SPKI): not covered here.

**Q4 Placement.** Sprite and Condor (idle harvesting, owner priority, eviction/checkpoint) [V]; V (decentralised host selection, mechanism unverified); Amoeba pool; Emerald pin/attach; Apple PCC (executor attestation). Lesson revised: owner priority and eviction are central design requirements, not evidence of failure.

**Q5 Irreversible effects.** Weak coverage. Urbit persists the event before running its effects [V]; behavior of unreleased effects after a crash is unconfirmed. Croquet keeps effects outside the deterministic core (interpretation). Condor: bounded redo for pure computation [V-secondary].

**Q6 Controlling other computers.** Plan 9 `cpu`; Fuchsia parent-defines-sandbox; Qubes dom0 commands qubes; Spritely handoffs; MDM/DDM: device evaluates declarations autonomously and reports status [V], closest to "A proposes, B admits".

**Q7 Native capabilities.** Plan 9 device import; Qubes ask/allow/deny with default deny [V]; Android one-time permission and auto-reset [V]; macOS TCC with MDM unable to grant screen/camera/mic [V-vendor]; Fuchsia routing; Apple Continuity (unverified detail). The OS gate stays outside the system's control, so a manifest must report truthfully (RF).

**Q8 Org lifecycle.** MDM container split [V-vendor, Android V], SCIM `active` semantics provider-defined [V], M365 manager-access window conditional [V], Tailscale key expiry [unverified]. Open questions (unchanged from original): which side of the container boundary a *computer* lies; org "disable" primitive on a computer it doesn't own; revoking external credentials the computer holds; retention/legal hold; mover re-evaluation; grace period.

**Q9 Mechanism vs policy; verifying configuration spaces.** Fuchsia verify_routes/scrutiny, Tailscale tests that reject failing policies [V], Nix module typing, Guix channel authorization [V], Qubes small admin domain, Croquet determinism, Apple PCC transparency log. None model-check distributed interleavings; formal methods covered in other surveys.

**Q10 Self-tuning within bounds.** A declarative envelope the modifier cannot edit (dom0 policy, manifest allowlist, org policy) with changes as rollbackable versions: Nix generations [from memory], Fuchsia, Android auto-reset [V], DDM autonomous evaluation under server-set declarations [V].

---

## Part D. Remaining unverified items

Smalltalk image claims; Croquet C5 2003 citation and reflector timestamping; Emerald forwarding addresses, GC, failure semantics; V pre-copy/multicast mechanism; Sprite state-reconstruction recovery and `migd` name; Sprite 1991 SP&E full text; Amoeba "run server" and CACM citation; Urbit Azimuth/proxy roles/Ethereum, post-crash effect handling; Fuchsia `zx_handle_duplicate`; ChromeOS; Android fully-managed/COPE; Windows Intune/MAM/Autopilot, Jamf smart groups, conditional access; APNs wake path; Apple PPPC OS-version cutoff, Handoff/Continuity details; unikernels (all); Google/GitHub ownership transfer; Nix fleet tools; Tailscale key expiry and SPOF claim; Solid/Upspin/Perkeep reasons-not-won.

## Primary sources (opened or search-summarised this pass)

- Plan 9 names: https://9p.io/sys/doc/names.html ; factotum https://swtch.com/plan9port/man/man4/factotum.html ; Inferno namespaces https://doc.cat-v.org/inferno/historical_documents/website/namespace
- Sparse capabilities: https://homepage.divms.uiowa.edu/~jones/opsys/spring97/notes/37.html ; Amoeba 1990: https://users.soe.ucsc.edu/~sbrandt/221/Papers/Dist/mullender-computer90.pdf
- Sprite: https://www2.eecs.berkeley.edu/Pubs/TechRpts/1990/6382.html
- V: https://cacm.acm.org/?p=10995 ; https://none.cs.umass.edu/~shenoy/courses/fall04/readings/Theimer_vsystem.pdf
- LOCUS/Parker: https://pages.cs.wisc.edu/~remzi/Classes/739/Fall2016/Papers/parker83detection.pdf ; https://pages.cs.wisc.edu/~zuyu/summaries/cs739/locus
- Oz: https://www.mozart-oz.org/documentation/system/node52/ ; https://www.mozart-oz.org/?p=130 ; https://webperso.info.ucl.ac.be/~pvr/failure2.pdf
- Urbit: https://docs.urbit.org/urbit-id/life-and-rift.md ; mirrors listed in A9
- Solid WAC https://solid.github.io/web-access-control-spec/ ; ACP https://docs.inrupt.com/guides/access-control-policies
- Tailscale: kb/1018/acls, kb/1226/tailnet-lock, kb/1232/derp-servers, kb/1337/policy-syntax
- Qubes: https://doc.qubes-os.org/en/latest/developer/services/qrexec.html
- Fuchsia routes: https://fuchsia.googlesource.com/fuchsia/+/57ef8eaaff96ca15cebfbd2e5605521ab24317e6/build/security/verifier/verify_routes.gni
- Guix: https://guix.gnu.org/manual/en/html_node/Specifying-Channel-Authorizations.html ; NixOS option-def (mirror above)
- Android: https://developer.android.com/training/permissions/requesting ; https://developer.android.com/work/managed-profiles
- Apple: https://support.apple.com/guide/security/sec3e341e75d/web ; https://security.apple.com/blog/private-cloud-compute/ ; https://developer.apple.com/videos/play/wwdc2022/10046
- RFC 7643 https://www.rfc-editor.org/rfc/rfc7643.html ; RFC 7644 ; OneDrive retention https://learn.microsoft.com/en-us/OneDrive/retention-and-deletion
- Condor, local-first, OCapN as in A22-A24.
- Retained from original [V as originally fetched]: Upspin overview, Perkeep overview, Qubes intro, Fuchsia capability routing, Tailscale ACL docs.
