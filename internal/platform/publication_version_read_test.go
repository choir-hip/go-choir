package platform

import (
	"context"
	"errors"
	"testing"
)

func TestReadPublicationVersionServesTheImmutableVersion(t *testing.T) {
	svc, _ := newVersionTestService(t)
	v1 := publishRevision(t, svc, 1, "rev-a", "Harbor opens.")
	v2 := publishRevision(t, svc, 2, "rev-b", "Harbor opens at dawn.")

	// An older version is still served after a newer one is published.
	got, err := svc.ReadPublicationVersion(context.Background(), v1.PublicationVersionID)
	if err != nil {
		t.Fatalf("read v1: %v", err)
	}
	if got.Content != "Harbor opens." || got.ManifestHash != v1.ManifestHash || got.OwnerID != "owner-1" || got.Revision != 1 {
		t.Fatalf("v1 read = %+v", got)
	}
	latest, err := svc.ReadPublicationVersion(context.Background(), v2.PublicationVersionID)
	if err != nil || latest.ManifestHash != v2.ManifestHash || latest.RoutePath != v2.PermalinkPath {
		t.Fatalf("v2 read = %+v err=%v", latest, err)
	}

	// Unknown ids and ids that are not versions are refused alike.
	for name, id := range map[string]string{"unknown": "choir.publication_version:owner-1:nope", "a publication": v1.PublicationID} {
		if _, err := svc.ReadPublicationVersion(context.Background(), id); !errors.Is(err, errVersionNotReadable) {
			t.Errorf("%s: err=%v, want errVersionNotReadable", name, err)
		}
	}
}
