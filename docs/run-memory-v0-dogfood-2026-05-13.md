# Run Memory v0 Dogfood Transcript

Date: 2026-05-13

## Scope

This is the proof note for `docs/mission-run-memory-v0.md`. The v0 target was not a product UI. It was durable provider context for tool-loop runs: persisted messages, persisted compaction checkpoints, bounded context-overflow recovery, observable events, and a child-run/appagent-adjacent path using the same memory substrate.

## Implementation Evidence

New durable substrate:

- `run_memory_entries` SQLite table with ordered entries, parent links, owner/run indexes, message JSON, compaction summaries, first-kept entry IDs, token estimates, reasons, and details.
- Store API: `AppendRunMemoryEntry`, `ListRunMemoryEntries`, and `LatestRunMemoryEntry`.
- Runtime memory manager that initializes tool-loop context from persisted entries, appends assistant/tool/injected/final messages, rebuilds context from latest compaction, and emits compaction/retry events.
- Tool-loop hooks for storage-independent memory behavior: `BeforeProviderCall`, `AfterAppendMessage`, and `OnProviderError`.
- Runtime error classification that marks unrecovered context overflow as `RunBlocked`, while ordinary provider timeouts and context deadlines remain failures or cancellations.

## Continuation Paths Exercised

`TestRuntimeRunMemoryThresholdCompaction` is the main dogfood path. A tool-loop run starts with a user prompt, receives a provider `tool_use`, executes the `echo` tool, persists the assistant tool call and user tool result, then compacts before the second provider call because the test threshold is intentionally tiny. The run continues from rebuilt memory and completes. The test asserts both the durable compaction entry and `loop.compaction.started` / `loop.compaction.completed` events.

`TestRuntimeRunMemoryOverflowRetriesOnceThenCompletes` exercises provider-shaped overflow. The first provider call returns `maximum context length exceeded`; run memory force-compacts, emits `loop.retry`, rebuilds context from the compacted checkpoint, retries once, and completes.

`TestRuntimeRunMemoryOverflowFailureBlocksRun` proves the bounded recovery law. A second overflow after the retry becomes `RunBlocked`, with no `finished_at`, and the error preserves context-overflow evidence.

`TestRuntimeManualRunMemoryCompaction` proves the manual path. A completed tool-loop run is force-compacted through `CompactRunMemory`, which writes a compaction entry with caller-supplied reason and uses the same evented compaction path as automatic threshold compaction.

`TestChildRunUsesRunMemory` starts a child co-super run and verifies the child run has persisted run-memory entries for its user and final assistant messages.

`TestRunMemoryCompactionDoesNotSplitToolResultPair` proves the cut-point invariant: compaction never keeps a `tool_result` message without also keeping its preceding assistant `tool_use` message.

## Verification Commands

Initial local run without ICU flags failed before project packages could build:

```sh
go test ./internal/store ./internal/types ./internal/events ./internal/runtime
```

Failure evidence:

```text
fatal error: 'unicode/regex.h' file not found
```

The repo has local Homebrew ICU headers at `/opt/homebrew/opt/icu4c@78`. With the required flags:

```sh
CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' \
CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' \
CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' \
go test ./internal/store ./internal/types ./internal/events ./internal/runtime
```

Result:

```text
ok github.com/yusefmosiah/go-choir/internal/store
ok github.com/yusefmosiah/go-choir/internal/types
ok github.com/yusefmosiah/go-choir/internal/events
ok github.com/yusefmosiah/go-choir/internal/runtime
```

Full suite:

```sh
CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' \
CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' \
CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' \
go test ./...
```

Result: all packages passed.

## Limits Left For The Next Run

The memory layer is durable, but execution resume after a process restart is still not a full continuation protocol; existing recovery can still mark interrupted running tasks failed. The next step should connect run memory to candidate-world promotion and run resumption semantics.

Compaction summaries are deterministic v0 summaries, not model-authored operational sufficient statistics. That is acceptable for v0 proof, but long leaps need richer summaries with objective, constraints, failed approaches, verification state, rollback points, and user taste updates.

Token counting is approximate. It is sufficient for deterministic threshold tests and first recovery behavior, but provider-specific token accounting should become a verification contract.
