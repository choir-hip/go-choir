package sourceproj

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Problem 14 (docs/problems/selfdev-arm-rounds-fail-at-freeze-and-capsules-lack-real-content-2026-11-03.md):
// a rebase cannot drop a hunk of the applied change P that the new base B'
// already carries. A dropped hunk is accepted, and recorded as upstreamed,
// only when B' holds that hunk's content. Ways the acceptance can go wrong,
// written before the code:
//   - partial overlap: B' carries some of a hunk's lines, not all, and the
//     whole hunk is dropped and passes (the rest of P's change is lost);
//   - the content was in B already (a common line such as `if err != nil {`
//     or `return err`), B' added a copy elsewhere, and a hunk that only adds
//     such lines is dropped as "upstreamed";
//   - B' carries the same names in another statement: P adds a guard
//     `if (!ok) throw new AuthRequiredError()`, B' only re-exports
//     AuthRequiredError, and the guard is dropped;
//   - a hunk whose meaning is in a string literal (`const route = '/admin'`)
//     is matched by a B' line with another literal (`'/public'`);
//   - a hunk that also removes lines is "upstreamed" while B' still has the
//     removed lines (the removal never happened);
//   - a comment-only hunk passes on no evidence (nothing to match);
//   - whitespace: B' wrote the same statement with other spacing or line
//     breaks and the hunk is refused (a false refusal; claude's case wrote
//     the exports in another shape and another place);
//   - the drop is silent: the verdict, the receipt and Freeze do not say it;
//   - a path that applies cleanly to B' (no conflict) but whose hunk B'
//     already carries elsewhere is refused (the candidate cannot keep a
//     duplicate export).

// claudeStackBase is frontend/src/lib/app-sdk.ts at 4036f0c37a84, the claude
// arm's stack base; claudeBootedBase is the file at eb1fb6408 (main's
// e5915e0ca re-exports); claudeHunk is round 3's app-sdk hunk, verbatim.
const claudeStackBase = `// The platform API a package may use (docs/design/app-packages-replan-2026-10-24.md §3.1).
//
// A first-party package under src/packages/<name>/ may import ` + "`svelte`" + `, its own
// files, and this file. ` + "`scripts/check-packages.mjs`" + ` refuses every other import
// at build time. Add a re-export here when a package needs a shell function;
// do not reach past it. This is the start of the "platform API range" a lock
// entry declares later.

// Media and file helpers (the shell's auth-renewing fetch and its upload live
// behind these).
export {
  appTitle,
  apiFileURL,
  clampNumber,
  loadContextContentItem,
  loadRecentMedia,
  mediaSourceIdentity,
  recentMediaAppContext,
  rememberRecentMedia,
  resolveMediaSource,
  uploadMediaFile,
} from './media-utils.js';

// Live events (the synced computer state channel).
export { addLiveEventListener, liveEventKind, liveEventPayload } from './live-events.js';
`

const claudeBootedBase = `// The platform API a package may use (docs/design/app-packages-replan-2026-10-24.md §3.1).
//
// A first-party package under src/packages/<name>/ may import ` + "`svelte`" + `, its own
// files, and this file. ` + "`scripts/check-packages.mjs`" + ` refuses every other import
// at build time. Add a re-export here when a package needs a shell function;
// do not reach past it. This is the start of the "platform API range" a lock
// entry declares later.

// Media and file helpers (the shell's auth-renewing fetch and its upload live
// behind these).
export {
  appTitle,
  apiFileURL,
  clampNumber,
  loadContextContentItem,
  MEDIA_FILE_ROUTES,
  mediaRouteForFileName,
  loadRecentMedia,
  mediaSourceIdentity,
  recentMediaAppContext,
  rememberRecentMedia,
  resolveMediaSource,
  uploadMediaFile,
} from './media-utils.js';

// Live events (the synced computer state channel).
export { addLiveEventListener, currentDeviceId, liveEventKind, liveEventPayload } from './live-events.js';

// Auth: the renewing fetch every /api call goes through, and its error when
// the session cannot be renewed.
export { AuthRequiredError, fetchWithRenewal } from './auth.js';

// The app registry: which app opens which file kind (Open with).
export { PACKAGE_MEDIA_ROUTES, getAppDefinition } from './apps/registry';
`

