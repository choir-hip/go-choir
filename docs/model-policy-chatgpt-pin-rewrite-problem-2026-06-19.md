# Model Policy ChatGPT Pin Rewrite Problem - 2026-06-19

## Problem

An explicit owner-visible model policy that pins foreground roles to ChatGPT can
be rewritten back to generated defaults when the runtime resolves
`System/model-policy.toml`.

Observed on Node B on 2026-06-19:

- `PUT /api/files/System/model-policy.toml` returned `200` and `file saved`.
- A subsequent `GET /api/files/System/model-policy.toml` showed generated
  DeepSeek/Xiaomi defaults instead of the written ChatGPT policy.
- `/api/model-policy/resolve?role=texture` continued to resolve from
  `/mnt/persistent/files/System/model-policy.toml` to Xiaomi/DeepSeek defaults.

The root cause is the legacy migration guard in
`shouldMigrateLegacyGeneratedModelPolicy`: a policy containing explicit ChatGPT
foreground pins can still match legacy generated-policy heuristics and be
overwritten by `defaultModelPolicyText`.

## Impact

Owner or operator attempts to change per-computer model routing through the
product file surface may appear successful but silently lose the intended
provider/model selection on the next model-policy resolution. This is especially
visible when an account lacks DeepSeek credits and needs foreground roles routed
to available ChatGPT models.

## Desired State

Generated policy should use the current ChatGPT foreground defaults:

- conductor: `chatgpt` / `gpt-5.4-mini` / `medium`
- texture: `chatgpt` / `gpt-5.5` / `low`
- researcher: `chatgpt` / `gpt-5.4-mini` / `medium`
- super: `chatgpt` / `gpt-5.5` / `medium`

Legacy `roles.vtext` should not be generated. Explicit modern ChatGPT policies
must be preserved by migration. Texture's Sources panel should show the effective
model-policy resolution for conductor, Texture, researcher, and super so this
state is inspectable without SSH.

## Evidence Class

Admissible evidence is a focused model-policy unit test, a frontend build or
type check for the Sources panel change, and deployed
`/api/model-policy/resolve` responses after the change reaches staging.

## Rollback

Revert the platform commit and restore Node B model-policy file backups from
`/var/lib/go-choir/model-policy-backups/20260619T143956Z` if needed.
