package proxy

import (
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// mockLineageReader is a test LineageReader that returns a fixed record.
type mockLineageReader struct {
	record LineageRecord
	err    error
}

func (m *mockLineageReader) GetLineage(ctx context.Context, ownerID, computerID string) (LineageRecord, error) {
	if m.err != nil {
		return LineageRecord{}, m.err
	}
	return m.record, nil
}

func TestLineageBasedRouteResolver_ResolvesFromRouteProfile(t *testing.T) {
	resolver := &LineageBasedRouteResolver{
		Reader: &mockLineageReader{
			record: LineageRecord{
				OwnerID:      vmctl.UniversalWirePlatformOwnerID,
				ComputerID:   vmctl.UniversalWirePlatformComputerID,
				RouteProfile: "universal-wire-platform/platform",
			},
		},
		OwnerID:    vmctl.UniversalWirePlatformOwnerID,
		ComputerID: vmctl.UniversalWirePlatformComputerID,
	}

	owner, desktop, err := resolver.ResolvePlatformRoute(context.Background())
	if err != nil {
		t.Fatalf("ResolvePlatformRoute: %v", err)
	}
	if owner != "universal-wire-platform" {
		t.Errorf("owner = %q, want %q", owner, "universal-wire-platform")
	}
	if desktop != "platform" {
		t.Errorf("desktop = %q, want %q", desktop, "platform")
	}
}

func TestLineageBasedRouteResolver_ResolvesFromCustomRouteProfile(t *testing.T) {
	resolver := &LineageBasedRouteResolver{
		Reader: &mockLineageReader{
			record: LineageRecord{
				RouteProfile: "computer-version-owner/computer-version-desktop",
			},
		},
	}

	owner, desktop, err := resolver.ResolvePlatformRoute(context.Background())
	if err != nil {
		t.Fatalf("ResolvePlatformRoute: %v", err)
	}
	if owner != "computer-version-owner" {
		t.Errorf("owner = %q, want %q", owner, "computer-version-owner")
	}
	if desktop != "computer-version-desktop" {
		t.Errorf("desktop = %q, want %q", desktop, "computer-version-desktop")
	}
}

func TestLineageBasedRouteResolver_EmptyRouteProfileFallsBack(t *testing.T) {
	resolver := &LineageBasedRouteResolver{
		Reader: &mockLineageReader{
			record: LineageRecord{
				RouteProfile: "",
			},
		},
	}

	_, _, err := resolver.ResolvePlatformRoute(context.Background())
	if err == nil {
		t.Fatal("expected error for empty route_profile, got nil")
	}
}

func TestLineageBasedRouteResolver_InvalidRouteProfileFallsBack(t *testing.T) {
	resolver := &LineageBasedRouteResolver{
		Reader: &mockLineageReader{
			record: LineageRecord{
				RouteProfile: "invalid-no-slash",
			},
		},
	}

	_, _, err := resolver.ResolvePlatformRoute(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid route_profile, got nil")
	}
}

func TestLineageBasedRouteResolver_NormalizesLegacyRoutePrefix(t *testing.T) {
	resolver := &LineageBasedRouteResolver{
		Reader: &mockLineageReader{
			record: LineageRecord{
				OwnerID:      vmctl.UniversalWirePlatformOwnerID,
				ComputerID:   vmctl.UniversalWirePlatformComputerID,
				RouteProfile: "route:platform-desktop",
			},
		},
		OwnerID:    vmctl.UniversalWirePlatformOwnerID,
		ComputerID: vmctl.UniversalWirePlatformComputerID,
	}

	owner, desktop, err := resolver.ResolvePlatformRoute(context.Background())
	if err != nil {
		t.Fatalf("ResolvePlatformRoute with legacy route: prefix: %v", err)
	}
	if owner != vmctl.UniversalWirePlatformOwnerID {
		t.Errorf("owner = %q, want %q (normalized from legacy route: prefix)", owner, vmctl.UniversalWirePlatformOwnerID)
	}
	if desktop != "platform-desktop" {
		t.Errorf("desktop = %q, want %q (extracted from legacy route: prefix)", desktop, "platform-desktop")
	}
}

func TestLineageBasedRouteResolver_RejectsEmptyLegacyRoutePrefix(t *testing.T) {
	resolver := &LineageBasedRouteResolver{
		Reader: &mockLineageReader{
			record: LineageRecord{
				OwnerID:      vmctl.UniversalWirePlatformOwnerID,
				ComputerID:   vmctl.UniversalWirePlatformComputerID,
				RouteProfile: "route:",
			},
		},
		OwnerID:    vmctl.UniversalWirePlatformOwnerID,
		ComputerID: vmctl.UniversalWirePlatformComputerID,
	}

	_, _, err := resolver.ResolvePlatformRoute(context.Background())
	if err == nil {
		t.Fatal("expected error for empty legacy route: prefix, got nil")
	}
}

func TestLineageBasedRouteResolver_LineageNotFoundFallsBack(t *testing.T) {
	resolver := &LineageBasedRouteResolver{
		Reader: &mockLineageReader{
			err: &platformRouteError{msg: "not found"},
		},
	}

	_, _, err := resolver.ResolvePlatformRoute(context.Background())
	if err == nil {
		t.Fatal("expected error for lineage not found, got nil")
	}
}

func TestLineageBasedRouteResolver_NilReaderFallsBack(t *testing.T) {
	resolver := &LineageBasedRouteResolver{
		Reader: nil,
	}

	_, _, err := resolver.ResolvePlatformRoute(context.Background())
	if err == nil {
		t.Fatal("expected error for nil reader, got nil")
	}
}

func TestLineageBasedRouteResolver_DefaultsOwnerAndComputer(t *testing.T) {
	resolver := &LineageBasedRouteResolver{
		Reader: &mockLineageReader{
			record: LineageRecord{
				RouteProfile: "universal-wire-platform/platform",
			},
		},
		// OwnerID and ComputerID left empty — should default to platform constants.
	}

	owner, desktop, err := resolver.ResolvePlatformRoute(context.Background())
	if err != nil {
		t.Fatalf("ResolvePlatformRoute: %v", err)
	}
	if owner != vmctl.UniversalWirePlatformOwnerID {
		t.Errorf("owner = %q, want default %q", owner, vmctl.UniversalWirePlatformOwnerID)
	}
	if desktop != vmctl.UniversalWirePlatformDesktopID {
		t.Errorf("desktop = %q, want default %q", desktop, vmctl.UniversalWirePlatformDesktopID)
	}
}

func TestSplitRouteProfile(t *testing.T) {
	cases := []struct {
		input   string
		owner   string
		desktop string
		ok      bool
	}{
		{"universal-wire-platform/platform", "universal-wire-platform", "platform", true},
		{"owner/desktop", "owner", "desktop", true},
		{"no-slash", "", "", false},
		{"", "", "", false},
		{"/desktop", "", "", false},
		{"owner/", "", "", false},
		{"  owner  /  desktop  ", "owner", "desktop", true},
	}
	for _, tc := range cases {
		owner, desktop, ok := splitRouteProfile(tc.input)
		if ok != tc.ok {
			t.Errorf("splitRouteProfile(%q): ok = %v, want %v", tc.input, ok, tc.ok)
			continue
		}
		if ok && (owner != tc.owner || desktop != tc.desktop) {
			t.Errorf("splitRouteProfile(%q): (%q, %q), want (%q, %q)", tc.input, owner, desktop, tc.owner, tc.desktop)
		}
	}
}

// TestStoreLineageReader_NilStore verifies the StoreLineageReader handles
// nil store gracefully.
func TestStoreLineageReader_NilStore(t *testing.T) {
	reader := &StoreLineageReader{Store: nil}
	_, err := reader.GetLineage(context.Background(), "owner", "computer")
	if err == nil {
		t.Fatal("expected error for nil store, got nil")
	}
}
