---
definition_version: 4

# S-DL — the search deadline ladder. First change of
# docs/search-plane-design-fast-deep-broad-2026-10-11.md (§7, rows 1-3 of §6);
# problem docs/problems/guest-search-timeout-equals-gateway-provider-timeout-2026-10-11.md.
readiness: drafted

review:
  reviewer: "none yet (owner review first: it is a red gateway change and, in effect, a routing decision about ChatGPT search)"
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-11T22:20:00Z
  source:
    canonical_ref: main 3e277431
    deploy_identity: staging choir.news b172895b
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: goal_candidate
      owner: this session
      touch: goal_owned
      recovery: git
  observed:
    - >-
      Three layers all allow 30 seconds: the guest's gateway search client
      (internal/search/search.go, hostlink.NewClient(30 * time.Second)), the
      gateway handler's whole search (internal/gateway/handlers.go,
      context.WithTimeout(r.Context(), 30*time.Second)), and each provider
      call (internal/gateway/searchplane/config.go RequestTimeout, default
      30 seconds via CHOIR_SEARCH_REQUEST_TIMEOUT_SECONDS).
    - >-
      Over 24 hours, 51 of 509 searches (10%) ran to exactly 30.0 seconds at
      the gateway and were logged as succeeded, usually with one provider's
      10 results; every guest-side search timeout in two saved demo traces
      matches one of them (search design §2).
    - >-
      The stalled provider is most likely ChatGPT web search (inference,
      medium to high confidence: neither side logs per-provider attempts for
      these searches). Its stalls are recorded as "caller gone", so its
      health never degrades and it stays in rotation.
    - >-
      Research reports these searches as "the search gateway timed out" and
      narrows its coverage (docs/problems/research-desk-structural-flaw-root-cause-2026-10-11.md,
      cause 5).

finish:
  deliver: >-
    Each layer gives up before the layer that called it: a provider call
    within about 10 seconds, the gateway's whole search within about 25,
    the guest within about 35. No search outlives its caller, the slow tail
    shortens, and a slow provider is charged for its own slowness.
  artifact: >-
    Three constants in three files, no new code path: the provider request
    timeout default (searchplane config, 10 seconds), the gateway handler's
    search deadline (25 seconds), the guest client's deadline (35 seconds).
    A test pins the order (provider < gateway < guest) so the three cannot
    drift back into a tie.
  acceptance:
    - unit test pins provider timeout < gateway deadline < guest deadline
    - "staging, 24 hours after the deploy: searches at or above 24.9 seconds with the caller gone at write, target 0 (gateway journal)"
    - "staging: p90 search duration, target 15 seconds or less (prediction: about 12)"
    - "staging: no context deadline exceeded on web search in a research-only demo's guest trace"
    - "staging: ChatGPT search's health state and strikes move when it stalls (health store, read-only)"
    - one research-only AI news demo; sources reaching Texture compared with the P2a demo (receipt P2a-deployed)
  rollback: git revert of the commit (three constants and a test)
  landing:
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: research gets every search's results, faster, and stops blaming the gateway for a race between deadlines
  goodharting_would_be: shortening deadlines until searches return fast and empty
  realism_axis: unit order test -> staging gateway journal over 24 hours -> research-only demo
  mutation_class: red (gateway provider calls) for the provider and gateway deadlines; orange for the guest client
  authority_sources:
    - docs/search-plane-design-fast-deep-broad-2026-10-11.md §7
    - owner 2026-10-11 ("search needs to be fast, iterative, deep, broad, recursive, comprehensive")
  must_preserve:
    - partial results at the gateway deadline are still returned
    - ChatGPT inference is unaffected (only the search provider's budget changes)
  excluded: [streaming or early-return search, the slow and async lanes, batch queries, the research job (separate station), restoring paid providers]
  protected_surfaces: [internal/gateway/handlers.go, internal/gateway/searchplane/config.go, internal/search/search.go]
  read_only_dependencies: [internal/researchtools/researchtools.go]

now:
  status: drafted
  slice: written from the search design for the owner's review; nothing built
  source_ref: 3e277431
  deploy_identity: staging b172895b
  candidate: none
  conjecture:
    id: SDL-1
    claim: >-
      Ordering the three deadlines removes the searches that outlive their
      caller and cuts p90 from about 30 seconds to about 12, without
      reducing results per search.
    test: the staging journal over 24 hours and one research-only demo
    edge: missing_oracle
    delta_o: searches at or above 24.9 seconds with the caller gone; p90; results per search
    scope_if_supported: staging's four search providers
    status: proposed
    evidence_refs: []
  decision:
    what: >-
      A 10-second provider budget mostly benches ChatGPT search, whose
      answers take about 23 seconds when they come. That is a routing
      decision for the owner: accept the bench, or keep ChatGPT search in a
      slow or async lane (search design G3 or G5) as a later station.
    kind: routing
    status: open (owner)
    evidence_ref: docs/search-plane-design-fast-deep-broad-2026-10-11.md §7, §8
  belief:
    believed_state: the 30-second tie is the cause of the guest-side timeouts; the slow provider is likely ChatGPT search
    main_uncertainty: whether ChatGPT search is the stalled provider (no per-provider attempt log for the capped searches)
    next_observation: the owner's review; then the staging journal after the change
  blocker_or_risk: red gateway change; ChatGPT search routing decision
  next_action: owner reviews this file and the search design's open questions

receipts: []
---

# S-DL — the search deadline ladder

Three layers of the search path all allow thirty seconds. When one
provider is slow, the gateway waits to its deadline and returns what it
has, but the guest gave up a moment earlier, so research sees a timeout
for a search the gateway logged as a success. One search in ten does
this today. This station orders the deadlines so each layer gives up
before its caller. It is the cheapest change in the search design with the
largest effect, and it leaves the bigger search work (streaming, lanes,
batch queries, the research job) to later stations.

Ways it can fail, written before the code:

- a deadline set from the wrong environment variable or default, so
  staging still ties (the order test reads the effective values);
- providers that are healthy but sometimes slow get struck into cooldown
  and the plane narrows to one provider (watch strikes and cooldowns in
  the health store for 24 hours);
- the gateway returns partial results so often that results per search
  fall (compare results per search before and after);
- a caller other than research relies on the 30-second guest deadline
  (search the callers of the gateway search client before the change);
- ChatGPT search is benched without the owner's say (the routing
  decision is named above and waits for the owner).
