package proxy

import (
	"context"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// PlatformRouteResolver resolves the platform computer's current routing
// identity. This decouples the proxy from hard-coded VM identity constants
// (the H031 heresy symptom) by allowing an external resolver to determine
// which owner/desktop the platform route should target.
//
// The resolver queries the ComputerSourceLineageRecord for the platform
// computer and returns the routing identity. If the resolver is nil or
// returns an error, the proxy falls back to the hard-coded
// UniversalWirePlatformOwnerID/UniversalWirePlatformDesktopID constants.
//
// This is the seam where route-over-ComputerVersion enters the proxy:
// instead of routing to a fixed VM identity, the route resolves through
// the promotion protocol's lineage state to determine which computer
// version is currently active for the platform route.
type PlatformRouteResolver interface {
	// ResolvePlatformRoute returns the owner ID and desktop ID that the
	// platform route (/api/universal-wire/stories) should target.
	// If the resolver cannot determine the route (e.g., lineage record
	// not found), it returns an error and the caller falls back to the
	// hard-coded constants.
	ResolvePlatformRoute(ctx context.Context) (ownerID, desktopID string, err error)
}

// resolvePlatformTarget resolves the platform route target using the
// resolver if available, falling back to the hard-coded constants.
// This is the route-over-ComputerVersion seam: the resolver queries the
// ComputerSourceLineage to determine the active computer version's
// routing identity, instead of using a fixed VM identity.
func (h *Handler) resolvePlatformTarget(ctx context.Context) (ownerID, desktopID string) {
	if h.routeResolver != nil {
		owner, desktop, err := h.routeResolver.ResolvePlatformRoute(ctx)
		if err == nil && owner != "" && desktop != "" {
			return owner, desktop
		}
		// Fall back to hard-coded constants if the resolver fails.
		// This preserves the current behavior as a safety net.
	}
	return vmctl.UniversalWirePlatformOwnerID, vmctl.UniversalWirePlatformDesktopID
}

// SetRouteResolver sets the platform route resolver. This is called
// during handler initialization if a route resolver is available.
// If not called, the handler uses the hard-coded platform constants.
func (h *Handler) SetRouteResolver(resolver PlatformRouteResolver) {
	h.routeResolver = resolver
}

// StaticPlatformRouteResolver is a PlatformRouteResolver that always
// returns the same owner/desktop. It is useful for testing and for
// environments where the platform computer's routing identity is
// statically configured.
type StaticPlatformRouteResolver struct {
	OwnerID   string
	DesktopID string
}

// ResolvePlatformRoute returns the static owner/desktop pair.
func (s StaticPlatformRouteResolver) ResolvePlatformRoute(ctx context.Context) (string, string, error) {
	owner := strings.TrimSpace(s.OwnerID)
	desktop := strings.TrimSpace(s.DesktopID)
	if owner == "" || desktop == "" {
		return "", "", errPlatformRouteNotConfigured
	}
	return owner, desktop, nil
}

// errPlatformRouteNotConfigured is returned when the static route resolver
// has empty owner or desktop IDs.
var errPlatformRouteNotConfigured = &platformRouteError{msg: "platform route not configured"}

type platformRouteError struct{ msg string }

func (e *platformRouteError) Error() string { return e.msg }
