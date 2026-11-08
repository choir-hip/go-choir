package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
)

// Package commands (Library S2 and S3,
// docs/design/library-package-propagation-2026-10-26.md 14): publish one
// applied operation's package change, which makes it public to every
// signed-in owner (owner act; owner, 2026-11-03); withdraw and reinstate a
// release (owner acts); and read the catalog: a package's history on one
// track, a search, and one release verified against its id. Share is
// retired: it sends nothing and says why.

// releaseApprove is the scope of the package owner acts (publish, withdraw,
// reinstate, install).
const releaseApprove = "computer:self_development:approve"

type packageReleaseView struct {
	ReleaseID   string          `json:"release_id"`
	CanonicalID string          `json:"canonical_id,omitempty"`
	PublishedAt string          `json:"published_at,omitempty"`
	Record      json.RawMessage `json:"record"`
	Listing     json.RawMessage `json:"listing,omitempty"`
}

func packageCommands() []*command {
	return []*command{
		{
			path: []string{"package", "search"}, summary: "Search the packages in your catalog",
			params: []param{pStr("query", "Text the package name contains")},
			scopes: selfDevRead, readOnly: true,
			handler: func(x *call) (any, error) {
				q := url.Values{}
				if query := x.trim("query"); query != "" {
					q.Set("q", query)
				}
				return getRaw(x, "/api/catalog/packages?"+q.Encode())
			},
		},
		{
			path: []string{"package", "history"}, summary: "List one package's releases on one track (yours by default), newest first",
			params: []param{
				pStr("package", "Package name").req(),
				pStr("track", "The track's owner id (default: your own)"),
				pInt("limit", "Releases per page (default 100, at most 500)"),
				pInt("before_seq", "Only releases below this seq (the next page's cursor, next_before_seq)"),
			},
			usage: "--package is required", scopes: selfDevRead, readOnly: true,
			handler: func(x *call) (any, error) {
				q := url.Values{}
				if track := x.trim("track"); track != "" {
					q.Set("track", track)
				}
				for _, key := range []string{"limit", "before_seq"} {
					if n := x.num(key); n > 0 {
						q.Set(key, strconv.FormatInt(n, 10))
					}
				}
				path := "/api/catalog/packages/" + url.PathEscape(x.trim("package")) + "/releases"
				if len(q) > 0 {
					path += "?" + q.Encode()
				}
				return getRaw(x, path)
			},
		},
		{
			path: []string{"package", "show"}, summary: "Show one package release, verified against its id",
			params: []param{pStr("release", "Package release id (the record's sha256)").req()},
			usage:  "--release is required", scopes: selfDevRead, readOnly: true,
			handler: func(x *call) (any, error) {
				releaseID := strings.ToLower(x.trim("release"))
				var view packageReleaseView
				if err := x.c.do(http.MethodGet, "/api/catalog/releases/"+url.PathEscape(releaseID), nil, &view); err != nil {
					return nil, err
				}
				record, err := pkgrelease.Decode(view.Record)
				if err == nil {
					err = pkgrelease.VerifyID(record, releaseID)
				}
				if err == nil && view.ReleaseID != releaseID {
					err = fmt.Errorf("the host answered release %q, not %q", view.ReleaseID, releaseID)
				}
				if err != nil {
					return nil, fmt.Errorf("package release %s does not verify: %w", releaseID, err)
				}
				return map[string]any{"release_id": view.ReleaseID, "published_at": view.PublishedAt, "record": record, "listing": view.Listing, "record_verified": true}, nil
			},
		},
		ownerAct(&command{
			path: []string{"package", "publish"}, summary: "Publish an applied operation's package change to the Library, where it is public: every signed-in owner can see and install it (owner act)",
			desc: "Publishing makes the release public: every signed-in owner sees it in the Library and can install it. " +
				"The first run answers confirm_paths_required with every path that becomes public and the review diff's size, and publishes nothing; " +
				"rerun with --confirm-paths <paths_digest> to publish. To take a release down later, withdraw it.",
			params: []param{
				computerParam().orSay(selfDevUsage),
				pStr("operation", "Applied operation ID").req(),
				pStr("package", "Package name (frontend/src/packages/<name>)").req(),
				pStr("test_command", "The check an owner runs after installing the release"),
				pStr("confirm_paths", "The paths_digest the publish preview showed: you confirm these paths become public"),
			},
			usage:  "--computer, --operation and --package are required",
			scopes: []string{releaseApprove}, idempotent: true,
			handler: func(x *call) (any, error) {
				body := map[string]any{"operation_id": x.trim("operation"), "package": x.trim("package")}
				if cmd := x.trim("test_command"); cmd != "" {
					body["test_command"] = cmd
				}
				if confirm := x.trim("confirm_paths"); confirm != "" {
					body["confirm_paths"] = confirm
				}
				out, err := sendRaw(x, http.MethodPost, computerPath(x, "/packages/releases"), body)
				var api *apiErrorResp
				if errors.As(err, &api) {
					if text := publishConfirmText(api, x.str("computer"), x.trim("operation"), x.trim("package"), x.trim("test_command")); text != "" {
						return nil, errors.New(text)
					}
				}
				return out, err
			},
		}),
		ownerAct(&command{
			path: []string{"package", "share"}, summary: "Retired: a published release is already public to every signed-in owner",
			desc: "Share is retired (owner, 2026-11-03): publishing is the act that makes a release public, visible to and installable by every signed-in owner. " +
				"This command sends nothing. To take a release down, use choir package withdraw.",
			params: []param{
				pStr("computer", "Ignored"),
				pStr("release", "Package release id (ignored)"),
				pList("owners", "Ignored").commas(),
				pBool("public", "Ignored"),
				pBool("private", "Ignored"),
				pStr("confirm_paths", "Ignored"),
			},
			scopes: []string{releaseApprove}, idempotent: true,
			handler: func(x *call) (any, error) {
				return map[string]any{"shared": false, "reason": pkgrelease.ReasonShareRetired, "message": pkgrelease.ShareRetiredDetail}, nil
			},
		}),
		packageExportCommand(),
		{
			path: []string{"package", "installs"}, summary: "List a computer's package installs from the Library (or one, with --install)",
			params: []param{computerParam().orSay(selfDevUsage), pStr("install", "Install ID")},
			usage:  selfDevUsage, scopes: selfDevRead, readOnly: true,
			handler: func(x *call) (any, error) {
				if id := x.trim("install"); id != "" {
					return getRaw(x, installsPath(x.str("computer"), "/"+url.PathEscape(id)))
				}
				return getRaw(x, installsPath(x.str("computer"), ""))
			},
		},
		{
			path: []string{"package", "reviews"}, summary: "Read an install review: the sealed verdict (seal checked), management's seat record (served model, pWithinCeiling, findings, reason) and the review's records",
			params: []param{pStr("release", "Package release id (sha256)").req()},
			usage:  "--release is required", scopes: readRuntime, readOnly: true,
			handler: func(x *call) (any, error) {
				return getRaw(x, "/api/packages/reviews?release="+url.QueryEscape(strings.ToLower(x.trim("release"))))
			},
		},
		packageInstallAct("install", "Install a published package release on your computer: the computer builds it into a module and records the install (owner act)"),
		packageInstallAct("uninstall", "Uninstall a package release the Library installed on your computer, removing its module from the launcher (owner act)"),
		packageStateAct("withdraw", pkgrelease.StateWithdrawn, "Withdraw a package release: nobody else sees it any more; it stays in your own history (owner act)"),
		packageStateAct("reinstate", pkgrelease.StateListed, "Reinstate a withdrawn or suspended package release while it can still head its track (owner act)"),
		ownerAct(&command{
			path: []string{"package", "request-force"}, summary: "Ask the platform to force an update of your release on its installers: a note to the platform operator; it changes nothing by itself (owner act)",
			params: []param{
				computerParam().orSay("--computer names your computer the request is made from"),
				pStr("release", "Your package release id").req(),
				pStr("note", "Why the platform should force the update").req(),
			},
			usage:  "--computer, --release and --note are required",
			scopes: []string{releaseApprove},
			handler: func(x *call) (any, error) {
				return sendRaw(x, http.MethodPost, "/api/catalog/records/"+url.PathEscape(strings.ToLower(x.trim("release")))+"/force-request", map[string]any{
					"computer_id": x.trim("computer"), "note": x.trim("note"),
				})
			},
		}),
		{
			path: []string{"platform", "yank"}, summary: "Platform operator only: yank a package release, or force its installers onto another release of its track (break glass, manual)",
			scopes:  []string{"public"},
			cliOnly: "the platform's break-glass act runs on the host against corpusd with the host credential and an operator token; no agent and no hosted server may reach it",
			legacy:  runPlatformYank,
		},
	}
}

