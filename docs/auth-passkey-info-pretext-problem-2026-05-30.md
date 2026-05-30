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
integration, but do not treat Pretext as a line-wrapping helper inside an
ordinary tooltip. That was a useful mitigation because it removed the normal
flow block insertion, but it did not exercise the core Pretext model.

The passkey heading and disclosure should become one measured inline
micro-layout:

- Use Pretext's rich-inline flow to prepare heading text, the `ⓘ` affordance as
  an atomic inline chip, and the explanatory copy as caller-owned fragments.
- Compute both collapsed and expanded layouts from the current card width and
  reserve the maximum required disclosure lane height up front.
- Render the measured lines/fragments directly instead of letting browser text
  flow decide whether hover changes card geometry.
- Keep the `ⓘ` as a real button for keyboard, pointer, and touch users, but
  place it inside the Pretext-rendered fragment stream.
- Remove the `:has()`-driven hover selector and any normal-flow block insertion
  caused by hover state.

This is intentionally a small pilot for the future VText/transclusion system:
Pretext owns text measurement and line reflow, while Choir owns the semantic
fragments and interactive affordances.

## Acceptance

- Hovering or focusing the `ⓘ` control shows the passkey explanation without
  changing the auth card dimensions.
- Clicking the `ⓘ` control remains available for touch and keyboard users.
- Register and login variants both render their specific copy.
- The implementation imports and exercises `@chenglou/pretext/rich-inline`, not
  only `layoutWithLines()` inside a popover.
- The rendered disclosure exposes measured Pretext lines/fragments for tests.
