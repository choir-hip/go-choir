# Choir Design Language - 2026-06-07

**Status:** design language reference for app and shell work.  
**Related:** [Theme System Contract v0](theme-system-contract-v0.md),
[Universal Wire SourceMaxx spec](./choir-universal-wire-style-vtext-dual-object-spec-2026-06-07.md).  
**Scope:** Choir desktop shell, VText, Universal Wire, and app surfaces across
Futuristic Noir, Carbon Fiber Kintsugi, and London Salmon.

## Purpose

Choir should feel like one coherent owner computer across themes. Themes are
not app-local skins and should not be selected inside individual apps. Theme
selection is OS-wide / desktop-wide state. Apps consume semantic theme tokens
and adapt their typography, surface contrast, and accent use without changing
capability or information architecture.

The design language is:

```text
owned computer
-> editorial work surfaces
-> durable version/source provenance
-> calm dense controls
-> theme-native material
-> readable VText-first content
```

Do not make app screens feel like generic SaaS dashboards. Choir surfaces should
feel like serious tools that can hold documents, sources, agents, and owner
state for long sessions.

## Reference Screenshots

The legal text in these screenshots is incidental. The screenshots are included
to capture theme behavior, typography, material, and control feel.

### Futuristic Noir - VText Reader

![Futuristic Noir VText reader](./assets/design-language/futuristic-noir-vtext-reader-2026-06-07.png)

### Futuristic Noir - Settings / Controls

![Futuristic Noir settings](./assets/design-language/futuristic-noir-settings-2026-06-07.png)

### London Salmon - VText Reader

![London Salmon VText reader](./assets/design-language/london-salmon-vtext-reader-2026-06-07.png)

### Carbon Fiber Kintsugi - VText Reader

![Carbon Fiber Kintsugi VText reader](./assets/design-language/carbon-fiber-kintsugi-vtext-reader-2026-06-07.png)

## Shared Shape

Across all themes, preserve these traits:

- The desktop is spatial: windows, shelf, icons, and prompt surface belong to
  one owner computer.
- App windows are substantial objects with clear titlebars and occluding
  surfaces. Foreground windows must not visually blend into background windows.
- Content areas should be quieter than controls. The reader/work surface is the
  center of gravity.
- VText is the primary reading and editing surface. Other apps should
  transclude or open VTexts instead of reimplementing document readers.
- Source/provenance affordances are visible but secondary. They should support
  inspection without turning the page into a trace dashboard.
- Controls are large enough to use repeatedly, but not so decorative that they
  dominate the content.
- Theme changes may alter mood, typography, contrast, and density. They must
  not hide sources, version state, ownership state, or available actions.

## Theme Personalities

### Futuristic Noir

Futuristic Noir is the default high-agency dark computer.

Use:

- deep blue-black app and desktop surfaces;
- cyan as the primary document/accent signal;
- cool blue selected states and focus rings;
- muted blue-gray body text;
- rounded shell controls with a low-glow material feel.

Avoid:

- turning every surface cyan;
- using neon accents for ordinary metadata;
- letting panels stack into a busy blue dashboard;
- relying on low-contrast gray text for primary reading.

Futuristic Noir works best when content has room and the accent is reserved for
headlines, active controls, citations, and source markers.

### London Salmon

London Salmon is warm editorial paper, not generic light mode.

Use:

- warm paper backgrounds;
- deep wine/brown text;
- salmon or rose shell tint;
- restrained teal/green for source callouts and evidence;
- serif editorial typography where the surface is document-heavy;
- quiet borders and dividers that feel like print rules.

Avoid:

- pure white/black default web styling;
- blue Futuristic Noir residue;
- beige-on-beige low contrast;
- overusing salmon as a fill color for controls;
- making source/provenance callouts look like ads or warning boxes.

London Salmon should feel like a private editorial/legal desktop: readable,
warm, precise, and serious.

### Carbon Fiber Kintsugi

Carbon Fiber Kintsugi is dark, tactile, and mineral.

Use:

- near-black app surfaces;
- warm ivory headings and primary text;
- muted bone/khaki secondary text;
- gold/ochre selected states and source markers;
- olive-brown toolbar material;
- strong contrast through typography, not bright color.

Avoid:

- cyan/blue residue;
- flat black with no material hierarchy;
- overly brown/orange dashboards;
- gold as a decorative border around everything;
- losing body readability by making all text tan.

Carbon Fiber Kintsugi should feel grounded and durable. It is the least "digital
glow" theme and should favor print-like contrast over neon effects.

## Typography

Choir supports two typographic modes:

- **Document/editorial mode:** serif-forward, generous line height, strong
  headings, source callouts that feel like marginalia or citations.
