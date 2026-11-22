# Gateway log lines carry raw guest strings (2026-11-22)

## Problem

The gateway's model-call lines interpolated strings the guest chose, unmodified:
`req.Model`, `req.ToolChoice` and `req.ReasoningEffort` on the inference line
(`internal/gateway/handlers.go`), the model and effort on the OpenAI-compatible
line, the model on the relay lines, and the provider-resolution error text
(which quotes the model). A JSON body can carry spaces, `=` and newlines, and
`log.Printf` writes them as they are. A guest could therefore write a field the
parsers read (` desk=fake run=fake`, a fake `provider=devin model=swe-2-flash`)
or a whole forged, timestamped line (`gateway: jev call ...`).

## Evidence

- Seat 2's review of `b049b8ff4` (first design, station F0) built a fixture shaped
  like the inference line, with `tool_choice=x desk=texture run=fake ... desk=management
  run=real`: `scripts/status-calls-by-desk` took the first ` desk=` and counted
  texture for a management line.
- `scripts/spend-report:49` reads `provider=[a-z-]* model=[^ ]*` from the same line: a
  money report. The vector predates the desk fields.
- The field values are not validated before the line is written (`ProviderRequest`
  fields are plain strings; the model is checked only against the catalog after
  resolution, and the error text then quotes it).

## Belief

Cleaning every guest-supplied string at the line (no space, control character or
non-ASCII byte; bounded; `-` when empty) closes the forgery for the model-call
lines, and a parse of the line's fixed tail (last match) keeps the status script
correct against lines an older gateway wrote raw. The desk and run themselves are
a guest's claim and stay one: any guest can name any desk.

## Rule

In the gateway and provider log lines walked below, no value that comes from a
guest request, and no provider or upstream free text, is written unless it has
passed one of two helpers (lines that carry only our own values need neither; the
table says which) (`internal/provider/logsafe.go`; `logField`, `logModelField` and `logText`
in `internal/gateway` call them):

- `LogField(s, max)`: an identifier (model, effort, tool_choice, artifact ref,
  endpoint, method, host, desk, run) loses spaces, control characters and
  non-ASCII, is cut to `max`, and is `-` when empty. It cannot start a field.
- `LogText(s, max)`: free text (an error, a reason, a summary) is cut to `max` and
  Go-quoted. A newline, a quote or a fake `gateway: ...` line stays inside the quotes.

Under both, `gateway.SafeLogWriter` is the process log's output (`cmd/gateway`):
one log call is one line, whatever a record holds (newline, return and control
characters become escapes). It is a backstop, not the rule.

Readers, as far as they were changed: `scripts/spend-report` and `scripts/status-paper-usage`
accept a success or request row only from a line that starts with the timestamp and
`gateway:`; `status-paper-usage`'s `devin_errors` skips gateway search lines (a guest's
query is written there); `status-hourly`, `status-search-reads` and `status-corpus-reuse`
take the LAST `run=... desk=` on a search request line, so a quoted query cannot supply
the attribution. This is not a claim that every log-reading script is closed; the
remaining error lists what is not.

## Origin of every interpolated value (grep of every `log.Printf`, `log.Print` and `logf`
call in `internal/gateway` and `internal/provider`, 2026-11-22)

