package agentcore

import (
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
)

func TestPackageUpdateNeedsBeforeAfter(t *testing.T) {
	update := pkgrelease.Parent{Kind: pkgrelease.ParentRelease, Tree: strings.Repeat("a", 40)}
	fresh := pkgrelease.Parent{Kind: pkgrelease.ParentBase, Tree: pkgrelease.EmptyTree}
	if err := packageUpdateNeedsBeforeAfter(update, false); err == nil || !strings.Contains(err.Error(), capsule.BeforeAfterRequiredReason) {
		t.Errorf("an update without a pair passed: %v", err)
	}
	if err := packageUpdateNeedsBeforeAfter(update, true); err != nil {
		t.Errorf("an update with a pair was refused: %v", err)
	}
	if err := packageUpdateNeedsBeforeAfter(fresh, false); err != nil {
		t.Errorf("a new package was refused: %v", err)
	}
}
