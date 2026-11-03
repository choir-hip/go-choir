package autoputer

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/ownerfiles"
)

// Maildir provides a crash-safe, standard Unix Maildir mailbox stored inside
// the computer's persistent encrypted filesystem (under File-CAS durability).
type Maildir struct {
	mu       sync.Mutex
	mailRoot string
	hostname string
}

// NewMaildir initializes or opens the Maildir directory structure.
func NewMaildir(mailRoot string) (*Maildir, error) {
	mailRoot = filepath.Clean(mailRoot)
	for _, sub := range []string{"cur", "new", "tmp"} {
		dir := filepath.Join(mailRoot, sub)
		if err := ownerfiles.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("maildir: create %s dir: %w", sub, err)
		}
	}
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "autoputer"
	}
	return &Maildir{
		mailRoot: mailRoot,
		hostname: hostname,
	}, nil
}

// DeliverMessage stores an incoming message into the Maildir following standard
// crash-safe Maildir conventions (write to tmp/, fsync, rename to new/).
// It deduplicates messages by Message-ID.
func (m *Maildir) DeliverMessage(ctx context.Context, messageID string, rawEML []byte) (string, error) {
	if m == nil {
		return "", fmt.Errorf("maildir: uninitialized")
	}
	// Lock order: the files lock, then the maildir's own mutex.
	tx := ownerfiles.Begin()
	defer tx.End()
	m.mu.Lock()
	defer m.mu.Unlock()

	messageID = strings.TrimSpace(messageID)
	if messageID != "" {
		if existing, found := m.findExistingMessageID(messageID); found {
			return existing, nil // Idempotent success on duplicate delivery
		}
	}

	tmpDir := filepath.Join(m.mailRoot, "tmp")
	newDir := filepath.Join(m.mailRoot, "new")

	rnd := make([]byte, 8)
	_, _ = rand.Read(rnd)
	baseName := fmt.Sprintf("%d.%d_%s.%s", time.Now().UnixNano(), os.Getpid(), hex.EncodeToString(rnd), m.hostname)

	tmpPath := filepath.Join(tmpDir, baseName)
	newPath := filepath.Join(newDir, baseName)

	file, err := tx.CreateExcl(tmpPath, 0o600)
	if err != nil {
		return "", fmt.Errorf("maildir: create tmp message: %w", err)
	}
	defer func() {
		if file != nil {
			_ = file.Close()
			_ = tx.Remove(tmpPath)
		}
	}()

	if _, err := file.Write(rawEML); err != nil {
		return "", fmt.Errorf("maildir: write raw message: %w", err)
	}
	if err := file.Sync(); err != nil {
		return "", fmt.Errorf("maildir: fsync raw message: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("maildir: close tmp message: %w", err)
	}
	file = nil

	if err := tx.Rename(tmpPath, newPath); err != nil {
		_ = tx.Remove(tmpPath)
		return "", fmt.Errorf("maildir: install message to new: %w", err)
	}

	// Fsync new/ directory
	_ = tx.SyncDirs(newDir)

	return filepath.Base(newPath), nil
}

func (m *Maildir) findExistingMessageID(targetMsgID string) (string, bool) {
	for _, sub := range []string{"new", "cur"} {
		dir := filepath.Join(m.mailRoot, sub)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			path := filepath.Join(dir, e.Name())
			if msgID := extractMessageID(path); msgID == targetMsgID {
				return e.Name(), true
			}
		}
	}
	return "", false
}

func extractMessageID(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()

	buf := make([]byte, 4096)
	n, err := file.Read(buf)
	if err != nil && n == 0 {
		return ""
	}
	lines := strings.Split(string(buf[:n]), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), "message-id:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "Message-ID:"))
		}
		if line == "" {
			break // End of headers
		}
	}
	return ""
}

// MailInboundHandler handles authenticated incoming mail delivery from host MTA.
type MailInboundHandler struct {
	maildir *Maildir
}

type mailInboundRequest struct {
	MessageID string `json:"message_id"`
	Recipient string `json:"recipient"`
	Sender    string `json:"sender"`
	Subject   string `json:"subject"`
	RawEML    string `json:"raw_eml"`
}

func NewMailInboundHandler(maildir *Maildir) *MailInboundHandler {
	return &MailInboundHandler{maildir: maildir}
}

func (h *MailInboundHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var input mailInboundRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if input.RawEML == "" {
		// If raw EML was not provided in JSON string, construct minimal RFC 822 format
		var buf bytes.Buffer
		if input.MessageID != "" {
			buf.WriteString("Message-ID: " + input.MessageID + "\r\n")
		}
		buf.WriteString("From: " + input.Sender + "\r\n")
		buf.WriteString("To: " + input.Recipient + "\r\n")
		buf.WriteString("Subject: " + input.Subject + "\r\n")
		buf.WriteString("Date: " + time.Now().UTC().Format(time.RFC1123Z) + "\r\n\r\n")
		input.RawEML = buf.String()
	}

	fileName, err := h.maildir.DeliverMessage(r.Context(), input.MessageID, []byte(input.RawEML))
	if err != nil {
		http.Error(w, fmt.Sprintf("deliver message: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":     "delivered",
		"message_id": input.MessageID,
		"filename":   fileName,
	})
}

func RegisterMailRoutes(s interface {
	HandleFunc(string, http.HandlerFunc)
}, maildir *Maildir) {
	if s == nil || maildir == nil {
		return
	}
	handler := NewMailInboundHandler(maildir)
	s.HandleFunc("/api/mail/inbound", func(w http.ResponseWriter, r *http.Request) {
		if err := requireAuth(r); err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		handler.ServeHTTP(w, r)
	})
}
