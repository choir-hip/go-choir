# Desktop UX review — 2026-07-10

## Mission

Improve Choir's desktop shell without replacing its established visual language.
The review focuses on recovery, keyboard access, focus management, theme editing,
feedback, and small interaction details that make the computer feel deliberate and
dependable.

## Mutation and rollback

- Mutation class: **orange** — frontend interaction and app-state behavior.
- Protected surfaces: none. This work does not change persistence, promotion,
  authentication renewal, VM lifecycle, provider routing, or deployment routing.
- Rollback: revert each atomic `FINDING-NNN` commit independently.
- Acceptance evidence: focused frontend regression tests, production build, and
  browser screenshots/interactions against a locally served frontend with mocked
  session data. Local proof is visual and interaction evidence only; it is not
  staging or platform acceptance.

## Observed problems

### FINDING-001 — failed app loads cannot reliably recover

The proposed Windsurf app-host error state retries the same failed dynamic import.
Browsers cache failed module loads, so the visible Retry action can immediately
return to the same error even after the transient network failure has cleared.
Initial app loading also has no visible progress state, and its spinner does not
respect reduced-motion preferences.

### FINDING-002 — keyboard routes and focus are unreliable

The proposed Command/Control+K shortcut compares `KeyboardEvent.key` to lowercase
`k`, while tested browser events report uppercase `K` with the modifier. The Desk
therefore does not open. Prompt autofocus also runs while the prompt is disabled
and is not retried when desktop startup completes, leaving focus on the document
body. Desktop icons and window controls need consistent keyboard and focus-visible
behavior.

### FINDING-003 — developer theme editing reports false success

The Settings developer editor parses custom theme JSON and announces that it was
applied, but normalization reconstructs the named preset and discards color edits.
The editor then restores the old value. This is a trust problem: the interface says
an action succeeded when it did not.

### FINDING-004 — transient feedback is difficult to control

Desktop notifications have no dismiss action. Error feedback uses a raw white text
literal instead of the theme contract, creating inconsistent contrast authority.
The notification is not announced with an explicit live-region role.

### FINDING-005 — window and Desk actions are ambiguous

Several controls use inconsistent glyphs, and the Desk action always says “Show
Desktop” even when its next action will restore windows. The result is small but
repeated uncertainty in the primary workspace.

## Intended outcome

1. Loading and failures are visible, calm, motion-safe, and recoverable.
2. Core desktop actions work by mouse, keyboard, and assistive technology, with
   intentional focus entry and return.
3. Theme editing either applies validated overrides truthfully or rejects them with
   a precise error; Apply and Revert are explicit.
4. Notifications are announced, theme-safe, and dismissible.
5. Window controls and Desk actions describe the action that will happen next.

## Conjecture delta

The desktop already has a coherent aesthetic. The main remaining UX gap is not a
need for more visual decoration; it is a lack of dependable state transitions and
clear recovery. Improving those transitions should raise perceived quality more
than restyling the shell.

## Heresy delta

- Discovered: false-success theme editing and unrecoverable retry behavior.
- Introduced: none at problem-documentation time.
- Repaired: none at problem-documentation time.
