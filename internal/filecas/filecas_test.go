package filecas

import (
	"bytes"
	"testing"
	"time"
)

func TestChunkBytesEdges(t *testing.T) {
	cases := []struct {
		name string
		data string
		want []string
	}{
		{name: "empty", want: []string{}},
		{name: "exact multiple", data: "abcdef", want: []string{"abc", "def"}},
		{name: "remainder", data: "abcdefg", want: []string{"abc", "def", "g"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ChunkBytes([]byte(tc.data), 3)
			if len(got) != len(tc.want) {
				t.Fatalf("chunk count = %d, want %d", len(got), len(tc.want))
			}
			for i := range got {
				if string(got[i]) != tc.want[i] {
					t.Fatalf("chunk %d = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestManifestDeterminism(t *testing.T) {
	now := time.Date(2026, 8, 26, 12, 0, 0, 123, time.UTC)
	entries := []FileEntry{{Path: "z", Mode: 0o600, Size: 1}, {Path: "a", Mode: 0o644, Size: 2}}
	first, err := BuildManifest("computer-1", entries, now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildManifest("computer-1", []FileEntry{entries[1], entries[0]}, now)
	if err != nil {
		t.Fatal(err)
	}
	if first.Root != second.Root || first.Files[0].Path != "a" {
		t.Fatalf("roots/order = %q/%q, first=%+v", first.Root, second.Root, first.Files)
	}
	if err := first.VerifyRoot(); err != nil {
		t.Fatal(err)
	}
}

func TestSealOpenChunk(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	sealed, digest, err := SealChunk(key, "computer-1", []byte("private chunk"))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := OpenChunk(key, "computer-1", digest, sealed)
	if err != nil || string(plain) != "private chunk" {
		t.Fatalf("open = %q, %v", plain, err)
	}
	tampered := append([]byte(nil), sealed...)
	tampered[len(tampered)-1] ^= 1
	if _, err := OpenChunk(key, "computer-1", digest, tampered); err == nil {
		t.Fatal("tampered chunk accepted")
	}
	if _, err := OpenChunk(key, "computer-other", digest, sealed); err == nil {
		t.Fatal("wrong computer accepted")
	}
}
