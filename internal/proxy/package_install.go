package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
)

// Package install at the proxy (docs/design/package-install-2026-11-04.md).
// The owner's install arrives on POST /api/computers/<id>/self-development/adoptions,
// the route the Library calls, as {release_id, record?, action?}. The proxy
// reads the release from corpusd as the owner it authenticated, so only a
// listed release that owner may see, verifies the record against its id
// and the supplied record against it, reads and verifies the release's
// tree_patch, and hands the computer {release_id, record, tree_patch,
// action}. A guest never reads the catalog; the tree only ever reaches a
// computer from here.

// maxInstallRequestBytes bounds the owner's install request: a record and
// room.
const maxInstallRequestBytes = pkgrelease.MaxRecordBytes + (16 << 10)

type packageInstallRequest struct {
	ReleaseID string          `json:"release_id"`
	Record    json.RawMessage `json:"record,omitempty"`
	Action    string          `json:"action,omitempty"`
}

// installRefusal is a refusal the proxy answers itself, in the computer's
// refusal shape ({error, reason}) so the Library reads one shape.
type installRefusal struct {
	status int
	reason string
	detail string
}

// packageInstallBody turns the owner's install request into the body the
// computer verifies and acts on.
func (h *Handler) packageInstallBody(ctx context.Context, ownerID string, raw []byte) ([]byte, *installRefusal) {
	var request packageInstallRequest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return nil, &installRefusal{http.StatusBadRequest, "request_invalid", "invalid install request"}
	}
	releaseID := strings.ToLower(strings.TrimSpace(request.ReleaseID))
	if !computerevent.IsSHA256(releaseID) {
		return nil, &installRefusal{http.StatusBadRequest, "release_id_invalid", "release_id must be a sha256"}
	}
	action := strings.TrimSpace(request.Action)
	if action != "" && action != "install" && action != "uninstall" {
		return nil, &installRefusal{http.StatusBadRequest, "request_invalid", "action must be install or uninstall"}
	}

	owner := url.Values{"owner_id": {ownerID}}.Encode()
	status, body, err := h.corpusdCall(ctx, http.MethodGet, "/internal/platform/packages/releases/"+releaseID+"?"+owner, "", nil)
	switch {
	case err != nil:
		return nil, &installRefusal{http.StatusBadGateway, "release_source_unavailable", "package store unavailable"}
	case status == http.StatusNotFound:
		return nil, &installRefusal{http.StatusNotFound, "release_not_published", "no published release " + releaseID}
	case status != http.StatusOK:
		return nil, &installRefusal{http.StatusBadGateway, "release_source_unavailable", "package store answered " + http.StatusText(status)}
	}
	var view struct {
		ReleaseID string          `json:"release_id"`
		Record    json.RawMessage `json:"record"`
		Listing   struct {
			State string `json:"state"`
		} `json:"listing"`
	}
	if err := json.Unmarshal(body, &view); err != nil {
		return nil, &installRefusal{http.StatusBadGateway, "release_source_unavailable", "malformed package release"}
	}
	// A withdrawn, suspended or yanked release is its publisher's to see,
	// never to install.
	if view.Listing.State != pkgrelease.StateListed {
		return nil, &installRefusal{http.StatusNotFound, "release_not_published", "release " + releaseID + " is " + view.Listing.State}
	}
	published, err := pkgrelease.Decode(view.Record)
	if err == nil {
		err = pkgrelease.VerifyID(published, releaseID)
	}
	if err != nil {
		return nil, &installRefusal{http.StatusBadGateway, "published_record_digest_mismatch", "the store's copy of " + releaseID + " does not verify: " + err.Error()}
	}
	if len(request.Record) > 0 {
		supplied, err := pkgrelease.Decode(request.Record)
		if err == nil {
			err = pkgrelease.VerifyID(supplied, releaseID)
		}
		if err != nil {
			return nil, &installRefusal{http.StatusUnprocessableEntity, "release_digest_mismatch", err.Error()}
		}
	}
	canonical, err := published.Canonical()
	if err != nil {
		return nil, &installRefusal{http.StatusBadGateway, "published_record_digest_mismatch", err.Error()}
	}

	blob := published.Source.TreePatch
	status, tree, err := h.corpusdCall(ctx, http.MethodGet, "/internal/platform/packages/releases/"+releaseID+"/blobs/"+blob.SHA256+"?"+owner, "", nil)
	switch {
	case err != nil:
		return nil, &installRefusal{http.StatusBadGateway, "release_source_unavailable", "package store unavailable"}
	case status == http.StatusNotFound:
		return nil, &installRefusal{http.StatusConflict, "patch_unavailable", "the store has no tree for " + releaseID}
	case status != http.StatusOK:
		return nil, &installRefusal{http.StatusBadGateway, "release_source_unavailable", "package store answered " + http.StatusText(status)}
	}
	if err := pkgrelease.VerifyBlob(blob, tree); err != nil {
		return nil, &installRefusal{http.StatusConflict, "patch_digest_mismatch", err.Error()}
	}
	out, err := json.Marshal(map[string]any{
		"release_id": releaseID, "record": json.RawMessage(canonical), "tree_patch": tree, "action": action,
	})
	if err != nil {
		return nil, &installRefusal{http.StatusInternalServerError, "request_invalid", err.Error()}
	}
	return out, nil
}
