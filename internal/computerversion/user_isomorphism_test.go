package computerversion

import (
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/base/model"
)

func TestUserIsomorphismCheckerPassesScopedFileManifestSemantics(t *testing.T) {
	version := baseSliceComputerVersion()
	left := isomorphismRealization(t, "left", version, []model.Event{baseCreateEvent(1, "a")})
	right := isomorphismRealization(t, "right", version, []model.Event{baseCreateEvent(1, "a")})

	result := UserIsomorphismChecker{}.CheckRealizations(left, right, fileManifestIsomorphismScope())
	if !result.UserIsomorphic() {
		t.Fatalf("expected scoped file manifest user isomorphism, got %#v", result)
	}
}

func TestUserIsomorphismCheckerNarrowsUnclaimedSemantic(t *testing.T) {
	version := baseSliceComputerVersion()
	left := isomorphismRealization(t, "left", version, []model.Event{baseCreateEvent(1, "a")})
	right := isomorphismRealization(t, "right", version, []model.Event{baseCreateEvent(1, "a")})
	scope := fileManifestIsomorphismScope()
	scope.RequiredSemantics = append(scope.RequiredSemantics, UserSemanticLiveProcessContinuity)

	result := UserIsomorphismChecker{}.CheckRealizations(left, right, scope)
	if result.Status != UserIsomorphismNarrowed {
		t.Fatalf("expected unclaimed live-process semantic to narrow, got %#v", result)
	}
}

func TestUserIsomorphismCheckerNarrowsUnsupportedSemantic(t *testing.T) {
	version := baseSliceComputerVersion()
	left := isomorphismRealization(t, "left", version, []model.Event{baseCreateEvent(1, "a")})
	right := isomorphismRealization(t, "right", version, []model.Event{baseCreateEvent(1, "a")})
	scope := fileManifestIsomorphismScope()
	scope.RequiredSemantics = append(scope.RequiredSemantics, UserSemanticLiveProcessContinuity)
	scope.CoveredSemantics = append(scope.CoveredSemantics, UserSemanticLiveProcessContinuity)
	scope.UnsupportedSemantics = append(scope.UnsupportedSemantics, UnsupportedUserSemantic{Semantic: UserSemanticLiveProcessContinuity, Reason: "process replay law not defined"})

	result := UserIsomorphismChecker{}.CheckRealizations(left, right, scope)
	if result.Status != UserIsomorphismNarrowed {
		t.Fatalf("expected unsupported live-process semantic to narrow, got %#v", result)
	}
}

func TestUserIsomorphismCheckerFailsObservationMismatch(t *testing.T) {
	version := baseSliceComputerVersion()
	left := isomorphismRealization(t, "left", version, []model.Event{baseCreateEvent(1, "a")})
	right := isomorphismRealization(t, "right", version, []model.Event{baseCreateEvent(1, "b")})

	result := UserIsomorphismChecker{}.CheckRealizations(left, right, fileManifestIsomorphismScope())
	if result.Status != UserIsomorphismNotEquivalent {
		t.Fatalf("expected mismatched observations to fail user isomorphism, got %#v", result)
	}
}

func isomorphismRealization(t *testing.T, id string, version ComputerVersion, events []model.Event) Realization {
	t.Helper()
	observations, err := BaseEventJournalObservationSet(id, version, events)
	if err != nil {
		t.Fatalf("extract %s: %v", id, err)
	}
	realization, err := (ProjectionMaterializer{ID: id, Observations: observations}).Materialize(context.Background(), version, CapabilityManifest{
		Materializer: id,
		Substrate:    "file-manifest-projection",
		Supported:    []ObservationKind{ObservationFileManifest},
	})
	if err != nil {
		t.Fatalf("materialize %s: %v", id, err)
	}
	return realization
}

func fileManifestIsomorphismScope() UserIsomorphismScope {
	return UserIsomorphismScope{
		Name:             "file-manifest-slice",
		ObservationKinds: []ObservationKind{ObservationFileManifest},
		RequiredSemantics: []UserSemantic{
			UserSemanticFilePath,
			UserSemanticFileContent,
			UserSemanticDeletionState,
		},
		CoveredSemantics: []UserSemantic{
			UserSemanticFilePath,
			UserSemanticFileContent,
			UserSemanticDeletionState,
		},
	}
}
