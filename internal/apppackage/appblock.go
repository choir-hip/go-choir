package apppackage

import (
	"bytes"
	"encoding/json"
	"math"
	"regexp"
)

// The app block's launcher, window, auth and theme reach typed shell code
// (data attributes, a content class, the launcher order and the auth preview).
// check-packages.mjs checks them for in-tree packages; this is the same check
// for any manifest, so a published one is held to it too.

var (
	shellDataAttrRe = regexp.MustCompile(`^data-[a-z0-9-]+$`)
	contentClassRe  = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
)

type launcherBlock struct {
	Menu        *bool    `json:"menu"`
	DesktopIcon *bool    `json:"desktopIcon"`
	Order       *float64 `json:"order"`
}

type geometryBlock struct {
	Width     *float64 `json:"width"`
	Height    *float64 `json:"height"`
	MinWidth  *float64 `json:"minWidth"`
	MinHeight *float64 `json:"minHeight"`
}

type windowBlock struct {
	Singleton *bool          `json:"singleton"`
	Heavy     *bool          `json:"heavy"`
	Desktop   *geometryBlock `json:"desktop"`
	Compact   *geometryBlock `json:"compact"`
}

type authBlock struct {
	Preview string `json:"preview"`
}

type themeBlock struct {
	Surface       string `json:"surface"`
	ShellDataAttr string `json:"shellDataAttr"`
	ContentClass  string `json:"contentClass"`
}

func strictBlock(raw json.RawMessage, into any) bool {
	if len(bytes.TrimSpace(raw)) == 0 {
		return false
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	return dec.Decode(into) == nil
}

func goodGeometry(g *geometryBlock) bool {
	if g == nil {
		return true
	}
	for _, v := range []*float64{g.Width, g.Height, g.MinWidth, g.MinHeight} {
		if v != nil && (math.IsNaN(*v) || math.IsInf(*v, 0) || *v <= 0) {
			return false
		}
	}
	return true
}

// validateAppBlock checks launcher, window, auth and theme with the shape
// check-packages.mjs holds in-tree packages to.
func (a *App) validateAppBlock() error {
	var l launcherBlock
	if !strictBlock(a.Launcher, &l) || l.Menu == nil || l.DesktopIcon == nil || l.Order == nil || math.IsNaN(*l.Order) || math.IsInf(*l.Order, 0) {
		return invalid("app.launcher needs menu, desktopIcon (booleans) and order (a number)")
	}
	var w windowBlock
	if !strictBlock(a.Window, &w) || w.Singleton == nil || w.Heavy == nil || !goodGeometry(w.Desktop) || !goodGeometry(w.Compact) {
		return invalid("app.window needs singleton and heavy (booleans) and positive geometry")
	}
	var au authBlock
	if !strictBlock(a.Auth, &au) || (au.Preview != "public-preview" && au.Preview != "public-readonly" && au.Preview != "private") {
		return invalid("app.auth.preview must be public-preview, public-readonly or private")
	}
	var th themeBlock
	if !strictBlock(a.Theme, &th) || (th.Surface != "standard" && th.Surface != "document" && th.Surface != "media") ||
		!shellDataAttrRe.MatchString(th.ShellDataAttr) || !contentClassRe.MatchString(th.ContentClass) {
		return invalid("app.theme needs surface standard, document or media, a data-* shellDataAttr and a css contentClass")
	}
	return nil
}
