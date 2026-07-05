//go:build linux


package capsule

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/mdlayher/vsock"
)

// HostClient is the vsock client used by the Executor (in the guest VM)
// to communicate with HostAuthority (on the Firecracker host).
// The Executor is trusted guest TCB (v14 decision). HostAuthority holds
// the Ed25519 private key and handles minting, revocation, and registration.
type HostClient struct {
	conn    net.Conn
	hostCID uint32 // typically 2 for the host on Firecracker
	port    uint32 // vsock port for HostAuthority listener
}

// HostRPCRequest is the wire format for RPCs to HostAuthority.
type HostRPCRequest struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

// HostRPCResponse is the wire format for responses from HostAuthority.
type HostRPCResponse struct {
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// RPC methods supported by HostAuthority.
const (
	MethodMintCapability    = "mint_capability"
	MethodRevokeCapability  = "revoke_capability"
	MethodGetRevokedCaps    = "get_revoked_caps"
	MethodRegisterCapsule   = "register_capsule"
	MethodUnregisterCapsule = "unregister_capsule"
	MethodRegisterActiveRun = "register_active_run"
	MethodUnregisterActiveRun = "unregister_active_run"
)

// NewHostClient creates a new vsock client to HostAuthority.
func NewHostClient(hostCID, port uint32) *HostClient {
	return &HostClient{
		hostCID: hostCID,
		port:    port,
	}
}

// Connect establishes a vsock connection to HostAuthority.
func (c *HostClient) Connect(ctx context.Context) error {
	conn, err := vsock.Dial(c.hostCID, c.port, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to HostAuthority via vsock (CID=%d, port=%d): %w",
			c.hostCID, c.port, err)
	}
	c.conn = conn
	return nil
}

// Close closes the vsock connection.
func (c *HostClient) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

// call sends an RPC request and returns the response.
func (c *HostClient) call(ctx context.Context, method string, params interface{}) (*HostRPCResponse, error) {
	if c.conn == nil {
		return nil, fmt.Errorf("not connected to HostAuthority")
	}

	paramBytes, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}

	req := HostRPCRequest{
		Method: method,
		Params: paramBytes,
	}

	// Set write deadline from context.
	if deadline, ok := ctx.Deadline(); ok {
		c.conn.SetWriteDeadline(deadline)
	} else {
		c.conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
	}

	if err := json.NewEncoder(c.conn).Encode(&req); err != nil {
		return nil, fmt.Errorf("failed to send RPC request: %w", err)
	}

	// Set read deadline from context.
	if deadline, ok := ctx.Deadline(); ok {
		c.conn.SetReadDeadline(deadline)
	} else {
		c.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	}

	var resp HostRPCResponse
	if err := json.NewDecoder(c.conn).Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to read RPC response: %w", err)
	}

	if resp.Error != "" {
		return nil, fmt.Errorf("HostAuthority error: %s", resp.Error)
	}

	return &resp, nil
}

// MintCapabilityRequest is the params for mint_capability.
type MintCapabilityRequest struct {
	AgentRunID string        `json:"agent_run_id"`
	Role       AgentRole     `json:"role"`
	CapsuleID  string        `json:"capsule_id"`
	TTL        time.Duration `json:"ttl"`
}

// MintCapability requests a new capability from HostAuthority.
func (c *HostClient) MintCapability(ctx context.Context, agentRunID string, role AgentRole, capsuleID string, ttl time.Duration) (*Capability, error) {
	resp, err := c.call(ctx, MethodMintCapability, MintCapabilityRequest{
		AgentRunID: agentRunID,
		Role:       role,
		CapsuleID:  capsuleID,
		TTL:        ttl,
	})
	if err != nil {
		return nil, err
	}

	var cap Capability
	if err := json.Unmarshal(resp.Result, &cap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal capability: %w", err)
	}
	return &cap, nil
}

// RevokeCapabilityRequest is the params for revoke_capability.
type RevokeCapabilityRequest struct {
	AgentRunID   string `json:"agent_run_id"`
	CapsuleID    string `json:"capsule_id"`
	CapabilityID string `json:"capability_id"`
}

// RevokeCapability requests revocation of a capability from HostAuthority.
func (c *HostClient) RevokeCapability(ctx context.Context, agentRunID, capsuleID, capabilityID string) error {
	_, err := c.call(ctx, MethodRevokeCapability, RevokeCapabilityRequest{
		AgentRunID:   agentRunID,
		CapsuleID:    capsuleID,
		CapabilityID: capabilityID,
	})
	return err
}

// GetRevokedCapsRequest is the params for get_revoked_caps.
type GetRevokedCapsRequest struct {
	CapsuleID string `json:"capsule_id"`
}

// GetRevokedCaps retrieves the revoked capability set for a capsule from HostAuthority.
// Returns per-capsule revoked IDs + global wildcard revoked IDs.
func (c *HostClient) GetRevokedCaps(ctx context.Context, capsuleID string) ([]string, error) {
	resp, err := c.call(ctx, MethodGetRevokedCaps, GetRevokedCapsRequest{
		CapsuleID: capsuleID,
	})
	if err != nil {
		return nil, err
	}

	var revokedIDs []string
	if err := json.Unmarshal(resp.Result, &revokedIDs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal revoked caps: %w", err)
	}
	return revokedIDs, nil
}

// RegisterCapsule tells HostAuthority about a newly spawned capsule.
func (c *HostClient) RegisterCapsule(ctx context.Context, capsuleID string) error {
	_, err := c.call(ctx, MethodRegisterCapsule, map[string]string{
		"capsule_id": capsuleID,
	})
	return err
}

// UnregisterCapsule tells HostAuthority a capsule has been destroyed.
func (c *HostClient) UnregisterCapsule(ctx context.Context, capsuleID string) error {
	_, err := c.call(ctx, MethodUnregisterCapsule, map[string]string{
		"capsule_id": capsuleID,
	})
	return err
}

// RegisterActiveRun tells HostAuthority about a newly active agent run.
func (c *HostClient) RegisterActiveRun(ctx context.Context, agentRunID string) error {
	_, err := c.call(ctx, MethodRegisterActiveRun, map[string]string{
		"agent_run_id": agentRunID,
	})
	return err
}

// UnregisterActiveRun tells HostAuthority an agent run has completed.
func (c *HostClient) UnregisterActiveRun(ctx context.Context, agentRunID string) error {
	_, err := c.call(ctx, MethodUnregisterActiveRun, map[string]string{
		"agent_run_id": agentRunID,
	})
	return err
}