// runPlatformYank is the platform operator's yank or forced update (F1,
// docs/design/forced-modularity-build-2026-11-04.md section 5). It runs on
// the host (ssh), calls corpusd's operator route directly over loopback with
// the host credential (hostauth), and names the operator; the token is read
// from CHOIR_PLATFORM_OPERATOR_TOKEN so it never appears in argv.
func runPlatformYank(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir platform yank", flag.ContinueOnError)
	fs.SetOutput(stderr)
	corpusd := fs.String("corpusd", "http://127.0.0.1:8086", "corpusd base URL on this host")
	operator := fs.String("operator", "", "Your platform operator name (CHOIR_PLATFORM_OPERATORS)")
	release := fs.String("release", "", "The release to yank (sha256)")
	reason := fs.String("reason", "", "Why; shown to every installer")
	forceTo := fs.String("force-to", "", "A listed release of the same track every installer moves to (a forced update)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	token := os.Getenv("CHOIR_PLATFORM_OPERATOR_TOKEN")
	if strings.TrimSpace(*operator) == "" || strings.TrimSpace(*release) == "" || strings.TrimSpace(*reason) == "" || token == "" {
		fmt.Fprintln(stderr, "choir platform yank: --operator, --release, --reason and CHOIR_PLATFORM_OPERATOR_TOKEN are required")
		return 2
	}
	body, _ := json.Marshal(map[string]string{"release_id": strings.ToLower(strings.TrimSpace(*release)), "reason": strings.TrimSpace(*reason), "forced_release": strings.ToLower(strings.TrimSpace(*forceTo))})
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(*corpusd, "/")+"/internal/platform/operator/packages/yank", bytes.NewReader(body))
	if err != nil {
		fmt.Fprintf(stderr, "choir platform yank: %v\n", err)
		return 2
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Choir-Operator", strings.TrimSpace(*operator))
	req.Header.Set("X-Choir-Operator-Token", token)
	hostauth.MarkInternal(req)
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		fmt.Fprintf(stderr, "choir platform yank: %v\n", err)
		return 1
	}
	defer func() { _ = resp.Body.Close() }()
	out, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	fmt.Fprintln(stdout, strings.TrimSpace(string(out)))
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(stderr, "choir platform yank: corpusd answered %d\n", resp.StatusCode)
		return 1
	}
	return 0
}

