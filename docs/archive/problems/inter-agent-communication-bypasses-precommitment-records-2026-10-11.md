# Inter-agent communication mostly bypasses precommitment records

Found 2026-10-11, when the owner directed that all inter-agent
communication go through the precommitment-records protocol, and the W2
file review showed W2 could not be built without it. Evidence class: code
reading plus counts from staging traces (1,286 run event streams, 1,874
unique cells, 2026-10-10 07:01Z to 2026-10-11 14:34Z: the M11
self-development reruns and the AI news demos S1d to S1h).

## What the design says

The Autopaper design (docs/autopaper-sw-design-2026-10-09.md §2): "Records
addressed to a Texture or a desk are deliveries that wake it. That is the
whole communication protocol between Textures and desks." The engineering
memo (docs/precommitment-records.md) requires a
prediction committed before the evidence that resolves it, resolved
against a verifiable outcome.

## What the system does

S0m (2026-10-01, complete) made the semantic acts record-native: Cast,
Ask, Note, Reply, Escalate, Report, ReportPacket, Precommit, Resolve,
Disagreement and Retract each mint a commitment record, and the record's
derived packet wakes the addressee (`internal/store/lifecycle_commit_act.go`).
Two gaps remain.

### 1. The main dispatch paths are not records

| Path | Carrier | Uses in the traces |
|---|---|---|
| Texture opens research | `ApplyTexture` control `open_researcher` → `IssueLifecycleControl` → source packet | 77 |
| Texture opens management | `ApplyTexture` control `open_persistent_super` → same | 73 |
| Texture follows up with an open desk | control with `target_work_item_id` plus packet | 133 |
| Research or management reports to Texture | `Report` / `ReportPacket` (record) | 216 |
| Management casts engineering | `Cast` (record) | 18 |
| `choir.Message` | assigned-desk authority contract, lifecycle record, or the old envelope for non-lifecycle callers (`internal/agentcore/rlm_reduce.go`) | 10 |
| `choir.Emit` | envelope write outside the cell tray, wakes immediately (`internal/agentcore/tools_desk.go`) | 1 |
| `choir.Assign` | direct broker call | 0 |
| Management to product routes | `choir.ProductAPI`, acting as the owner | (not counted) |
| `Precommit`, `Resolve`, `Ask`, `Reply` | records | 0 |

So every dispatch Texture makes (283 uses) travels as a lifecycle control
outside the ledger, and only the answers come back as records. The ledger
is a one-way report channel.

Runtime-originated wakes are a further set of kinds outside the ledger
(`internal/actorruntime/handler.go`): `initial_dispatch`,
`coagent_result`, `channel_message`, six deadline kinds,
`selfdev_materialization_retry`, `source_poll_due`,
`lifecycle_work_assigned`, `lifecycle_cancellation`, `owner_revision`,
`cancel`. These are not agent speech, but they are part of the account of
what woke whom.

### 2. No record is a precommitment

A Cast needs an objective and an Ask a question; neither carries a
prediction. `CommitmentPrecommit` (`internal/types/commitment.go`) holds a
question, a distribution and a resolver desk, with no deadline and no
subject key, and the only resolution path is a desk's `choir.Resolve`,
which is a model turn. Nothing in the runtime resolves a record by
counting. In the traces, no desk ever called `Precommit` or `Resolve`.

## What it costs

- **S1h (PD1 receipt):** Texture spent research rounds two through seven
  asking research to confirm leads; research reported the Haiku 5.5 claim
  six times and the task-force lead four. With a frozen prediction on each
  question, those rounds would score as no surprise and the waste would be
  a measured number, not a hand-read trace. The fix tried instead was
  prompt wording (25c8b5b0).
- **W2 (file review, receipt W2-review1):** management has no path to
  deliver evidence to a Texture outside its own trajectory (channel mail
  refuses Texture targets; the report path binds to one exact delivered
  control), and `ProductAPI` lets management write a Texture as the owner.
  The W2 draft invented a watermark delivery over `channel_message`, which
  drops a message when management has no parked run. Each new station
  builds its own carrier around the missing protocol.
- **Sole-writer rule:** "only the Texture desk writes a Texture" is not
  enforced against management's `ProductAPI` routes.

## Test

The protocol is in place when, on staging:

- every wake of one desk by another derives from a commitment record
  addressed to it, and the reducer refuses `Message`, `Emit` and the
  `ApplyTexture` controls (they no longer exist on the surface);
- every act that asks for work carries a typed prediction frozen at
  commit, and the act that answers it resolves it, by count where a count
  answers the question, with the score on the record;
- a trace count like the table above shows zero off-ledger dispatches.
