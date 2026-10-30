# Calibration Level 2: Precommitments Through the Choir CLI and MCP

Date: 2026-10-14. Author: Claude (subagent of the lead), for owner review.
Level 1 is [docs/calibration/claude-ledger.md](../../calibration/claude-ledger.md):
claims in a Markdown table, settled by hand. This document designs level 2,
where any agent writes the same records into a Choir computer through an
API call, from the Choir CLI or a Choir MCP server. The calibration record
then belongs to the agent's computer, not to this repository.

Owner, 2026-10-14: "The second level, an even better way, is using the
Choir CLI or the future Choir MCP so that you write the precommitments as
an API call into the Choir computer that you have access to. That way
these scores aren't limited to our development repo [...] now we get to
run the experiment to prove or provide evidence against the notion that
you can improve calibration with in-context reinforcement learning."

Related designs: [precommitment records](../../precommitment-records.md)
(the merged design: §2 the record, §4.6 one schema across computers, §5
scoring, multi-scorers, self never governs), and its archived sources
[predictions and scoring](../precommitment/precommitment-predictions-and-scoring-design-2026-10-11.md)
§4 (separate the answer from the score; one score record per scorer);
[claims E0/E6a results](claims-e0-e6a-results-2026-10-14.md) §5 (the
feedback loop); [owner notes](../../reports/owner-notes-2026-10-14.md) §1 and §3 (PSA as
in-context RL; multi-scoring; calibrated self-scoring).

**Status (2026-10-14, branch `calibration-level2`, not pushed or
deployed):** built as designed: the routes, the scopes, the CLI, the
import and the MCP server (§8 has the day-to-day use). One change from
the first draft: a claim settled only by its claimant counts in the
summary as the agent's own learning signal, and the summary says how many
are self-only, because self never governs oversight
(precommitment-records §5.2).

## 1. What an agent does

```text
1 claim     choir precommit add --claim "..." --p 0.85 --type forecast
            --criterion "how it settles" --deadline 2026-10-15T18:00:00Z
            (recorded before the settling evidence exists; server-stamped time)
2 work      ...
3 settle    choir precommit settle <claim-id> --outcome false
            --evidence "CI 38579173174" --method independent --scorer claude-code/lead
            (one record per scorer; a settled record is never edited)
4 read      choir precommit calibration --agent claude-code/lead --text
            (a short track record the agent pastes into its context
            before it states its next probability)
```

The MCP server exposes the same four operations as tools
(`precommit_add`, `precommit_settle`, `precommit_list`,
`calibration_summary`), so an MCP-capable agent (Claude Code, Claude
Desktop, any MCP client) gets them without shelling out.

## 2. Records: no new kinds

Both records are `choir.commitment_record` objects (already registered in
the object graph registry, which is now a write gate) using record kinds
that already exist. Nothing new is registered.

| Act | Record kind | Typed body used | New additive field |
|---|---|---|---|
| Claim | `precommit` | `Precommit{Question, Distribution{"true": p, "false": 1-p}}`, `Prediction{Hypothesis, Questions[1], CommittedAt}`, `Provenance{AgentID, ModelID, ContextRef = session id, CommittedAt}` | `calibration.claim` |
| Settlement | `resolve` | `Resolve{Verdict, EvidenceRefs}`, `ParentID` and `RelatedIDs` = the claim id (the existing resolution link), `Observation`, `Provenance{AgentID = scorer, ModelID = scorer model, ResolvedAt}`, `Discrepancy` | `calibration.settlement` |

The additive field is one typed sub-object, `CommitmentRecord.Calibration`,
following the pattern of `Cast`, `Report`, `Support` and `Captures`:

```go
type CommitmentCalibration struct {
    Schema     string                 `json:"schema"` // "choir.calibration.v1"
    Agent      string                 `json:"agent"`  // the claimant, on both records
    Claim      *CalibrationClaim      `json:"claim,omitempty"`
    Settlement *CalibrationSettlement `json:"settlement,omitempty"`
}
```

- **Claim** carries: text, type (`assertion` | `forecast` | `commitment`),
  probability that the claim is true, settlement criterion, deadline,
  project, tags, session id, and for imported rows `stated_at`,
  `reconstructed` and `source_ref`.
