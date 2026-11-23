---
definition_version: 7
readiness: intent
authority: 'the owner, 2026-11-23: "Podcasts seem to be skipped again. I think because we were running a podcast transcription service on my Mac but stopped that. We need to resume it then find better infra to run that"; on Synthetic: "measure how it works and figure out how to use synthetic DeepSeek at max sustained throughput in addition to Devin flash"; "the operations with podcast transcription as something that we want to do earlier, and synthetic as well. I kind of want to do those things after we get our plan defined."'
review:
  reviewer: none (an intention recorded from the owner's words; reviewed before it becomes executable)
  frozen_ref: none
  verdict: none
start:
  captured_at: 2026-11-23T22:02Z
  source:
    canonical_ref: origin/main bcea7eefd
    deploy_identity: staging and the paper on a2ba96767
  what_is_true_now:
  - Podcast transcripts come from cmd/podcast-worker, which ran inside the lead's Mac session and stops when the session ends. A podcast with no transcript has no clips and the radio skips it.
  - 'Synthetic is in the fallback tail at a fixed one in ten: 265 of about 5,700 calls in three hours on 2026-11-23, no refusals. Its weekly bucket refills 2 per cent every 202 minutes (measured from its response header), about 0.6 per cent an hour sustained; it sat at 97 per cent full.'
  - The gateway reads Synthetic's remaining percentage on every response and does nothing with it. Devin is paced by hourly token count and sheds 40 to 160 calls an hour.
  - Keenable answered every web search on 2026-11-23; the budget script still holds the old key's credits and renewal date.
  - The system map records the paper's main model as free until 2026-12-04.
finish:
  deliver: Podcasts are transcribed without anyone's laptop, and model calls are routed by each provider's measured headroom so no allowance sits idle while another sheds.
  artifact: 'on main: the podcast worker on a durable host, routing that reads provider headroom, and the search budget on the new key''s numbers'
  acceptance:
  - action: a new episode of a subscribed podcast appears as clips in the radio with no Mac session running
    proves: transcription runs unattended
    evidence_class: deployed proof
  - action: read a day of gateway lines by provider
    proves: Synthetic carries steady load near its refill rate and Devin sheds less, with no call refused or queued by us
    evidence_class: gateway journal counts
  rollback: each station is a commit that reverts
  landing:
    required: true
    environment: staging and the owner's phone where the station touches the app
    required_receipts:
    - pushed_commit
    - ci
    - deploy
    - deployed_acceptance
value:
  better_means: the share of each provider's allowance that is used rises and the calls refused for quota fall, with no cap or queue on our own calls
  goodharting_would_be: a rate limit named routing; a transcription host that is another laptop; filling an allowance with calls nobody needed
homotopy:
  realism_axis: the worker as it is on a durable host first, then on-device transcription; fixed weights first, then weights from the reported headroom
boundaries:
  mutation_class: red for gateway routing; orange for the worker's host
  authority_sources:
  - the owner's words in docs/design/design-session-2026-11-23.md; CLAUDE.md
  owner_rulings:
  - decision: transcription resumes, then finds durable infrastructure
    said: We need to resume it then find better infra to run that
    'on': '2026-11-23'
  - decision: Synthetic is used at its sustained rate beside Devin flash
    said: figure out how to use synthetic DeepSeek at max sustained throughput in addition to Devin flash
    'on': '2026-11-23'
  - decision: these are built after the plan is defined
    said: I kind of want to do those things after we get our plan defined. So then the building incorporates them
    'on': '2026-11-23'
  must_preserve:
  - 'no limits on our own model calls: routing chooses who serves a call and never delays or refuses one'
  - a grant that spends the owner's money is the owner's act
  - the worker fetches only public feeds; a private computer's data never crosses owners
  excluded:
  - on-device transcription of what a listener plays (M4 records the idea)
stations:
- id: O0
  name: The podcast worker on a durable host
  scope: where it runs (a host with direct egress, or the build machine through a proxy), as a unit that survives sessions; until then the lead resumes it by hand and says so
  class: orange
  review: one seat
  depends_on: []
  status: pending
- id: O1
  name: Routing by reported headroom
  scope: the pool's weight for a provider follows the remaining allowance it reports, with Devin's pacer as the model; counts logged so a shift is seen
  class: red
  review: two seats
  depends_on: []
  status: pending
- id: O2
  name: The search budget on the new key
  scope: the new key's credits and renewal date in the budget script
  class: yellow
  review: none
  depends_on: []
  status: pending
- id: O3
  name: After the free month
  scope: what the paper's main model costs from 2026-12-04 and what carries the load then
  class: green
  review: none
  depends_on: []
  status: pending
decisions_for_the_owner:
- id: Q1
  question: Where may the podcast worker run?
  why: it needs direct egress to podcast hosts, which the build machine's address guard refuses
  options:
  - a small durable host
  - the build machine through a redesigned proxy
  - Node B
  default: resumed by hand on the Mac until the owner chooses
- id: Q2
  question: What are the new Keenable key's credits and renewal date?
  why: the budget arithmetic needs them
  options:
  - the owner reads them from the provider's usage page
  default: the old numbers stay and the reading is marked stale
now:
  status: working
  as_of: 2026-11-23T22:02Z
  slice: deferred until the plan is defined, on the owner's word
  source_ref: origin/main bcea7eefd
  deploy_identity: staging on a2ba96767
  candidate: none
  believed_state: measured on 2026-11-23; nothing built
  main_uncertainty: what the batching experiment (M5 E5) says the steady load should be, since that is the load Synthetic would carry
  next_observation: M5 E5; the owner's answers
  blocker: none
  next_action: wait for the plan; O2 when the owner gives the numbers
operating_notes:
- 'O1 follows the engine''s design: the batchable intake load is the natural fit for a steady weekly allowance.'
receipts:
- docs/design/design-session-2026-11-23.md
- docs/research/provider-usage-apis-2026-11-06.md
---

# Operations: podcast transcription and provider headroom

Part of track T4. Two pieces of operations the owner wants early once the plan is defined: podcasts transcribed without a laptop, and model calls routed by what each provider reports it has left.
