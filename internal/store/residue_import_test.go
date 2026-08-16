package store

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestImportResidueSnapshotRequiresBoundTape(t *testing.T) {
	productStore := openProjectStore(t)
	_, err := productStore.ImportResidueSnapshot(context.Background())
	if !errors.Is(err, ErrResidueImportUnbound) {
		t.Fatalf("unbound import error=%v", err)
	}
}

func TestImportResidueSnapshotRefusesDesktopOnly(t *testing.T) {
	ctx := context.Background()
	productStore := openProjectStore(t)
	computerID := "computer-residue-desktop"
	prepareGenesis(t, productStore, computerID, "genesis-residue-desktop")
	if err := productStore.SaveDesktopStateForSession(ctx, residueDesktopState("win-d"), types.DesktopSessionContext{
		SessionID: "browser-d", IsDriver: true, DriverUntil: time.Now().UTC().Add(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	if err := bindResidueTape(t, productStore, computerID); err != nil {
		t.Fatal(err)
	}
	_, err := productStore.ImportResidueSnapshot(ctx)
	if !errors.Is(err, ErrResidueImportSplit) {
		t.Fatalf("desktop-only import error=%v", err)
	}
}

func TestImportResidueSnapshotRefusesOGOnly(t *testing.T) {
	ctx := context.Background()
	productStore := openProjectStore(t)
	computerID := "computer-residue-og"
	prepareGenesis(t, productStore, computerID, "genesis-residue-og")
	if err := productStore.ogStore.PutObject(ctx, residueObject(computerID, "obj-og")); err != nil {
		t.Fatal(err)
	}
	if err := bindResidueTape(t, productStore, computerID); err != nil {
		t.Fatal(err)
	}
	_, err := productStore.ImportResidueSnapshot(ctx)
	if !errors.Is(err, ErrResidueImportSplit) {
		t.Fatalf("og-only import error=%v", err)
	}
}

func TestImportResidueSnapshotCoMovesDesktopAndOG(t *testing.T) {
	ctx := context.Background()
	productStore := openProjectStore(t)
	computerID := "computer-residue-both"
	prepareGenesis(t, productStore, computerID, "genesis-residue-both")
	if err := productStore.SaveDesktopStateForSession(ctx, residueDesktopState("win-r"), types.DesktopSessionContext{
		SessionID: "browser-r", IsDriver: true, DriverUntil: time.Now().UTC().Add(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	if err := productStore.ogStore.PutObject(ctx, residueObject(computerID, "obj-r")); err != nil {
		t.Fatal(err)
	}
	if _, err := productStore.db.ExecContext(ctx,
		`INSERT INTO desktop_sessions (
			owner_id, desktop_id, session_id, device_id, viewport_profile,
			visibility_state, last_input_at, driver_until, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"owner-1", types.PrimaryDesktopID, "stale-tab", "device-1", "wide",
		"hidden", time.Now().UTC().Format(time.RFC3339Nano), time.Now().UTC().Add(time.Minute).Format(time.RFC3339Nano),
		time.Now().UTC().Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano),
	); err != nil {
		t.Fatal(err)
	}

	before, err := productStore.Head(ctx, computerID)
	if err != nil {
		t.Fatal(err)
	}
	if err := bindResidueTape(t, productStore, computerID); err != nil {
		t.Fatal(err)
	}
	result, err := productStore.ImportResidueSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Appended || result.Desktops != 1 || result.Objects != 1 || result.Sessions != 1 {
		t.Fatalf("import result=%+v", result)
	}
	after, err := productStore.Head(ctx, computerID)
	if err != nil {
		t.Fatal(err)
	}
	if after == nil || before == nil || after.Sequence != before.Sequence+1 {
		t.Fatalf("head before=%+v after=%+v", before, after)
	}

	state, err := productStore.GetDesktopStateForSession(ctx, "owner-1", types.PrimaryDesktopID, "projected")
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Windows) != 1 || state.Windows[0].WindowID != "win-r" {
		t.Fatalf("imported desktop=%+v", state)
	}
	stored, err := productStore.ogStore.GetObject(ctx, "obj-r")
	if err != nil {
		t.Fatal(err)
	}
	if stored.CanonicalID != "obj-r" {
		t.Fatalf("imported object=%+v", stored)
	}

	var visibility string
	var lastInput, driverUntil sql.NullString
	row := productStore.db.QueryRowContext(ctx,
		`SELECT visibility_state, last_input_at, driver_until FROM desktop_sessions
		  WHERE owner_id=? AND desktop_id=? AND session_id=?`,
		"owner-1", types.PrimaryDesktopID, "stale-tab")
	if err := row.Scan(&visibility, &lastInput, &driverUntil); err != nil {
		t.Fatal(err)
	}
	if visibility != "" || lastInput.Valid || driverUntil.Valid {
		t.Fatalf("presence leaked onto projected session visibility=%q last=%v driver=%v", visibility, lastInput, driverUntil)
	}

}

func TestImportResidueSnapshotNoopWhenEmpty(t *testing.T) {
	ctx := context.Background()
	productStore := openProjectStore(t)
	computerID := "computer-residue-empty"
	prepareGenesis(t, productStore, computerID, "genesis-residue-empty")
	if err := bindResidueTape(t, productStore, computerID); err != nil {
		t.Fatal(err)
	}
	before, err := productStore.Head(ctx, computerID)
	if err != nil {
		t.Fatal(err)
	}
	result, err := productStore.ImportResidueSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result.Appended {
		t.Fatalf("empty import appended: %+v", result)
	}
	after, err := productStore.Head(ctx, computerID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Sequence != before.Sequence {
		t.Fatalf("empty import moved head %d -> %d", before.Sequence, after.Sequence)
	}
}

func residueDesktopState(windowID string) types.DesktopState {
	return types.DesktopState{
		OwnerID: "owner-1",
		Windows: []types.WindowState{{
			WindowID: windowID, AppID: "texture", Title: "Residue",
			Geometry: types.WindowGeometry{X: 8, Y: 9, Width: 10, Height: 11},
			Mode:     types.WindowNormal, ZIndex: 1,
		}},
		ActiveWindowID: windowID,
		UpdatedAt:      time.Date(2026, 8, 16, 18, 10, 0, 0, time.UTC),
	}
}

func residueObject(computerID, objectID string) objectgraph.Object {
	now := time.Date(2026, 8, 16, 18, 11, 0, 0, time.UTC)
	return objectgraph.Object{
		CanonicalID: objectID, ObjectKind: "choir.texture_revision", OwnerID: "owner-1",
		ComputerID: computerID, VersionID: "v1", ContentHash: storeTestDigest('7'),
		Body: []byte(`{"text":"residue"}`), Metadata: json.RawMessage(`{}`),
		CreatedAt: now, UpdatedAt: now,
	}
}

func bindResidueTape(t *testing.T, productStore *Store, computerID string) error {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	signer := computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "test"}, PrivateKey: privateKey}
	appender, err := computerevent.NewComputerEventAppender(
		computerID,
		liveTapePinner{signer: signer},
		productStore,
		liveTapeCAS{signer: signer, projection: productStore},
		liveTapeVerifier{},
	)
	if err != nil {
		return err
	}
	return productStore.BindProjectionTape(computerID, appender)
}