- **Operational/tool mode:** sans-forward, compact labels, clear controls,
  dense but scannable rows.

VText readers lean editorial. Settings, source lists, controls, and operational
apps lean sans. Mixed surfaces such as Universal Wire should use editorial type for
article content and quieter sans metadata for source counts, freshness, filters,
and controls.

Do not scale type with viewport width. Use responsive layout, line length, and
spacing instead.

## Surfaces And Materials

Use the semantic tokens from [Theme System Contract v0](theme-system-contract-v0.md).
New app CSS should express roles, not literal colors.

Surface roles:

- `surface-app`: app body and durable foreground surfaces.
- `surface-pane`: titlebars, toolbars, sidebars, and structural panes.
- `surface-card`: repeated list items only when an item genuinely needs a
  bounded object.
- `surface-control`: buttons, inputs, segmented controls.
- `surface-inset`: low-emphasis wells and code/preformatted regions.

For content-heavy apps, avoid card walls. Use typography, whitespace, and
subtle rules before boxes.

## Universal Wire UI Direction

Universal Wire should adapt this design language as a newspaper-like VText
collection surface.

Reference direction mockups:

![Universal Wire desktop mockup across Futuristic Noir, London Salmon, and Carbon Fiber Kintsugi](./assets/design-language/universal-wire-desktop-three-themes-2026-06-07.png)

![Universal Wire mobile mockup across Futuristic Noir, London Salmon, and Carbon Fiber Kintsugi](./assets/design-language/universal-wire-mobile-three-themes-2026-06-07.png)

These mockups are visual direction, not product proof. The implementation must
still use the real theme tokens, real app shell, product-path data, and browser
screenshots.

If a mockup appears to show story boxes, vertical rules, or repeated text
buttons, treat that as image-generator noise. The product rule is stronger:
article text and whitespace provide the structure.

Desktop:

- no app-local theme selector;
- masthead/status line only, not a marketing header;
- columns of article text, not a grid of cards;
- no borders, rules, or boxes around each story;
- less metadata visible by default, more article;
- source chronology column remains visible and useful;
- provenance/style disclosure is compact and inline;
- full article reading, editing, forking, style changes, and publication happen
  through the normal VText app;
- every article is openable as a VText;
- do not repeat `Open in VText` as visible label text for each article; use a
  small VText icon/glyph, click target, or contextual action instead.

Mobile:

- single-column article flow;
- source chronology reachable as a compact top affordance or section;
- article text remains primary;
- metadata stays inline and sparse;
- no nested panel scrolling.
- mobile remains a responsive app inside the Choir web desktop/shell; do not
  design it as a native phone app with OS-native status bars, notches, or tab
  bars.

Theme behavior:

- Futuristic Noir Universal Wire: dark wire desk with cyan used sparingly for live
  source freshness and citation markers.
- London Salmon Universal Wire: warm broadsheet with wine text and restrained teal
  evidence/source accents.
- Carbon Fiber Kintsugi Universal Wire: black/ivory newspaper with gold source/freshness
  signals and minimal chrome.

## Source And Provenance Display

VText versions carry source provenance. Sources are per-version, not per-VText.
UI should therefore present source/provenance as a property of the visible
version, not as a detached graph panel.

Good patterns:

- inline citation markers;
- compact source callouts;
- "Open source" links;
- version/source disclosure near article actions;
- source chronology as a readable list;
- VText traversal/index views only when they help navigation.

Bad patterns:

- large provenance dashboards inside the reading surface;
- repeated metadata cards;
- source panels that scroll independently inside other panels;
- decorative graph visualizations with no VText traversal purpose.

## Controls

Controls should be calm, tactile, and theme-native.

Use:

- segmented controls for mode choices;
- icon buttons where the command is familiar;
- text buttons for clear editorial commands such as `Revise`, `Sources`,
  and `Publish`;
- compact icon/glyph actions for repeated per-article VText opening;
- compact chips for filters such as source class or region;
- visible selected states that match the active theme.

Avoid:

- theme selectors inside app-specific screens;
- oversized pill controls that compete with article text;
- control rows that repeat the same action in every panel;
- raw accent colors that do not come from semantic tokens.

## Accessibility And Verification

Before shipping theme-sensitive UI:

- verify Futuristic Noir, London Salmon, and Carbon Fiber Kintsugi;
- check foreground windows remain visually opaque over background windows;
- check article text has enough contrast and comfortable line length;
- check metadata does not overpower content;
- check no app-local theme selector was introduced;
- check no nested scroll panels were added to content-heavy surfaces;
- capture screenshots for desktop and mobile when layout changes are material.
