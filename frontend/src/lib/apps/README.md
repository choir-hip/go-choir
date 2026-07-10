# App Building API

This file is the maintained app-building guide. Earlier API design prose is
available in Git history.

Adding a shell app should be one registry/component operation:

1. Create the Svelte component under `frontend/src/lib`.
2. Add one `ChoirAppDefinition` entry in `registry.ts`.
3. Use `AppSurface` automatically through `AppHost`; use `AppToolbar`, `AppButton`, `AppCard`, `AppList`, `AppTabs`, and `AppEmptyState` for app-local UI.

The registry entry owns app identity, launcher order, desktop icon visibility, mobile switcher participation, singleton/heavy/window geometry, logged-out preview policy, auth-required actions, themed shell data attributes, and surface type. DeskSheet, desktop icons, mobile app switching, app windows, heavy-app metadata, and overview tests derive from the registry projections.

Logged-out preview data is local UI data owned by the component that renders it. Durable/shared/private mutation, provider spend, account data, publish/send/import/upload/activate, rollback/roll-forward, and owner-scoped actions must dispatch `authrequired` at action time.

New app CSS should arrange the app, not define a theme. Prefer the shared primitives and `--choir-*` tokens for panels, controls, text, radii, and shadows so futuristic-noir, carbon-fiber-kintsugi, london-salmon, and future schema-v2 themes apply without per-app rescue selectors.
