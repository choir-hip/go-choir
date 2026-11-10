package autoputer

import (
	"context"
	"fmt"
	"net/http"

	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// platformReceiptRuntime is the runtime surface the receipt needs
// (agentcore.Runtime.EmitProductEvent).
type platformReceiptRuntime interface {
	EmitProductEvent(ctx context.Context, ownerID, desktopID string, kind types.EventKind, payload map[string]any) (types.EventRecord, error)
}

// recordPlatformPolicyAct writes the platform operator's receipt for a policy
// write as a platform.computer.model_policy event on the computer's tape. The
// owner is the computer's owner (the authenticated user the proxy forwarded
// as); the payload names the operator account id, never an email address.
func recordPlatformPolicyAct(r *http.Request, rt platformReceiptRuntime, act *PolicyAct) error {
	ownerID, err := server.AuthenticatedOwner(r)
	if err != nil {
		return fmt.Errorf("operator receipt: %w", err)
	}
	payload := map[string]any{
		"operator_account_id": act.OperatorAccountID,
		"act":                 act.Act,
		"path":                act.Path,
		"revision":            act.Revision,
		"old_sha256":          act.OldSHA256,
		"new_sha256":          act.NewSHA256,
		"bytes":               act.Bytes,
	}
	rec, err := rt.EmitProductEvent(r.Context(), ownerID, desktopIDFromRequest(r), types.EventPlatformComputerModelPolicy, payload)
	if err != nil {
		return fmt.Errorf("operator receipt: %w", err)
	}
	act.ReceiptID = rec.EventID
	return nil
}
