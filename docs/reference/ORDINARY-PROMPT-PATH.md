# The ordinary prompt path: the policy and where it lives

Owner, 2026-11-16 (said again after months of regressions):

> "The whole policy is fast iterative responses with early evidence. Should
> make many revisions after each research search round not wait until many
> research rounds are done before research sends to texture. And texture is
> supposed to respond from weights even before it gets a message from
> research. V0 is supposed to be the raw prompt. V1 should be a fast response
> from texture model weights. V2 should be after first search round research
> message to texture. And later versions can also have eg texture messaging
> research or other agents messaging research so it's not 1:1 for a revision
> after each search round."

> "The point of the prompt bar router is to decide whether to open a new
> texture or work in an existing one. Not to have multiple ways to open a new
> texture. And yes, v1 should be from model priors, with uncertainty noted.
> Research should use the same cell for sending to texture and doing the next
> research round. Texture should be woken by new messages."

## Where each clause lives

The policy is elicited by orientation, not enforced by gates (owner,
2026-11-16: "we should be able to elicit correct behavior with the right
prompt. And if the behavior is wrong, it means the prompt is wrong."; see
[why-texture-2026-06-15.md](../why-texture-2026-06-15.md), "Runtime Boundary":
semantics in oriented agents, invariants in the runtime). The runtime holds
only mechanical invariants: delivery, wake, identity, provenance. No turn is
refused for lacking a Write, no `Done` is held back, and the kernel stages no
packet on the desk's behalf.

| Clause | Where it lives | Test |
|---|---|---|
| One way to open a Texture from the prompt bar; the router chooses new or existing | `internal/promptroute/inputclass.go` (request opens a new Texture; edit and comment revise an existing one; add_source opens none), `internal/textureowner/texture_handoff.go` (the desk's first objective is the owner's words) | `TestThereIsOneWayToOpenANewTextureAndItHandsTheDeskTheOwnersWords`, `TestTheRouterOffersOneNewTextureClassAndNoKindOfAsk`, and the non-canonical list of `TestRegisterRoutesPreservesCanonicalTable` |
| V0 is the raw prompt | `texture_handoff.go` | same first test |
| V1 from model priors, uncited, uncertainty noted; V2 at research's first message; later revisions as messages arrive | The Texture prompt (`internal/textureprompts/texture.yaml`), to be rewritten as an explanation of the system. The runtime only records what happened: `texture_revision_metadata.go` `markModelPriorInterim` puts `model_prior_interim` in the metadata of a document's first agent revision when it has no source ref and no message had reached the desk. Nothing reads that stamp today. | `TestANewTexturesFirstTurnCommitsAModelPriorV1BeforeAnyResearchMessageExists`, `TestLaterRevisionsAreNotMarkedModelPrior` |
| Research reports as it reads, in the same cell as the next round | The research prompt and overlay (`promptstore/defaults/research.yaml`, `runtimeprompts/overlays/rlm_research_runtime.yaml`), to be rewritten likewise. The packet decoder reads `source_refs` and a claim's `status` instead of refusing the packet (`yaegikernel/report_packet.go`). | `TestPacketAliasesAreAcceptedAndOtherUnknownFieldsAreStillRefused` |
| Texture is woken by new messages | The runtime's delivery invariant: a record starts a turn, and `agentcore/inbox_records.go` `recheckRecordsInbox`, the one place a committed turn parks, starts one more turn for a record the turn did not see. The store keeps a document trajectory live while its Texture inbox has unread records or a conversation has an open round (`lifecycleSettlementReady`). | `TestAResearchMessageWakesTextureAndYieldsARevision`, `TestAResearchMessageAfterDoneOnALiveTrajectoryWakesTextureAndYieldsARevision`, agentcore `TestTextureRunParkedWithMailItDidNotSeeStartsOneMoreTurn` |

