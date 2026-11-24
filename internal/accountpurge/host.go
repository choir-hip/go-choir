package accountpurge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/auth"
	"github.com/yusefmosiah/go-choir/internal/hostauth"
	"github.com/yusefmosiah/go-choir/internal/maild"
	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// Host is Steps over HTTP to the host services: vmctl, corpusd, maild and the
// gateway, all on loopback with the host-internal credential
// (hostauth.MarkInternal). Every destructive request carries the deletion
// record's request time, which the receiving service checks again.
type Host struct {
	VMCTL      *vmctl.Client
	CorpusdURL string
	MaildURL   string
	GatewayURL string
	HTTP       *http.Client
}

// NewHost builds a Host from the four base URLs.
func NewHost(vmctlURL, corpusdURL, maildURL, gatewayURL string) *Host {
	return &Host{
		VMCTL: vmctl.NewClient(vmctlURL), CorpusdURL: strings.TrimRight(corpusdURL, "/"),
		MaildURL: strings.TrimRight(maildURL, "/"), GatewayURL: strings.TrimRight(gatewayURL, "/"),
		HTTP: &http.Client{Timeout: 2 * time.Minute},
	}
}

var _ Steps = (*Host)(nil)

// post sends a JSON body to a host service and returns the status and body. A
// 403 is a refusal by the service and is reported as one.
func (h *Host) post(ctx context.Context, base, path string, body any, internalMarker bool) (int, []byte, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+path, bytes.NewReader(raw))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if internalMarker {
		req.Header.Set("X-Internal-Caller", "true")
	}
	hostauth.MarkInternal(req)
	resp, err := h.HTTP.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	out, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	return resp.StatusCode, out, nil
}

// ErrServiceRefused is a refusal by the service that holds the data.
var ErrServiceRefused = errors.New("refused by the service")

func (h *Host) call(ctx context.Context, base, path string, body any, out any) error {
	status, raw, err := h.post(ctx, base, path, body, true)
	if err != nil {
		return err
	}
	switch {
	case status == http.StatusForbidden:
		return fmt.Errorf("%w: %s %s", ErrServiceRefused, path, strings.TrimSpace(string(raw)))
	case status != http.StatusOK && status != http.StatusAccepted:
		return fmt.Errorf("%s answered %d: %s", path, status, strings.TrimSpace(string(raw)))
	}
	if out != nil {
		return json.Unmarshal(raw, out)
	}
	return nil
}

func stamp(a Authz) string { return a.RequestedAt().UTC().Format(time.RFC3339) }

func (h *Host) Computers(ctx context.Context, userID string) ([]vmctl.AccountComputer, error) {
	return h.VMCTL.AccountComputers(ctx, userID)
}

func (h *Host) StopComputers(ctx context.Context, userID string) error {
	return h.VMCTL.AccountStop(ctx, userID)
}

func (h *Host) SuspendTakes(ctx context.Context, userID string) error {
	return h.call(ctx, h.CorpusdURL, platform.AccountSuspendPath, map[string]string{"user_id": userID, "action": "suspend"}, nil)
}

func (h *Host) RestoreTakes(ctx context.Context, userID string) error {
	return h.call(ctx, h.CorpusdURL, platform.AccountSuspendPath, map[string]string{"user_id": userID, "action": "restore"}, nil)
}

func (h *Host) PurgeMail(ctx context.Context, a Authz) (Counts, error) {
	var out maild.AccountPurgeCounts
	if err := h.call(ctx, h.MaildURL, maild.AuthPurgeAccountPath,
		map[string]string{"owner_id": a.UserID(), "deletion_requested_at": stamp(a)}, &out); err != nil {
		return nil, err
	}
	return Counts{"aliases": out.Aliases, "alias_receipts": out.AliasReceipts, "whitelist": out.Whitelist, "provider_index": out.ProviderIndex,
		"webhook_events": out.WebhookEvents, "approval_index": out.ApprovalIndex, "owner_address": out.OwnerAddress, "digest": out.Digest,
		"mailbox_dir": b2i(out.MailboxDir), "attachments_dir": b2i(out.Attachments)}, nil
}

func b2i(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func (h *Host) PurgeData(ctx context.Context, a Authz) (Counts, error) {
	var out platform.AccountPurgeCounts
	if err := h.call(ctx, h.CorpusdURL, platform.AccountPurgePath,
		map[string]any{"user_id": a.UserID(), "computer_ids": a.Computers(), "deletion_requested_at": stamp(a)}, &out); err != nil {
		return nil, err
	}
	c := Counts{"takes_deleted": int64(out.TakesDeleted), "authors_detached": int64(out.AuthorsDetached), "captures_deleted": int64(out.CapturesDeleted),
		"texture_documents_deleted": int64(out.TextureDocumentsDeleted), "texture_revisions_deleted": int64(out.TextureRevisionsDeleted)}
	for t, n := range out.ProjectionRows {
		c["proj:"+t] = n
	}
	// What stays by decision (the publication layer) and what nobody decided
	// about are separate lines of the receipt: the second must read empty.
	for k, n := range out.KeptByDecision {
		c["kept:"+k] = n
	}
	for k, n := range out.Undecided {
		c["undecided:"+k] = n
	}
	for t, n := range out.RemainingTextures {
		c["remaining:"+t] = n
	}
	return c, nil
}

func (h *Host) DestroyComputers(ctx context.Context, a Authz) (Counts, Destroyed, error) {
	res, err := h.VMCTL.AccountDestroy(ctx, a.UserID())
	if err != nil {
		return nil, Destroyed{}, err
	}
	return Counts{"computers": int64(len(res.Computers)), "vm_states": int64(res.VMStates)}, Destroyed{ComputerIDs: res.Computers, VMIDs: res.VMIDs}, nil
}

func (h *Host) ForgetGateway(ctx context.Context, a Authz, computerID string) error {
	return h.call(ctx, h.GatewayURL, "/provider/v1/credentials/forget",
		map[string]string{"computer_id": computerID, "deletion_requested_at": stamp(a)}, nil)
}

func (h *Host) PurgeTape(ctx context.Context, a Authz) (TapeResult, error) {
	if len(a.Computers()) == 0 {
		return TapeResult{Counts: Counts{"computers": 0}}, nil
	}
	var job platform.AccountTapeJob
	if err := h.call(ctx, h.CorpusdURL, platform.AccountPurgeTapePath,
		map[string]any{"user_id": a.UserID(), "computer_ids": a.Computers(), "deletion_requested_at": stamp(a)}, &job); err != nil {
		return TapeResult{}, err
	}
	switch job.State {
	case "running":
		return TapeResult{Running: true}, nil
	case "done":
		return TapeResult{Counts: Counts{"files_deleted": int64(job.Deleted), "bytes_deleted": job.BytesGone,
			"computers_deleted": int64(job.ComputersDeleted), "rows_deleted": job.RowsDeleted, "archives": int64(job.Archives)}}, nil
	}
	if job.Refusal != "" {
		return TapeResult{Refusal: job.Refusal, Permanent: job.Permanent}, nil
	}
	// A failed job is dropped by the next call (corpusd starts a new one).
	return TapeResult{}, fmt.Errorf("tape purge failed: %s", job.Error)
}

var _ = auth.DeletionStep{}
