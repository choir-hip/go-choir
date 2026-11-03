# App Building API

This file is the maintained app-building guide. Earlier API design prose is
available in Git history.

Adding a shell app should be one registry/component operation:

1. Create the Svelte component under `frontend/src/lib`.
2. Add one `ChoirAppDefinition` entry in `registry.ts`.
3. Use `AppSurface` automatically through `AppHost`.

The registry entry owns app identity, launcher order, desktop icon visibility, singleton/heavy/window geometry, logged-out preview policy, themed shell data attributes, and surface type. DeskSheet, desktop icons, app windows, heavy-app metadata, and overview tests derive from the registry projections.

Logged-out preview data is local UI data owned by the component that renders it. Durable/shared/private mutation, provider spend, account data, publish/send/import/upload/activate, rollback/roll-forward, and owner-scoped actions must dispatch `authrequired` at action time.

New app CSS should arrange the app, not define a theme. Prefer the shared primitives and `--choir-*` tokens for panels, controls, text, radii, and shadows so futuristic-noir, carbon-fiber-kintsugi, london-salmon, and future schema-v2 themes apply without per-app rescue selectors.

## Context menus, Open with, launch files and upload

Every app, a package included, gets these from `app-sdk.ts`:

- `use:contextMenu={items | () => items | { items, label, longPress }}` gives an
  element a context menu: right-click, a long press on touch, and Shift+F10 or
  the Menu key. `openContextMenu({ x, y, items, label })` opens one at a point.
  An item is `{ id, label, icon, shortcut, disabled, danger, onSelect, items }`
  (one level of `items` makes a submenu); `{ separator: true }` is a rule.
- A package declares the files it opens in `choir-package.json`:
  `"opens": [{ "media_type": "...", "extensions": ["pptx"], "default": false }]`.
  Extensions are lower case without the dot. An entry without `default` (or
  `true`) makes the package the default opener, which no other app may also
  claim; `"default": false` lists the package under Open with only, so it may
  name an extension another app already opens. `openersForFile(name)` answers
  the list Files shows.
- `getLaunchFile()` (during component setup) returns the file the window was
  opened on, `{ path, name, mediaType, url, sourceUrl, openedWith }`, or null;
  `launchFileStore()` follows the window when the shell hands it a new file.
- `pickAndUploadFiles({ accept, folder, multiple })` opens the system picker and
  uploads by Files' own route (PUT /api/files/<folder>/<name>); it resolves to
  the stored paths and rejects with an `UploadError` (`paths`, `failures`,
  `authRequired`). `uploadFiles(files, { folder })` uploads files you already
  have, and `use:dropZone={{ folder, accept, onUploaded, onError }}` uploads
  dropped files (the node carries `data-drop-active` while files hover).

## Owner-only views

No platform-owner signal reaches packages yet; hide owner-only views. The
shell reads `/api/shell/bootstrap` once in `Desktop.svelte` (`bootstrapData`,
which carries `computer_id` and no owner flag), keeps it there, and exports
nothing about it from `app-sdk.ts`. The platform owner is a server-side identity
(`vmctl.UniversalWirePlatformOwnerID` in the proxy), not something the frontend
knows. Do not invent a client check; add an SDK export when a signal exists.
