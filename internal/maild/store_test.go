package maild

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) (*Store, *Config) {
	t.Helper()
	dir := t.TempDir()
	cfg := &Config{
		Port:            DefaultPort,
		DBPath:          filepath.Join(dir, "mail.db"),
		StorageRoot:     filepath.Join(dir, "mail"),
		PrimaryDomain:   "choir.news",
		RootOwnerID:     "user-root",
		ResendBaseURL:   DefaultResendBaseURL,
		WebhookMaxBytes: DefaultWebhookMaxBody,
	}
	if err := cfg.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}
	store, err := OpenStore(cfg.DBPath)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.EnsureSchema(cfg); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	return store, cfg
}

func TestEnsureSchemaSeedsRootAlias(t *testing.T) {
	store, _ := newTestStore(t)
	alias, err := store.ResolveAlias(context.Background(), "choir.news", "000")
	if err != nil {
		t.Fatalf("ResolveAlias: %v", err)
	}
	if alias.TargetID != "user-root" {
		t.Fatalf("TargetID = %q, want user-root", alias.TargetID)
	}
	if alias.ReceivePolicyID != DefaultPublicPolicyID {
		t.Fatalf("ReceivePolicyID = %q, want %q", alias.ReceivePolicyID, DefaultPublicPolicyID)
	}
}

func TestRecordWebhookEventIdempotent(t *testing.T) {
	store, _ := newTestStore(t)
	event := WebhookEvent{
		ID:                "event-row-1",
		Provider:          providerResend,
		ProviderEventID:   "evt-1",
		ProviderMessageID: "email-1",
		EventType:         "email.received",
		RawPayload:        `{"id":"evt-1"}`,
		ReceivedAt:        time.Now(),
	}
	created, err := store.RecordWebhookEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("RecordWebhookEvent first: %v", err)
	}
	if !created {
		t.Fatalf("first insert created = false")
	}
	created, err = store.RecordWebhookEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("RecordWebhookEvent duplicate: %v", err)
	}
	if created {
		t.Fatalf("duplicate insert created = true")
	}
	count, err := store.CountWebhookEvents(context.Background())
	if err != nil {
		t.Fatalf("CountWebhookEvents: %v", err)
	}
	if count != 1 {
		t.Fatalf("webhook count = %d, want 1", count)
	}
}