- **Settlement** carries: claim id, outcome (`true` | `false` | `partial`
  | `unresolvable`), scorer method (`self` | `independent` | `rule` |
  `owner`), scorer name and model, the claim's probability copied at
  settlement, and the Brier score `(p - y)^2` with y = 1, 0 or 0.5
  (`partial`); no Brier for `unresolvable`.
- **Identity** goes on the record the protocol already defines: the
  claimant's model on `Provenance.ModelID` (this also starts repairing
  finding F2, "model_id is empty on every desk record", for this record
  class), the session on `Provenance.ContextRef`.
- **Ids.** A claim is `calib:<agent-slug>:<16 hex>`; the hex is derived
  from the agent name and the caller's idempotency key when one is given,
  random otherwise. A settlement is `<claim-id>:settle:<method>:<scorer-slug>`:
  one settlement per scorer per claim, as in the scoring design §4.1.
- **Discrepancy** maps `true` → confirmed, `false` → contradicted,
  `partial` → qualified, `unresolvable` → unresolved, so the existing
  supervision views (`ResolveCommitments`, `AccrualByAgent`) read the
  records without change.

Why not the proposed `score` record kind (scoring design §4.1)? It was
never built, and the 2026-10-12 revision withdrew runtime scoring. A
`resolve` record per scorer, carrying its scorer identity and Brier score,
is the same shape the design asked for (one record per scorer per target,
never `CommitmentRecord.Scores`) without a new kind.

## 3. API (guest computer, behind the proxy)

All routes are under `/api/precommit/`. The proxy maps that prefix to two
new API key scopes, `precommit:read` (GET) and `precommit:write`
(everything else); `admin` keeps passing. An agent's key therefore needs
neither `write:runtime` (which starts runs) nor `read:runtime`.

| Route | Does |
|---|---|
| `POST /api/precommit/claims` | Record a claim. 201 new; 200 on an idempotent replay with the same body; 409 when the idempotency key was used for a different claim |
| `GET /api/precommit/claims?agent=&model=&type=&project=&status=open\|settled\|overdue\|all&limit=` | List claims with their settlements, governing outcome and Brier |
| `GET /api/precommit/claims/{id}` | One claim |
| `POST /api/precommit/claims/{id}/settlements` | Append one scorer's settlement. 201 new; 200 identical replay; 409 when that scorer already settled the claim differently |
| `GET /api/precommit/calibration?agent=&model=&type=&project=&scorer=governing\|self\|independent\|rule\|owner&group_by=agent,model,type&include_reconstructed=` | The calibration summary |

There is no PUT, PATCH or DELETE: a claim is never edited and a
settlement is never edited. Corrections are appended by another scorer,
and disagreement between scorers is kept and shown.

**Governing outcome.** When several scorers settled a claim, the
governing one follows the scoring design's order, rule > owner >
independent > self (ties: earliest). The summary can also be computed
under one scorer method, which is how self-scores are compared with
independent ones.

**The summary** (per group, default agent × model × claim type, plus an
overall row): counts (claims, open, overdue, settled, resolved,
unresolvable), Brier on resolved claims, strict Brier (unresolvable counted
as false, as E6a does for agents), base rate, mean stated p, hit rate,
**overconfidence index** (mean confidence in the stated direction minus
the hit rate; positive is overconfident), calibration bins of width 0.1 by
stated p (n, mean p, observed rate), and the **self-score gap** (claims
with both a self and a non-self settlement: agreements, self-favourable
disagreements, self-unfavourable disagreements). It also returns
`context`: a short plain-text track record (Brier, overconfidence, the
bins, the largest misses, overdue claims) written to be pasted into an
agent's context. That text is the in-context reinforcement signal of
owner note 1.

## 4. CLI

```text
choir precommit add --claim TEXT --p P [--type assertion|forecast|commitment]
                    [--criterion TEXT] [--deadline RFC3339] [--project P] [--tag T]...
                    [--agent NAME] [--model M] [--session S] [--key IDEMPOTENCY]
choir precommit settle <claim-id> --outcome true|false|partial|unresolvable
                    --evidence REF [--evidence REF]... [--method self|independent|rule|owner]
                    [--scorer NAME] [--scorer-model M] [--note TEXT]
choir precommit list [--agent] [--model] [--type] [--project] [--status open|settled|overdue|all] [--limit N]
choir precommit calibration [--agent] [--model] [--type] [--project] [--scorer governing|...]
                    [--group-by agent,model,type] [--include-reconstructed=true] [--text]
choir precommit import-ledger --file docs/calibration/claude-ledger.md --model M [--apply]
choir mcp            (stdio MCP server; see §5)
```

