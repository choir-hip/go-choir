package proxy

import (
	"context"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// LineageRecord is a minimal view of ComputerSourceLineageRecord for route
// resolution. It carries only the fields the proxy needs to determine
// routing identity. This avoids importing the store package directly into
// the proxy.
type LineageRecord struct {
	OwnerID         string
	ComputerID      string
	ActiveSourceRef string
	RouteProfile    string
}

// LineageReader queries computer source lineage records. The store.Store
// satisfies this interface. It is defined here to keep the proxy package
// decoupled from the store package.
type LineageReader interface {
	GetLineage(ctx context.Context, ownerID, computerID string) (LineageRecord, error)
}

// LineageBasedRouteResolver resolves the platform route by querying the
// ComputerSourceLineageRecord for the platform computer. The lineage
// record's route_profile (or active_source_ref) determines the routing
// identity.
//
// If the lineage record is not found or the route_profile is empty, the
// resolver returns an error and the proxy falls back to the hard-coded
// UniversalWirePlatformOwnerID/UniversalWirePlatformDesktopID constants.
//
// This is the route-over-ComputerVersion implementation: instead of
// routing to a fixed VM identity, the route resolves through the
// promotion protocol's lineage state to determine which computer version
// is currently active for the platform route.
type LineageBasedRouteResolver struct {
	Reader     LineageReader
	OwnerID    string // typically vmctl.UniversalWirePlatformOwnerID
	ComputerID string // typically vmctl.UniversalWirePlatformComputerID
}

// ResolvePlatformRoute queries the lineage record and returns the routing
// identity for the platform computer.
func (r *LineageBasedRouteResolver) ResolvePlatformRoute(ctx context.Context) (string, string, error) {
	ownerID := strings.TrimSpace(r.OwnerID)
	computerID := strings.TrimSpace(r.ComputerID)
	if ownerID == "" {
		ownerID = vmctl.UniversalWirePlatformOwnerID
	}
	if computerID == "" {
		computerID = vmctl.UniversalWirePlatformComputerID
	}

	if r.Reader == nil {
		return "", "", &platformRouteError{msg: "lineage reader not configured"}
	}

	rec, err := r.Reader.GetLineage(ctx, ownerID, computerID)
	if err != nil {
		return "", "", &platformRouteError{msg: "lineage lookup failed: " + err.Error()}
	}

	// The route_profile field carries the routing identity in the canonical
	// format "owner_id/computer_id". Legacy values with a "route:" prefix are
	// normalized using the resolver's known owner context so that persisted
	// records written before the format fix resolve correctly without requiring
	// a data migration.
	routeProfile := strings.TrimSpace(rec.RouteProfile)
	if routeProfile == "" {
		return "", "", &platformRouteError{msg: "lineage route_profile is empty"}
	}
	if strings.HasPrefix(routeProfile, "route:") {
		legacyComputerID := strings.TrimSpace(strings.TrimPrefix(routeProfile, "route:"))
		if legacyComputerID == "" {
			return "", "", &platformRouteError{msg: "invalid route_profile format: " + routeProfile}
		}
		routeProfile = ownerID + "/" + legacyComputerID
	}
	owner, desktop, ok := splitRouteProfile(routeProfile)
	if !ok {
		return "", "", &platformRouteError{msg: "invalid route_profile format: " + routeProfile}
	}

	return owner, desktop, nil
}

// splitRouteProfile splits a route profile string "owner_id/desktop_id"
// into its components. Returns false if the format is invalid.
func splitRouteProfile(profile string) (owner, desktop string, ok bool) {
	parts := strings.SplitN(profile, "/", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	owner = strings.TrimSpace(parts[0])
	desktop = strings.TrimSpace(parts[1])
	if owner == "" || desktop == "" {
		return "", "", false
	}
	return owner, desktop, true
}