const claudeHunk = `// The shell's auth-renewing fetch and the media routes (which app opens which file).
export { fetchWithRenewal, AuthRequiredError } from './auth.js';
export { mediaRouteForFileName, MEDIA_FILE_ROUTES } from './media-utils.js';

`

const sdkPath = "frontend/src/lib/app-sdk.ts"

// upRepo is a repository with B (file at base) and B' (file at booted) on one
// path, and P made on B by edit.
type upRepo struct {
	repo, base, booted string
	p                  []byte
}

func newUpRepo(t *testing.T, path, base, booted string, edit func(root string)) *upRepo {
	t.Helper()
	repo, root := baseRepo(t)
	b := platformMoved(t, repo, root, func(r string) { write(t, r, path, base) })
	p := patchFrom(t, repo, b, edit)
	scratch := t.TempDir()
	tgit(t, scratch, "clone", "-q", "--no-checkout", repo, ".")
	tgit(t, scratch, "checkout", "-q", b)
	write(t, scratch, path, booted)
	write(t, scratch, "frontend/src/platform-other.txt", "platform\n")
	tgit(t, scratch, "add", "-A")
	tgit(t, scratch, "commit", "-q", "-m", "platform")
	b2 := tgit(t, scratch, "rev-parse", "HEAD")
	tgit(t, repo, "fetch", "-q", scratch, "HEAD:refs/platform/up")
	return &upRepo{repo: repo, base: b, booted: b2, p: p}
}

func (u *upRepo) verify(t *testing.T, edit func(root string)) RebaseVerdict {
	t.Helper()
	cand := patchFrom(t, u.repo, u.booted, edit)
	v, err := VerifyRebase(context.Background(), u.repo, RebaseCheck{StackBase: u.base, BootedBase: u.booted, Patch: u.p, CandidateBase: u.booted, Candidate: cand})
	if err != nil {
		t.Fatalf("VerifyRebase: %v", err)
	}
	return v
}

func insertAt(t *testing.T, root, rel, anchor, text string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	raw, err := os.ReadFile(full)
	if err != nil {
		t.Fatal(err)
	}
	i := strings.Index(string(raw), anchor)
	if i < 0 {
		t.Fatalf("anchor %q not in %s", anchor, rel)
	}
	i += len(anchor)
	if err := os.WriteFile(full, []byte(string(raw[:i])+text+string(raw[i:])), 0o644); err != nil {
		t.Fatal(err)
	}
}

const pkgFile = "frontend/src/packages/files-next/index.ts"

// TestVerifyRebaseClaudeCaseDropsTheUpstreamedAppSDKHunk is the 11:15Z
// failure: claude's round 3 added app-sdk re-exports, main added the same
// names in another shape, the correct rebase leaves app-sdk.ts as B' has it.
func TestVerifyRebaseClaudeCaseDropsTheUpstreamedAppSDKHunk(t *testing.T) {
	u := newUpRepo(t, sdkPath, claudeStackBase, claudeBootedBase, func(root string) {
		insertAt(t, root, sdkPath, "} from './media-utils.js';\n\n", claudeHunk)
		write(t, root, pkgFile, "export const files = 1;\n")
	})
	// The package path survives; app-sdk.ts is left as the new base has it.
	v := u.verify(t, func(root string) { write(t, root, pkgFile, "export const files = 1;\n") })
	if !v.Pass {
		t.Fatalf("claude's rebase must pass with the app-sdk hunk upstreamed: %+v", v)
	}
	if len(v.Upstreamed) != 1 || v.Upstreamed[0].Path != sdkPath || v.Upstreamed[0].Hunk == "" {
		t.Fatalf("the drop must be recorded as upstreamed: %+v", v.Upstreamed)
	}
	if strings.Join(v.Resolved, ",") != sdkPath {
		t.Fatalf("app-sdk.ts is a resolved path: %+v", v)
	}
	// Dropping the package file too is still a missing change.
	if v := u.verify(t, func(root string) { write(t, root, "frontend/src/other.txt", "x\n") }); v.Pass {
		t.Fatalf("a candidate without the package file passed: %+v", v)
	}
	// Keeping claude's hunk as well (a duplicate export) is not what a correct
	// rebase leaves either: the merge with the hunk is still accepted, as before.
}

