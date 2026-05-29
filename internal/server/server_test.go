package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestHealthHandler(t *testing.T) {
	s := NewServer("test-service", "8099")
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	s.defaultHealthHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if body["status"] != "ok" {
		t.Errorf("expected status \"ok\", got %q", body["status"])
	}
	if body["service"] != "test-service" {
		t.Errorf("expected service \"test-service\", got %q", body["service"])
	}

	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected Content-Type to contain application/json, got %q", ct)
	}
}

func TestHealthHandlerIncludesAddrAfterStart(t *testing.T) {
	s := NewServer("test-addr-service", "0")

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.Start()
	}()
	defer func() {
		s.Shutdown()
		wg.Wait()
	}()

	time.Sleep(100 * time.Millisecond)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	s.defaultHealthHandler(w, req)

	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	// After the server starts, the Addr field should be populated.
	if body["addr"] == "" {
		t.Error("expected non-empty addr in health response after server start")
	}
	if !strings.HasPrefix(body["addr"], "127.0.0.1:") {
		t.Errorf("expected addr to start with 127.0.0.1:, got %q", body["addr"])
	}
}

func TestHealthHandlerServiceName(t *testing.T) {
	names := []string{"auth", "proxy", "vmctl", "gateway", "sandbox"}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			s := NewServer(name, "8099")
			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			w := httptest.NewRecorder()
			s.defaultHealthHandler(w, req)

			var body map[string]string
			if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
				t.Fatalf("failed to decode response body: %v", err)
			}
			if body["service"] != name {
				t.Errorf("expected service %q, got %q", name, body["service"])
			}
		})
	}
}

func TestHealthHandlerMethodNotAllowed(t *testing.T) {
	s := NewServer("test-service", "8099")
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/health", nil)
			w := httptest.NewRecorder()
			s.defaultHealthHandler(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("expected status 405 for %s, got %d", method, w.Code)
			}
		})
	}
}

func TestPortFromEnv(t *testing.T) {
	envVar := "TEST_PORT_FOR_ENV"
	_ = os.Setenv(envVar, "9999")
	defer func() { _ = os.Unsetenv(envVar) }()

	port := PortFromEnv(envVar, "8081")
	if port != "9999" {
		t.Errorf("expected port 9999, got %q", port)
	}
}

func TestPortDefault(t *testing.T) {
	envVar := "TEST_PORT_DEFAULT_UNSET"
	_ = os.Unsetenv(envVar)

	port := PortFromEnv(envVar, "8081")
	if port != "8081" {
		t.Errorf("expected default port 8081, got %q", port)
	}
}

func TestBindHostDefault(t *testing.T) {
	_ = os.Unsetenv("SERVER_HOST")

	host := BindHostFromEnv()
	if host != "127.0.0.1" {
		t.Errorf("expected default bind host 127.0.0.1, got %q", host)
	}
}

func TestBindHostFromEnv(t *testing.T) {
	_ = os.Setenv("SERVER_HOST", "0.0.0.0")
	defer func() { _ = os.Unsetenv("SERVER_HOST") }()

	host := BindHostFromEnv()
	if host != "0.0.0.0" {
		t.Errorf("expected bind host 0.0.0.0, got %q", host)
	}
}

