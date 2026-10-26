package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
)

// Package commands (Library S2,
// docs/design/library-package-propagation-2026-10-26.md 14): publish one
// applied operation's package change at scope owner (owner act), and read
// the owner's catalog: a package's history, a search, and one release
// verified against its id.

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
			path: []string{"package", "history"}, summary: "List one package's releases on your track, newest first",
			params: []param{pStr("package", "Package name").req()},
			usage:  "--package is required", scopes: selfDevRead, readOnly: true,
			handler: func(x *call) (any, error) {
				return getRaw(x, "/api/catalog/packages/"+url.PathEscape(x.trim("package"))+"/releases")
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
	}
}