Agent identity defaults come from `$CHOIR_AGENT`, `$CHOIR_MODEL` and
`$CHOIR_SESSION`, so a session sets them once. Host, key and computer
use the existing CLI mechanism (`$CHOIR_HOST`, `$CHOIR_API_KEY` or
`--api-key-file`, `--computer`/`$CHOIR_COMPUTER`). Output is JSON;
`calibration --text` prints only the pasteable context.

`import-ledger` parses the level-1 tables. Settled rows become a claim
(`reconstructed`, `stated_at` = the ledger date, idempotency key
`claude-ledger:<n>`) plus one settlement: the lead's own claims as
`self`, subagents' claims as `independent` (the lead settled them against
CI, staging and traces). Open rows become claims whose criterion is the
"Settles by" column. It is a dry run that prints the planned requests
unless `--apply` is given, and every write is idempotent, so a rerun
writes nothing new.

## 5. MCP server

> Update (2026-10-14, branch `cli-mcp-parity`): `choir mcp` now serves
> every CLI command that is not declared CLI-only (51 tools), generated
> from one command registry. The four tools below keep their names and
> arguments. See
> [choir-cli-mcp-parity-2026-10-14.md](../../reports/choir-cli-mcp-parity-2026-10-14.md).

The repository has no MCP plumbing and no MCP dependency. A stdio MCP
server is small enough to write directly in Go with the standard
library: newline-delimited JSON-RPC 2.0 on stdin/stdout, logging on
stderr. It is `choir mcp`, a subcommand of the existing CLI, so it reuses
the CLI's host, key and computer configuration and its HTTP client. It
adds no dependency and no new binary.

Methods: `initialize` (answers the client's protocol version when it is
one we know: 2024-11-05, 2025-03-26 or 2025-06-18; otherwise the newest),
`notifications/initialized`, `ping`, `tools/list`, `tools/call`. Unknown
methods answer -32601; a malformed line answers -32700; notifications
get no response.

| Tool | Arguments | Calls |
|---|---|---|
| `precommit_add` | `claim`, `p` (required); `type`, `criterion`, `deadline`, `project`, `tags`, `agent`, `model`, `session_id`, `idempotency_key` | `POST /api/precommit/claims` |
| `precommit_settle` | `claim_id`, `outcome`, `evidence` (required); `method`, `scorer`, `scorer_model`, `note` | `POST /api/precommit/claims/{id}/settlements` |
| `precommit_list` | `agent`, `model`, `type`, `project`, `status`, `limit` | `GET /api/precommit/claims` |
| `calibration_summary` | `agent`, `model`, `type`, `project`, `scorer`, `group_by` | `GET /api/precommit/calibration`; the result's first text block is the pasteable context |

A tool failure (an HTTP error from the computer) returns a result with
`isError: true` and the error text, as the MCP spec asks, not a JSON-RPC
error. Agent identity defaults come from the server's environment
(`CHOIR_AGENT`, `CHOIR_MODEL`, `CHOIR_SESSION`), set in the MCP client's
server configuration, for example in Claude Code:

```json
{"mcpServers": {"choir": {"command": "choir", "args": ["mcp"],
  "env": {"CHOIR_HOST": "https://choir.news", "CHOIR_COMPUTER": "<computer id>",
          "CHOIR_AGENT": "claude-code/lead", "CHOIR_MODEL": "claude-opus-5-5"}}}}
```

The key is read from `$CHOIR_API_KEY` in the server's environment, or
from a mode-0600 file named by `$CHOIR_API_KEY_FILE`, never from a tool
argument (tool arguments are written by the model).

## 6. Failure modes, written before the code

1. **A claim is backdated.** The server stamps `committed_at`; a client
   value is kept only as `stated_at` on imported rows, which are marked
   `reconstructed` and excluded from the summary unless asked for. Level 1
   rows 1-12 were reconstructed the same day: hindsight picks which claims
   get recorded, so they must not count as prospective evidence by default.
2. **A settled claim is edited.** No edit route exists. A replayed
   settlement with an identical body is a 200 no-op; a different body from
   the same scorer is 409. The store write is conditional on not-exists,
   so two concurrent different settlements cannot both land; the loser is
   re-read and compared.
3. **An idempotency key is reused for a different claim.** 409, never a
   silent overwrite and never a silent success.
