package shotsandbox

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// TestMain makes the test binary stand in for three things: the sandbox
// stage (the launcher re-executes it), and a fake autoputer in three moods.
func TestMain(m *testing.M) {
	if mode := os.Getenv("SHOTSANDBOX_FAKE"); mode != "" {
		runFakeAutoputer(mode)
		return
	}
	testStageHook() // runs the stage and exits when this process is one
	os.Exit(m.Run())
}

// runFakeAutoputer: "ok" serves /api on AUTOPUTER_PORT, echoing the owner
// header and counting desktop resets in $WORKDIR/resets; "die" prints a
// panic and exits 3; "silent" never listens.
func runFakeAutoputer(mode string) {
	switch mode {
	case "die":
		fmt.Fprintln(os.Stderr, "panic: store schema is newer than this build")
		os.Exit(3)
	case "silent":
		time.Sleep(time.Hour)
	}
	port := os.Getenv("AUTOPUTER_PORT")
	mux := http.NewServeMux()
	mux.HandleFunc("/api/ping", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "user=%s store=%s files=%s trust=%s provider=%s", r.Header.Get("X-Authenticated-User"),
			os.Getenv("RUNTIME_STORE_PATH"), os.Getenv("AUTOPUTER_FILES_ROOT"), os.Getenv("CHOIR_GUEST_TRUST_LOOPBACK"), os.Getenv("RUNTIME_LLM_PROVIDER"))
	})
	mux.HandleFunc("/api/desktop/state", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || !strings.Contains(r.Header.Get("X-Authenticated-User"), "owner") {
			http.Error(w, "bad reset", http.StatusBadRequest)
			return
		}
		f, err := os.OpenFile("resets", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err == nil {
			fmt.Fprintln(f, "reset")
			f.Close()
		}
		w.WriteHeader(http.StatusOK)
	})
	_ = http.ListenAndServe("127.0.0.1:"+port, mux)
	os.Exit(4)
}
