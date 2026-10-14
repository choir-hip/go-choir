package claudedelegate

// The grant is what vmctl writes beside the token on an allowlisted
// computer's credential disk, and what the guest reads back
// (internal/vmmanager/claude_delegate.go writes it; internal/capsule reads
// it). It never carries the token.
const (
	// GrantFile and TokenFile are the credential-disk file names.
	GrantFile = "claude-delegate.json"
	TokenFile = "claude-oauth-token"
	// GrantAvailable is the only grant status that admits the claude mode.
	GrantAvailable = "available"
	// GrantUnavailable carries a reason instead of a token.
	GrantUnavailable = "unavailable"
)

// Grant is the grant record.
type Grant struct {
	ComputerID string `json:"computer_id"`
	Status     string `json:"status"`
	Reason     string `json:"reason,omitempty"`
	ExpiresAt  string `json:"expires_at,omitempty"`
	GrantedAt  string `json:"granted_at,omitempty"`
}
