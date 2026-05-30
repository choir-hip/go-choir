# Auth Passkey Info Hover Reflow Problem

**Date:** 2026-05-30

## Problem

The auth entry screen renders the passkey information text as a normal block
inside `.auth-view` and reveals it with:

```css
.auth-view h2:has(.passkey-info:hover) ~ .passkey-tooltip
```

On Chrome, Safari, and Comet, hovering the `ⓘ` control in "Create a passkey ⓘ"
can make the surrounding auth card expand. Atlas did not reproduce the same
visible failure, which suggests browser-specific handling around `:has()`,
inline heading layout, hover invalidation, and block insertion into normal flow.

The product problem is not merely the `ⓘ` glyph. The auth card should have a
stable layout while optional explanatory text appears. Hover/focus/click hints
must not resize the card, shift the email input, or change the passkey action
target.

## Strategy

Use the installed `@chenglou/pretext` library for the first minimal Pretext
integration:

- Treat the hint body as a small manually-laid-out text block.
- Use Pretext to split the text into rendered lines for the current viewport
  width.
- Render those lines in an absolutely positioned popover anchored to the
  `ⓘ` control, outside normal document flow.
- Remove the `:has()`-driven hover selector so browser hover invalidation does
  not control card height.

## Acceptance

- Hovering or focusing the `ⓘ` control shows the passkey explanation without
  changing the auth card dimensions.
- Clicking the `ⓘ` control remains available for touch and keyboard users.
- Register and login variants both render their specific copy.
- The implementation imports and exercises `@chenglou/pretext`.
