# The Claude delegate is under-equipped (2026-11-03)

Owner, 2026-11-03: "we can have luna delegate to claude", then "I just want
to verify that that works". Cluster 2 of
[the trace-review synthesis](../evidence/selfdev-trace-review-synthesis-2026-11-03.md),
from the luna and claude arm reports beside it.

## Problems and evidence

1. **Thin brief.** The delegate's prompt is `spec.objective` alone
   (`Prompt`, internal/claudedelegate/spec.go). gpt-6-luna squeezes it to
   about 700 characters and drops the shot list. R11a's delegate: "I searched
   the repo ... found no list"; the Calendar delegate: "I never received the
   owner's request text or the list of twelve shots. I picked twelve myself".
   Four rounds.
2. **No build or shot environment.** `Env` drops every `CHOIR_` variable, so
   `CHOIR_FRONTEND_DEPS` (already in the broker's delegate base env) never
   reaches claude, and `CHOIR_BROWSER`/`FONTCONFIG_FILE` were never in that
   base env. R11a: "no $CHOIR_FRONTEND_DEPS or $CHOIR_BROWSER"; Calendar:
   "choir-shot stopped: CHOIR_FRONTEND_DEPS is not set", then `npx vite`
   refused by the registry (403). Luna spent 6.5 minutes doing the shots
   itself.
3. **Copied turn cap.** The overlay's example `"max_turns": 40` is copied
   into every brief. R12 ended `error_max_turns` at 13.4 of its 40 minutes.
4. **Resume cannot work.** Each run gets its own HOME
   (`<run dir>/home`, server.go `Start`), and Claude Code keeps sessions
   under `$HOME/.claude`, so `--resume <session_id>` finds nothing. Luna R12:
   "Fresh delegate for shots 11-12 after resume failed".
5. **"Delegate first" is too narrow.** The overlay keeps "running a check or
   a test" with the engineer and says not to call the delegate for it. Claude
   arm 82187f26: 157 cells, 0 delegate calls; the round was shots, a corpus
   and mocks in /tmp, which the text, read literally, keeps from the delegate.
6. **Network choice shuts the delegate out.** Management Cast `"forbidden"`
   (b45deac0: `failed to listen on 127.0.0.1:8082: operation not permitted`)
   and `"ecosystem_proxy"` (c1d6f9b9, 21f67d48) on a delegate computer; both
   withhold the delegate.

## Fix

A delegate affordance pass (orange; rollback: revert): spec fields `shots`,
`checks` and `owner_words` rendered in full into the prompt; the two store
paths and fontconfig passed to claude; the turn cap derived from the wall
clock; one HOME per capsule so resume works within an assignment; the
overlay's delegate scope widened to the proof work; one management sentence
on leaving the network unset.
