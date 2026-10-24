# Svelte with backticks cannot be written through a Go string literal (2026-10-24)

Status: open when written. Recorded 2026-10-24 by FP0's loop-gaps lane. Mutation
class of this record: green. Gap H of
`docs/design/app-packages-replan-2026-10-24.md` section 5.

## What happened

On the b@c app-rewrite run (`/Users/wiz/Dev/lanes/_runs/apprewrite/log.md`,
the Slides attempt 2 lines), engineering's candidate for `SlidesApp.svelte` came
back as a 12 KB file minified onto 21 lines. The owner rejected it as
unreviewable. The log's trace note: the first write cell used a Go raw string,
the Svelte source holds backticks, the cell failed to compile ("illegal
character NUL", "expected ';'"), and the model then packed the whole file into
one escaped double-quoted string. The request text was patched afterwards with
a placeholder method (`@@BT@@`, restored with `strings.ReplaceAll` before the
write).

## Why, from the code

- `choir.WriteFile(path, content string)` (`internal/yaegikernel/choir.go:457`)
  takes the file as a Go `string` that the cell must spell as a Go literal. The
  only literal that holds a multi-line file without escaping is a raw string,
  and a raw string cannot contain a backtick. Svelte and JavaScript template
  literals use backticks (`` `a ${n} b` ``), so any component that has one ends
  the raw string early.
- Reproduced in this lane (a `yaegikernel` session cell, not on a computer):
  a raw string holding `` const s = `a ${n} b`; `` fails with
  `_.go:3:14: expected ';', found a (and 2 more errors)`, while the same cell
  with the backtick written as the placeholder and restored by
  `strings.ReplaceAll(src, "@@BT@@", "\x60")` writes 21 bytes and returns nil.
  The kernel is not mangling the cell: `normalizeCellSource` re-emits raw
  strings verbatim (`go/format` keeps literal text), and the rejected cell
  failed at the Go scanner.
- The model's remaining ways to write the file are an interpreted string with
  every newline and quote escaped (one line, which is the minified output the
  owner rejected), or hundreds of `+ "\x60" +` splices. Neither reads as a
  natural edit, so the failure pushes the model toward minification. Nothing in
  the engineering prompt says how to write a backtick: the one backtick rule it
  states ("stray backticks indicate a markdown fence leaked into source",
  `internal/runtimeprompts/overlays/rlm_engineering_runtime.yaml`, Error Recovery
  Doctrine) tells the model to remove them.
- Unexplained: the log's "illegal character NUL" error. A raw string cut at its
  first inner backtick yields "expected ';'" errors, not NUL, and I could not
  reproduce a NUL from the evidence on hand. The run's cell source is not in
  the local run directory. This is a hypothesis-free gap: the trace of the
  attempt-2 write cell (on the test computer) would settle it, and the fix below
  does not depend on it.

## Failure modes, written before the fix

1. A Svelte file with template-literal backticks, written in one cell: must
   arrive on disk byte-for-byte, with its newlines and indentation.
2. A file with no backticks written the same way: unchanged.
3. A file that really needs the placeholder text itself (`@@BT@@` in its body):
   the existing `choir.WriteFile` still writes it verbatim. Only the new verb
   substitutes, so no existing write changes meaning.
4. A read-only role (research) must still be refused the new verb, and a desk
   without `WriteFile` must not gain it.
5. The new verb must dispatch the same broker action as `WriteFile`
   (`write_file`), so the daemon's role table and the capsule jail apply
   unchanged.
6. Empty content or a missing path: same error as `WriteFile`.

## Fix chosen

A sibling verb, `choir.WriteFileBT(path, content)`: `WriteFile` after replacing
every `@@BT@@` in `content` with a backtick. It exists in a desk's module
set exactly when `WriteFile` does, and refuses a read-only role exactly as
`WriteFile` does. It uses the placeholder the brief already teaches, so a
running conductor's request text stays valid, and it removes the
`strings.ReplaceAll` step a model forgets or gets wrong. The engineering overlay
gets one sentence naming the verb and replacing the "stray backticks" advice
for file content.

Not chosen: a base64 variant (a model cannot encode 800 lines reliably), a
bytes variant (same quoting problem, in `[]byte("...")`), a staged-blob
`WriteFileFrom(path, ref)` (needs a blob store the cell does not have yet; the
replan names it as the larger design), and a cell-source rewrite (the scanner
cannot tell an inner backtick from a closing one).

## Classification

Fix class: orange (runtime behavior of a desk verb), tier 1. Rollback: revert;
the verb is additive and `WriteFile` is untouched.
