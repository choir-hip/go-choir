package proxy

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
)

// Release blobs at the proxy (choir package export, design 10.2), written
// before the code. Failure modes pinned:
//
//	X1  the read names an owner the caller chose rather than the one the
//	    proxy authenticated;
//	X2  an answer that is not the blob asked for is relayed: another
//	    content type (an HTML error page), bytes that hash to another
//	    digest, or more than the patch cap;
//	X3  a malformed id or digest reaches corpusd;
//	X4  a key without the read scope reads it.

func TestPackageBlobReadIsTheAuthenticatedOwnersAndVerified(t *testing.T) {
	env := newPackageProxyEnv(t)
	reader := env.secret(t, "computer:self_development:read")
	id, patch := strings.Repeat("a", 64), []byte("diff --git a/x b/x\n")
	digest := computerevent.DigestBytes(patch)
	html, wrong, big := strings.Repeat("b", 64), strings.Repeat("d", 64), strings.Repeat("e", 64)
	env.corpusdBlobs = map[string][2]string{
		digest: {string(patch), "application/octet-stream"},
		html:   {"<script>alert(1)</script>", "text/html"},
		wrong:  {"not the blob", "application/octet-stream"},
		big:    {strings.Repeat("x", pkgrelease.MaxBlobBytes+1), "application/octet-stream"},
	}

	rec := env.send(t, http.MethodGet, "/api/catalog/releases/"+id+"/blobs/"+digest+"?owner_id=evil", "", reader)
	if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), patch) || rec.Header().Get("Content-Type") != "application/octet-stream" ||
		rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("blob read = %d %v", rec.Code, rec.Header())
	}
	if want := "corpusd GET /internal/platform/packages/releases/" + id + "/blobs/" + digest + " internal=true query=owner_id=" + env.user; !strings.Contains(env.joined(), want) || strings.Contains(env.joined(), "evil") {
		t.Fatalf("X1 calls %v; want %q", env.joined(), want)
	}
	for _, d := range []string{html, wrong, big} {
		if rec := env.send(t, http.MethodGet, "/api/catalog/releases/"+id+"/blobs/"+d, "", reader); rec.Code != http.StatusBadGateway || strings.Contains(rec.Body.String(), "<script>") {
			t.Fatalf("X2 %s… = %d", d[:4], rec.Code)
		}
	}
	if rec := env.send(t, http.MethodGet, "/api/catalog/releases/"+id+"/blobs/"+strings.Repeat("c", 64), "", reader); rec.Code != http.StatusNotFound {
		t.Fatalf("a missing blob = %d", rec.Code)
	}
	before := env.joined()
	for _, path := range []string{"/api/catalog/releases/" + id + "/blobs/not-a-sha", "/api/catalog/releases/x/blobs/" + digest, "/api/catalog/releases/" + id + "/blobs/" + digest + "/x"} {
		if rec := env.send(t, http.MethodGet, path, "", reader); rec.Code != http.StatusNotFound {
			t.Fatalf("X3 %s = %d", path, rec.Code)
		}
	}
	if env.joined() != before {
		t.Fatalf("X3 a malformed path reached corpusd: %s", strings.TrimPrefix(env.joined(), before))
	}
	if rec := env.send(t, http.MethodGet, "/api/catalog/releases/"+id+"/blobs/"+digest, "", env.secret(t, "computer:self_development:approve")); rec.Code != http.StatusForbidden {
		t.Fatalf("X4 a blob read without the read scope = %d", rec.Code)
	}
}
