package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/maild"
)

func setupMaildctlStore(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cfg := &maild.Config{
		Port:            maild.DefaultPort,
		DBPath:          filepath.Join(dir, "mail.db"),
		StorageRoot:     filepath.Join(dir, "mail"),
		PrimaryDomain:   "choir.news",
		RootOwnerID:     "owner-000",
		ResendBaseURL:   maild.DefaultResendBaseURL,
		WebhookMaxBytes: maild.DefaultWebhookMaxBody,
	}
	if err := cfg.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}
	store, err := maild.OpenStore(cfg.DBPath)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer func() { _ = store.Close() }()
	if err := store.EnsureSchema(cfg); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	if _, err := store.RecordWebhookEvent(context.Background(), maild.WebhookEvent{
		ID:                "evt-row-1",
		Provider:          "resend",
		ProviderEventID:   "evt-1",
		ProviderMessageID: "email-1",
		EventType:         "email.received",
		RawPayload:        `{"id":"evt-1"}`,
		ReceivedAt:        time.Now(),
	}); err != nil {
		t.Fatalf("RecordWebhookEvent: %v", err)
	}
	return cfg.DBPath
}

func TestRunStats(t *testing.T) {
	dbPath := setupMaildctlStore(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{"stats", "--db", dbPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run stats code=%d stderr=%s", code, stderr.String())
	}
	var stats maild.StoreStats
	if err := json.Unmarshal(stdout.Bytes(), &stats); err != nil {
		t.Fatalf("decode stats: %v", err)
	}
	if stats.Aliases != 1 || stats.WebhookEvents != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}

func TestRunAliases(t *testing.T) {
	dbPath := setupMaildctlStore(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{"aliases", "--db", dbPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run aliases code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"LocalPart": "000"`) {
		t.Fatalf("aliases output missing 000 alias: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), `"TargetID": "owner-000"`) {
		t.Fatalf("aliases output missing owner: %s", stdout.String())
	}
}

func TestRunMessagesRequiresOwner(t *testing.T) {
	dbPath := setupMaildctlStore(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{"messages", "--db", dbPath}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("run messages code=%d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "--owner is required") {
		t.Fatalf("stderr missing owner requirement: %s", stderr.String())
	}
}

func TestRunWebhooksPrintsEmptyArray(t *testing.T) {
	dir := t.TempDir()
	cfg := &maild.Config{
		Port:            maild.DefaultPort,
		DBPath:          filepath.Join(dir, "mail.db"),
		StorageRoot:     filepath.Join(dir, "mail"),
		PrimaryDomain:   "choir.news",
		RootOwnerID:     "owner-000",
		ResendBaseURL:   maild.DefaultResendBaseURL,
		WebhookMaxBytes: maild.DefaultWebhookMaxBody,
	}
	if err := cfg.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}
	store, err := maild.OpenStore(cfg.DBPath)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	if err := store.EnsureSchema(cfg); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"webhooks", "--db", cfg.DBPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run webhooks code=%d stderr=%s", code, stderr.String())
	}
	if strings.TrimSpace(stdout.String()) != "[]" {
		t.Fatalf("webhooks output = %q, want []", stdout.String())
	}
}

func TestRunIngressEventsPrintsOwnerScopedRows(t *testing.T) {
	dbPath := setupMaildctlStore(t)
	store, err := maild.OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	for _, event := range []maild.EmailIngressEvent{
		{ID: "ingress-1", MessageID: "msg-1", SourcePacketID: "source-1", OwnerID: "owner-000", ConductorSubmissionID: "submission-1", Status: "accepted", CreatedAt: "2026-05-26T12:00:00Z"},
		{ID: "ingress-2", MessageID: "msg-2", SourcePacketID: "source-2", OwnerID: "other-owner", ConductorSubmissionID: "submission-2", Status: "accepted", CreatedAt: "2026-05-26T12:01:00Z"},
	} {
		if err := store.RecordIngressEvent(context.Background(), event); err != nil {
			t.Fatalf("RecordIngressEvent: %v", err)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"ingress-events", "--db", dbPath, "--owner", "owner-000"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run ingress-events code=%d stderr=%s", code, stderr.String())
	}
	var events []maild.EmailIngressEvent
	if err := json.Unmarshal(stdout.Bytes(), &events); err != nil {
		t.Fatalf("decode ingress events: %v", err)
	}
	if len(events) != 1 || events[0].ID != "ingress-1" || events[0].ConductorSubmissionID != "submission-1" {
		t.Fatalf("events = %+v", events)
	}
}
