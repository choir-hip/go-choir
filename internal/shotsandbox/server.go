package shotsandbox

// The server half of scripts/choir-shot.mjs, in Go: the autoputer's
// environment, the static frontend server that proxies /api to it as the
// owner (WebSocket upgrades included), the /auth stub, the readiness wait and
// the per-view desktop reset.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DesktopStatePath is the shell's own save path for its windows.
const DesktopStatePath = "/api/desktop/state"

// isProxied is the rule for what the served frontend sends to the autoputer:
// every /api URL. /auth/session is answered locally; other /auth URLs 404.
func isProxied(p string) bool { return p == "/api" || strings.HasPrefix(p, "/api/") }

var contentTypes = map[string]string{
	".html": "text/html; charset=utf-8", ".js": "text/javascript", ".mjs": "text/javascript", ".css": "text/css",
	".json": "application/json", ".svg": "image/svg+xml", ".png": "image/png", ".woff2": "font/woff2",
	".ico": "image/x-icon", ".webmanifest": "application/manifest+json", ".wasm": "application/wasm",
}

func typeOf(name string) string {
	ext := strings.ToLower(path.Ext(name))
	if t, ok := contentTypes[ext]; ok {
		return t
	}
	if t := mime.TypeByExtension(ext); t != "" {
		return t
	}
	return "application/octet-stream"
}

// NewFrontendHandler serves dist (an SPA: an unknown path without an
// extension answers index.html, an unknown asset 404s), proxies /api to the
// autoputer on 127.0.0.1:autoputerPort as user, and answers /auth/session.
func NewFrontendHandler(dist string, autoputerPort int, user string) http.Handler {
	target := &url.URL{Scheme: "http", Host: "127.0.0.1:" + strconv.Itoa(autoputerPort)}
	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(target)
			r.Out.Host = target.Host
			// The owner identity is the server's to state, never the page's.
			r.Out.Header.Del("X-Authenticated-User")
			r.Out.Header.Set("X-Authenticated-User", user)
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusBadGateway)
			fmt.Fprintf(w, "autoputer: %v", err)
		},
	}
	dist = filepath.Clean(dist)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch {
		case isProxied(p):
			proxy.ServeHTTP(w, r)
			return
		case p == "/auth/session":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"authenticated": true, "user": map[string]string{"id": user, "username": user}})
			return
		case strings.HasPrefix(p, "/auth/"):
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		clean := path.Clean("/" + p)
		file := filepath.Join(dist, filepath.FromSlash(clean))
		if rel, err := filepath.Rel(dist, file); err != nil || strings.HasPrefix(rel, "..") {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		info, err := os.Stat(file)
		if err != nil || info.IsDir() {
			if path.Ext(clean) != "" {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			file = filepath.Join(dist, "index.html") // the SPA shell
		}
		f, err := os.Open(file)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		defer f.Close()
		w.Header().Set("Content-Type", typeOf(file))
		_, _ = io.Copy(w, f)
	})
}

