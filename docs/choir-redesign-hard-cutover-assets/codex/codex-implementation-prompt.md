# Codex task prompt

Implement the Choir redesign as a hard cutover.

Do not preserve `BottomBar.svelte`, `.bottom-bar`, `[data-bottom-bar]`, `--choir-bottom-bar-height`, or any bottom-only naming. Rename/replace the old shell control with `PromptSurface.svelte`, introduce `DeskSheet.svelte`, use TetraMark as the Desk button, and support `layout.promptSurfacePlacement: 'top' | 'bottom'` from the schema-v2 theme.

Replace the theme system with exactly three schema-v2 presets: `futuristic-noir`, `carbon-fiber-kintsugi`, and `london-salmon`. Remove old presets and normalize legacy stored themes to the new default.

Update all affected tests. Tests should target `[data-prompt-surface]`, `[data-desk-menu-button]`, `[data-desk-sheet]`, `[data-desk-sheet-app]`, `[data-window-tray-item]`, and `[data-online-indicator]`, not old bottom-bar/start-menu selectors.

Use the implementation brief in this folder as the source of truth.
