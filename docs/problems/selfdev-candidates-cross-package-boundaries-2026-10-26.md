# A Self-Development Candidate Can Cross Its App's Package Boundary

Date: 2026-10-26. Found by the SR9 investigation (read-only). Problem
first; no fix in this commit. Status: open; fixed by Library steps S2 and
S4 (docs/design/library-package-propagation-2026-10-26.md, sections 3.2 and
3.3, failure mode L15).

An app package has a boundary at build time. A self-development candidate
has none: the same change can touch the package and the shell.

## What happens

The package boundary is enforced where Vite resolves modules
(frontend/scripts/package-boundary-plugin.mjs:1-18). A module under
`src/packages/<name>/` may reach only its own package, the shell's
`src/lib/app-sdk.ts`, the svelte runtime and declared dependencies.
`frontend/scripts/check-packages.mjs` checks the manifest and directory
rules. Both look at what a package imports. Neither looks at what a
candidate changes.

The candidate's path rules stop at safety: an unsafe path or a symlink is
refused (internal/capsule/source_patch.go:70-99), and the host builder
refuses certain paths for a build (internal/hostbuild/service.go:380,
`RefusePatchPaths`). Nothing checks that a change to an app stays inside that
app's directory. `release.Record.Validate` (internal/release/record.go:194-200)
accepts any clean repository path.

## Evidence

- The Reader redesign candidate on the test computer (selfdev-aae30fb7)
  was a request to restyle one app. Its scope was
  `frontend/src/packages/reader`, `frontend/src/app.css`,
  `frontend/src/reader.js`, `frontend/reader-chrome.css`,
  `frontend/read.html` and `frontend/src/lib/theme.ts`
  (docs/definitions/choir-t1a-app-self-development-2026-10-26.md, the `now`
  card). Two of the files are shell files shared by every app:
  `frontend/src/app.css` and `frontend/src/lib/theme.ts`. The mission lists
  app.css as a protected surface ("shared overrides") and records that it
  greys all document text in the Reader today. The other three are the
  Reader's own entry files outside the package.
- Nothing refused the candidate for crossing out of the package. It was
  caught by a person reading the scope.

## Why it matters

- A change to one app can alter every app. A package release that carries it
  carries the shell change to each adopter.
- The package is the unit the Library wants to share. A change that is not
  confined to the package cannot be one, so it must be a platform change
  that goes through review and main.

Fix: Library step S2 refuses a package publish with
`package_change_not_confined`, naming the outside paths, and the platform
part goes through the reviewed path to main. Step S4 exports a package to
main as a client verb with a CI check.
