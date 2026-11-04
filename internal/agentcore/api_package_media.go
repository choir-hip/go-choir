package agentcore

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// Release media at the publisher
// (docs/problems/library-releases-have-no-screenshots-2026-11-04.md). A
// package release names its round's accepted screenshots and walkthrough
// video: only what the applied operation's bound frozen bundle carries, the
// shots the judge re-hashed before apply, never a file by path. The draft
// names them; GET .../packages/operations/<op>/media/<sha256> answers the
// bytes of exactly those, for the proxy to verify and store at publish.

// operationMedia is the media a release of operation names, in record
// order (screenshots by name, then the video), with the bytes of each. A
// screen is skipped when it is blank, over the release bounds, or its bytes
// on disk are no longer its digest; a bundle with none yields none.
func (rt *Runtime) operationMedia(operation selfdev.Operation) ([]pkgrelease.Media, [][]byte, error) {
	if strings.TrimSpace(rt.selfdevUpdaterRoot) == "" || !computerevent.IsSHA256(operation.BundleDigest) {
		return nil, nil, errBundleUnavailable
	}
	bundleRoot := filepath.Join(rt.selfdevUpdaterRoot, "incoming", operation.BundleDigest)
	bundle, _, err := readBoundSelfDevelopmentBundle(bundleRoot, operation)
	if err != nil {
		return nil, nil, err
	}
	stage := filepath.Join(bundleRoot, filepath.FromSlash(capsule.BundleScreensDir))
	read := func(item pkgrelease.Media) ([]byte, bool) {
		raw, err := os.ReadFile(filepath.Join(stage, item.Name))
		if err != nil || pkgrelease.VerifyMedia(item, raw) != nil {
			return nil, false
		}
		return raw, true
	}
	screens := append([]capsule.ScreenFile{}, bundle.Screens...)
	sort.Slice(screens, func(i, j int) bool { return screens[i].Name < screens[j].Name })
	var items []pkgrelease.Media
	var data [][]byte
	for _, s := range screens {
		if len(items) == pkgrelease.MaxScreenshots {
			break
		}
		if s.Blank || s.Bytes > pkgrelease.MaxScreenshotBytes || s.Width > pkgrelease.MaxScreenshotSide || s.Height > pkgrelease.MaxScreenshotSide || !capsule.ValidScreenName(s.Name) {
			continue
		}
		item := pkgrelease.Media{Kind: pkgrelease.MediaKindScreenshot, Name: s.Name, MediaType: pkgrelease.MediaTypePNG, SHA256: s.SHA256, Bytes: int(s.Bytes), Width: s.Width, Height: s.Height}
		if raw, ok := read(item); ok {
			items, data = append(items, item), append(data, raw)
		}
	}
	if v := bundle.Video; v != nil && v.Bytes <= pkgrelease.MaxVideoBytes && capsule.ValidVideoName(v.Name) {
		item := pkgrelease.Media{Kind: pkgrelease.MediaKindVideo, Name: v.Name, MediaType: pkgrelease.MediaTypeWebM, SHA256: v.SHA256, Bytes: int(v.Bytes)}
		if raw, ok := read(item); ok {
			items, data = append(items, item), append(data, raw)
		}
	}
	return items, data, nil
}

// packageReleaseMedia answers one item of operationID's release media.
func (h *APIHandler) packageReleaseMedia(w http.ResponseWriter, r *http.Request, ownerID, computerID, operationID, digest string) {
	ctx := r.Context()
	digest = strings.ToLower(strings.TrimSpace(digest))
	if !computerevent.IsSHA256(digest) {
		writeAPIJSON(w, http.StatusNotFound, releaseAPIError{Error: "media not found"})
		return
	}
	if !h.rt.computerSettingOn(ctx, ownerID, computerID, store.SettingPackagesPublish) {
		writeAPIJSON(w, http.StatusConflict, releaseAPIError{Error: "package publishing is off on this computer; the owner turns on the setting " + store.SettingPackagesPublish, Reason: "packages_publish_off"})
		return
	}
	operation, err := h.rt.selfdevOperations.Get(ctx, computerID, operationID)
	if err != nil {
		writeAPIJSON(w, http.StatusNotFound, releaseAPIError{Error: "self-development operation not found"})
		return
	}
	if operation.State != selfdev.StateApplied {
		writeAPIJSON(w, http.StatusConflict, releaseAPIError{Error: "operation is " + operation.State + "; only an applied operation is published", Reason: "operation_not_applied"})
		return
	}
	items, data, err := h.rt.operationMedia(operation)
	if err != nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, releaseAPIError{Error: err.Error(), Reason: "bundle_unavailable"})
		return
	}
	for i, item := range items {
		if item.SHA256 == digest {
			w.Header().Set("Content-Type", item.MediaType)
			w.Header().Set("Content-Length", strconv.Itoa(len(data[i])))
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(data[i])
			return
		}
	}
	writeAPIJSON(w, http.StatusNotFound, releaseAPIError{Error: "media not found: the operation's release names no such item"})
}
