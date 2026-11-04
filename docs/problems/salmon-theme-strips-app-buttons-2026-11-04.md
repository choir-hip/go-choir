# The London Salmon theme strips every app button to bare text (2026-11-04)

Status: fixed in the commit that adds this file.

## Evidence

- `frontend/src/app.css`, the rule
  `:root[data-theme-id='london-salmon'] .app-content :is(button, .button,
  .icon-button, .primary-action, .secondary-action, ...)`, from b6c2781db
  (2026-05-30), set `background: transparent`, `border-color: transparent`
  and `box-shadow: none`, all `!important`. Under that preset no app's own
  button styling could show; only hover and selected states got a fill.
- Self-development rounds were judged on "buttons look like buttons in every
  preset". Under london-salmon they failed again and again on bare-text
  buttons: crouton Calendar r1 to r3, deepseek Calendar r2 to r4 ("Today",
  "Import"), crouton Library r4 (judge, 07:30Z). Engineers fixed their own
  CSS and the theme overrode it. Found by lane C reading the code after the
  repeats (FP0 self-development lane, 2026-11-04).

## Fix

Drop the three resets. The preset keeps its serif italic; buttons take the
base `.app-content` control surface and shadow, which London Salmon defines
in `frontend/src/lib/theme.ts` (`controlBg`, `controlShadow`). Rollback:
revert.
