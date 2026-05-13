# Podcast Radio Brief Proof - 2026-05-13

## Mission Pressure

The podcast surface was only a feed renderer with audio controls. The next safe deformation was to make it a durable semantic projection without bypassing product topology.

## Change

- Podcast is now exposed as a desktop icon, not only a Start menu app.
- The Podcast app opens to a durable feed library backed by `/api/content/items`.
- A selected RSS content artifact is parsed into feed metadata, stable episode IDs, playable episode counts, and a listen-path ID.
- The listen path can be opened as an initial VText radio brief through the desktop window path.
- VText initial revision metadata can preserve source URL, content artifact ID, app hint, and `created_from=podcast_radio_brief`.

## Verification

- `cd frontend && pnpm build`
- `git diff --check`
- `CHOIR_SERVICES_FOREGROUND=1 ./start-services.sh`
- `cd frontend && npx playwright test podcast-radio-brief.spec.js --workers=1 --timeout=120000`
- `cd frontend && npx playwright test trace-settings-registry.spec.js --workers=1 --timeout=120000`

The Playwright proof seeds a podcast feed through the real `/api/content/items` API, launches Podcast from the desktop icon, selects the durable feed artifact, verifies the listen path and audio controls, opens the radio brief in VText, and verifies the generated VText contains the feed and radio work queue.

## Residual Risk

This is still a narrow radio projection. Clips, narration beats, citations, listen/resume state, and appagent-owned radio updates remain future work. The important invariant is now present: podcast feeds can become VText semantic artifacts through the real desktop/content/VText path.
