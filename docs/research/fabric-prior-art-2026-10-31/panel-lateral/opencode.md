
> build · particle-ai/particle-1
→ Read docs/design/resource-fabric-2026-10-31.md 
$ wc -l docs/design/resource-fabric-2026-10-31.md docs/design/branching-tape-2026-10-31.md 2>/dev/null; ls docs/design | head -50
1274 docs/design/resource-fabric-2026-10-31.md
  427 docs/design/branching-tape-2026-10-31.md
 1701 total
agent-tree-budgets-2026-10-21.md
app-agent-network-policy-2026-10-21.md
app-design-bar-2026-10-26.md
app-desk-compute-capsule-2026-10-21.md
app-package-carries-app-desk-2026-10-21.md
app-packages-replan-2026-10-24.md
append-receipt-trim-2026-10-31.md
branching-tape-2026-10-31.md
bundle-authenticity-2026-10-31.md
capsule-long-exec-2026-10-15.md
choir-data-system-frame-2026-10-30.md
choir-intro-showreel.html
ci-node-b-build-2026-10-24.md
ci-prove-once-2026-10-24.md
ci-runner-slots-2026-10-24.md
claude-delegate-slice1-2026-10-14.md
computer-history-restore-audit-2026-10-30.md
computer-update-architecture-rnd-2026-10-24.md
data-system-build-sheet-2026-10-30.md
delegate-one-round-build-2026-10-21.md
density-d2-snapshots-survive-deploys-2026-10-26.md
density-d6-page-cache-return-2026-10-26.md
egress-grant-object-2026-10-21.md
email-sign-in-2026-10-31.md
engineering-network-grants-2026-10-10.md
file-object-2026-10-24.md
fp10-snapshot-levers-and-host-slab-2026-10-22.md
guest-tool-layers-2026-10-31.md
layered-guest-image-2026-10-31.md
library-package-propagation-2026-10-26.md
object-graph-redesign-2026-10-30.md
object-storage-2026-10-30.md
offhost-weekly-prune-node-a.patch
one-tape-durability-2026-10-31.md
outboard-capsules-arch-11-12-2026-10-22.md
progressive-delivery-2026-10-26.md
recoverable-changes-sr7-2026-10-24.md
research-more-goes-to-research-2026-10-21.md
resource-fabric-2026-10-31.md
secrets-vault-design-2026-10-28.md
small-object-storage-2026-10-31.md
tape-group-commit-2026-10-31.md
tasks-precommit-icrl-2026-10-24.md
texture-evidence-embeds-te-2026-10-25.md
texture-holds-the-trajectory-2026-10-25.md
texture-spec-follows-owner-2026-10-25.md
trace-segment-log-2026-10-22.md
u4-stack-rebase-build-2026-10-26.md
visual-evidence-slice-2026-10-15.md
vsock-control-plane-and-nic-removal-2026-10-22.md
> build · particle-ai/particle-1
→ Read docs/design/resource-fabric-2026-10-31.md [offset=940, limit=340]
→ Read docs/design/branching-tape-2026-10-31.md 
> build · particle-ai/particle-1
I read both design docs (§0–§11 of the fabric; all of branching-tape) before breaking the frame. Drafting the lateral seat's response now.

---

# Lateral seat: break the frame

## 1. The frame I reject

The framing quietly makes **the machine ("site") the atom of the design and the total order the atom of correctness.** That is not a neutral starting point; it is a consensus-system stance imported wholesale, and then Q1–Q10 are the apology for it.

Three nested assumptions, in order of load-bearing weight:

1. **A computer needs a single serialization point, so "one writer per branch" is a property of correctness.** It isn't. It is a property of *deterministic replay*. The doc never separates "we need a total order so `Reduce` is a pure function of the log" from "we need a total order so concurrent acts cannot conflict." The first is true; the second is not, and 80% of the mechanism (leases, epochs, fences, handoffs, branches, no-merge, effects digests, publication-and-adoption) exists to serve the conflation.
2. **"Site" is the resource atom.** But the scarce, varying, trust-laden resources are *authority* (who may act), *data domicile* (whose bytes may rest where), *attention* (the owner), and *effects* (what touched the world). Machines are *where execution happens* — the cheapest, most swappable variable in the whole system. The doc already half-knows this: §0 rule 1 says the writer is "a host, never a VM," i.e. the identity of the computer is already decoupled from the machine that serves it.
3. **Placement is a plan.** §1.2 makes the management desk the allocator of work to sites. That is central economic planning over a fleet whose state the planner provably cannot see (a laptop's battery, a colleague's meeting, a site's real free memory one second from now).

And one doctrinal inconsistency worth naming: the repo's own contract already says **each app desk is the sole writer of its app's object kinds, and Texture is the sole writer of canonical revisions.** That is *per-kind* write authority. The resource fabric then flattens it to *per-tape* write authority. The generalization is unjustified, and it is the single assumption that forces the entire branch apparatus.

## 2. The reframe

**Reframe: a computer is a bundle of enumerated authorities over kinds, plus a journal of commitments. The tape is one encoding of that journal, not the authority itself. Placement is matching under rights, not planning.**

Accounting is the right distant analogy. A bank does not keep one global ledger with one writer; it keeps a *journal* (append-only, ordered) and a *ledger* (the obligations), and it reconciles with other banks by **clearing** — settlement of *effects*, not merging of *state*. Nobody calls that "a merge" and nobody calls it unsafe. Double-entry bookkeeping is the oldest proven protocol for "many writers, one truth," and it wins by making the unit of reconciliation the *obligation*, not the *row*.

So the corrected invariants, in place of "one writer per branch":

- **Authority is conserved and attenuates.** No act creates authority; a grant is a transfer whose scope is a subset of the granter's (this is already §4.3, "a grant cannot be wider than its granter" — it just isn't the organizing principle). This is a conservation law, and conservation laws are what make a configuration space *provably* safe without enumerating it (Q9's answer).
- **Every irreversible effect class has exactly one fenced emitter at a time.** Mail has an emitter; payment has an emitter; Texture's canonical revision has an emitter. These are the *only* things that need a lease.
- **Content is causally consistent per kind; the total order is over commitments, not rows.** Replay is per-kind; the journal records only "kind K head advanced to H under epoch E." This is the journal/ledger split, and it preserves rebuildability — which is the product.

This reframes Q1 from "who holds the tape" to "which effect classes does this act emit, and who is each one's fenced emitter?" Failover stops being an all-or-nothing branch switch and becomes *per-emitter*: the Mail emitter can fail over to the server while Texture keeps writing on the Mac, because they don't conflict. Branching-tape's case 1 (the "high likelihood" case) mostly evaporates: phone edits to Texture and Mac edits to Texture are the same kind and do conflict; phone mail triage and Mac modelling do not.

## 3. What this changes in practice

- **Q1 (writer authority):** leases become per-effect-class, not per-branch. The epoch lives on the emitter, not the tape. Offline work is free for non-emitting kinds, which is most work.
- **Q2 (reachability):** unbundle the home. Today §0 rule 2 fuses three independent things into one durable authority — the lease row, the follower copy, and the route — and then needs failover for the fusion. Separate them: authority in the owner's keys (hardware, portable, like SSH), routing in a disposable anycast relay (like TURN/HLR), storage in content-addressed replicas. A home becomes a *cache and a switchboard*, replaceable in an afternoon, not a jurisdiction.
- **Q3 (access control):** stop treating trust as a site property *and* a relation. It is a relation only. The site publishes *claims* (operator, hardware, OS grants); the owner's policy decides. "Trust class" is a cache of a decision, not a fact about a machine.
- **Q4 (placement):** market, not plan. Sites advertise prices and limits; work carries a budget; local admission is the property right. The management desk sets the *constitution* (what may be traded, whose data may cross which boundary) and never clears the order book. This is the only placement rule that scales to heterogeneous orgs without a global planner, and the only one that survives the owner's own unpredictability.
- **Q5 (effects):** already right in spirit (§2.3), but it becomes the *center* of the design instead of a footnote. Precommitment-with-home-durable-record is exactly clearing.
- **Q10 (self-tuning inside proven bounds):** the bounds stop being "rings R0–R2" and become the conservation law plus per-effect-class fencing. Bounds you can state as invariants are bounds a computer can safely tune inside; a ring ladder is not a proof.

The kernel shrinks to two rules — *authority attenuates* and *every effect has one emitter* — which is what "small kernel, all configurations safe" actually requires.

## 4. The sharpest objection to my own reframe

**I trade portability for concurrency, and portability is the product.** The tape's total order is not there for tidiness: it is what makes `Reduce` deterministic, which is what makes a computer *rebuildable from a bundle*, which is what lets it move to a Mac, be forked, be restored, and be audited. A per-kind partial order pushes conflict resolution into every kind — precisely the CRDT-per-kind explosion branching-tape §8 rejected for good reason — and a computer that cannot be rebuilt from a log is no longer a portable personal computer; it is a cluster with a personality. Second, my "most acts don't conflict" is false the moment read-your-writes enters: a desk reads what another desk wrote, and under partial order there is no defined instant at which that read is legal.

The honest synthesis is narrower than my reframe: **total order over the commitment journal, partial order over content** — which is a *new mechanism*, not a simplification, and it would need the proof the branch design already has. So test the frame rather than adopt mine: when the next design question is asked, ask **"what authority does this act require?"** before **"where does this run?"** If the answers ever pull apart — and they do, in Q1 and Q2 — the site frame is doing work the authority frame should be doing.