func TestNewServerBindsToLocalhostByDefault(t *testing.T) {
	_ = os.Unsetenv("SERVER_HOST")

	s := NewServer("test-localhost", "0")

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.Start()
	}()
	defer func() {
		s.Shutdown()
		wg.Wait()
	}()

	time.Sleep(100 * time.Millisecond)

	addr := s.Addr()
	if !strings.HasPrefix(addr, "127.0.0.1:") {
		t.Errorf("expected server to bind to 127.0.0.1, got addr %q", addr)
	}

	// Verify the server is reachable on localhost.
	resp, err := http.Get("http://" + addr + "/health")
	if err != nil {
		t.Fatalf("failed to reach /health on localhost: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestServerStartAndAcceptsRequests(t *testing.T) {
	s := NewServer("test-service", "0") // port 0 = OS picks a free port

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.Start()
	}()

	// Give the server a moment to start
	time.Sleep(100 * time.Millisecond)

	// Server should be listening now
	addr := s.Addr()
	if addr == "" {
		t.Fatal("server address is empty after start")
	}

	resp, err := http.Get("http://" + addr + "/health")
	if err != nil {
		t.Fatalf("failed to reach /health: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	// Shutdown the server
	s.Shutdown()
	wg.Wait()
}

func TestGracefulShutdownOnSIGTERM(t *testing.T) {
	s := NewServer("test-sigterm", "0")

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.Start()
	}()

	time.Sleep(100 * time.Millisecond)

	addr := s.Addr()
	// Verify server is up
	resp, err := http.Get("http://" + addr + "/health")
	if err != nil {
		t.Fatalf("server not reachable before SIGTERM: %v", err)
	}
	_ = resp.Body.Close()

	// Send SIGTERM to ourselves
	p, _ := os.FindProcess(os.Getpid())
	_ = p.Signal(syscall.SIGTERM)

	// Wait for server to shut down with a timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success: server shut down cleanly
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down within 5 seconds of SIGTERM")
	}
}

func TestGracefulShutdownOnSIGINT(t *testing.T) {
	s := NewServer("test-sigint", "0")

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.Start()
	}()

	time.Sleep(100 * time.Millisecond)

	addr := s.Addr()
	resp, err := http.Get("http://" + addr + "/health")
	if err != nil {
		t.Fatalf("server not reachable before SIGINT: %v", err)
	}
	_ = resp.Body.Close()

	p, _ := os.FindProcess(os.Getpid())
	_ = p.Signal(syscall.SIGINT)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down within 5 seconds of SIGINT")
	}
}

func TestGracefulShutdownWaitsForInFlightRequest(t *testing.T) {
	s := NewServer("test-inflight", "0")
	s.shutdownTimeout = 2 * time.Second

	started := make(chan struct{})
	release := make(chan struct{})
	var startedOnce sync.Once

	s.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		startedOnce.Do(func() { close(started) })
		<-release
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("done"))
	})

	startDone := make(chan struct{})
	go func() {
		s.Start()
		close(startDone)
	}()

	time.Sleep(100 * time.Millisecond)
	addr := s.Addr()

	slowDone := make(chan struct{})
	go func() {
		resp, err := http.Get("http://" + addr + "/slow")
		if err != nil {
			t.Errorf("slow request error: %v", err)
			close(slowDone)
			return
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("slow request got status %d, expected 200", resp.StatusCode)
		}
		close(slowDone)
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("slow request did not start")
	}

	select {
	case <-startDone:
		t.Fatal("server returned from Start before shutdown was requested")
	default:
	}

	shutdownDone := make(chan struct{})
	go func() {
		s.Shutdown()
		close(shutdownDone)
	}()

	select {
	case <-startDone:
		t.Fatal("Start returned before the in-flight request completed")
	case <-time.After(100 * time.Millisecond):
	}

	close(release)

	select {
	case <-slowDone:
	case <-time.After(2 * time.Second):
		t.Fatal("in-flight request was not completed during graceful shutdown")
	}
	select {
	case <-shutdownDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Shutdown did not complete after in-flight request finished")
	}
	select {
	case <-startDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after graceful shutdown completed")
	}
}

func TestShutdownTimeoutFromEnv(t *testing.T) {
	t.Setenv("SERVER_SHUTDOWN_TIMEOUT", "11m")
	if got := ShutdownTimeoutFromEnv(); got != 11*time.Minute {
		t.Fatalf("timeout = %s, want 11m", got)
	}
}

func TestShutdownTimeoutFromEnvInvalidFallsBack(t *testing.T) {
	t.Setenv("SERVER_SHUTDOWN_TIMEOUT", "not-a-duration")
	if got := ShutdownTimeoutFromEnv(); got != defaultShutdownTimeout {
		t.Fatalf("timeout = %s, want %s", got, defaultShutdownTimeout)
	}
}
