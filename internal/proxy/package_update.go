package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
)

// Package updates at the proxy (F1, docs/design/forced-modularity-build-2026-11-04.md
// section 5). The shell asks at load, and the Library on open:
//
//	POST /api/computers/<id>/packages/updates
//
// The proxy reads the computer's installed set, asks corpusd for each
// install's update facts (the installed release's listing, the track's
// newest and newest soaked listed releases, and a forced target), reads and
// verifies each candidate's record and tree as an install does, and hands
// the computer the lot. The computer decides by its owner's update policy;
// a guest never reads the catalog.

// packageUpdateCandidate mirrors the computer's PackageUpdateCandidate.
type packageUpdateCandidate struct {
	Release   pkgrelease.TrackRelease `json:"release"`
	Record    json.RawMessage         `json:"record"`
	TreePatch []byte                  `json:"tree_patch"`
	View      json.RawMessage         `json:"view,omitempty"`
}

type packageUpdateOffer struct {
	Package   string                   `json:"package"`
	ReleaseID string                   `json:"release_id"`
	Installed *pkgrelease.TrackRelease `json:"installed,omitempty"`
	Newest    *packageUpdateCandidate  `json:"newest,omitempty"`
	Soaked    *packageUpdateCandidate  `json:"soaked,omitempty"`
	Forced    *packageUpdateCandidate  `json:"forced,omitempty"`
}

// corpusdUpdateOffer is one offer as corpusd answers it.
type corpusdUpdateOffer struct {
	Release pkgrelease.TrackRelease `json:"release"`
	View    json.RawMessage         `json:"view"`
}

type corpusdUpdateFacts struct {
	Package   string                   `json:"package"`
	Track     string                   `json:"track"`
	ReleaseID string                   `json:"release_id"`
	Installed *pkgrelease.TrackRelease `json:"installed,omitempty"`
	Newest    *corpusdUpdateOffer      `json:"newest,omitempty"`
	Soaked    *corpusdUpdateOffer      `json:"soaked,omitempty"`
	Forced    *corpusdUpdateOffer      `json:"forced,omitempty"`
}

// packageUpdatesBody builds the computer's update sync body for ownerID.
// installed reads the computer's installed list.
func (h *Handler) packageUpdatesBody(ctx context.Context, ownerID string, installed func(context.Context) ([]byte, error)) ([]byte, *installRefusal) {
	raw, err := installed(ctx)
	if err != nil {
		return nil, &installRefusal{http.StatusBadGateway, "computer_unavailable", "the computer's installed list: " + err.Error()}
	}
	var list struct {
		Packages []struct {
			Name      string `json:"name"`
			ReleaseID string `json:"release_id"`
			Track     string `json:"track"`
		} `json:"packages"`
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, &installRefusal{http.StatusBadGateway, "computer_unavailable", "malformed installed list"}
	}
	type install struct {
		Package   string `json:"package"`
		Track     string `json:"track"`
		ReleaseID string `json:"release_id"`
	}
	ask := []install{}
	for _, p := range list.Packages {
		if p.Track == "" || p.ReleaseID == "" {
			continue
		}
		ask = append(ask, install{Package: p.Name, Track: p.Track, ReleaseID: p.ReleaseID})
	}
	offers := []packageUpdateOffer{}
	if len(ask) > 0 {
		body, _ := json.Marshal(map[string]any{"owner_id": ownerID, "installs": ask})
		status, answer, err := h.corpusdCall(ctx, http.MethodPost, "/internal/platform/packages/updates", "application/json", body)
		if err != nil || status != http.StatusOK {
			return nil, &installRefusal{http.StatusBadGateway, "release_source_unavailable", fmt.Sprintf("package store answered %d", status)}
		}
		var facts struct {
			Updates []corpusdUpdateFacts `json:"updates"`
		}
		if err := json.Unmarshal(answer, &facts); err != nil {
			return nil, &installRefusal{http.StatusBadGateway, "release_source_unavailable", "malformed update facts"}
		}
		for _, f := range facts.Updates {
			offer := packageUpdateOffer{Package: f.Package, ReleaseID: f.ReleaseID, Installed: f.Installed}
			offer.Newest = h.updateCandidate(ctx, ownerID, f.Newest)
			offer.Soaked = h.updateCandidate(ctx, ownerID, f.Soaked)
			offer.Forced = h.updateCandidate(ctx, ownerID, f.Forced)
			offers = append(offers, offer)
		}
	}
	out, err := json.Marshal(map[string]any{"packages": offers})
	if err != nil {
		return nil, &installRefusal{http.StatusInternalServerError, "request_invalid", err.Error()}
	}
	return out, nil
}

// updateCandidate reads and verifies one offered release's record and tree
// as an install does; an offer that does not verify is left out.
func (h *Handler) updateCandidate(ctx context.Context, ownerID string, offer *corpusdUpdateOffer) *packageUpdateCandidate {
	if offer == nil {
		return nil
	}
	var view struct {
		Record json.RawMessage `json:"record"`
	}
	if json.Unmarshal(offer.View, &view) != nil {
		return nil
	}
	id := strings.ToLower(offer.Release.ReleaseID)
	rec, err := pkgrelease.Decode(view.Record)
	if err == nil {
		err = pkgrelease.VerifyID(rec, id)
	}
	if err != nil {
		return nil
	}
	canonical, err := rec.Canonical()
	if err != nil {
		return nil
	}
	owner := url.Values{"owner_id": {ownerID}}.Encode()
	blob := rec.Source.TreePatch
	status, tree, err := h.corpusdCall(ctx, http.MethodGet, "/internal/platform/packages/releases/"+id+"/blobs/"+blob.SHA256+"?"+owner, "", nil)
	if err != nil || status != http.StatusOK || pkgrelease.VerifyBlob(blob, tree) != nil {
		return nil
	}
	return &packageUpdateCandidate{Release: offer.Release, Record: canonical, TreePatch: tree, View: offer.View}
}

// guestInstalledReader reads the computer's installed list as the owner.
func (h *Handler) guestInstalledReader(autoputerURL, ownerID, computerID string) func(context.Context) ([]byte, error) {
	return func(ctx context.Context) ([]byte, error) {
		target, err := joinBasePath(autoputerURL, "/api/packages/installed")
		if err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("X-Authenticated-User", ownerID)
		req.Header.Set("X-Authenticated-Computer", computerID)
		client := h.autoputerHTTP
		if client == nil {
			client = http.DefaultClient
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("answered %d", resp.StatusCode)
		}
		return io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	}
}
