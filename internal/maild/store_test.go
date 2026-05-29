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
		Port:             DefaultPort,
		DBPath:           filepath.Join(dir, "mail.db"),
		StorageRoot:      filepath.Join(dir, "mail"),
		PrimaryDomain:    "choir.news",
		RootOwnerID:      "user-root",
		ResendBaseURL:    DefaultResendBaseURL,
		WebhookMaxBytes:  DefaultWebhookMaxBody,
		APIMaxBytes:      DefaultAPIMaxBody,
		ProviderMaxBytes: DefaultProviderMaxBody,
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
	policy, err := store.GetReceivePolicy(context.Background(), DefaultTrustedWorkflowPolicyID)
	if err != nil {
		t.Fatalf("GetReceivePolicy trusted workflow: %v", err)
	}
	if policy.AllowPublicInbound || !policy.RequireSenderWhitelist || !policy.RequireSecretAlias || !policy.AllowAutoAgentRead {
		t.Fatalf("trusted workflow policy = %+v", policy)
	}
}

func TestEnsureSchemaReconcilesRootAliasOwner(t *testing.T) {
	store, cfg := newTestStore(t)
	cfg.RootOwnerID = "real-founder"
	if err := store.EnsureSchema(cfg); err != nil {
		t.Fatalf("EnsureSchema second pass: %v", err)
	}
	alias, err := store.ResolveAlias(context.Background(), "choir.news", "000")
	if err != nil {
		t.Fatalf("ResolveAlias: %v", err)
	}
	if alias.TargetID != "real-founder" {
		t.Fatalf("TargetID = %q, want real-founder", alias.TargetID)
	}
}

func TestEnsureSchemaRepairsActiveApprovalTokensForSentDrafts(t *testing.T) {
	store, cfg := newTestStore(t)
	alias, err := store.ResolveAlias(context.Background(), "choir.news", "000")
	if err != nil {
		t.Fatalf("ResolveAlias: %v", err)
	}
	draft, err := store.CreateDraft(context.Background(), "user-root", alias, createDraftRequest{
		ToAddresses: []string{"friend@example.com"},
		Subject:     "Stale approval token repair",
		TextBody:    "Already sent.",
	})
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	token, err := store.CreateDraftApprovalToken(context.Background(), draft, "owner@example.com", time.Hour)
	if err != nil {
		t.Fatalf("CreateDraftApprovalToken: %v", err)
	}
	if _, err := store.db.ExecContext(context.Background(), `UPDATE email_drafts SET status = 'sent' WHERE id = ?`, draft.ID); err != nil {
		t.Fatalf("force sent draft: %v", err)
	}

	if err := store.EnsureSchema(cfg); err != nil {
		t.Fatalf("EnsureSchema repair: %v", err)
	}
	repaired, err := store.GetDraftApprovalToken(context.Background(), token.Token)
	if err != nil {
		t.Fatalf("GetDraftApprovalToken: %v", err)
	}
	if repaired.Status != "stale_sent" || repaired.UsedAt == "" {
		t.Fatalf("repaired token = %+v, want stale_sent with used_at", repaired)
	}
}

func TestEnsureSchemaRepairsRejectedDrafts(t *testing.T) {
	store, cfg := newTestStore(t)
	alias, err := store.ResolveAlias(context.Background(), "choir.news", "000")
	if err != nil {
		t.Fatalf("ResolveAlias: %v", err)
	}
	draft, err := store.CreateDraft(context.Background(), "user-root", alias, createDraftRequest{
		ToAddresses: []string{"friend@example.com"},
		Subject:     "Rejected repair",
		TextBody:    "Do not send.",
	})
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	token, err := store.CreateDraftApprovalToken(context.Background(), draft, "owner@example.com", time.Hour)
	if err != nil {
		t.Fatalf("CreateDraftApprovalToken: %v", err)
	}
	if err := store.UseDraftApprovalToken(context.Background(), token.ID, "rejected"); err != nil {
		t.Fatalf("UseDraftApprovalToken rejected: %v", err)
	}

	if err := store.EnsureSchema(cfg); err != nil {
		t.Fatalf("EnsureSchema repair: %v", err)
	}
	repaired, err := store.GetDraft(context.Background(), "user-root", draft.ID)
	if err != nil {
		t.Fatalf("GetDraft: %v", err)
	}
	if repaired.Status != "draft_rejected" {
		t.Fatalf("draft status = %q, want draft_rejected", repaired.Status)
	}
	drafts, err := store.ListDrafts(context.Background(), "user-root", 10)
	if err != nil {
		t.Fatalf("ListDrafts: %v", err)
	}
	if len(drafts) != 0 {
		t.Fatalf("repaired rejected draft still listed: %+v", drafts)
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

func TestConfigureTrustedWorkflowAlias(t *testing.T) {
	store, _ := newTestStore(t)
	alias, err := store.ConfigureTrustedWorkflowAlias(context.Background(), TrustedWorkflowAliasConfig{
		OwnerID:       "user-root",
		Domain:        "choir.news",
		LocalPart:     "000+invite-test",
		SenderAddress: "sender@example.com",
	})
	if err != nil {
		t.Fatalf("ConfigureTrustedWorkflowAlias: %v", err)
	}
	if alias.TargetID != "user-root" || alias.LocalPart != "000+invite-test" || alias.ReceivePolicyID != DefaultTrustedWorkflowPolicyID {
		t.Fatalf("alias = %+v", alias)
	}
	ok, err := store.IsSenderWhitelisted(context.Background(), "user-root", alias.ID, "sender@example.com")
	if err != nil {
		t.Fatalf("IsSenderWhitelisted: %v", err)
	}
	if !ok {
		t.Fatalf("sender was not whitelisted")
	}
}
