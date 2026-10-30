package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/release"
)

// Release commands (P7/P8, docs/archive/reports/p6-p8-release-adopt-2026-10-15.md):
// list and show published releases, publish an applied operation as a
// release (owner act), adopt a published release as source onto a computer's
// own tree (owner act), and read a computer's adoptions.

const releaseApprove = "computer:self_development:approve"

type releaseView struct {
	ReleaseID   string          `json:"release_id"`
	CanonicalID string          `json:"canonical_id,omitempty"`
	PublishedAt string          `json:"published_at,omitempty"`
	Record      json.RawMessage `json:"record"`
}

// readVerifiedRelease reads one published release and verifies its record
// against its id client-side: what the owner adopts is what hashes to the
// id they named.
func readVerifiedRelease(x *call, releaseID string) (releaseView, release.Record, error) {
	releaseID = strings.ToLower(strings.TrimSpace(releaseID))
	var view releaseView
	if err := x.c.do(http.MethodGet, "/api/releases/"+url.PathEscape(releaseID), nil, &view); err != nil {
		return view, release.Record{}, err
	}
	record, err := release.Decode(view.Record)
	if err == nil {
		err = release.VerifyID(record, releaseID)
	}
	if err == nil && view.ReleaseID != releaseID {
		err = fmt.Errorf("the host answered release %q, not %q", view.ReleaseID, releaseID)
	}
	if err != nil {
		return view, release.Record{}, fmt.Errorf("release %s does not verify: %w", releaseID, err)
	}
	return view, record, nil
}

func releaseCommands() []*command {
	return []*command{
		{
			path: []string{"release", "list"}, summary: "List published software releases (source records)",
			params: []param{
				pInt("limit", "Maximum releases to return").dflt(50).between(1, 200),
				pStr("publisher", "Only releases published from this ComputerID"),
			},
			usage: "--limit must be between 1 and 200", scopes: selfDevRead, readOnly: true,
			handler: func(x *call) (any, error) {
				q := url.Values{}
				q.Set("limit", strconv.FormatInt(x.num("limit"), 10))
				if publisher := x.trim("publisher"); publisher != "" {
					q.Set("publisher_computer_id", publisher)
				}
				return getRaw(x, "/api/releases?"+q.Encode())
			},
		},
		{
			path: []string{"release", "show"}, summary: "Show one published release, verified against its id",
			params: []param{pStr("release", "Release id (the record's sha256)").req()},
			usage:  "--release is required", scopes: selfDevRead, readOnly: true,
			handler: func(x *call) (any, error) {
				view, record, err := readVerifiedRelease(x, x.str("release"))
				if err != nil {
					return nil, err
				}
				return map[string]any{"release_id": view.ReleaseID, "published_at": view.PublishedAt, "record": record, "record_verified": true}, nil
			},
		},
		{
			path: []string{"release", "adoptions"}, summary: "List a computer's release adoptions (or one, with --adoption)",
			params: []param{computerParam().orSay(selfDevUsage), pStr("adoption", "Adoption ID")},
			usage:  selfDevUsage, scopes: selfDevRead, readOnly: true,
			handler: func(x *call) (any, error) {
				if id := x.trim("adoption"); id != "" {
					return getRaw(x, selfDevPath(x.str("computer"), "adoptions/"+url.PathEscape(id)))
				}
				return getRaw(x, selfDevPath(x.str("computer"), "adoptions"))
			},
		},
		ownerAct(&command{
			path: []string{"release", "publish"}, summary: "Publish an applied self-development operation as a source release (owner act)",
			params: []param{
				computerParam().orSay(selfDevUsage),
				pStr("operation", "Applied operation ID").req(),
				pStr("test_command", "The check an adopter runs after applying the patch"),
			},
			usage:  "--computer and --operation are required",
			scopes: []string{releaseApprove}, idempotent: true,
			handler: func(x *call) (any, error) {
				body := map[string]any{"operation_id": x.trim("operation")}
				if cmd := x.trim("test_command"); cmd != "" {
					body["test_command"] = cmd
				}
				return sendRaw(x, http.MethodPost, computerPath(x, "/releases"), body)
			},
		}),
		ownerAct(&command{
			path: []string{"release", "adopt"}, summary: "Adopt a published release as source onto a computer's own tree (owner act)",
			params: []param{
				computerParam().orSay(selfDevUsage),
				pStr("release", "Release id (the record's sha256)").req(),
			},
			usage:  "--computer and --release are required",
			scopes: []string{"computer:self_development:read", releaseApprove}, idempotent: true,
			handler: func(x *call) (any, error) {
				view, _, err := readVerifiedRelease(x, x.str("release"))
				if err != nil {
					return nil, err
				}
				return sendRaw(x, http.MethodPost, selfDevPath(x.str("computer"), "adoptions"), map[string]any{
					"release_id": view.ReleaseID, "record": view.Record,
				})
			},
		}),
	}
}
