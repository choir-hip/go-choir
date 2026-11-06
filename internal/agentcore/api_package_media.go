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
		path := filepath.Join(stage, item.Name)
		if info, err := os.Lstat(path); err != nil || !info.Mode().IsRegular() || info.Size() != int64(item.Bytes) {
			return nil, false
		}
		raw, err := os.ReadFile(path)
		if err != nil || pkgrelease.VerifyMedia(item, raw) != nil {
			return nil, false
		}
		return raw, true
	}
	screens := append([]capsule.ScreenFile{}, bundle.Screens...)
	sort.Slice(screens, func(i, j int) bool { return screens[i].Name < screens[j].Name })
	// Members of a before/after pair are chosen first, so the cap never
	// drops them; the chosen items are then listed by name.
	paired := pairedViews(screens)
	sort.SliceStable(screens, func(i, j int) bool { return paired[screenViewKey(screens[i])] && !paired[screenViewKey(screens[j])] })
	type chosen struct {
		item pkgrelease.Media
		raw  []byte
	}
	var picks []chosen
	// A record names each digest once: a second screen with the same bytes
	// (two shots rendering the same pixels, a continuity pair among them) is
	// skipped, never a refusal of the whole draft. The pair itself is proved
	// on the bundle's screens (bundleHasBeforeAfter), not on these items.
	named := map[string]bool{}
	for _, s := range screens {
		if len(picks) == pkgrelease.MaxScreenshots {
			break
		}
		if named[s.SHA256] || s.Blank || s.Bytes > pkgrelease.MaxScreenshotBytes || s.Width > pkgrelease.MaxScreenshotSide || s.Height > pkgrelease.MaxScreenshotSide || !capsule.ValidScreenName(s.Name) {
			continue
		}
		item := pkgrelease.Media{Kind: pkgrelease.MediaKindScreenshot, Name: s.Name, MediaType: pkgrelease.MediaTypePNG, SHA256: s.SHA256, Bytes: int(s.Bytes), Width: s.Width, Height: s.Height}
		if raw, ok := read(item); ok {
			picks = append(picks, chosen{item, raw})
			named[s.SHA256] = true
		}
	}
	sort.Slice(picks, func(i, j int) bool { return picks[i].item.Name < picks[j].item.Name })
	var items []pkgrelease.Media
	var data [][]byte
	for _, p := range picks {
		items, data = append(items, p.item), append(data, p.raw)
	}
	if v := bundle.Video; v != nil && !named[v.SHA256] && v.Bytes <= pkgrelease.MaxVideoBytes && capsule.ValidVideoName(v.Name) {
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
	operation, err := h.rt.selfdevOperations.Get(ctx, computerID, operationID)
	if err != nil {
		writeAPIJSON(w, http.StatusNotFound, releaseAPIError{Error: "self-development operation not found"})
		return
	}
	switch operation.State {
	case selfdev.StateAccepted, selfdev.StateMaterializing, selfdev.StateApplied:
	default:
		writeAPIJSON(w, http.StatusConflict, releaseAPIError{Error: "operation is " + operation.State + "; only a decided package operation is published", Reason: "operation_not_decided"})
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

// screenViewKey is a screen's view when it follows the before/after naming.
func screenViewKey(s capsule.ScreenFile) string {
	_, view := capsule.ScreenRole(s.Name)
	return view
}

// pairedViews are the views with both a before and an after screen.
func pairedViews(screens []capsule.ScreenFile) map[string]bool {
	before, after := map[string]bool{}, map[string]bool{}
	for _, s := range screens {
		role, view := capsule.ScreenRole(s.Name)
		switch {
		case view == "":
		case role == capsule.ScreenBefore:
			before[view] = true
		default:
			after[view] = true
		}
	}
	out := map[string]bool{}
	for v := range before {
		if after[v] {
			out[v] = true
		}
	}
	delete(out, "")
	return out
}

// bundleHasBeforeAfter says the operation's bound bundle carries a
// before/after pair of one view (the bundle's screens, not the published
// media, which dedupe identical pixels and cap the count).
func (rt *Runtime) bundleHasBeforeAfter(operation selfdev.Operation) (bool, error) {
	if strings.TrimSpace(rt.selfdevUpdaterRoot) == "" || !computerevent.IsSHA256(operation.BundleDigest) {
		return false, errBundleUnavailable
	}
	bundle, _, err := readBoundSelfDevelopmentBundle(filepath.Join(rt.selfdevUpdaterRoot, "incoming", operation.BundleDigest), operation)
	if err != nil {
		return false, err
	}
	return capsule.HasBeforeAfterPair(bundle.Screens), nil
}
