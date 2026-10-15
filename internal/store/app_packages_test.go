package store

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/apppackage"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

const appPackageOwner = "owner-pkg"

// appPackageInstall builds a correctly bound install event from a temporary
// static package source tree.
func appPackageInstall(t *testing.T, name string, mutate func(*apppackage.Manifest)) apppackage.Event {
	t.Helper()
	dir := t.TempDir()
	entry := filepath.Join(dir, "frontend", "index.js")
	if err := os.MkdirAll(filepath.Dir(entry), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entry, []byte("export function mount(t) { t.textContent = '"+name+"'; return { destroy() {} }; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	digest, err := apppackage.SourceDigest(dir)
	if err != nil {
		t.Fatal(err)
	}
	m := apppackage.Manifest{
		Schema:   apppackage.ManifestSchemaV1,
		Name:     name,
		Version:  "0.1.0",
		Display:  apppackage.Display{Title: name},
		Source:   apppackage.SourceRef{Digest: digest},
		Frontend: apppackage.Frontend{Entry: "frontend/index.js", ABI: apppackage.FrontendABIV1},
		Build:    apppackage.BuildSpec{Recipe: apppackage.RecipeStaticV1, Outputs: []string{"frontend/index.js"}},
		Data:     apppackage.DataSpec{Store: apppackage.DataStoreKVV1, SchemaVersion: 1, ReadsSchemaVersions: []int{1}},
		Restore:  apppackage.RestoreSpec{DataScope: apppackage.DataScopeComputerTape, OnPurge: apppackage.OnPurgeDeleteData},
		Grants:   []json.RawMessage{},
	}
	if mutate != nil {
		mutate(&m)
	}
	raw, err := apppackage.MarshalManifest(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, apppackage.ManifestFile), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	manifest, build, _, err := apppackage.BuildStatic(dir)
	if err != nil {
		t.Fatal(err)
	}
	md, _ := manifest.Digest()
	bd, _ := build.Digest()
	return apppackage.Event{Op: apppackage.OpInstall, Package: name, Manifest: &manifest, Build: &build, ManifestDigest: md, BuildDigest: bd}
}

func appPackageOp(op apppackage.Op, name string) apppackage.Event {
	return apppackage.Event{Op: op, Package: name}
}

// recordingPinner captures every projection batch the appender pins, in tape
// order, so a test can replay a prefix of the tape into a fresh store.
type recordingPinner struct {
	liveTapePinner
	mu      sync.Mutex
	batches []computerevent.ProjectionBatch
}

func (p *recordingPinner) PinNonPrivatePayload(ctx context.Context, computerID string, payload []byte, mediaType, privacyClass, pinIntentCommitment string) (computerevent.PinResult, error) {
	if mediaType == computerevent.ProjectionBatchMediaType {
		var batch computerevent.ProjectionBatch
		if err := json.Unmarshal(payload, &batch); err == nil {
			p.mu.Lock()
			p.batches = append(p.batches, batch)
			p.mu.Unlock()
		}
	}
	return p.liveTapePinner.PinNonPrivatePayload(ctx, computerID, payload, mediaType, privacyClass, pinIntentCommitment)
}

func (p *recordingPinner) recorded() []computerevent.ProjectionBatch {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]computerevent.ProjectionBatch(nil), p.batches...)
}

func bindRecordingTape(t *testing.T, productStore *Store, computerID string) *recordingPinner {
	t.Helper()
	prepareGenesis(t, productStore, computerID, "genesis-"+computerID)
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer := computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "test"}, PrivateKey: privateKey}
	pinner := &recordingPinner{liveTapePinner: liveTapePinner{signer: signer}}
	appender, err := computerevent.NewComputerEventAppender(computerID, pinner, productStore, liveTapeCAS{signer: signer, projection: productStore}, liveTapeVerifier{})
	if err != nil {
		t.Fatal(err)
	}
	if err := productStore.BindProjectionTape(computerID, appender); err != nil {
		t.Fatal(err)
	}
	return pinner
}

