// The shell SDK's version (runtime install, S12;
// docs/design/s12-runtime-install-build-2026-11-04.md). An installed package
// names the range it needs (choir-package.json app.requires_sdk, "^MAJOR.MINOR",
// default "^1.0"); the shell loads it only when SDK_VERSION is in range. Bump
// MINOR when app-sdk.ts gains a name, MAJOR when one changes or leaves, then
// write the new lock: node scripts/sdk-surface.mjs --write. CI refuses a
// surface change without the bump it needs (src/lib/sdk-surface/<version>.json;
// docs/design/modularity-check-build-2026-11-04.md).
export const SDK_VERSION = '2.0.0';

/** Whether version (MAJOR.MINOR.PATCH) satisfies range (^MAJOR.MINOR). */
export function sdkSatisfies(range: string, version: string = SDK_VERSION): boolean {
  const r = /^\^(\d{1,4})\.(\d{1,4})$/.exec(String(range || ''));
  const v = /^(\d+)\.(\d+)\.(\d+)$/.exec(String(version || ''));
  if (!r || !v) return false;
  return Number(v[1]) === Number(r[1]) && Number(v[2]) >= Number(r[2]);
}
