package proxy

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/auth"
)

// W3 (internal/auth/whoami.go): the handle GET /auth/whoami returns is the
// byline a published opinion carries, so the phone shows one name per account.
func TestWhoamiHandleIsTheOpinionByline(t *testing.T) {
	for _, id := range []string{"user-1", "0a1b2c3d-example"} {
		if auth.PublicHandle(id) != opinionAuthorLabel(id) {
			t.Fatalf("handle %q != byline %q", auth.PublicHandle(id), opinionAuthorLabel(id))
		}
	}
}
