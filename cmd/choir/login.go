package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// `choir login`, `choir logout` and `choir mcp install`
// (docs/design/cli-agent-onboarding-2026-11-03.md). Login runs the device
// authorization flow against the auth service and writes a full-access key to
// the credentials file, which the CLI and `choir mcp` read when no flag or
// env names a key.

const (
	credentialsEnvDir  = "XDG_CONFIG_HOME"
	credentialsSubpath = "choir/credentials"
	apiKeyFileEnvVar   = "CHOIR_API_KEY_FILE"
)

// loginPollStep is how much a slow_down (or a refused poll) adds to the
// interval (RFC 8628 3.5). Tests shorten it.
var loginPollStep = 5 * time.Second

// storedCredentials is the credentials file.
type storedCredentials struct {
	Host      string `json:"host"`
	APIKey    string `json:"api_key"`
	KeyID     string `json:"key_id,omitempty"`
	Label     string `json:"label,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

func credentialsPath() (string, error) {
	if dir := strings.TrimSpace(os.Getenv(credentialsEnvDir)); dir != "" {
		return filepath.Join(dir, credentialsSubpath), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find the credentials file: %w", err)
	}
	return filepath.Join(home, ".config", credentialsSubpath), nil
}

// loadCredentials reads the credentials file: (nil, nil) when there is none.
// A file that is not a regular mode-0600 file is refused, not read.
func loadCredentials() (*storedCredentials, error) {
	path, err := credentialsPath()
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		return nil, fmt.Errorf("%s must be a regular mode-0600 file (chmod 600 it, or run choir login again)", path)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var c storedCredentials
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("%s is not valid JSON: %w", path, err)
	}
	if !strings.HasPrefix(c.APIKey, apiKeyPrefix) || c.Host == "" {
		return nil, fmt.Errorf("%s holds no usable key; run choir login", path)
	}
	return &c, nil
}

// saveCredentials writes the file atomically, mode 0600 in a 0700 directory.
func saveCredentials(c *storedCredentials) (string, error) {
	path, err := credentialsPath()
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return "", err
	}
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(dir, ".credentials-*")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return "", err
	}
	if _, err := tmp.Write(append(raw, '\n')); err != nil {
		_ = tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return "", err
	}
	return path, nil
}

func normalizeHost(h string) string { return strings.TrimRight(strings.TrimSpace(h), "/") }

// resolveCredentials is the one resolution for the CLI and `choir mcp`:
// --api-key-file, $CHOIR_API_KEY, $CHOIR_API_KEY_FILE, the credentials file.
// The host is --host, then $CHOIR_HOST, then the credentials file's host
// (only when the key came from it), then the default. A key from the
// credentials file is never sent to another host.
func resolveCredentials(apiKeyFile, hostFlag string) (key, host string, err error) {
	explicitHost := normalizeHost(hostFlag)
	if explicitHost == "" {
		explicitHost = normalizeHost(os.Getenv(hostEnvVar))
	}
	switch {
	case strings.TrimSpace(apiKeyFile) != "":
		key, err = readCLISecretFile(apiKeyFile, cliStdin)
	case strings.TrimSpace(os.Getenv(apiKeyEnvVar)) != "":
		key = strings.TrimSpace(os.Getenv(apiKeyEnvVar))
	case strings.TrimSpace(os.Getenv(apiKeyFileEnvVar)) != "":
		key, err = readCLISecretFile(os.Getenv(apiKeyFileEnvVar), cliStdin)
	default:
		creds, cerr := loadCredentials()
		if cerr != nil {
			return "", "", cerr
		}
		if creds != nil {
			credHost := normalizeHost(creds.Host)
			if explicitHost != "" && explicitHost != credHost {
				return "", "", fmt.Errorf("the saved login is for %s, not %s: run choir login --host %s, or pass a key", credHost, explicitHost, explicitHost)
			}
			return creds.APIKey, credHost, nil
		}
	}
	if err != nil {
		return "", "", err
	}
	if key == "" {
		return "", "", fmt.Errorf("api key required: run choir login, or set --api-key-file, $%s or $%s", apiKeyEnvVar, apiKeyFileEnvVar)
	}
	if explicitHost == "" {
		explicitHost = defaultHost
	}
	return key, explicitHost, nil
}

func cliUserAgent() string { return "choir-cli/v0 (" + runtime.GOOS + ")" }

// ---- choir login ----

type deviceBegin struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

type deviceToken struct {
	APIKey string   `json:"api_key"`
	KeyID  string   `json:"key_id"`
	Label  string   `json:"label"`
	Scopes []string `json:"scopes"`
	Error  string   `json:"error"`
	Desc   string   `json:"error_description"`
}

func postJSON(hc *http.Client, url string, body any) (int, []byte, error) {
	raw, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", cliUserAgent())
	resp, err := hc.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	out, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode, out, err
}

// revokeKey revokes a key with itself (a key may always revoke itself). It
// reports whether the key is gone: revoked now, or already unknown (401).
func revokeKey(hc *http.Client, host, keyID, secret string) (bool, error) {
	req, err := http.NewRequest(http.MethodDelete, host+"/auth/api-keys/"+esc(keyID), nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+secret)
	req.Header.Set("User-Agent", cliUserAgent())
	resp, err := hc.Do(req)
	if err != nil {
		return false, err
	}
	_ = resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusNoContent, http.StatusOK, http.StatusUnauthorized, http.StatusNotFound:
		return true, nil
	}
	return false, fmt.Errorf("revoke %s: HTTP %d", keyID, resp.StatusCode)
}

func runLogin(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir login", flag.ContinueOnError)
	fs.SetOutput(stderr)
	hostFlag := fs.String("host", "", "Choir host. Defaults to $CHOIR_HOST, then the saved login's host, then "+defaultHost)
	hostname, _ := os.Hostname()
	defLabel := "choir CLI"
	if hostname != "" {
		defLabel = "choir CLI on " + hostname
	}
	label := fs.String("label", defLabel, "Name shown on the approve page and on the key in Settings")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "choir login: unexpected argument %q\n", fs.Arg(0))
		return 2
	}
	old, oldErr := loadCredentials()
	if oldErr != nil {
		fmt.Fprintf(stderr, "choir login: ignoring the saved login: %v\n", oldErr)
	}
	host := normalizeHost(*hostFlag)
	if host == "" {
		host = normalizeHost(os.Getenv(hostEnvVar))
	}
	if host == "" && old != nil {
		host = normalizeHost(old.Host)
	}
	if host == "" {
		host = defaultHost
	}
	hc := newHTTPClient(30 * time.Second)

	status, raw, err := postJSON(hc, host+"/auth/device/begin", map[string]string{"label": *label})
	if err != nil {
		fmt.Fprintf(stderr, "choir login: %v\n", err)
		return 1
	}
	var b deviceBegin
	if status != http.StatusOK || json.Unmarshal(raw, &b) != nil || b.DeviceCode == "" || b.UserCode == "" {
		fmt.Fprintf(stderr, "choir login: %s/auth/device/begin answered HTTP %d: %s\n", host, status, truncate(strings.TrimSpace(string(raw)), 300))
		return 1
	}
	link := b.VerificationURIComplete
	if link == "" {
		link = b.VerificationURI
	}
	expires := time.Duration(b.ExpiresIn) * time.Second
	if expires <= 0 {
		expires = 10 * time.Minute
	}
	fmt.Fprintf(stderr, "To sign in, open this link in a browser where you are signed in to Choir:\n\n  %s\n\nCheck that the page shows the code %s, then approve. Approving grants a full-access key (every scope, owner acts included) to this machine.\nWaiting for approval (the code expires in %s)...\n",
		link, b.UserCode, expires.Round(time.Second))

	interval := time.Duration(b.Interval) * time.Second
	deadline := time.Now().Add(expires)
	for {
		if time.Now().After(deadline) {
			fmt.Fprintln(stderr, "choir login: the code expired before it was approved; run choir login again")
			return 1
		}
		time.Sleep(interval)
		status, raw, err := postJSON(hc, host+"/auth/device/token", map[string]string{"device_code": b.DeviceCode})
		if err != nil {
			// The network may come back before the code expires.
			interval += loginPollStep
			continue
		}
		var tok deviceToken
		_ = json.Unmarshal(raw, &tok)
		if status == http.StatusOK && strings.HasPrefix(tok.APIKey, apiKeyPrefix) {
			creds := &storedCredentials{Host: host, APIKey: tok.APIKey, KeyID: tok.KeyID, Label: tok.Label, CreatedAt: time.Now().UTC().Format(time.RFC3339)}
			path, err := saveCredentials(creds)
			if err != nil {
				fmt.Fprintf(stderr, "choir login: approved, but writing the credentials file failed: %v (revoke key %s in Settings)\n", err, tok.KeyID)
				_, _ = revokeKey(hc, host, tok.KeyID, tok.APIKey)
				return 1
			}
			if old != nil && old.KeyID != "" && old.KeyID != tok.KeyID {
				if gone, err := revokeKey(hc, normalizeHost(old.Host), old.KeyID, old.APIKey); !gone {
					fmt.Fprintf(stderr, "choir login: could not revoke the previous key %s (%v); revoke it in Settings\n", old.KeyID, err)
				}
			}
			fmt.Fprintf(stderr, "Signed in. choir and choir mcp now use this key with no flags or env.\nAdd Choir to an agent: choir mcp install claude-code (or cursor, claude-desktop, codex).\n")
			return writeJSON(stdout, map[string]any{"logged_in": true, "host": host, "key_id": tok.KeyID, "label": tok.Label, "credentials": path})
		}
		switch tok.Error {
		case "authorization_pending":
		case "slow_down":
			interval += loginPollStep
		case "access_denied":
			fmt.Fprintln(stderr, "choir login: the login was denied on the approve page")
			return 1
		case "expired_token":
			fmt.Fprintln(stderr, "choir login: the code expired before it was approved; run choir login again")
			return 1
		case "invalid_grant":
			fmt.Fprintln(stderr, "choir login: the login request is unknown or was already used; run choir login again")
			return 1
		default:
			if status == http.StatusTooManyRequests || status >= 500 {
				interval += loginPollStep
				continue
			}
			fmt.Fprintf(stderr, "choir login: token poll answered HTTP %d: %s\n", status, truncate(strings.TrimSpace(string(raw)), 300))
			return 1
		}
	}
}

// ---- choir logout ----

func runLogout(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		fmt.Fprintln(stderr, "choir logout: takes no arguments")
		return 2
	}
	creds, err := loadCredentials()
	if err != nil {
		fmt.Fprintf(stderr, "choir logout: %v\n", err)
		return 1
	}
	if creds == nil {
		fmt.Fprintln(stderr, "choir logout: not logged in (no credentials file)")
		return 1
	}
	if creds.KeyID != "" {
		gone, err := revokeKey(newHTTPClient(30*time.Second), normalizeHost(creds.Host), creds.KeyID, creds.APIKey)
		if !gone {
			fmt.Fprintf(stderr, "choir logout: could not revoke key %s: %v (the credentials file is kept; try again, or revoke it in Settings)\n", creds.KeyID, err)
			return 1
		}
	}
	path, _ := credentialsPath()
	if err := os.Remove(path); err != nil {
		fmt.Fprintf(stderr, "choir logout: %v\n", err)
		return 1
	}
	return writeJSON(stdout, map[string]any{"logged_out": true, "revoked": creds.KeyID})
}

// ---- choir mcp install ----

var mcpInstallHosts = []string{"claude-code", "cursor", "claude-desktop", "codex"}

func runMCPInstall(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir mcp install", flag.ContinueOnError)
	fs.SetOutput(stderr)
	printOnly := fs.Bool("print", false, "Print the config entry instead of writing it")
	var positional []string
	rest := args
	for {
		if err := fs.Parse(rest); err != nil {
			return 2
		}
		if fs.NArg() == 0 {
			break
		}
		positional = append(positional, fs.Arg(0))
		rest = fs.Args()[1:]
	}
	if len(positional) != 1 {
		fmt.Fprintf(stderr, "choir mcp install: name one agent host (%s)\n", strings.Join(mcpInstallHosts, "|"))
		return 2
	}
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(stderr, "choir mcp install: %v\n", err)
		return 1
	}
	entry := map[string]any{"command": exe, "args": []string{"mcp"}}
	home, _ := os.UserHomeDir()
	switch positional[0] {
	case "claude-code":
		cmdArgs := []string{"mcp", "add", "--scope", "user", "choir", "--", exe, "mcp"}
		if *printOnly {
			fmt.Fprintln(stdout, "claude "+strings.Join(cmdArgs, " "))
			return 0
		}
		claude, err := exec.LookPath("claude")
		if err != nil {
			fmt.Fprintf(stderr, "choir mcp install: claude is not on PATH; run this where it is:\n  claude %s\n", strings.Join(cmdArgs, " "))
			return 1
		}
		c := exec.Command(claude, cmdArgs...)
		c.Stdout, c.Stderr = stderr, stderr
		if err := c.Run(); err != nil {
			fmt.Fprintf(stderr, "choir mcp install: claude mcp add: %v\n", err)
			return 1
		}
		return writeJSON(stdout, map[string]any{"installed": "claude-code", "server": "choir"})
	case "cursor":
		return installMCPJSON(filepath.Join(home, ".cursor", "mcp.json"), "cursor", entry, *printOnly, stdout, stderr)
	case "claude-desktop":
		path := filepath.Join(home, ".config", "Claude", "claude_desktop_config.json")
		if runtime.GOOS == "darwin" {
			path = filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json")
		}
		return installMCPJSON(path, "claude-desktop", entry, *printOnly, stdout, stderr)
	case "codex":
		dir := strings.TrimSpace(os.Getenv("CODEX_HOME"))
		if dir == "" {
			dir = filepath.Join(home, ".codex")
		}
		return installCodexTOML(filepath.Join(dir, "config.toml"), exe, *printOnly, stdout, stderr)
	}
	fmt.Fprintf(stderr, "choir mcp install: unknown agent host %q (%s)\n", positional[0], strings.Join(mcpInstallHosts, "|"))
	return 2
}

// installMCPJSON sets mcpServers.choir in a JSON config, keeping every other
// key. A file that is not JSON is refused rather than overwritten.
func installMCPJSON(path, name string, entry map[string]any, printOnly bool, stdout, stderr io.Writer) int {
	if printOnly {
		raw, _ := json.MarshalIndent(map[string]any{"mcpServers": map[string]any{"choir": entry}}, "", "  ")
		fmt.Fprintf(stdout, "%s\n", raw)
		fmt.Fprintf(stderr, "(merge into %s)\n", path)
		return 0
	}
	doc := map[string]any{}
	mode := os.FileMode(0o600)
	if raw, err := os.ReadFile(path); err == nil {
		if len(bytes.TrimSpace(raw)) > 0 {
			if err := json.Unmarshal(raw, &doc); err != nil {
				fmt.Fprintf(stderr, "choir mcp install: %s is not valid JSON (%v); fix it or use --print\n", path, err)
				return 1
			}
		}
		if info, err := os.Stat(path); err == nil {
			mode = info.Mode().Perm()
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(stderr, "choir mcp install: %v\n", err)
		return 1
	}
	servers, _ := doc["mcpServers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	servers["choir"] = entry
	doc["mcpServers"] = servers
	raw, _ := json.MarshalIndent(doc, "", "  ")
	if err := writeFileAtomic(path, append(raw, '\n'), mode); err != nil {
		fmt.Fprintf(stderr, "choir mcp install: %v\n", err)
		return 1
	}
	return writeJSON(stdout, map[string]any{"installed": name, "config": path, "server": "choir"})
}

func installCodexTOML(path, exe string, printOnly bool, stdout, stderr io.Writer) int {
	section := fmt.Sprintf("[mcp_servers.choir]\ncommand = %q\nargs = [\"mcp\"]\n", exe)
	if printOnly {
		fmt.Fprint(stdout, section)
		fmt.Fprintf(stderr, "(append to %s)\n", path)
		return 0
	}
	raw, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(stderr, "choir mcp install: %v\n", err)
		return 1
	}
	if strings.Contains(string(raw), "[mcp_servers.choir]") {
		fmt.Fprintf(stderr, "choir mcp install: %s already has [mcp_servers.choir]; left unchanged\n", path)
		return writeJSON(stdout, map[string]any{"installed": "codex", "config": path, "server": "choir", "unchanged": true})
	}
	mode := os.FileMode(0o600)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	out := string(raw)
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	if out != "" {
		out += "\n"
	}
	if err := writeFileAtomic(path, []byte(out+section), mode); err != nil {
		fmt.Fprintf(stderr, "choir mcp install: %v\n", err)
		return 1
	}
	return writeJSON(stdout, map[string]any{"installed": "codex", "config": path, "server": "choir"})
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
