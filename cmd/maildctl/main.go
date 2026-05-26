package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/maild"
)

const defaultNodeBDBPath = "/var/lib/go-choir/mail/mail.db"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 2
	}
	cmd := args[0]
	fs := flag.NewFlagSet("maildctl "+cmd, flag.ContinueOnError)
	fs.SetOutput(stderr)
	dbPath := fs.String("db", defaultDBPath(), "maild SQLite database path")
	limit := fs.Int("limit", 50, "maximum rows to return")
	ownerID := fs.String("owner", "", "mailbox owner id")
	messageID := fs.String("message", "", "message id")
	includeBody := fs.Bool("body", false, "include message body fields")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}

	ctx := context.Background()
	store, err := maild.OpenStore(*dbPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "maildctl: open store: %v\n", err)
		return 1
	}
	defer func() { _ = store.Close() }()

	var out any
	switch cmd {
	case "stats":
		out, err = store.Stats(ctx)
	case "aliases":
		out, err = store.ListAliases(ctx)
	case "webhooks":
		out, err = store.ListWebhookEvents(ctx, *limit)
	case "messages":
		if strings.TrimSpace(*ownerID) == "" {
			_, _ = fmt.Fprintln(stderr, "maildctl messages: --owner is required")
			return 2
		}
		folder := strings.TrimSpace(fs.Arg(0))
		out, err = store.ListMessages(ctx, *ownerID, folder, *limit)
	case "message":
		if strings.TrimSpace(*ownerID) == "" || strings.TrimSpace(*messageID) == "" {
			_, _ = fmt.Fprintln(stderr, "maildctl message: --owner and --message are required")
			return 2
		}
		out, err = loadMessageDetail(ctx, store, *ownerID, *messageID, *includeBody)
	case "attachments":
		if strings.TrimSpace(*ownerID) == "" || strings.TrimSpace(*messageID) == "" {
			_, _ = fmt.Fprintln(stderr, "maildctl attachments: --owner and --message are required")
			return 2
		}
		out, err = store.ListAttachments(ctx, *ownerID, *messageID)
	case "source-packet":
		if strings.TrimSpace(*ownerID) == "" || strings.TrimSpace(*messageID) == "" {
			_, _ = fmt.Fprintln(stderr, "maildctl source-packet: --owner and --message are required")
			return 2
		}
		packet, msg, packetErr := store.GetSourcePacketForMessage(ctx, *ownerID, *messageID)
		err = packetErr
		out = map[string]any{"message": msg, "source_packet": packet}
	case "ingress-events":
		if strings.TrimSpace(*ownerID) == "" {
			_, _ = fmt.Fprintln(stderr, "maildctl ingress-events: --owner is required")
			return 2
		}
		out, err = store.ListIngressEvents(ctx, *ownerID, *messageID, *limit)
	default:
		printUsage(stderr)
		return 2
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "maildctl %s: %v\n", cmd, err)
		return 1
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(out); err != nil {
		_, _ = fmt.Fprintf(stderr, "maildctl: encode json: %v\n", err)
		return 1
	}
	return 0
}

type messageDetail struct {
	Message     maild.EmailMessage      `json:"message"`
	TextBody    string                  `json:"text_body,omitempty"`
	HTMLBody    string                  `json:"html_body,omitempty"`
	Attachments []maild.EmailAttachment `json:"attachments,omitempty"`
}

func loadMessageDetail(ctx context.Context, store *maild.Store, ownerID, messageID string, includeBody bool) (messageDetail, error) {
	msg, err := store.GetMessage(ctx, ownerID, messageID)
	if err != nil {
		return messageDetail{}, err
	}
	attachments, err := store.ListAttachments(ctx, ownerID, messageID)
	if err != nil {
		return messageDetail{}, err
	}
	detail := messageDetail{Message: msg, Attachments: attachments}
	if includeBody {
		detail.TextBody = msg.TextBody
		detail.HTMLBody = msg.HTMLBody
		detail.Message.TextBody = ""
		detail.Message.HTMLBody = ""
	} else {
		detail.Message.TextBody = ""
		detail.Message.HTMLBody = ""
	}
	return detail, nil
}

func defaultDBPath() string {
	if v := strings.TrimSpace(os.Getenv("MAILD_DB_PATH")); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("MAILD_STORAGE_ROOT")); v != "" {
		return filepath.Join(v, "mail.db")
	}
	return defaultNodeBDBPath
}

func printUsage(w io.Writer) {
	_, _ = fmt.Fprintln(w, `usage: maildctl <command> [flags]

commands:
  stats                         print safe mailbox counters
  aliases                       list configured aliases
  webhooks [--limit N]          list recent webhook receipts without raw payloads
  messages --owner ID [folder]  list inbox, sent, or quarantine messages
  message --owner ID --message ID [--body]
  attachments --owner ID --message ID
  source-packet --owner ID --message ID
  ingress-events --owner ID [--message ID] [--limit N]

common flags:
  --db PATH                     maild SQLite database path`)
}
