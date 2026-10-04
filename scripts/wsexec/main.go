// wsexec: tiny host-side helper that opens a guest autoputer terminal WS and
// runs one shell command inside the guest PTY. Used by the S1a refusal-matrix
// probe to issue truly guest-originated curl requests.
//
// Usage: wsexec -url ws://GUEST:8085/api/terminal/ws -user OWNER -cmd 'curl …'
// Stdout: PTY output until the __WS_EXEC_DONE__ marker prints, then the exit
// code on the last line as EXIT=n.
package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	var url, user, cmd string
	var timeout time.Duration
	flag.StringVar(&url, "url", "", "ws:// guest terminal endpoint")
	flag.StringVar(&user, "user", "", "X-Authenticated-User value")
	flag.StringVar(&cmd, "cmd", "", "command to run")
	flag.DurationVar(&timeout, "timeout", 30*time.Second, "overall deadline")
	flag.Parse()
	if url == "" || cmd == "" {
		fmt.Fprintln(os.Stderr, "usage: wsexec -url WSURL -user OWNER -cmd CMD")
		os.Exit(2)
	}

	hdr := http.Header{}
	if user != "" {
		hdr.Set("X-Authenticated-User", user)
	}
	hdr.Set("X-Internal-Caller", "true")
	conn, resp, err := websocket.DefaultDialer.Dial(url, hdr)
	if err != nil {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		fmt.Fprintf(os.Stderr, "dial failed (http %d): %v\n", status, err)
		os.Exit(1)
	}
	defer conn.Close()

	marker := fmt.Sprintf("__WS_EXEC_DONE_%d__", time.Now().UnixNano())
	_ = conn.WriteJSON(map[string]any{"type": "resize", "cols": 220, "rows": 50})
	_ = conn.WriteJSON(map[string]any{"type": "input", "data": cmd + "; echo " + marker + "=$?\n"})

	deadline := time.Now().Add(timeout)
	var out strings.Builder
	for time.Now().Before(deadline) {
		_ = conn.SetReadDeadline(deadline)
		var msg struct {
			Type string `json:"type"`
			Data string `json:"data"`
		}
		if err := conn.ReadJSON(&msg); err != nil {
			break
		}
		if msg.Type != "output" {
			continue
		}
		out.WriteString(msg.Data)
		s := out.String()
		if idx := strings.Index(s, marker+"="); idx >= 0 {
			rest := s[idx+len(marker)+1:]
			fields := strings.Fields(rest)
			code := "?"
			if len(fields) > 0 {
				code = fields[0]
			}
			fmt.Print(s[:idx])
			fmt.Printf("\nEXIT=%s\n", code)
			return
		}
	}
	fmt.Print(out.String())
	fmt.Println("\nEXIT=timeout")
}
