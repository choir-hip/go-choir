# MissionGradient: App Building API Hard Cutover - v0

Status: draft
Target branch: `codex/redesign-hard-cutover-node-a`
Primary acceptance host: `https://choir-ip.com`
Protected host: `node-b` / `https://choir.news`

## Goal String

```text
/goal Run docs/mission-app-building-api-hard-cutover-v0.md as a Codex-operated MissionGradient mission on branch codex/redesign-hard-cutover-node-a. Preserve Node A as the disposable design lab at https://choir-ip.com and do not touch main, Node B, choir.news, production auth, production mail, Resend behavior, or production secrets. Hard-cut the frontend app-building API so adding a new app is one manifest/component operation instead of edits across Desktop.svelte, PromptSurface, DeskSheet, DesktopIcons, overview tests, theme selectors, and fixture lists. Use via negativa: delete scattered app lists, delete long appId render branches, delete duplicated launcher order, delete theme-specific per-app rescue CSS where tokenized primitives can replace it, and delete compatibility cruft rather than wrapping it. Introduce a typed AppDefinition contract that owns id, name, icon, component, launcher visibility/order, desktop-icon visibility, singleton/heavy/window geometry, logged-out preview policy, auth-required actions, fixture/demo context, themed shell data attributes, and app surface type. Render DeskSheet, desktop icons, mobile app switcher, DesktopOverview metadata, and app windows from the same registry. New apps must inherit all three schema-v2 themes through shared AppSurface/AppToolbar/AppButton/AppCard/AppList primitives and choir theme tokens, with no hardcoded dark toolbar failure like the London Salmon Files regression. Maintain the logged-out preview invariant: app opening and fixture browsing are public, while durable/shared/private mutation, provider spend, account data, publish/send/import/upload/activate, rollback/roll-forward, and owner-scoped actions request auth at action time. Prove the cutover by migrating every current app in the registry, adding one tiny fixture-only sample app behind the manifest if useful as a verifier, running build and Playwright regression, and using Computer Use to inspect DeskSheet, desktop icons, mobile prompt switching, Files, VText, Trace, Settings, media apps, and DesktopOverview across futuristic-noir, carbon-fiber-kintsugi, and london-salmon. Stop only when app addition has a documented minimal API, old scattered wiring is deleted, all current apps still open logged out, and Node A deployed health plus visual evidence confirm the branch commit; otherwise stop with the exact invariant-level blocker and the smallest safe next probe.
```

## Real Artifact

The artifact is not a new abstraction for its own sake. It is a smaller frontend shell where app identity, launcher presence, window behavior, auth boundary, fixtures, and theme participation are declared once.

The acceptance smell to remove is: "I added an app and had to remember six unrelated files."

## Current Evidence

Owner QA on 2026-05-29 found a concrete failure: Files in London Salmon displayed a dark local toolbar with low-contrast dark/oxblood controls. That happened because app-local CSS and global theme affordance CSS were allowed to disagree. This class of bug will recur for every future app unless the app-building API makes theme participation automatic.

Current duplicated surfaces include:

- `APP_REGISTRY` in `frontend/src/lib/stores/desktop.js`.
- `launcherAppIds` in `frontend/src/lib/PromptSurface.svelte`.
- Desktop icon filtering in `frontend/src/lib/stores/desktop.js`.
- A long `win.appId === ...` render chain in `frontend/src/lib/Desktop.svelte`.
- Hardcoded app/test lists in `frontend/tests/prompt-surface-hard-cutover.spec.js`.
- Global theme rescue selectors in `frontend/src/app.css` that must know too much about app internals.

## Cognitive Transforms

### Via Negativa

The mission succeeds by deleting obligations:

- Delete scattered app lists.
- Delete app render switchboards.
- Delete duplicated launcher order.
- Delete local dark toolbar defaults where shared primitives should own surface styling.
- Delete new-app instructions that say "also remember to edit tests, Desk, prompt, desktop icons, and themes."

