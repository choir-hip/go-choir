package zot

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type SessionConfig struct {
	SessionID string
	RootDir   string
	UserID    string
}

type eventRecord struct {
	Time      string `json:"time"`
	SessionID string `json:"session_id"`
	Type      string `json:"type"`
	Text      string `json:"text,omitempty"`
	Command   string `json:"command,omitempty"`
	Output    string `json:"output,omitempty"`
	ExitCode  int    `json:"exit_code,omitempty"`
	Report    string `json:"report,omitempty"`
	Error     string `json:"error,omitempty"`
}

func RunSession(cfg SessionConfig, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	if strings.TrimSpace(cfg.SessionID) == "" {
		cfg.SessionID = fmt.Sprintf("zot-%d", time.Now().UnixNano())
	}
	if strings.TrimSpace(cfg.RootDir) == "" {
		cfg.RootDir = "."
	}
	sessionDir := filepath.Join(cfg.RootDir, ".choir", "zot", "sessions", safePathSegment(cfg.SessionID))
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		fmt.Fprintf(stderr, "zot: create session directory: %v\n", err)
		return 1
	}
	logPath := filepath.Join(sessionDir, "session.jsonl")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintf(stderr, "zot: open session log: %v\n", err)
		return 1
	}
	defer func() { _ = logFile.Close() }()

	writeEvent(logFile, cfg.SessionID, eventRecord{Type: "start", Text: "zot repair session started"})
	fmt.Fprintf(stdout, "zot repair session %s\n", cfg.SessionID)
	fmt.Fprintf(stdout, "session log: %s\n", logPath)
	fmt.Fprint(stdout, "zot> ")

	scanner := bufio.NewScanner(stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r\n")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			fmt.Fprint(stdout, "zot> ")
			continue
		}
		if trimmed == "exit" || trimmed == "quit" {
			writeEvent(logFile, cfg.SessionID, eventRecord{Type: "done", Text: "zot repair session ended by user"})
			fmt.Fprintln(stdout, "zot session closed")
			return 0
		}
		if strings.HasPrefix(trimmed, "!") {
			runCommand(cfg, logFile, stdout, trimmed[1:])
			fmt.Fprint(stdout, "zot> ")
			continue
		}
		reportPath := writeDiagnosisReport(cfg, sessionDir, trimmed)
		writeEvent(logFile, cfg.SessionID, eventRecord{Type: "diagnosis_report", Text: trimmed, Report: reportPath})
		fmt.Fprintf(stdout, "diagnosis report: %s\n", reportPath)
		fmt.Fprint(stdout, "zot> ")
	}
	if err := scanner.Err(); err != nil {
		writeEvent(logFile, cfg.SessionID, eventRecord{Type: "error", Error: err.Error()})
		fmt.Fprintf(stderr, "zot: input error: %v\n", err)
		return 1
	}
	writeEvent(logFile, cfg.SessionID, eventRecord{Type: "done", Text: "zot repair session input closed"})
	return 0
}

func runCommand(cfg SessionConfig, logFile io.Writer, stdout io.Writer, command string) {
	command = strings.TrimSpace(command)
	if command == "" {
		return
	}
	writeEvent(logFile, cfg.SessionID, eventRecord{Type: "command", Command: command})
	cmd := exec.Command("/bin/sh", "-lc", command)
	cmd.Dir = cfg.RootDir
	cmd.Env = append(os.Environ(), "ZOT_SESSION_ID="+cfg.SessionID)
	output, err := cmd.CombinedOutput()
	exitCode := 0
	if err != nil {
		exitCode = 1
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}
	text := string(output)
	writeEvent(logFile, cfg.SessionID, eventRecord{Type: "command_result", Command: command, Output: text, ExitCode: exitCode})
	if text != "" {
		fmt.Fprint(stdout, text)
		if !strings.HasSuffix(text, "\n") {
			fmt.Fprintln(stdout)
		}
	}
	fmt.Fprintf(stdout, "[exit %d]\n", exitCode)
}

func writeDiagnosisReport(cfg SessionConfig, sessionDir, prompt string) string {
	reportPath := filepath.Join(sessionDir, "diagnosis.md")
	body := strings.Builder{}
	body.WriteString("# zot diagnosis report\n\n")
	body.WriteString("- session: `" + cfg.SessionID + "`\n")
	if cfg.UserID != "" {
		body.WriteString("- user: `" + cfg.UserID + "`\n")
	}
	body.WriteString("- root: `" + cfg.RootDir + "`\n")
	body.WriteString("- prompt: " + prompt + "\n\n")
	body.WriteString("## Current theory\n\n")
	body.WriteString("zot is running out-of-process from the runtime MAS. This report is an ordinary markdown artifact; Texture may open it, but zot does not write canonical `.texture` files.\n\n")
	body.WriteString("## Evidence handles\n\n")
	body.WriteString("- session log: `session.jsonl`\n")
	body.WriteString("- command actuator: `!` lines executed from the user computer root\n")
	body.WriteString("- unified runtime evidence remains in machine-readable product logs and API records\n")
	_ = os.WriteFile(reportPath, []byte(body.String()), 0o644)
	return reportPath
}

func writeEvent(w io.Writer, sessionID string, rec eventRecord) {
	rec.Time = time.Now().UTC().Format(time.RFC3339Nano)
	rec.SessionID = sessionID
	if b, err := json.Marshal(rec); err == nil {
		_, _ = w.Write(append(b, '\n'))
	}
}

func safePathSegment(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "session"
	}
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	return b.String()
}
