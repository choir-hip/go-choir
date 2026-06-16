----------------------------- MODULE wire_pipeline -----------------------------
(***************************************************************************)
(* Spec of the redesigned Universal Wire news pipeline.                    *)
(*                                                                         *)
(* Layer: sits ABOVE actor_protocol.tla. That spec proves messages wake    *)
(* actors; this one assumes durable state transitions happen (some actor   *)
(* eventually does the work) and checks the pipeline's own logic:          *)
(* trajectory lifecycle, coverage/dedup, publication, edition, settlement. *)
(*                                                                         *)
(* Source design: docs/choir-rearchitecture-durable-actors-2026-06-11.md   *)
(* §3 (review v2) — publication trajectories with explicit settlement,     *)
(* replacing transient in-run decisions and run-tree liveness.             *)
(*                                                                         *)
(* The redesign's claims being checked:                                    *)
(*                                                                         *)
(*  1. SuppressedImpliesPublished (safety): an item may be suppressed as   *)
(*     "already covered" ONLY against the PUBLISHED corpus — never against *)
(*     unpublished drafts. This is the f44065ed bug class: the processor   *)
(*     concluded "already covered" from guest-local unpublished revisions  *)
(*     and the front page stayed empty. Weaken SuppressItem's guard to     *)
(*     include drafting stories and TLC produces that incident as a trace. *)
(*                                                                         *)
(*  2. EditionHonest (safety): the public edition lists only published     *)
(*     stories — the list never lies. This is the list/open split-brain    *)
(*     bug ("honesty before completeness", Wire mission lesson 2). Remove  *)
(*     UpdateEdition's publish guard and TLC shows the lying front page.   *)
(*                                                                         *)
(*  3. SettledSound (safety): a settled trajectory is published AND in the *)
(*     edition — settlement is earned, not declared. This is the explicit  *)
(*     settlement rule replacing root-run-completion as liveness truth.    *)
(*                                                                         *)
(*  4. EveryItemSettles (liveness): every fetched item's story eventually  *)
(*     settles, despite duplicate items, abandoned drafts (evicted Texture   *)
(*     chains), and full interleaving of concurrent processors. This is    *)
(*     the property maxProc=1 was protecting by serialization; here it     *)
(*     holds under arbitrary concurrency because decisions are durable     *)
(*     trajectory state, not in-run conclusions.                           *)
(*                                                                         *)
(* Recovery parallel: ReopenFromItem is this layer's sweep — an item that  *)
(* opened a trajectory whose draft chain died re-opens it. It exists only  *)
(* because the open-decision is DURABLE (the trajectory ledger). In the    *)
(* old design that decision lived inside a processor run and died with it. *)
(*                                                                         *)
(* Abstractions: semantic dedup is modeled as exact story identity         *)
(* (StoryOf); crash needs no action here because every variable is         *)
(* durable — in-flight work loss is modeled by AbandonDraft, which is      *)
(* bounded for the same reason evictions are bounded in actor_protocol     *)
(* (unbounded abandon/reopen without progress is a livelock; the bound is  *)
(* the per-owner activation cap).                                          *)
(***************************************************************************)

EXTENDS Naturals, FiniteSets

CONSTANTS
  Items,        \* source items arriving from cycles, e.g. {i1, i2, i3}
  Stories,      \* underlying stories, e.g. {s1, s2}
  MaxAbandons   \* bound on abandoned draft chains (impl: activation caps)

\* Which story an item is about. Duplicate coverage of one story across
\* cycles/sources is the interesting case: i1 and i2 are the same story.
StoryOf == (CHOOSE f \in [Items -> Stories] :
              \A s \in Stories : \E i \in Items : f[i] = s)

VARIABLES
  item,      \* item[i]  : "pending" | "fetched" | "opened" | "suppressed"
  story,     \* story[s] : "none" | "drafting" | "published" | "settled"
  edition,   \* set of stories listed on the public front page
  abandons   \* number of abandoned draft chains so far

vars == <<item, story, edition, abandons>>

ItemStates  == {"pending", "fetched", "opened", "suppressed"}
StoryStages == {"none", "drafting", "published", "settled"}

TypeOK ==
  /\ item \in [Items -> ItemStates]
  /\ story \in [Stories -> StoryStages]
  /\ edition \subseteq Stories
  /\ abandons \in 0..MaxAbandons

Init ==
  /\ item = [i \in Items |-> "pending"]
  /\ story = [s \in Stories |-> "none"]
  /\ edition = {}
  /\ abandons = 0

--------------------------------------------------------------------------
(* Source cycle *)

FetchItem(i) ==
  /\ item[i] = "pending"
  /\ item' = [item EXCEPT ![i] = "fetched"]
  /\ UNCHANGED <<story, edition, abandons>>

--------------------------------------------------------------------------
(* Processor decisions — durable trajectory state, never in-run memory.   *)
(* The three guards are disjoint: a fetched item is opened (new story),   *)
(* attached (story already in flight), or suppressed (story already       *)
(* PUBLISHED — and only published).                                        *)

OpenFromItem(i) ==
  /\ item[i] = "fetched"
  /\ story[StoryOf[i]] = "none"
  /\ item' = [item EXCEPT ![i] = "opened"]
  /\ story' = [story EXCEPT ![StoryOf[i]] = "drafting"]
  /\ UNCHANGED <<edition, abandons>>

AttachItem(i) ==
  /\ item[i] = "fetched"
  /\ story[StoryOf[i]] = "drafting"
  /\ item' = [item EXCEPT ![i] = "opened"]
  /\ UNCHANGED <<story, edition, abandons>>

\* THE coverage rule: suppress only against the published corpus.
\* (Sabotage: add "drafting" to this guard set — TLC reproduces the
\* empty-front-page incident as a counterexample trace.)
SuppressItem(i) ==
  /\ item[i] = "fetched"
  /\ story[StoryOf[i]] \in {"published", "settled"}
  /\ item' = [item EXCEPT ![i] = "suppressed"]
  /\ UNCHANGED <<story, edition, abandons>>

--------------------------------------------------------------------------
(* Drafting and publication *)

\* Texture work concludes; autonomous publish writes the corpus and records
\* the publication ref on the trajectory — one durable transition here.
Publish(s) ==
  /\ story[s] = "drafting"
  /\ story' = [story EXCEPT ![s] = "published"]
  /\ UNCHANGED <<item, edition, abandons>>

\* A drafting chain dies (eviction, failed Texture work). Durable trajectory
\* state returns to none; opened items make it re-openable (see Reopen).
\* Bounded for the same reason evictions are bounded in actor_protocol.
AbandonDraft(s) ==
  /\ story[s] = "drafting"
  /\ abandons < MaxAbandons
  /\ story' = [story EXCEPT ![s] = "none"]
  /\ abandons' = abandons + 1
  /\ UNCHANGED <<item, edition>>

--------------------------------------------------------------------------
(* Edition and settlement *)

\* The front page lists a story only once it is published — never before.
\* (Sabotage: drop the publish guard — TLC shows the lying list.)
UpdateEdition(s) ==
  /\ story[s] \in {"published", "settled"}
  /\ s \notin edition
  /\ edition' = edition \cup {s}
  /\ UNCHANGED <<item, story, abandons>>

\* Settlement is earned: published, listed, nothing left to do.
Settle(s) ==
  /\ story[s] = "published"
  /\ s \in edition
  /\ story' = [story EXCEPT ![s] = "settled"]
  /\ UNCHANGED <<item, edition, abandons>>

--------------------------------------------------------------------------
(* Recovery — this layer's sweep. Exists only because the open-decision
   is durable trajectory state; in the old design it died with the run. *)

ReopenFromItem(i) ==
  /\ item[i] = "opened"
  /\ story[StoryOf[i]] = "none"
  /\ story' = [story EXCEPT ![StoryOf[i]] = "drafting"]
  /\ UNCHANGED <<item, edition, abandons>>

--------------------------------------------------------------------------

Next ==
  \/ \E i \in Items : FetchItem(i)
  \/ \E i \in Items : OpenFromItem(i)
  \/ \E i \in Items : AttachItem(i)
  \/ \E i \in Items : SuppressItem(i)
  \/ \E i \in Items : ReopenFromItem(i)
  \/ \E s \in Stories : Publish(s)
  \/ \E s \in Stories : AbandonDraft(s)
  \/ \E s \in Stories : UpdateEdition(s)
  \/ \E s \in Stories : Settle(s)

(* Fairness: the pipeline's standing obligations — processors decide,
   drafts publish, the edition updates, settlements close, reopens fire.
   Fetching and abandoning are environment. *)
Fairness ==
  /\ \A i \in Items : WF_vars(OpenFromItem(i))
  /\ \A i \in Items : WF_vars(AttachItem(i))
  /\ \A i \in Items : WF_vars(SuppressItem(i))
  /\ \A i \in Items : WF_vars(ReopenFromItem(i))
  /\ \A s \in Stories : WF_vars(Publish(s))
  /\ \A s \in Stories : WF_vars(UpdateEdition(s))
  /\ \A s \in Stories : WF_vars(Settle(s))

Spec == Init /\ [][Next]_vars /\ Fairness

--------------------------------------------------------------------------
(* INVARIANTS *)

\* an item suppressed as "covered" has a PUBLISHED story — the f44065ed rule
SuppressedImpliesPublished ==
  \A i \in Items :
    item[i] = "suppressed" => story[StoryOf[i]] \in {"published", "settled"}

\* the public list never lies — edition only references published stories
EditionHonest ==
  \A s \in edition : story[s] \in {"published", "settled"}

\* settlement is earned: published and listed
SettledSound ==
  \A s \in Stories :
    story[s] = "settled" => s \in edition

--------------------------------------------------------------------------
(* TEMPORAL PROPERTIES *)

\* every fetched item's story eventually settles — the front page fills,
\* under full concurrency, duplicates, and bounded abandonment
EveryItemSettles ==
  \A i \in Items :
    (item[i] = "fetched") ~> (story[StoryOf[i]] = "settled")

\* every published story eventually appears on the front page
EditionConverges ==
  \A s \in Stories :
    (story[s] = "published") ~> (s \in edition)

================================================================================
