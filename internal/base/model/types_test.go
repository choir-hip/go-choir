package model

import (
	"testing"
	"time"
)

func TestItemIDValid(t *testing.T) {
	cases := []struct {
		id   ItemID
		want bool
	}{
		{"base_item_abc", true},
		{"base_item_", false},
		{"", false},
		{"abc", false},
		{"base_item_00000000-0000-0000-0000-000000000000", true},
	}
	for _, c := range cases {
		if got := c.id.Valid(); got != c.want {
			t.Errorf("ItemID(%q).Valid() = %v, want %v", c.id, got, c.want)
		}
	}
}

func TestBlobRefValid(t *testing.T) {
	hex64 := "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	cases := []struct {
		b    BlobRef
		want bool
	}{
		{"", true}, // folders
		{BlobRef("sha256:" + hex64), true},
		{BlobRef("sha256:abc"), false},      // too short
		{BlobRef("sha256:nothex!!"), false}, // wrong length + not hex
		{BlobRef("sha256:" + hex64 + "x"), false},
		{BlobRef("blake3:" + hex64), false}, // wrong prefix
	}
	for _, c := range cases {
		if got := c.b.Valid(); got != c.want {
			t.Errorf("BlobRef(%q).Valid() = %v, want %v", c.b, got, c.want)
		}
	}
}

func TestVersionIDValid(t *testing.T) {
	cases := []struct {
		v    VersionID
		want bool
	}{
		{"", true}, // deleted item
		{"base_ver_abc", true},
		{"base_ver_", false},
		{"abc", false},
	}
	for _, c := range cases {
		if got := c.v.Valid(); got != c.want {
			t.Errorf("VersionID(%q).Valid() = %v, want %v", c.v, got, c.want)
		}
	}
}

func TestEventIDValid(t *testing.T) {
	cases := []struct {
		e    EventID
		want bool
	}{
		{"base_evt_abc", true},
		{"base_evt_", false},
		{"", false},
		{"abc", false},
	}
	for _, c := range cases {
		if got := c.e.Valid(); got != c.want {
			t.Errorf("EventID(%q).Valid() = %v, want %v", c.e, got, c.want)
		}
	}
}

func TestItemKindValid(t *testing.T) {
	if !KindFile.Valid() {
		t.Error("KindFile should be valid")
	}
	if !KindFolder.Valid() {
		t.Error("KindFolder should be valid")
	}
	if (ItemKind("blob")).Valid() {
		t.Error("unknown kind should be invalid")
	}
}

func TestEventTypeValid(t *testing.T) {
	for _, e := range []EventType{EventCreate, EventUpdate, EventDelete, EventMove} {
		if !e.Valid() {
			t.Errorf("EventType(%q) should be valid", e)
		}
	}
	if (EventType("noop")).Valid() {
		t.Error("unknown event type should be invalid")
	}
}

func TestSyncStateValid(t *testing.T) {
	for _, s := range []SyncState{StateSynced, StateLocalOnly, StateRemoteOnly, StateConflict, StateStuck} {
		if !s.Valid() {
			t.Errorf("SyncState(%q) should be valid", s)
		}
	}
	if (SyncState("pending")).Valid() {
		t.Error("unknown sync state should be invalid")
	}
}

func TestItemValid(t *testing.T) {
	now := time.Now()
	good := Item{
		ItemID:         "base_item_1",
		OwnerID:        "owner",
		Kind:           KindFile,
		CurrentVersion: "base_ver_1",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if !good.Valid() {
		t.Error("valid item rejected")
	}
	badID := good
	badID.ItemID = "nope"
	if badID.Valid() {
		t.Error("item with bad ID accepted")
	}
	badKind := good
	badKind.Kind = "blob"
	if badKind.Valid() {
		t.Error("item with bad kind accepted")
	}
	badVer := good
	badVer.CurrentVersion = "nope"
	if badVer.Valid() {
		t.Error("item with bad version accepted")
	}
}

func TestVersionValid(t *testing.T) {
	hex64 := "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	fileVer := Version{
		VersionID: "base_ver_1",
		ItemID:    "base_item_1",
		BlobRef:   BlobRef("sha256:" + hex64),
	}
	if !fileVer.Valid() {
		t.Error("valid file version rejected")
	}
	folderVer := Version{
		VersionID: "base_ver_2",
		ItemID:    "base_item_2",
		BlobRef:   "",
	}
	if !folderVer.Valid() {
		t.Error("valid folder version rejected")
	}
	folderWithHash := folderVer
	folderWithHash.ContentHash = hex64
	if folderWithHash.Valid() {
		t.Error("folder version with content hash accepted")
	}
	badID := fileVer
	badID.VersionID = "nope"
	if badID.Valid() {
		t.Error("version with bad ID accepted")
	}
}

func TestBlobValid(t *testing.T) {
	hex64 := "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	b := Blob{BlobRef: BlobRef("sha256:" + hex64), SizeBytes: 10, SHA256: hex64}
	if !b.Valid() {
		t.Error("valid blob rejected")
	}
	mismatch := b
	mismatch.SHA256 = "00"
	if mismatch.Valid() {
		t.Error("blob with mismatched sha256 accepted")
	}
	neg := b
	neg.SizeBytes = -1
	if neg.Valid() {
		t.Error("blob with negative size accepted")
	}
}

func TestEventValid(t *testing.T) {
	e := Event{
		EventID:   "base_evt_1",
		OwnerID:   "owner",
		ItemID:    "base_item_1",
		EventType: EventUpdate,
		CursorSeq: 1,
	}
	if !e.Valid() {
		t.Error("valid event rejected")
	}
	noOwner := e
	noOwner.OwnerID = ""
	if noOwner.Valid() {
		t.Error("event without owner accepted")
	}
	negSeq := e
	negSeq.CursorSeq = -1
	if negSeq.Valid() {
		t.Error("event with negative cursor accepted")
	}
}

func TestSyncStatusValid(t *testing.T) {
	s := SyncStatus{
		OwnerID:        "owner",
		DeviceID:       "dev",
		ItemID:         "base_item_1",
		State:          StateSynced,
		LocalVersionID: "base_ver_1",
	}
	if !s.Valid() {
		t.Error("valid sync status rejected")
	}
	badState := s
	badState.State = "pending"
	if badState.Valid() {
		t.Error("sync status with bad state accepted")
	}
}

func TestItemLocation(t *testing.T) {
	i := Item{ParentItemID: "base_item_parent", Name: "notes.txt"}
	p, n := i.Location()
	if p != "base_item_parent" || n != "notes.txt" {
		t.Errorf("Location() = (%q,%q), want (base_item_parent, notes.txt)", p, n)
	}
}
