# Frontend App Building API

The Choir desktop app model is registry-first. Adding an app should be one
component plus one manifest entry, not edits spread across the shell.

## Add An App

1. Create the Svelte app component under `frontend/src/lib`.
2. Add one `ChoirAppDefinition` entry in
   `frontend/src/lib/apps/registry.ts`.
3. Render app-local UI with the shared primitives in
   `frontend/src/lib/apps`: `AppSurface`, `AppToolbar`, `AppButton`,
   `AppCard`, `AppList`, `AppTabs`, and `AppEmptyState`.

The registry entry owns:

- app id, name, icon, description, and component import;
- Desk launcher, desktop icon, and mobile switcher participation;
- launcher ordering;
- singleton/heavy/window geometry policy;
- logged-out preview policy and auth-required action names;
- themed shell data attributes and app surface kind.

`DeskSheet`, desktop icons, mobile app switching, app window hosts, heavy-app
metadata, and Overview metadata derive from registry projections. Do not add a
second app list for a new surface.

## Preview Boundary

Logged-out app surfaces may show local public preview data when the app's
registry policy allows `public-preview` or `public-readonly`. Preview data is
not backend proof, is not private account data, and must never be written into
authenticated user state.

The auth boundary is action-time, not app-open-time. App opening, browsing, and
theme inspection may be public. Durable/shared/private mutation, provider
spend, account data, publish/send/import/upload/activate, rollback/roll-forward,
and owner-scoped actions dispatch `authrequired`.

## Theme Boundary

New apps inherit the three schema-v2 themes through shared primitives and
`--choir-*` tokens. App CSS should arrange the app and express app-specific
information density; it should not redefine theme palettes, toolbar colors, or
button systems unless the shared primitives are missing a real capability.

Current theme ids are:

- `futuristic-noir`
- `carbon-fiber-kintsugi`
- `london-salmon`

## Shell Boundary

The production shell is `Desktop.svelte` plus `PromptSurface`, `DeskSheet`,
`FloatingDesktopIcons`, `FloatingWindow`, and `AppHost`.

The old `BottomBar` surface is deleted. Do not reintroduce bottom-bar
compatibility selectors or app launch lists.

The `CHOIR BIOS` boot console is intentionally separate from the desktop shell.
It remains a bootstrap mode surface for authenticated computer startup and
should not be restyled into the desktop theme vocabulary unless the boot
experience itself is being redesigned.
