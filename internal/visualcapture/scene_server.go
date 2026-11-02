package visualcapture

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

// The capture server plays one package scene at a time (packages.go). With
// no scene it is the static, signed-out server. During a scene it also
// answers, and only these: /auth/session as a signed-in fixture user, the
// four /api routes the shell reads to restore one window
// (docs/design/app-design-bar-2026-10-26.md "How the screenshots were
// made"), and the fixture documents under FixturePrefix. Every other /api
// route stays 503, and writes are accepted, then dropped, only on the
// routes the shell saves to while it restores the window.

// sceneUserID is the fixture user a scene signs in as.
const sceneUserID = "capture-fixture-user"

// sceneFixedTime stamps every scene answer, so its inputs are the same on
// every run.
const sceneFixedTime = "2026-01-01T12:00:00Z"

// sceneReads are the /api routes a scene answers.
var sceneReads = map[string]bool{
	"/api/shell/bootstrap":   true,
	"/api/desktop/state":     true,
	"/api/preferences/theme": true,
	"/api/media/recents":     true,
}

// sceneWrites are the routes whose writes a scene accepts and drops.
var sceneWrites = map[string]bool{
	"/api/desktop/state":     true,
	"/api/preferences/theme": true,
	"/api/media/recents":     true,
}

type sceneState struct {
	scene   Scene
	answers map[string][]byte
}

type captureServer struct {
	root   string
	static http.Handler
	scene  atomic.Pointer[sceneState]
	holdNS atomic.Int64
}

func newCaptureServer(root string) *captureServer {
	return &captureServer{root: root, static: staticServer(root)}
}

// sceneHoldPath is an image the scene's page loads and the server answers
// only after the hold: it keeps the page's load event, and so Chromium's
// screenshot, back by that much wall time (SceneBrowserArgs).
const sceneHoldPath = "/choir-capture-hold.gif"

// sceneTimeoutSlackMS is how far past the hold Chromium's --timeout lies.
const sceneTimeoutSlackMS = 15000

// sceneHoldTag is what a scene's index.html gains before </body>.
const sceneHoldTag = `<img src="` + sceneHoldPath + `" alt="" width="1" height="1" style="position:absolute;left:-9999px">`

// transparentGIF is a 1x1 transparent GIF.
var transparentGIF = []byte("GIF89a\x01\x00\x01\x00\x80\x00\x00\x00\x00\x00\x00\x00\x00!\xf9\x04\x01\x00\x00\x00\x00,\x00\x00\x00\x00\x01\x00\x01\x00\x00\x02\x02D\x01\x00;")

// setHold sets how long the hold image waits.
func (s *captureServer) setHold(d time.Duration) { s.holdNS.Store(int64(d)) }

// setScene makes sc the scene the server plays (nil: none). kinds are the
// package's fixture kinds, listed as its recent documents.
func (s *captureServer) setScene(sc *Scene, kinds []Fixture) {
	if sc == nil {
		s.scene.Store(nil)
		return
	}
	st := &sceneState{scene: *sc, answers: sceneAnswers(*sc, kinds)}
	s.scene.Store(st)
}

func (s *captureServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	st := s.scene.Load()
	if st == nil {
		s.static.ServeHTTP(w, r)
		return
	}
	p := r.URL.Path
	write := r.Method == http.MethodPut || r.Method == http.MethodPost
	switch {
	case p == "/auth/session" && r.Method == http.MethodGet:
		writeSceneJSON(w, st.answers[p])
	case p == "/" && r.Method == http.MethodGet:
		s.serveSceneIndex(w, r)
	case p == sceneHoldPath && r.Method == http.MethodGet:
		t := time.NewTimer(time.Duration(s.holdNS.Load()))
		defer t.Stop()
		select {
		case <-t.C:
		case <-r.Context().Done():
			return
		}
		w.Header().Set("Content-Type", "image/gif")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(transparentGIF)
	case strings.HasPrefix(p, FixturePrefix) && (r.Method == http.MethodGet || r.Method == http.MethodHead):
		name := strings.TrimPrefix(p, FixturePrefix)
		data, ok := fixtureFiles[name]
		if !ok || strings.Contains(name, "/") {
			http.NotFound(w, r)
			return
		}
		ct := ""
		for _, f := range fixtures {
			if f.File == name {
				ct = f.MediaType
			}
		}
		w.Header().Set("Content-Type", ct)
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(data)
	case write && sceneWrites[p]:
		_, _ = io.Copy(io.Discard, io.LimitReader(r.Body, 256<<10))
		if p == "/api/preferences/theme" {
			writeSceneJSON(w, st.answers[p])
			return
		}
		writeSceneJSON(w, []byte(`{"ok":true,"updated_at":"`+sceneFixedTime+`"}`))
	case r.Method == http.MethodGet && sceneReads[p]:
		writeSceneJSON(w, st.answers[p])
	default:
		s.static.ServeHTTP(w, r)
	}
}