Token streaming of Texture's writing is wanted later and is not part of this
path; it would attach where `commitTextureWrite` takes the model's output.

## What was removed (2026-11-16)

The story ask (`POST /api/autopaper/ask`, `ask_story.go`, the `ask_story` input
class, the `asked=story` revision metadata and the `ask_intent` run metadata).
It was a second way to open a Texture from the prompt bar, and its first objective ("open the
researcher ... then write the document from what is found") kept the desk from
writing a v1 for 12 minutes. Its only consumers were the web prompt bar
(`frontend/src/lib/prompt-submit.ts`, `conductor.js`: it posted classified
"ask a story" texts there) and the Autoradio design docs; both now send the
text to `POST /api/prompt-bar`. Nothing read `asked` or `ask_intent`.
`add_source` stays: it adds a feed and opens no Texture.

"One way to open a Texture" is a statement about the prompt bar. Other
creators of documents exist and are not prompt-bar routes: `POST
/api/texture/documents` (a lifecycle create), `POST /api/texture/fork` (a
reader's own document from a published version), the source-open and
corpus-wake handoffs (`HandoffKindSourceOpen`, `HandoffKindCorpusWake`), and
management's `Cast("texture:new")`, which opens a story. None of them takes a
prompt as an ask kind, and none changes how a prompt document's first turn
starts.

## How a desk's prompt text reaches a run

(Read from the code, 2026-11-16.) `agentcore.systemPromptForRun` builds a run's
system prompt from the shared core, the desk's role prompt and the desk's
runtime overlay. The role prompt comes from `promptstore.Store.Load(owner,
role)`: a per-owner file `<prompt root>/users/<owner>/<role>.md` if it exists
(source `user`), else the default. The defaults are embedded in the binary
(`promptstore/defaults/*.yaml`; Texture's is `textureprompts/texture.yaml`) and
re-seeded into `<prompt root>/defaults/<role>.md` at every load. The research
overlay is Go-embedded (`runtimeprompts/overlays/*.yaml`) and has no override.
The user file is written by `PUT /api/prompts/<role>` and removed by `DELETE`
(a reset); on a deployed computer those writes are refused unless test APIs are
enabled. Prompt `version:` numbers are metadata: nothing compares or refuses on
them. Tests that pin prompt wording or size bounds do refuse a longer or
differently worded prompt; the Texture and research bounds are sanity checks
only since 2026-11-16, and the V1 wording is not pinned.

## Where the wake guarantee stops (finding, 2026-11-16)

A record wakes Texture while its trajectory is live. It does not wake a
Texture whose trajectory has settled: `handleInboxRecord`
(`actorruntime/handler.go:538`) calls `ReconcileAgentWake`, which returns no
run when `classifyTextureLifecycleActivationSnapshot` says false for a settled
trajectory (`texture_controller.go:456-471`, the no-run log at `:525`). The
records stay unread. Only the owner's next revise reaches a settled document,
by continuing it on a successor trajectory (`ContinueLifecycleDocument`,
`store/lifecycle_continue.go:117`, called from `texture_agent_revision.go:363`).
Settlement itself waits for the Texture inbox to be read and for every round
the desk cast to close (`lifecycleSettlementReady`, `store/lifecycle.go:2948`),
so a reply to a round Texture cast cannot be late; unsolicited mail from
another desk can. This is pinned, not changed, by
`TestAMessageToASettledTextureWakesNothing`, and it departs from "Texture
should be woken by new messages": a decision for the owner.

## The packet decoder's corners

The decoder reads a claim's `source_refs` as `source_ids` and keeps a claim's
`status` in its text, and nothing else. It still refuses the packet when a
`status` is over 40 runes, is not a string, or contains a newline or other
control character (a label is one short plain line; anything else is text a
stranger could use to pass instructions through the packet), and when both
`source_refs` and `source_ids` are present (which spelling is meant is not for
the platform to guess). It adds no line to the cell result.
