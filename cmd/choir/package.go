package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
)

// Package commands (Library S2 and S3,
// docs/design/library-package-propagation-2026-10-26.md 14): publish one
// applied operation's package change at scope owner (owner act); share,
// withdraw and reinstate a release (owner acts, gated by the named
// computer's packages.share); and read the catalog the caller may see: a
// package's history on one track, a search, and one release verified
// against its id.

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
			path: []string{"package", "publish"}, summary: "Publish an applied operation's package change to your own catalog (owner act)",
			params: []param{
				computerParam().orSay(selfDevUsage),
				pStr("operation", "Applied operation ID").req(),
				pStr("package", "Package name (frontend/src/packages/<name>)").req(),
				pStr("test_command", "The check an adopter runs after applying the release"),
			},
			usage:  "--computer, --operation and --package are required",
			scopes: []string{releaseApprove}, idempotent: true,
			handler: func(x *call) (any, error) {
				body := map[string]any{"operation_id": x.trim("operation"), "package": x.trim("package")}
				if cmd := x.trim("test_command"); cmd != "" {
					body["test_command"] = cmd
				}
				return sendRaw(x, http.MethodPost, computerPath(x, "/packages/releases"), body)
			},
		}),
		ownerAct(&command{
			path: []string{"package", "share"}, summary: "Set who may see a package release: named owners, everyone, or only you (owner act)",
			desc: "A widening first answers confirm_paths_required with every path that would leave and the review diff's size; " +
				"rerun with --confirm-paths <paths_digest> to share them. Narrowing (--private, or fewer owners) needs no confirmation.",
			params: []param{
				computerParam().orSay("--computer names the computer whose packages.share setting gates the act"),
				pStr("release", "Package release id").req(),
				pList("owners", "Owner ids to share with (comma-separated)").commas(),
				pBool("public", "Share with every owner on this cloud"),
				pBool("private", "Only you (and your computers)"),
				pStr("confirm_paths", "The paths_digest the widening's preview showed"),
			},
			usage:  "--computer, --release, and one of --owners, --public or --private are required",
			scopes: []string{releaseApprove}, idempotent: true,
			handler: func(x *call) (any, error) {
				owners, public, private := x.list("owners"), x.flag("public"), x.flag("private")
				named := 0
				for _, set := range []bool{len(owners) > 0, public, private} {
					if set {
						named++
					}
				}
				if named != 1 {
					return nil, usagef("name exactly one of --owners, --public or --private")
				}
				body := map[string]any{"computer_id": x.trim("computer")}
				switch {
				case private:
					body["scopes"] = []string{pkgrelease.ScopeOwner}
				case public:
					body["scopes"] = []string{pkgrelease.ScopeOwner, pkgrelease.ScopePublic}
				default:
					body["scopes"] = []string{pkgrelease.ScopeOwner, pkgrelease.ScopeShared}
					body["shared_with"] = map[string]any{"owners": owners}
				}
				if confirm := x.trim("confirm_paths"); confirm != "" {
					body["confirm_paths"] = confirm
				}
				return sendRaw(x, http.MethodPatch, listingPath(x), body)
			},
		}),
		packageStateAct("withdraw", pkgrelease.StateWithdrawn, "Withdraw a package release: it leaves the catalog heads and stays in the history (owner act)"),
		packageStateAct("reinstate", pkgrelease.StateListed, "Reinstate a withdrawn or suspended package release while it can still head its track (owner act)"),
	}
}

func listingPath(x *call) string {
	return "/api/catalog/records/" + url.PathEscape(strings.ToLower(x.trim("release"))) + "/listing"
}

func packageStateAct(verb, state, summary string) *command {
	return ownerAct(&command{
		path: []string{"package", verb}, summary: summary,
		params: []param{
			computerParam().orSay("--computer names the computer whose packages.share setting gates the act"),
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