// packageInstallAct installs or uninstalls one package release on a
// computer (docs/design/package-install-2026-11-04.md). The proxy reads the
// release and its tree from the catalog as the owner and verifies both; the
// command sends only the id and the action.
func packageInstallAct(verb, summary string) *command {
	return ownerAct(&command{
		path: []string{"package", verb}, summary: summary,
		params: []param{
			computerParam().orSay(selfDevUsage),
			pStr("release", "Package release id (the record's sha256)").req(),
		},
		usage:  "--computer and --release are required",
		scopes: []string{"computer:self_development:read", releaseApprove}, idempotent: true,
		handler: func(x *call) (any, error) {
			return sendRaw(x, http.MethodPost, installsPath(x.str("computer"), ""), map[string]any{
				"release_id": strings.ToLower(x.trim("release")), "action": verb,
			})
		},
	})
}

// installsPath is the Library's install route on a computer.
func installsPath(computerID, rest string) string {
	return "/api/computers/" + url.PathEscape(strings.TrimSpace(computerID)) + "/packages/installs" + rest
}

func listingPath(x *call) string {
	return "/api/catalog/records/" + url.PathEscape(strings.ToLower(x.trim("release"))) + "/listing"
}

func packageStateAct(verb, state, summary string) *command {
	return ownerAct(&command{
		path: []string{"package", verb}, summary: summary,
		params: []param{
			computerParam().orSay("--computer names your computer the act is made from"),
			pStr("release", "Package release id").req(),
			pStr("reason", "Why, recorded on the listing"),
		},
		usage:  "--computer and --release are required",
		scopes: []string{releaseApprove}, idempotent: true,
		handler: func(x *call) (any, error) {
			body := map[string]any{"computer_id": x.trim("computer"), "state": state}
			if reason := x.trim("reason"); reason != "" {
				body["reason"] = reason
			}
			return sendRaw(x, http.MethodPatch, listingPath(x), body)
		},
	})
}

