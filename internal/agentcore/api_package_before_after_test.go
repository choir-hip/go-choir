package agentcore

import (
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
)

func TestPackageUpdateNeedsBeforeAfter(t *testing.T) {
	shot := func(name string) pkgrelease.Media {
		return pkgrelease.Media{Kind: pkgrelease.MediaKindScreenshot, Name: name}
	}
	update := pkgrelease.Parent{Kind: pkgrelease.ParentRelease, Tree: strings.Repeat("a", 40)}
	fresh := pkgrelease.Parent{Kind: pkgrelease.ParentBase, Tree: pkgrelease.EmptyTree}
	pair := []pkgrelease.Media{shot("01-before-shelf.png"), shot("01-after-shelf.png")}
	after := []pkgrelease.Media{shot("01-after-shelf.png")}

	if err := packageUpdateNeedsBeforeAfter(update, after); err == nil || !strings.Contains(err.Error(), capsule.BeforeAfterRequiredReason) {
		t.Errorf("an update with after shots only passed: %v", err)
	}
	if err := packageUpdateNeedsBeforeAfter(update, nil); err == nil {
		t.Error("an update with no media passed")
	}
	if err := packageUpdateNeedsBeforeAfter(update, pair); err != nil {
		t.Errorf("an update with a pair was refused: %v", err)
	}
	if err := packageUpdateNeedsBeforeAfter(fresh, after); err != nil {
		t.Errorf("a new package with after shots was refused: %v", err)
	}
	video := []pkgrelease.Media{{Kind: pkgrelease.MediaKindVideo, Name: "01-before-shelf.png"}, shot("01-after-shelf.png")}
	if err := packageUpdateNeedsBeforeAfter(update, video); err == nil {
		t.Error("a video counted as a before shot")
	}
}