4. **Self-scoring is passed off as independent.** `independent` requires a
   scorer name different from the claimant; `self` requires the claimant's
   name. Names are declared, not authenticated (residual R1), but the
   check stops the honest mistake.
5. **Brier math is wrong at the edges.** p outside [0,1], NaN, and an
   empty set are refused or return n = 0 with null scores, never NaN in
   JSON. `partial` scores against 0.5. `unresolvable` is excluded from
   Brier and counted as false in strict Brier.
6. **The overconfidence index has the wrong sign.** A claim stated at 0.2
   that turns out false is a correct low-confidence call (confidence 0.8
   in "false", a hit). The index is computed on the stated direction, so
   a well-calibrated agent scores near 0 and an agent whose 0.95 claims
   hold half the time scores about +0.45.
7. **External claims crowd the desks out of their packs.** The desk pack
   (`ListCommitmentClaimRecords`) reads the newest 256 non-directive
   records before filtering by desk. A busy external agent would push a
   desk's own records out of its window. The query excludes records with
   a `calibration` body before the limit, exactly as it already excludes
   directives (P1).
8. **External claims leak into Texture's evidence feed.** That read is
   scoped by addressee; calibration records have none.
9. **Scope escalation.** `/api/precommit/` needs `precommit:*` (or
   `admin`); a `precommit:write` key cannot start runs or write Texture,
   and a `write:runtime` key cannot write claims. Keys stay delegable only
   within the caller's scopes (unchanged).
10. **The summary silently truncates.** The list reads at most 10,000
    calibration records; the response says `truncated: true` when it hit
    the cap.
11. **A settlement for a missing or non-calibration record.** 404; a
    settlement cannot target a desk's record through this route.
12. **The import double-writes or mis-parses.** Dry run by default;
    deterministic idempotency keys; an unparseable row is reported and
    skipped, never guessed. `--apply` requires an explicit `--model`.
13. **The MCP server writes to stdout outside the protocol.** All logs go
    to stderr; every stdout line is one JSON-RPC message.
14. **The pasteable context anchors on pending scores.** It contains only
    settled outcomes and counts of open claims, never a scorer's verdict
    on a pending item (E6a guard).

## 7. Ceremony

