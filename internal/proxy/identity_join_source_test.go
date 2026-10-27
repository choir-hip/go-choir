package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/routeledger"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// The route join must accept exactly the source-id shapes the closure
// validator accepts (40 or 64 hex), no more.
func TestCurrentImmutableIdentityJoinsSourceCommitShapes(t *testing.T) {
	createdAt := time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name   string
		source string
		ok     bool
	}{
		{"git sha1", strings.Repeat("1", 40), true},
		{"source tree digest", strings.Repeat("ab", 32), true},
		{"41 hex", strings.Repeat("1", 41), false},
		{"63 hex", strings.Repeat("1", 63), false},
		{"64 non-hex", strings.Repeat("z", 64), false},
		{"empty", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			closure, err := computerversion.NewCodeClosure(tc.source, []computerversion.CodeArtifact{{
				Name: "autoputer", SHA256: strings.Repeat("a", 64), URI: "nix-store+sha256://" + strings.Repeat("a", 64) + "/nix/store/test-autoputer",
			}}, createdAt)
			if !tc.ok {
				// The closure validator refuses this shape, so no stored or
				// served closure can carry it; the join must agree.
				if err == nil {
					t.Fatalf("closure validator admitted source %q", tc.source)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			program, err := computerversion.NewArtifactProgram([]computerversion.ArtifactProgramEntry{{
				Kind: "test", ContentSHA256: strings.Repeat("b", 64), ArtifactURI: "artifact+sha256://" + strings.Repeat("b", 64) + "/test/state",
			}}, createdAt)
			if err != nil {
				t.Fatal(err)
			}
			version := computerversion.ComputerVersion{CodeRef: closure.Ref, ArtifactProgramRef: program.Ref}
			slotID, _ := routeledger.RouteSlotID("owner-private", "primary")
			receipt := routeledger.TransitionReceipt{
				ID: "11111111-1111-4111-8111-111111111111", RouteSlotID: slotID, Kind: routeledger.TransitionBootstrap,
				New: version, CommittedGeneration: 1, ApprovalRef: routeledger.ApprovalRef("approval:sha256:" + strings.Repeat("c", 64)),
				PromotionCertificateRef: routeledger.PromotionCertificateRef("certificate:sha256:" + strings.Repeat("d", 64)),
				IdempotencyKey:          routeledger.IdempotencyKey("idempotency:source-shape"), CommittedAt: createdAt,
			}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewEncoder(w).Encode(vmctl.RouteResolution{
					Slot:          routeledger.Slot{ID: slotID, Current: version, Generation: 1, LatestReceiptID: receipt.ID},
					LatestReceipt: receipt, CodeClosure: closure, ArtifactProgram: program,
				})
			}))
			defer srv.Close()
			h := &Handler{vmctlClient: vmctl.NewClient(srv.URL)}
			identity, err := h.currentImmutableIdentity(t.Context(), "owner-private", "primary")
			if err != nil || identity == nil || !identity.Joined || identity.CodeCommit != tc.source {
				t.Fatalf("identity=%+v err=%v, want joined with source %q", identity, err, tc.source)
			}
		})
	}
}