// The same with no conflict: B' put the exports at the end, so P's hunk still
// applies cleanly at its place; a candidate that keeps a duplicate would not
// build, and the one that drops it passes.
func TestVerifyRebaseACleanPathWhoseHunkTheBaseCarriesElsewhere(t *testing.T) {
	booted := claudeStackBase + "\nexport { AuthRequiredError, fetchWithRenewal } from './auth.js';\nexport { MEDIA_FILE_ROUTES, mediaRouteForFileName } from './media-utils.js';\n"
	u := newUpRepo(t, sdkPath, claudeStackBase, booted, func(root string) {
		insertAt(t, root, sdkPath, "} from './media-utils.js';\n\n", claudeHunk)
		write(t, root, pkgFile, "export const files = 1;\n")
	})
	v := u.verify(t, func(root string) { write(t, root, pkgFile, "export const files = 1;\n") })
	if !v.Pass || len(v.Upstreamed) != 1 {
		t.Fatalf("a clean path whose hunk B' carries elsewhere: %+v", v)
	}
	for _, p := range v.Clean {
		if p == sdkPath {
			t.Fatalf("an upstreamed path is not applied unchanged: %+v", v)
		}
	}
}

func TestVerifyRebaseAPartlyUpstreamedHunkMustSurvive(t *testing.T) {
	// B' re-exports the auth names but not the media routes.
	booted := strings.Replace(claudeBootedBase, "  MEDIA_FILE_ROUTES,\n  mediaRouteForFileName,\n", "", 1)
	u := newUpRepo(t, sdkPath, claudeStackBase, booted, func(root string) {
		insertAt(t, root, sdkPath, "} from './media-utils.js';\n\n", claudeHunk)
	})
	v := u.verify(t, func(root string) { write(t, root, "frontend/src/other.txt", "x\n") })
	if v.Pass || len(v.Upstreamed) != 0 {
		t.Fatalf("a hunk B' carries only in part was dropped: %+v", v)
	}
}

const goBase = "package x\n\nfunc A() error {\n\terr := a()\n\t_ = err\n\treturn nil\n}\n\nfunc B() error {\n\tif err := b(); err != nil {\n\t\treturn err\n\t}\n\treturn nil\n}\n"

func TestVerifyRebaseCommonLinesAreNotUpstreamed(t *testing.T) {
	// P adds an error check in A; B' adds the same lines in a new function C.
	// Every token was in B already: nothing says B' made P's change.
	pAdd := "\tif err != nil {\n\t\treturn err\n\t}\n"
	booted := goBase + "\nfunc C() error {\n\terr := c()\n" + pAdd + "\treturn nil\n}\n"
	const path = "internal/y.go"
	u := newUpRepo(t, path, goBase, booted, func(root string) {
		insertAt(t, root, path, "\terr := a()\n", pAdd)
	})
	v := u.verify(t, func(root string) { write(t, root, "frontend/src/other.txt", "x\n") })
	if v.Pass || len(v.Upstreamed) != 0 {
		t.Fatalf("a hunk of common lines was dropped as upstreamed: %+v", v)
	}
}

func TestVerifyRebaseTheSameNamesInAnotherStatementAreNotUpstreamed(t *testing.T) {
	base := "import { x } from './x.js';\n\nexport function f(ok) {\n  return x(ok);\n}\n"
	booted := base + "\nexport { AuthRequiredError } from './auth.js';\n"
	const path = "frontend/src/lib/f.ts"
	u := newUpRepo(t, path, base, booted, func(root string) {
		insertAt(t, root, path, "export function f(ok) {\n", "  if (!ok) throw new AuthRequiredError();\n")
	})
	v := u.verify(t, func(root string) { write(t, root, "frontend/src/other.txt", "x\n") })
	if v.Pass || len(v.Upstreamed) != 0 {
		t.Fatalf("a guard was dropped because B' re-exports its error: %+v", v)
	}
}

func TestVerifyRebaseAnotherStringLiteralIsNotUpstreamed(t *testing.T) {
	base := "export const name = 'files';\n"
	booted := base + "export const route = '/public';\n"
	const path = "frontend/src/lib/r.ts"
	u := newUpRepo(t, path, base, booted, func(root string) {
		insertAt(t, root, path, "'files';\n", "export const route = '/admin';\n")
	})
	v := u.verify(t, func(root string) { write(t, root, "frontend/src/other.txt", "x\n") })
	if v.Pass || len(v.Upstreamed) != 0 {
		t.Fatalf("a hunk with another literal was dropped as upstreamed: %+v", v)
	}
}