### Real Object

The real object is an app-to-shell contract. A new app should declare how it appears, opens, previews, gates auth, and consumes theme tokens. The shell should not rediscover that knowledge through string comparisons.

### Affordance Invariant

If an app is launchable, it appears in DeskSheet automatically. If it is desktop-visible, it appears as a desktop icon automatically. If it is open, it appears in mobile switching and DesktopOverview automatically. If it uses shared primitives, all current and future themes apply automatically.

### Anti-Registry Bloat

A manifest is useful only if it replaces wiring. A second metadata file that still requires editing the old branches is failure.

## Invariants

- Do not touch Node B or `main`.
- Do not weaken logged-out preview boundaries.
- Do not send fixture data to authenticated backend state.
- Do not introduce a plugin system that hides simple Svelte imports behind runtime magic.
- Do not preserve old render branches as a fallback after the new host works.
- Do not require theme-specific CSS for each new app.
- Do not make tests depend on a stale manually copied app list when they can derive expectations from the registry or a generated stable projection.

## Target API Shape

```ts
export type ChoirAppDefinition = {
  id: string;
  name: string;
  icon: string;
  description: string;
  component: ComponentType;
  launcher: {
    desk: boolean;
    desktopIcon: boolean;
    mobileSwitcher: boolean;
    order: number;
  };
  window: {
    singleton: boolean;
    heavy: boolean;
    desktop?: WindowGeometry;
    compact?: WindowGeometry;
  };
  auth: {
    preview: 'public-demo' | 'public-readonly' | 'private';
    requiresAuthFor: string[];
  };
  theme: {
    surface: 'standard' | 'document' | 'media' | 'terminal';
    shellDataAttr: string;
  };
  fixtures?: {
    loggedOutContext?: Record<string, unknown>;
  };
};
```

## Shared Primitives

The hard cutover should introduce or consolidate primitives before app-specific polish:

- `AppSurface`
- `AppToolbar`
- `AppButton`
- `AppCard`
- `AppList`
- `AppEmptyState`
- `AppTabs`

These components consume `--choir-*` tokens and theme radii/shadows. App CSS may arrange domain-specific layout, but should not redefine a theme from scratch.

## Dense Feedback

- Build and focused Playwright after each migration slice.
- Computer Use checks at mobile and desktop sizes after the first full registry-host pass.
- A visual matrix for all app registry entries under all three themes.
- A simple "new app addition" verifier: adding a fixture-only sample app should require only one manifest/component entry, then appear in DeskSheet and theme tests automatically.

## Stopping Condition

Stop only when:

- All current apps render through the new app host.
- DeskSheet, desktop icons, mobile switcher, and DesktopOverview derive from the same app definitions.
- Scattered old lists/render branches are deleted.
- The London Salmon Files-toolbar class of bug is structurally prevented by shared primitives or tokenized surface selectors.
- Node A is deployed at the final commit and `https://choir-ip.com/health` reports that commit.
- Build, focused Playwright, and Computer Use evidence are recorded.

## Run Checkpoint & Resumption State

status: draft
last checkpoint: mission authored from owner QA after London Salmon Files toolbar regression
current artifact state: Node A redesign branch has coherent app visuals but app wiring remains scattered
what shipped: not yet executed
what was proven: Files toolbar regression shows app-local CSS can escape the theme system
unproven or partial claims: no app-building API exists yet; no registry-host migration is complete
belief-state changes: maintainability is now a first-class redesign acceptance criterion
remaining error field: scattered app wiring, duplicated launcher lists, app-local theming gaps
highest-impact remaining uncertainty: how small the typed app contract can be while deleting all current switchboard wiring
next executable probe: implement a thin AppDefinition/AppHost slice for one easy app and one complex app, then delete the corresponding `Desktop.svelte` render branches
suggested resume goal string: use the Goal String above
evidence artifact refs: owner screenshot of London Salmon Files toolbar regression, 2026-05-29
rollback refs: branch history before this mission document