// resetDesktop empties the owner's persisted desktop (PUT /api/desktop/state
// with no windows, as the driver) through the served origin, so a view starts
// from a clean desktop: the shell restores saved windows on load, and the
// autoputer keeps them across views.
func resetDesktop(ctx context.Context, client *http.Client, base string) error {
	body := strings.NewReader(`{"windows":[],"active_window_id":"","driver":true}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, base+DesktopStatePath, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Choir-Driver", "true")
	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("desktop reset: %w", err)
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 1<<20))
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return fmt.Errorf("desktop reset failed (%d)", res.StatusCode)
	}
	return nil
}

// autoputerEnv is the autoputer's whole environment: a scrubbed base, the
// private state, one loopback port and the stub model provider. It carries no
// gateway token, so a model call can only reach the stub.
func autoputerEnv(spec Spec, port int) []string {
	env := append(baseEnv(spec),
		"RUNTIME_STORE_PATH="+spec.StorePath,
		"AUTOPUTER_FILES_ROOT="+spec.FilesRoot,
		"AUTOPUTER_PORT="+strconv.Itoa(port),
		"CHOIR_GUEST_TRUST_LOOPBACK=1",
		"RUNTIME_LLM_PROVIDER=stub",
	)
	return env
}

// baseEnv is what the stage's children share: PATH and LANG, HOME and TMPDIR
// in the work directory, fontconfig, and the launcher's allowlisted extras.
func baseEnv(spec Spec) []string {
	env := []string{
		"PATH=" + firstNonEmpty(os.Getenv("PATH"), "/usr/bin:/bin"),
		"LANG=" + firstNonEmpty(os.Getenv("LANG"), "C.UTF-8"),
		"HOME=" + filepath.Join(spec.WorkDir, "home"),
		"TMPDIR=" + filepath.Join(spec.WorkDir, "tmp"),
	}
	if spec.Fontconfig != "" {
		env = append(env, "FONTCONFIG_FILE="+spec.Fontconfig)
	}
	return append(env, spec.PassEnv...)
}

func firstNonEmpty(a ...string) string {
	for _, s := range a {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

// tailBuffer keeps the last max bytes written to it.
type tailBuffer struct {
	mu  sync.Mutex
	buf []byte
	max int
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, p...)
	if len(t.buf) > t.max {
		t.buf = t.buf[len(t.buf)-t.max:]
	}
	return len(p), nil
}

func (t *tailBuffer) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return strings.TrimSpace(string(bytes.ToValidUTF8(t.buf, []byte("?"))))
}

// freeLoopbackPort asks the stage's own loopback for an unused port.
func freeLoopbackPort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// waitForPort waits for 127.0.0.1:port to accept a connection while the
// process is alive (exited closes when it is not): a death before readiness
// is a failure with the process's own last words.
func waitForPort(ctx context.Context, port int, exited <-chan struct{}, deadline time.Duration, tail *tailBuffer) error {
	end := time.Now().Add(deadline)
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	for {
		select {
		case <-exited:
			return fmt.Errorf("the candidate autoputer exited before port %d accepted connections: %s", port, tailOr(tail, "(no output)"))
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		conn, err := net.DialTimeout("tcp", addr, time.Second)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		if time.Now().After(end) {
			return fmt.Errorf("the candidate autoputer did not accept connections on %s within %s: %s", addr, deadline, tailOr(tail, "(no output)"))
		}
		select {
		case <-time.After(150 * time.Millisecond):
		case <-ctx.Done():
			return ctx.Err()
		case <-exited:
		}
	}
}

func tailOr(t *tailBuffer, fallback string) string {
	if s := t.String(); s != "" {
		return s
	}
	return fallback
}

// startAutoputer starts spec.Binary over the private state with port and
// returns the process, a channel closed when it exits, and its output tail.
func startAutoputer(spec Spec, port int) (*exec.Cmd, <-chan struct{}, *tailBuffer, error) {
	if spec.Binary == "" {
		return nil, nil, nil, errors.New("no autoputer binary")
	}
	tail := &tailBuffer{max: 4096}
	log, err := os.OpenFile(filepath.Join(spec.WorkDir, "autoputer.log"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, nil, nil, err
	}
	cmd := exec.Command(spec.Binary)
	cmd.Env = autoputerEnv(spec, port)
	cmd.Dir = spec.WorkDir
	cmd.Stdout = io.MultiWriter(log, tail)
	cmd.Stderr = io.MultiWriter(log, tail)
	setChildAttr(cmd, spec)
	if err := cmd.Start(); err != nil {
		_ = log.Close()
		return nil, nil, nil, fmt.Errorf("the candidate autoputer did not start: %w", err)
	}
	exited := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		_ = log.Close()
		close(exited)
	}()
	return cmd, exited, tail, nil
}
