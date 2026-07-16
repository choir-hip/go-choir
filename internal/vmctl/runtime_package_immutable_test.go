package vmctl

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerversion"
)

type runtimeInputFixture struct{ closure computerversion.CodeClosure }

func (f runtimeInputFixture) ResolveCode(_ context.Context, ref computerversion.CodeRef) (computerversion.CodeClosure, error) {
	if ref != f.closure.Ref {
		return computerversion.CodeClosure{}, computerversion.ErrInputNotFound
	}
	return f.closure, nil
}
func (f runtimeInputFixture) ResolveArtifactProgram(context.Context, computerversion.ArtifactProgramRef) (computerversion.ArtifactProgram, error) {
	return computerversion.ArtifactProgram{}, computerversion.ErrInputNotFound
}

type runtimeArtifactFixture struct{ payload []byte }

type fixtureReadSeekCloser struct{ *bytes.Reader }

func (fixtureReadSeekCloser) Close() error { return nil }

func (f runtimeArtifactFixture) OpenSeekableArtifact(context.Context, string, string) (computerversion.ReadSeekCloser, error) {
	return fixtureReadSeekCloser{bytes.NewReader(f.payload)}, nil
}

func TestHandleRuntimePackageServesImmutableCodeClosureArtifact(t *testing.T) {
	var archive bytes.Buffer
	tw := tar.NewWriter(&archive)
	content := []byte("runtime")
	if err := tw.WriteHeader(&tar.Header{Name: "bin/sandbox", Mode: 0o755, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	payload := archive.Bytes()
	digest := sha256.Sum256(payload)
	hexDigest := hex.EncodeToString(digest[:])
	closure, err := computerversion.NewCodeClosure(strings.Repeat("a", 40), []computerversion.CodeArtifact{{
		Name: "sandbox-runtime.tar", SHA256: hexDigest, URI: "artifact+sha256://" + hexDigest + "/runtime/sandbox.tar",
	}}, time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(NewOwnershipRegistry("http://sandbox.test"))
	handler.routeAuthority = &RouteAuthority{inputs: runtimeInputFixture{closure: closure}}
	handler.SetImmutableArtifactOpener(runtimeArtifactFixture{payload: payload})
	request := httptest.NewRequest(http.MethodGet, "/internal/vmctl/runtime-package/sandbox?code_ref="+string(closure.Ref), nil)
	request.Header.Set("X-Internal-Caller", "true")
	response := httptest.NewRecorder()
	handler.HandleRuntimePackage(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	if !bytes.Equal(response.Body.Bytes(), payload) {
		t.Fatalf("runtime payload = %q", response.Body.Bytes())
	}
}

func TestValidateRuntimePackageTarRefusesEscapingSymlink(t *testing.T) {
	var archive bytes.Buffer
	tw := tar.NewWriter(&archive)
	if err := tw.WriteHeader(&tar.Header{Name: "bin", Mode: 0o755, Typeflag: tar.TypeDir}); err != nil {
		t.Fatal(err)
	}
	if err := tw.WriteHeader(&tar.Header{Name: "bin/sandbox", Mode: 0o755, Typeflag: tar.TypeSymlink, Linkname: "../../host"}); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := validateRuntimePackageTar(bytes.NewReader(archive.Bytes())); err == nil {
		t.Fatal("expected escaping symlink refusal")
	}
}