func TestVerifyRebaseAHunkThatRemovesLinesIsNotUpstreamed(t *testing.T) {
	// P replaces the live-events line; B' adds the new names elsewhere but
	// keeps the old line.
	u := newUpRepo(t, sdkPath, claudeStackBase, claudeStackBase+"export { currentDeviceId } from './live-events.js';\n", func(root string) {
		write(t, root, sdkPath, strings.Replace(claudeStackBase, "addLiveEventListener, liveEventKind", "addLiveEventListener, currentDeviceId, liveEventKind", 1))
	})
	v := u.verify(t, func(root string) { write(t, root, "frontend/src/other.txt", "x\n") })
	if v.Pass || len(v.Upstreamed) != 0 {
		t.Fatalf("a replacing hunk was dropped as upstreamed: %+v", v)
	}
}

func TestVerifyRebaseACommentOnlyHunkIsNotUpstreamed(t *testing.T) {
	booted := claudeStackBase + "\n// The shell's auth-renewing fetch and the media routes (which app opens which file).\n"
	u := newUpRepo(t, sdkPath, claudeStackBase, booted, func(root string) {
		insertAt(t, root, sdkPath, "} from './media-utils.js';\n\n", "// The shell's auth-renewing fetch and the media routes (which app opens which file).\n")
	})
	v := u.verify(t, func(root string) { write(t, root, "frontend/src/other.txt", "x\n") })
	if v.Pass || len(v.Upstreamed) != 0 {
		t.Fatalf("a comment-only hunk was dropped as upstreamed: %+v", v)
	}
}

func TestVerifyRebaseOneHunkUpstreamedTheOtherMustSurvive(t *testing.T) {
	// P: claude's exports hunk, and a second hunk adding a new export at the
	// end that B' does not carry.
	u := newUpRepo(t, sdkPath, claudeStackBase, claudeBootedBase, func(root string) {
		insertAt(t, root, sdkPath, "} from './media-utils.js';\n\n", claudeHunk)
		insertAt(t, root, sdkPath, "liveEventPayload } from './live-events.js';\n", "export { openWith } from './open-with.js';\n")
	})
	both := u.verify(t, func(root string) { write(t, root, "frontend/src/other.txt", "x\n") })
	if both.Pass {
		t.Fatalf("the hunk B' does not carry was dropped: %+v", both)
	}
	kept := u.verify(t, func(root string) {
		insertAt(t, root, sdkPath, "from './apps/registry';\n", "export { openWith } from './open-with.js';\n")
	})
	if !kept.Pass || len(kept.Upstreamed) != 1 {
		t.Fatalf("the surviving hunk kept, the upstreamed one dropped: %+v", kept)
	}
}

// Review r1 (DeepSeek F1): P sets a new key to true; B' added the key with
// false, and true on another line. The value is not carried.
func TestVerifyRebaseAnInvertedValueIsNotUpstreamed(t *testing.T) {
	base := "export const defaults = {\n  cache: false,\n};\n"
	booted := "export const defaults = {\n  cache: false,\n  telemetry: false,\n  audited: true,\n};\n"
	const path = "frontend/src/lib/config.ts"
	u := newUpRepo(t, path, base, booted, func(root string) {
		insertAt(t, root, path, "  cache: false,\n", "  telemetry: true,\n")
	})
	v := u.verify(t, func(root string) { write(t, root, "frontend/src/other.txt", "x\n") })
	if v.Pass || len(v.Upstreamed) != 0 {
		t.Fatalf("an inverted value was dropped as upstreamed: %+v", v)
	}
	// The same value on one line B' added is carried.
	same := strings.Replace(booted, "telemetry: false", "telemetry: true", 1)
	u = newUpRepo(t, path, base, same, func(root string) {
		insertAt(t, root, path, "  cache: false,\n", "  telemetry: true,\n")
	})
	if v := u.verify(t, func(root string) { write(t, root, "frontend/src/other.txt", "x\n") }); !v.Pass || len(v.Upstreamed) != 1 {
		t.Fatalf("the same value on a line B' added: %+v", v)
	}
}