// publishConfirmText renders the 409 confirm_paths_required (or
// confirm_paths_mismatch) preview in full: the server's statement, every
// path, the digest and the command that confirms it. The generic error
// cuts bodies at 300 characters, which hid the digest and the paths.
// It returns "" for any other error.
func publishConfirmText(e *apiErrorResp, computer, operation, pkg, testCommand string) string {
	if e.Status != http.StatusConflict {
		return ""
	}
	var refusal struct {
		Error   string `json:"error"`
		Reason  string `json:"reason"`
		Preview *struct {
			Paths            []string `json:"paths"`
			PathsDigest      string   `json:"paths_digest"`
			ReviewPatchBytes int      `json:"review_patch_bytes"`
			Questions        []string `json:"precommitment_questions"`
			TestCommand      string   `json:"test_command"`
			Media            []string `json:"media"`
		} `json:"preview"`
	}
	if json.Unmarshal([]byte(e.Body), &refusal) != nil || refusal.Preview == nil || refusal.Preview.PathsDigest == "" ||
		(refusal.Reason != "confirm_paths_required" && refusal.Reason != "confirm_paths_mismatch") {
		return ""
	}
	pv := refusal.Preview
	var b strings.Builder
	fmt.Fprintf(&b, "publishing is not done yet (%s). %s\n", refusal.Reason, strings.TrimSpace(refusal.Error))
	fmt.Fprintf(&b, "Publishing makes this release public: every signed-in owner can see and install it.\n")
	fmt.Fprintf(&b, "Paths that become public (%d):\n", len(pv.Paths))
	for _, p := range pv.Paths {
		fmt.Fprintf(&b, "  %s\n", p)
	}
	fmt.Fprintf(&b, "Review diff: %d bytes\n", pv.ReviewPatchBytes)
	if pv.TestCommand != "" {
		fmt.Fprintf(&b, "Test command: %s\n", pv.TestCommand)
	}
	for _, q := range pv.Questions {
		fmt.Fprintf(&b, "Precommitment question: %s\n", q)
	}
	if len(pv.Media) == 0 {
		fmt.Fprintf(&b, "Screenshots: none (the Library shows no screenshots)\n")
	}
	for _, m := range pv.Media {
		fmt.Fprintf(&b, "Public media: %s\n", m)
	}
	fmt.Fprintf(&b, "paths_digest: %s\n", pv.PathsDigest)
	cmd := fmt.Sprintf("choir package publish --computer %s --operation %s --package %s", computer, operation, pkg)
	if testCommand != "" {
		cmd += " --test-command " + strconv.Quote(testCommand)
	}
	fmt.Fprintf(&b, "To publish, rerun:\n  %s --confirm-paths %s", cmd, pv.PathsDigest)
	return b.String()
}