func tapeEventCount(t *testing.T, productStore *Store, computerID string) int {
	t.Helper()
	var n int
	if err := productStore.db.QueryRow(`SELECT COUNT(*) FROM computer_event_index WHERE computer_id=? AND event_kind=?`, computerID, string(computerevent.EventProjectionBatchRecorded)).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// snapshot is what one tape head shows about the hello package.
type appPackageSnapshot struct {
	present bool
	enabled bool
	count   string // the "count" datum's JSON, or "" when absent
}

func snapshotAppPackage(t *testing.T, productStore *Store, computerID string) appPackageSnapshot {
	t.Helper()
	ctx := context.Background()
	inv, err := productStore.AppPackageInventory(ctx, computerID)
	if err != nil {
		t.Fatal(err)
	}
	_, present := inv.Packages["hello"]
	snap := appPackageSnapshot{present: present, enabled: inv.Enabled("hello")}
	if present {
		datum, err := productStore.GetAppPackageDatum(ctx, computerID, "hello", "count")
		if err == nil {
			snap.count = string(datum.Value)
		} else if !errors.Is(err, ErrNotFound) {
			t.Fatal(err)
		}
	}
	return snap
}

// TestStoreAppPackageLifecycleOnTape drives install, enable, data, disable and
// purge through a bound tape: every act is one projection batch, and purge
// under delete_data removes the data in the same batch (F19).
func TestStoreAppPackageLifecycleOnTape(t *testing.T) {
	ctx := context.Background()
	productStore := openProjectStore(t)
	computerID := "computer-pkg-tape"
	bindRecordingTape(t, productStore, computerID)

	before := tapeEventCount(t, productStore, computerID)
	inv, ev, err := productStore.RecordAppPackageEvent(ctx, appPackageOwner, computerID, appPackageInstall(t, "hello", nil))
	if err != nil {
		t.Fatal(err)
	}
	if ev.Seq != 1 || ev.Prev != "" || inv.Packages["hello"].Status != apppackage.StatusInstalled {
		t.Fatalf("install: ev=%+v inv=%+v", ev, inv.Packages)
	}
	if got := tapeEventCount(t, productStore, computerID); got != before+1 {
		t.Fatalf("install appended %d tape events, want 1", got-before)
	}
	if _, _, err := productStore.RecordAppPackageEvent(ctx, appPackageOwner, computerID, appPackageOp(apppackage.OpEnable, "hello")); err != nil {
		t.Fatal(err)
	}
	if err := productStore.PutAppPackageDatum(ctx, appPackageOwner, computerID, apppackage.Datum{Package: "hello", Key: "count", Value: json.RawMessage(`3`)}); err != nil {
		t.Fatal(err)
	}
	datum, err := productStore.GetAppPackageDatum(ctx, computerID, "hello", "count")
	if err != nil || string(datum.Value) != "3" || datum.SchemaVersion != 1 {
		t.Fatalf("datum: %+v %v", datum, err)
	}
	if _, _, err := productStore.RecordAppPackageEvent(ctx, appPackageOwner, computerID, appPackageOp(apppackage.OpDisable, "hello")); err != nil {
		t.Fatal(err)
	}
	// Disabled packages keep their data readable.
	if _, err := productStore.GetAppPackageDatum(ctx, computerID, "hello", "count"); err != nil {
		t.Fatalf("disabled package lost its data: %v", err)
	}
	beforePurge := tapeEventCount(t, productStore, computerID)
	inv, ev, err = productStore.RecordAppPackageEvent(ctx, appPackageOwner, computerID, appPackageOp(apppackage.OpPurge, "hello"))
	if err != nil {
		t.Fatal(err)
	}
	if got := tapeEventCount(t, productStore, computerID); got != beforePurge+1 {
		t.Fatalf("purge appended %d tape events, want 1 (event and tombstones together)", got-beforePurge)
	}
	if _, present := inv.Packages["hello"]; present || ev.Seq != 4 {
		t.Fatalf("after purge: ev=%+v inv=%+v", ev, inv.Packages)
	}
	if _, err := productStore.GetAppPackageDatum(ctx, computerID, "hello", "count"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("purged package data still readable: %v", err)
	}
	// Reinstalling does not resurrect delete_data data.
	if _, _, err := productStore.RecordAppPackageEvent(ctx, appPackageOwner, computerID, appPackageInstall(t, "hello", nil)); err != nil {
		t.Fatal(err)
	}
	if _, err := productStore.GetAppPackageDatum(ctx, computerID, "hello", "count"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("reinstall resurrected purged data: %v", err)
	}
	events, err := productStore.AppPackageEvents(ctx, computerID)
	if err != nil || len(events) != 5 {
		t.Fatalf("events: %d %v", len(events), err)
	}
}

// TestStoreAppPackageReplayPrefix is the restore property (F23, F24): the
// state at any tape head is what replaying the tape up to that head, with
// the generic projector, gives a fresh store.
func TestStoreAppPackageReplayPrefix(t *testing.T) {
	ctx := context.Background()
	productStore := openProjectStore(t)
	computerID := "computer-pkg-replay"
	pinner := bindRecordingTape(t, productStore, computerID)

	type head struct {
		batches int
		want    appPackageSnapshot
	}
	var heads []head
	mark := func() {
		heads = append(heads, head{batches: len(pinner.recorded()), want: snapshotAppPackage(t, productStore, computerID)})
	}
	mark() // before install
	acts := []func() error{
		func() error {
			_, _, err := productStore.RecordAppPackageEvent(ctx, appPackageOwner, computerID, appPackageInstall(t, "hello", nil))
			return err
		},
		func() error {
			_, _, err := productStore.RecordAppPackageEvent(ctx, appPackageOwner, computerID, appPackageOp(apppackage.OpEnable, "hello"))
			return err
		},
		func() error {
			return productStore.PutAppPackageDatum(ctx, appPackageOwner, computerID, apppackage.Datum{Package: "hello", Key: "count", Value: json.RawMessage(`7`)})
		},
		func() error {
			_, _, err := productStore.RecordAppPackageEvent(ctx, appPackageOwner, computerID, appPackageOp(apppackage.OpDisable, "hello"))
			return err
		},
		func() error {
			_, _, err := productStore.RecordAppPackageEvent(ctx, appPackageOwner, computerID, appPackageOp(apppackage.OpPurge, "hello"))
			return err
		},
	}
	for _, act := range acts {
		if err := act(); err != nil {
			t.Fatal(err)
		}
		mark()
	}
	if heads[0].want.present || !heads[3].want.enabled || heads[3].want.count != "7" || heads[5].want.present {
		t.Fatalf("live snapshots are not the expected story: %+v", heads)
	}

	batches := pinner.recorded()
	for i, h := range heads {
		h := h
		t.Run("head"+string(rune('0'+i)), func(t *testing.T) {
			fresh := openProjectStore(t)
			tx, err := fresh.db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			for _, batch := range batches[:h.batches] {
				if err := projectBatchForReplay(ctx, tx, batch); err != nil {
					_ = tx.Rollback()
					t.Fatalf("replay: %v", err)
				}
			}
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
			if got := snapshotAppPackage(t, fresh, computerID); got != h.want {
				t.Fatalf("replayed head %d = %+v, live was %+v", i, got, h.want)
			}
		})
	}
}

// TestStoreAppPackageDataGuards covers F21 and F22 on an unbound store.
func TestStoreAppPackageDataGuards(t *testing.T) {
	ctx := context.Background()
	productStore := openTestStore(t)
	computerID := "computer-pkg-guards"
	put := func(pkg, key, value string) error {
		return productStore.PutAppPackageDatum(ctx, appPackageOwner, computerID, apppackage.Datum{Package: pkg, Key: key, Value: json.RawMessage(value)})
	}
	if err := put("hello", "count", "1"); !errors.Is(err, apppackage.ErrNotInstalled) {
		t.Fatalf("datum for an absent package: %v", err)
	}
	if _, _, err := productStore.RecordAppPackageEvent(ctx, appPackageOwner, computerID, appPackageInstall(t, "hello", nil)); err != nil {
		t.Fatal(err)
	}
	if err := put("hello", "count", "1"); !errors.Is(err, apppackage.ErrNotEnabled) {
		t.Fatalf("datum for an installed, not enabled package: %v", err)
	}
	if _, _, err := productStore.RecordAppPackageEvent(ctx, appPackageOwner, computerID, appPackageOp(apppackage.OpEnable, "hello")); err != nil {
		t.Fatal(err)
	}
	for name, args := range map[string][2]string{
		"key-upper":   {"Count", "1"},
		"key-slash":   {"a/b", "1"},
		"key-empty":   {"", "1"},
		"value-json":  {"count", "{not json"},
		"value-empty": {"count", ""},
	} {
		if err := put("hello", args[0], args[1]); !errors.Is(err, apppackage.ErrInvalidDatum) {
			t.Fatalf("%s accepted: %v", name, err)
		}
	}
	big := make([]byte, apppackage.MaxDatumBytes+1)
	for i := range big {
		big[i] = '1'
	}
	if err := put("hello", "count", string(big)); !errors.Is(err, apppackage.ErrInvalidDatum) {
		t.Fatalf("oversized value accepted: %v", err)
	}
	if err := put("hello", "count", `{"n":1}`); err != nil {
		t.Fatal(err)
	}

	// A data-less package takes no data.
	if _, _, err := productStore.RecordAppPackageEvent(ctx, appPackageOwner, computerID, appPackageInstall(t, "plain", func(m *apppackage.Manifest) {
		m.Data = apppackage.DataSpec{Store: apppackage.DataStoreNone}
	})); err != nil {
		t.Fatal(err)
	}
	if _, _, err := productStore.RecordAppPackageEvent(ctx, appPackageOwner, computerID, appPackageOp(apppackage.OpEnable, "plain")); err != nil {
		t.Fatal(err)
	}
	if err := put("plain", "count", "1"); !errors.Is(err, apppackage.ErrUnsupported) {
		t.Fatalf("datum for a data-less package: %v", err)
	}
	// Refused lifecycle acts append nothing.
	if _, _, err := productStore.RecordAppPackageEvent(ctx, appPackageOwner, computerID, appPackageOp(apppackage.OpPurge, "hello")); !errors.Is(err, apppackage.ErrInvalidTransition) {
		t.Fatalf("purge while enabled: %v", err)
	}
	events, err := productStore.AppPackageEvents(ctx, computerID)
	if err != nil || len(events) != 4 {
		t.Fatalf("events after a refused purge: %d %v", len(events), err)
	}
}

// TestStoreAppPackageRetainData shows retain_data keeping data across purge
// and a compatible reinstall reading it.
func TestStoreAppPackageRetainData(t *testing.T) {
	ctx := context.Background()
	productStore := openTestStore(t)
	computerID := "computer-pkg-retain"
	retain := func(m *apppackage.Manifest) { m.Restore.OnPurge = apppackage.OnPurgeRetainData }
	for _, ev := range []apppackage.Event{appPackageInstall(t, "hello", retain), appPackageOp(apppackage.OpEnable, "hello")} {
		if _, _, err := productStore.RecordAppPackageEvent(ctx, appPackageOwner, computerID, ev); err != nil {
			t.Fatal(err)
		}
	}
	if err := productStore.PutAppPackageDatum(ctx, appPackageOwner, computerID, apppackage.Datum{Package: "hello", Key: "count", Value: json.RawMessage(`5`)}); err != nil {
		t.Fatal(err)
	}
	for _, ev := range []apppackage.Event{appPackageOp(apppackage.OpDisable, "hello"), appPackageOp(apppackage.OpPurge, "hello")} {
		if _, _, err := productStore.RecordAppPackageEvent(ctx, appPackageOwner, computerID, ev); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := productStore.GetAppPackageDatum(ctx, computerID, "hello", "count"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("data of an absent package readable: %v", err)
	}
	if _, _, err := productStore.RecordAppPackageEvent(ctx, appPackageOwner, computerID, appPackageInstall(t, "hello", retain)); err != nil {
		t.Fatal(err)
	}
	datum, err := productStore.GetAppPackageDatum(ctx, computerID, "hello", "count")
	if err != nil || string(datum.Value) != "5" {
		t.Fatalf("retained data after reinstall: %+v %v", datum, err)
	}
}

// TestStoreAppPackageConcurrentInstall covers F25: concurrent acts never fork
// the event chain.
func TestStoreAppPackageConcurrentInstall(t *testing.T) {
	ctx := context.Background()
	productStore := openTestStore(t)
	computerID := "computer-pkg-race"
	installs := []apppackage.Event{appPackageInstall(t, "hello", nil), appPackageInstall(t, "hello", nil), appPackageInstall(t, "other", nil)}
	var wg sync.WaitGroup
	errs := make([]error, len(installs))
	for i := range installs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _, errs[i] = productStore.RecordAppPackageEvent(ctx, appPackageOwner, computerID, installs[i])
		}(i)
	}
	wg.Wait()
	helloOK := 0
	for i, err := range errs[:2] {
		if err == nil {
			helloOK++
		} else if !errors.Is(err, apppackage.ErrAlreadyInstalled) {
			t.Fatalf("install %d: %v", i, err)
		}
	}
	if helloOK != 1 || errs[2] != nil {
		t.Fatalf("concurrent installs: hello ok=%d, other err=%v", helloOK, errs[2])
	}
	inv, err := productStore.AppPackageInventory(ctx, computerID)
	if err != nil {
		t.Fatalf("chain forked: %v", err)
	}
	if inv.Seq != 2 || len(inv.Packages) != 2 {
		t.Fatalf("inventory after race: seq=%d packages=%v", inv.Seq, inv.Packages)
	}
}