// serveSceneIndex serves the built index.html with the hold image added.
func (s *captureServer) serveSceneIndex(w http.ResponseWriter, r *http.Request) {
	full, ok := regularUnder(s.root, "index.html")
	if !ok {
		http.NotFound(w, r)
		return
	}
	page, err := os.ReadFile(full)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if i := bytes.LastIndex(page, []byte("</body>")); i >= 0 {
		page = append(page[:i:i], append([]byte(sceneHoldTag), page[i:]...)...)
	} else {
		page = append(page, sceneHoldTag...)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(page)
}

func writeSceneJSON(w http.ResponseWriter, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(body)
}

// sceneAnswers builds a scene's fixed answers.
func sceneAnswers(sc Scene, kinds []Fixture) map[string][]byte {
	mustJSON := func(v any) []byte {
		raw, err := json.Marshal(v)
		if err != nil {
			panic(err)
		}
		return raw
	}
	geometry := map[string]int{"x": 24, "y": 16, "width": DesktopWindowWidth, "height": DesktopWindowHeight}
	if sc.Viewport == ViewportPhone {
		geometry = map[string]int{"x": 0, "y": 0, "width": PhoneWidth, "height": PhoneHeight}
	}
	mode := "normal"
	if strings.HasPrefix(sc.State, stateMax) {
		mode = "maximized"
	}
	appContext := map[string]any{}
	title := sc.Package
	if sc.Fixture != "" {
		f := fixtures[sc.Fixture]
		source := FixturePrefix + f.File
		if sc.State == StateError {
			source = FixturePrefix + "missing-" + f.File
		}
		title = f.Title
		appContext = map[string]any{"windowTitle": f.Title, "title": f.Title, "fileName": f.File, "sourceUrl": source, "mediaType": f.MediaType}
	}
	if mode == "maximized" {
		// A cheap generic hook: a package that renders a present view can
		// read it; the shell maximizes the window either way. Fullscreen
		// itself needs a user gesture, which a screenshot run cannot give.
		appContext["present"] = true
	}
	var recents []map[string]any
	for _, f := range kinds {
		recents = append(recents, map[string]any{
			"identity": "capture-fixture-" + f.Kind, "kind": sc.Package, "title": f.Title, "file_name": f.File,
			"source_url": FixturePrefix + f.File, "media_type": f.MediaType, "opened_at": sceneFixedTime,
		})
	}
	if recents == nil {
		recents = []map[string]any{}
	}
	return map[string][]byte{
		"/auth/session": mustJSON(map[string]any{"authenticated": true, "user": map[string]any{
			"id": sceneUserID, "user_id": sceneUserID, "email": "capture-fixture@example.invalid", "display_name": "Capture fixture",
		}}),
		"/api/shell/bootstrap": mustJSON(map[string]any{"ok": true, "computer_id": "computer-capture-fixture"}),
		"/api/desktop/state": mustJSON(map[string]any{
			"owner_id": sceneUserID,
			"windows": []map[string]any{{
				"window_id": "capture-window-1", "app_id": sc.Package, "title": title, "geometry": geometry,
				"mode": mode, "z_index": 5, "app_context": appContext,
			}},
			"active_window_id": "capture-window-1", "icon_positions": map[string]any{}, "updated_at": sceneFixedTime,
		}),
		"/api/preferences/theme": mustJSON(map[string]any{"theme": map[string]any{"schema_version": 2, "id": sc.Theme}}),
		"/api/media/recents":     mustJSON(map[string]any{"items": recents}),
	}
}
