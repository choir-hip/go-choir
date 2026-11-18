# The Antigravity product login is suspended, and it served consensus review seat 1

Date: 2026-11-17. Status: cause believed, not proved; panel already changed.
Owner report: Antigravity CLI answered `hi` with a ToS suspension at 19:09
local, about 26 minutes after a Google AI Ultra trial activated on the same
Google account.

## What happened

The Antigravity CLI 1.5.10 accepted the login and served quota refreshes, then
refused generation:

```
PERMISSION_DENIED (code 403): This service has been disabled in this account
for violation of Terms of Service. Please submit an appeal to continue using
this product.
```

Gemini on the web kept working on the same account, so this is not an account
suspension: it is the AI Developer Tools surface (Antigravity, Gemini CLI,
Gemini Code Assist) that is disabled.

## Evidence

- `~/.gemini/antigravity-cli/log/cli-20261117_190835.log:204` and `:206`: the
  403 above, from `https://daily-cloudcode-pa.googleapis.com/v1internal:...`
  (lines 194-204). `:84` shows `applyAuthResult: authMethod=consumer`, and the
  `loadCodeAssist` calls at `:194-198` returned before the refused generation:
  the token and the entitlement were fine, generation was refused.
- Same log family, sessions on 2026-11-12 and 2026-11-15: zero `PERMISSION_DENIED`
  and zero `violation`. The block is new on 2026-11-17.
- omp held an OAuth credential for the same product: `auth_credentials` row
  for provider `google-antigravity` in `~/.omp/agent/agent.db`, `updated_at`
  2026-11-15 21:51 UTC.
- That credential was used as a general model gateway, not only for Gemini.
  `model_usage` in the same database, last-used times:
  `google-antigravity/gemini-4-argon` 2026-10-28, `google-antigravity/claude-sonnet-5-5`
  2026-10-28, `google-antigravity/claude-opus-5-5-medium` 2026-10-08.
- Google's own appeal form (`forms.gle/hGzM9MEUv2azZsrb9`, resolved to
  `docs.google.com/forms/d/e/1FAIpQLScOJWibQ_-hYuVv63kJTgEqlgAaOwRLGFTbXm-QY1gz8U0CGA/viewform`)
  names the violation in its acknowledgement text: "Using third party
  software, tools, or services ... to access my Antigravity/Gemini CLI login".
  The Antigravity Additional Terms, section 6, and the Antigravity FAQ say the
  same, and Google's forum moderators describe an automated suspension on
  detection.
- The panel called it. `skills/agentic-consensus/SKILL.md` pinned
  `google-antigravity/gemini-4-hydrogen-high` as review seat 1, and
  `agentic-consensus-runner.sh` also routed an Anthropic anchor
  (`google-antigravity/claude-opus-4-6`) through the same login.

## Belief state

Believed: the suspension is Google's automated response to omp using the
Antigravity product login as a model gateway — including for non-Google
models — and the Ultra upgrade on 2026-11-17 is what put the account in front
of an abuse review that then found the bridge. Confidence medium-high: the
stated rule and the observed usage match exactly, but no notice names the
tool or the date of the offending traffic.

Not ruled out: the upgrade transaction itself being flagged, or usage from a
different client on the same account. The account has exactly one OAuth
grant family in this repository's tools, so a second bridge is unlikely from
this machine and cannot be excluded elsewhere.

## Still unknown

- Which call Google detected, and when. There is no per-request evidence on
  our side beyond the usage timestamps above.
- Whether the one-time appeal succeeds. The form is currently answering "This
  form is not accepting responses now"; the outcome arrives only as restored
  or not-restored access after 24-72 hours, with no email.

## What changed here

Nothing that calls the provider again. `omp-gemini4-hydrogen` and
`omp-claude-opus46` are deleted from the runner and the probe, not re-pointed
(a re-point is the same violation), and `SKILL.md` records the removal and
the ban. Tier 2 review is blocked until the owner names a seat 1 rather than
being downgraded to one model silently.

The claims scripts carry the same note and refuse rather than call:
`scripts/claims_lib.py` raises on a cache miss for the suspended id (cached
results still read, so a rerun stays reproducible), and
`scripts/l2a_window_pairs.py:judge` raises instead of returning "not served",
which the caller would have counted as a misroute. `scripts/icrl_calibration.py`
reads its judge through `claims_lib`, so `measure` still works on settlements
already written.

## Next step

The owner submits the appeal once, signed in to the affected account, after
revoking third-party OAuth grants at `myaccount.google.com/connections` and
before signing back in. A Gemini seat is rebuilt on a Vertex or AI Studio API
key (`google/gemini-4-hydrogen`), never on the product login. The account's
own access, if it returns, returns through the official `agy` CLI
(`~/.local/bin/agy`, 1.5.10), interactively — never an OAuth bridge. A second
suspension is final.