// Review r1 (DeepSeek F3): a Go line that starts with `*` is code.
func TestVerifyRebaseAStarLineIsCode(t *testing.T) {
	base := "package x\n\nfunc copyInto(dst, src *int) {\n}\n"
	booted := base + "\nvar copied bool\n\nfunc markCopied() {\n\tcopied = true\n}\n"
	const path = "internal/z.go"
	u := newUpRepo(t, path, base, booted, func(root string) {
		insertAt(t, root, path, "src *int) {\n", "\t*dst = *src\n\tcopied = true\n")
	})
	v := u.verify(t, func(root string) { write(t, root, "frontend/src/other.txt", "x\n") })
	if v.Pass || len(v.Upstreamed) != 0 {
		t.Fatalf("a hunk with a `*` code line was dropped as upstreamed: %+v", v)
	}
}

// Review r1 (Devin F1): words in a trailing comment B' added are not code
// B' added.
func TestVerifyRebaseATrailingCommentIsNotEvidence(t *testing.T) {
	base := "import { init } from './init.js';\n\nexport function boot(req) {\n  init();\n}\n"
	booted := strings.Replace(base, "  init();\n", "  init(req); // also wires fetchWithRenewal\n", 1)
	const path = "frontend/src/lib/boot.ts"
	u := newUpRepo(t, path, base, booted, func(root string) {
		insertAt(t, root, path, "export function boot(req) {\n", "  fetchWithRenewal(req);\n")
	})
	v := u.verify(t, func(root string) { write(t, root, "frontend/src/other.txt", "x\n") })
	if v.Pass || len(v.Upstreamed) != 0 {
		t.Fatalf("a comment's words carried a call: %+v", v)
	}
}

// Review r2 (Devin F1): the inversion with the key already mentioned in B.
func TestVerifyRebaseAnInvertedValueWithAKnownKeyIsNotUpstreamed(t *testing.T) {
	base := "export const defaults = {\n  cache: false,\n};\n// telemetry flag pending\n"
	booted := "export const defaults = {\n  cache: false,\n  telemetry: false,\n  audited: true,\n};\n// telemetry flag pending\n"
	const path = "frontend/src/lib/config.ts"
	u := newUpRepo(t, path, base, booted, func(root string) {
		insertAt(t, root, path, "  cache: false,\n", "  telemetry: true,\n")
	})
	if v := u.verify(t, func(root string) { write(t, root, "frontend/src/other.txt", "x\n") }); v.Pass || len(v.Upstreamed) != 0 {
		t.Fatalf("an inverted value with a key B mentions was dropped as upstreamed: %+v", v)
	}
}

// Review r2 (Devin F2): comment text the new base added is no evidence, in a
// file without known comment syntax or inline in one with it.
func TestVerifyRebaseCommentTextTheBaseAddedIsNotEvidence(t *testing.T) {
	cases := []struct{ name, path, base, booted, anchor, add string }{
		{"yaml comment line", "deploy/app.yaml", "limits:\n  cpu: 1\n", "limits:\n  cpu: 1\n  # timeout: 30\n", "  cpu: 1\n", "  timeout: 30\n"},
		{"yaml trailing comment", "deploy/app.yaml", "limits:\n  cpu: 1\n", "limits:\n  cpu: 1\n  memory: 2 # timeout: 30\n", "  cpu: 1\n", "  timeout: 30\n"},
		{"inline block comment", "frontend/src/lib/config3.ts", "export const defaults = {\n  cache: false,\n};\n", "export const defaults = {\n  cache: false,\n  audited: true, /* telemetry: true */\n};\n", "  cache: false,\n", "  telemetry: true,\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u := newUpRepo(t, tc.path, tc.base, tc.booted, func(root string) { insertAt(t, root, tc.path, tc.anchor, tc.add) })
			if v := u.verify(t, func(root string) { write(t, root, "frontend/src/other.txt", "x\n") }); v.Pass || len(v.Upstreamed) != 0 {
				t.Fatalf("a comment carried P's line as upstreamed: %+v", v)
			}
		})
	}
	// The same yaml line, uncommented, is carried.
	u := newUpRepo(t, "deploy/app.yaml", "limits:\n  cpu: 1\n", "limits:\n  cpu: 1\n  timeout: 30\n", func(root string) { insertAt(t, root, "deploy/app.yaml", "  cpu: 1\n", "  timeout: 30\n") })
	if v := u.verify(t, func(root string) { write(t, root, "frontend/src/other.txt", "x\n") }); !v.Pass {
		t.Fatalf("the same yaml line: %+v", v)
	}
}
