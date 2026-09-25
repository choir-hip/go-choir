package types

import "testing"

func TestAuthorKindValid(t *testing.T) {
	validKinds := []AuthorKind{AuthorUser, AuthorAppAgent}
	for _, k := range validKinds {
		if !k.Valid() {
			t.Errorf("expected %q to be valid", k)
		}
	}

	invalidKinds := []AuthorKind{"worker", "system", "", "admin"}
	for _, k := range invalidKinds {
		if k.Valid() {
			t.Errorf("expected %q to be invalid", k)
		}
	}
}
