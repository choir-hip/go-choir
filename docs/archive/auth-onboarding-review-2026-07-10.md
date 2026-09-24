# Auth and onboarding UX review — 2026-07-10

## Mission

Make the signed-out-to-private-computer transition understandable, reassuring,
and easy to recover. Preserve Choir's passkey-only security model and deferred
authentication boundary while improving the words, focus behavior, and choice
architecture around it.

## Mutation and rollback

- Mutation class: **orange** — frontend authentication flow and state behavior.
- Protected surfaces touched: authentication presentation only. Credential
  creation, verification, cookies, renewal, and authorization boundaries remain
  unchanged.
- Rollback: revert each atomic `AUTH-FINDING-NNN` commit independently.
- Acceptance evidence: focused Playwright interaction tests, production frontend
  build, and responsive browser screenshots with mocked signed-out session data.
  Local proof does not establish staging authentication acceptance.

## Observed problems

### AUTH-FINDING-001 — the copy explains Choir's internals instead of the user's next step

The first screen leads with “Keep the preview. Protect the changes” and terms such
as “durable,” “spend-bearing,” and “private computer state.” These phrases encode
product architecture but do not answer the immediate questions: why am I signing
in, what will the browser ask me to do, and will I return to my interrupted action?
The intent panel says “Private action” even for a generic sign-in.

### AUTH-FINDING-002 — returning users are routed through an account-creation-first choice

The overlay defaults to “Create passkey,” while sign-in is labeled “Use passkey.”
The distinction between creating an account and returning to one is indirect.
Switching choices also clears the email address, adding needless re-entry at the
moment a user realizes they chose the wrong path.

### AUTH-FINDING-003 — focus and errors do not help users recover

Opening the modal does not intentionally place focus in the active email field,
closing it does not return focus to the desktop, and Escape is not a supported
exit. Input errors are visually separate from their field, while passkey failures
use ceremony and endpoint terminology instead of explaining what changed and what
to try next.

## Intended outcome

1. The overlay names the interrupted action and promises to return to it.
2. Returning users see “Sign in” first; new users can clearly choose “Create account.”
3. Passkeys are explained in familiar device-language without security theater.
4. Email survives mode switches, focus enters and leaves predictably, and errors
   provide a concrete recovery path.

## Conjecture delta

Deferred authentication is a product strength only if the boundary feels like a
continuation of work, not a wall. Intent-first copy plus predictable focus should
make the passkey prompt feel like a short device check inside the current task.

## Heresy delta

- Discovered: architecture-language presented as onboarding copy; account creation
  prioritized over returning-user sign-in; raw ceremony-style recovery messages.
- Introduced: none at problem-documentation time.
- Repaired: none at problem-documentation time.