Origins: ours (code, config, an authenticated computer id), guest (a field of a
request body or header), provider (an upstream's answer or error text).

| Lines | Values | Origin | Handling |
| --- | --- | --- | --- |
| gateway `handlers.go` inference request | model, tool_choice, reasoning_effort | guest | `logModelField` |
| same | desk, run | guest (headers) | `logField` 64 |
| same | computer id, provider name, counts, media summary | ours (the summary's artifact refs were guest) | refs now `LogField` (`provider/media.go`) |
| `handlers.go` reasoning_off ignored | model | guest | `logModelField` |
| `handlers.go` provider resolution failed | error text (quotes the guest's model) | guest | `logText` (bounded to 300 bytes, quoted; the model route's body cap is 160 MiB) |
| `handlers.go` provider call failed, streaming failed | error text, sanitized text | provider (echoes the guest's model) | `logText` |
| `inferenceSuccessLine` | response model, provider name | provider (the guest's string for bedrock, zai and others via `effectiveModel`) | `logModelField` |
| `handlers.go` search request/for/failed | query (`%q`), run/desk/conv, error text; provider names, hashes | guest; provider (error) | `%q`; `logField`; `logText` |
| `handlers.go` issued/ensure credential lines | computer id, product `%q`, peer `%q`/address, expiry | operator route, ours | none needed |
| `handlers.go` authentication denied, marshal, encode, stream write | our errors | ours | none |
| `openai_compat.go` chat request | model, reasoning_effort | guest | `logModelField` |
| same, resolution failed | error text | guest | `logText` |
| same, provider/streaming/streaming tool call failed (3) | error text | provider | `logText` |
| `anthropic_relay.go` success and failure lines | model (checked against the relay's list), path, desk/run | ours (model from the list); guest (desk/run) | `logModelField`; `logField` |
| same, upstream call failed | upstream error text | provider | `logText` |
| same, refused/denied/budget lines | computer id, VM id | ours | none |
| `circuit_breaker.go` quota/cooling/hard failure | model label | guest | `modelLabel` now cleans (the same label is in error text returned to the guest) |
| same, hard failure | error text | provider | `logText` |
| `jev_limiter.go`, `jev.go`, `host_judgment.go` | computer id/label, class, site (cleaned), desk/run (cleaned), counts; error text on failure | ours; guest (cleaned); provider (error) | `logText` for the error |
| `host_fetch.go` | computer id; method, host; refused reason `%q`; error `%q` | ours; guest (method and host of the request) | `logField` for method and host |
| `source_api.go` | provider name; endpoint; upstream error text | ours; guest (endpoint); provider | `logField`; `logText` |
| `body_caps.go`, `bearer_binding.go`, `call_tally.go`, `devin_pacing.go`, `paid_budget.go`, `spend_file_lock.go`, `search_plane.go` | route, caller (an authenticated computer id), sizes, dates, paths, counts, config values `%q` | ours | none |
| provider `provider.go`, `river.go`, `openrouter.go` call lines | model id, reasoning effort | guest | `LogField` |
| provider response, chat completion, stream complete, stream_usage_missing | response model | provider | `LogField` |
| provider stream error | error type | provider | `LogField` |
| provider `rejected_request.go`, synthetic quota | reason, quota summary | provider | `LogText` (reason was `%q` unbounded) |
| provider resolved lines, key parked lines, OpenCode keys | seed model, region, key names, status | ours (config) | none |

## A wrong reason, corrected

An earlier draft of this record left the provider failure lines raw, saying that
quoting them would change what `scripts/status-paper-usage` matches. That was wrong:
a Go-quoted string keeps the substrings the script reads (`provider quota exhausted
(devin)`, `provider devin model`, `429`, `quota`), and only escapes quotes, backslashes
and control characters. The failure lines are quoted and bounded now
(`handlers.go`, `openai_compat.go`), and fixtures Q1 to Q3 in
`scripts/test-status-scripts.sh` read the old unquoted and the new quoted form.

## Fix (same commit as the desk on every call)

`TestGuestStringsCannotForgeAModelCallLine` and `TestGuestAndProviderTextCannotForgeALogLine`
(hostile model, effort, tool_choice and an upstream error with a newline and a fake
`inference succeeded for autoputer ...` line, through the success, provider failure,
circuit-breaker, OpenAI-compatible and resolution lines, then `scripts/spend-report`
over the captured journal); `TestSafeLogWriterWritesOneLinePerRecord`;
status-script fixtures Q1 to Q4 (old and new line forms) and D7.
`scripts/status-calls-by-desk` takes the last match of the fixed tail.

## Remaining error

- `status-paper-usage`'s `devin_errors` still counts any other timestamped line that
  holds `provider=devin` or `provider devin model` and an error word; the lines that can
  carry guest text and reach it (the quoted failure lines, the circuit-breaker lines)
  hold at most one cleaned or quoted guest value, so a guest can add a count by making
  a real failure line, not by writing a line of its own. A report, not money.
- `paper_lines_20m` counts any line that holds the paper's VM id, so a guest search
  query can keep the "paper not publishing" flag from firing (monitoring only, older).
- Old raw journal lines (written before this change) stay forgeable; only new lines
  are cleaned.
- `modelLabel` now cleans, and the label is also in the "circuit open" error text the
  guest receives and in the breaker snapshot keys: a model that cleans to another
  model's label shares its snapshot entry. Cosmetic for the registry; it is a change
  to a response text, not only to a log.

- A cleaned hostile model survives as one odd model label on the true call (the spend
  report shows it as the served model); it cannot add a row. The label is the guest's
  or the provider's, as before.
- The paper console lines the status script reads as denominators
  (`management story choice`, `story publish:`) are written by guest code; a hostile
  guest can forge them. They are counts for a report, not money.
- Other processes' logs (vmctl, corpusd, the platform) were not walked; this record
  covers `internal/gateway` and `internal/provider`.
- Scripts that read gateway lines, from a grep of `scripts/` and `.github/`: `spend-report`
  and `status-paper-usage` (anchored in this change), `status-calls-by-desk` (fixed tail),
  `proofs/jev-call-stats.sh` (fixed), `status-hourly` and `status-search-reads` (search
  lines: query `%q`, attribution cleaned), `host-fetch-proof.py` and `proofs/net01-host-fetch.sh`
  (fetch lines: method and host cleaned now), `proofs/fp7-gateway.sh`, `paper-liveness-probe.py`
  (success and body-cap lines).