**Mutation class.** Orange overall (new product API, a new read query, a
change to the desk pack query's filter, CLI and MCP surfaces), with one red
touch: the proxy's API-key scope mapping and the set of valid scopes
(auth). Rollback: revert the commits; the records already written stay
readable as ordinary `precommit`/`resolve` commitment records, and the
`calibration` field is ignored by older readers (the body accepts unknown
fields).

**Protected surfaces touched.** API-key scope mapping (proxy) and valid
scope set (auth). Not touched: Texture canonical writes, checkpoint/route
projection, vmctl, gateway/provider calls, run acceptance.

**Conjecture delta.** Conjecture: an agent that sees its own settled
track record (Brier, overconfidence, bins, largest misses) in context
before stating a probability becomes better calibrated over time, without
weight changes. This change makes the conjecture testable outside this
repository; it does not test it. The test needs (a) enough prospective
claims per agent and model (at least ~30 settled before reading anything
into Brier), (b) an A/B: sessions with and without the pasted context,
and (c) a held-out sample never shown, so the agent learns the world and
not the scorer.

**Admissible evidence.** Local tests prove the API, the math, the CLI
and the MCP framing against a local Dolt store. Staging proof (an API key
with `precommit:write`, a claim written and settled through the deployed
proxy) is owed by the lead after landing; nothing here is deployed.

**Heresy delta.** Discovered: none new (F2, empty model ids on desk
records, is already recorded). Introduced: agent identity on these
records is declared by the caller, not bound to the key (R1). Repaired:
none.

**Standing questions.** Single authority: the guest store's commitment
records; the summary is a view recomputed on read. Restart: records are
durable Dolt objects; nothing is in memory. Fate-sharing: a write needs
proxy, vmctl routing and the guest runtime (R3). No-SSH operability: the
whole surface is the CLI under a scoped key.

## 8. Day to day: how the lead (Claude Code) uses it

Once deployed, the owner mints one key per computer with only the
precommit scopes (Settings, or
`choir api-key create --scopes precommit:read,precommit:write --computer <id>`),
stored mode 0600. The lead's session sets its identity once:

```sh
export CHOIR_AGENT=claude-code/lead CHOIR_MODEL=claude-opus-5-5 \
       CHOIR_SESSION=<session id> CHOIR_PROJECT=go-choir CHOIR_COMPUTER=<id>
```

Subagents get `CHOIR_AGENT=claude-code/subagent` (and their own model),
with a `--tag` naming their task.

1. **Start of session:** `choir precommit calibration --text` and paste the
   output into working context; `choir precommit list --status overdue`
   to see what is owed.
2. **Before a checkable claim** ("tests pass", "the fix is live", "no
   behaviour change", "the drill reaches Freeze"): `choir precommit add
   --claim ... --p ... --criterion ... [--deadline ...]`. The ledger's
   rule stands: an unqualified "ready" claim is 0.95 unless stated
   otherwise, and the track record is where that number gets corrected.
3. **When the evidence lands** (CI run, staging check, trace): `choir
   precommit settle <id> --outcome ... --evidence ...`. The lead settling
   a subagent's claim is recorded as independent; settling its own, as
   self. A rule (CI itself) can settle with `--method rule --scorer ci`.
4. **Once:** `choir precommit import-ledger --model claude-opus-5-5
   --apply` brings the level-1 rows in (settled rows as reconstructed, so
   they count only with `--include-reconstructed`).
5. **With MCP instead of shell:** register `choir mcp` as in §5; the same
   four tools, and `calibration_summary`'s first text block is the paste.

What the imported level-1 ledger renders today, for the lead (from the
end-to-end test; four resolved claims, so the text itself says it is too
early to read):

```text
Calibration track record (agent=claude-code/lead; scorer: governing; includes 4 claims reconstructed after the fact).
Settled 4 (4 resolved, 0 unresolvable); open 6, of which 0 past deadline.
Brier 0.553 (0 is perfect; always saying 0.5 scores 0.250); strict Brier 0.553 (unresolvable counted as false).
Overconfidence +0.61: mean confidence in the stated direction 0.86, hit rate 0.25.
By stated p: 0.9-1.0 n=3 held 33%; 0.7-0.8 n=1 held 0%;
4 of the 4 settled claims are settled only by yourself: a learning signal, not independent evidence.
Largest misses: "'the search fix is live' (said before CI finished)" (p 0.95, false); ...
Only 4 resolved: too few to read calibration; keep recording.
```

## 9. Residuals

- **R1. Agent identity is declared.** Any holder of a `precommit:write`
  key can write as any agent name. Next step: bind the agent name to the
  key (the key label, or a key-scoped `agent` claim the proxy injects as a
  trusted header).
- **R2. Held-out sample and A/B.** The summary shows everything settled.
  The experiment needs a held-out 10% never shown and a per-session
  on/off switch for the pasted context.
- **R3. Fate-sharing.** A claim write fails while the computer is
  unreachable. The CLI could queue locally and flush; not built.
- **R4. Desks do not use this yet.** Desks should write the same records
  through their own in-cell module surface (`choir.TrackRecord`, E6a §5),
  not through this HTTP route.
- **R5. Corrections.** A scorer cannot amend its own settlement; another
  scorer appends. An explicit `amends` link may be needed.
- **R6. Scale.** Reads join up to 10,000 records in memory per request.
  Fine for one agent's history; a paged index is needed for an
  organisation's.
- **R7. Import typing.** The level-1 table has no type column, so the
  import types settled rows as assertions and open rows as forecasts (a
  stated rule, not a per-row guess). Rows 7, 10, 11 and 12 were really
  forecasts. The generated goal ledger (`docs/calibration/goal-ledger.md`,
  `scripts/goal_precommitments.py`) is a second level-1 source an import
  could read.
- **R8. The reducer's 10,000-record scan.** `mechanicalResolveForReport`
  lists up to 10,000 commitment records (newest first) to see whether a
  desk precommit is already resolved, a known linear scan
  (precommitment-records, scoring design §1.1). Calibration records count
  toward that window, so a busy external agent brings the day the check
  silently misses an old resolution closer. The desk pack's window is
  protected (§6 item 7); this one is not, because changing it touches the
  lifecycle reducer (red). The fix is the indexed lookup the scoring
  design already asks for.
- **R9. Staging proof owed.** Nothing here is deployed; the acceptance is
  a claim written, settled and summarised through `choir.news` with a
  `precommit:*` key, and a `write:runtime`-only key refused.
