package apppackage

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Level is a publishing level (plan section 5.1). It is derived from the
// grants a package declares, from this catalogue and nowhere else: a manifest
// cannot state its own level.
type Level string

const (
	// LevelDefault: own kinds, the act set, captures.read, requests to other
	// desks. Anyone may publish; install shows the declared list.
	LevelDefault Level = "default"
	// LevelExtended: ceilings the owner approves per grant at install; the
	// install waits on a sealed install review verdict (MA7.5).
	LevelExtended Level = "extended"
	// LevelPrivileged: first-party only, refused for any other origin at
	// validation and again at the daemon's frame check.
	LevelPrivileged Level = "privileged"
)

func (l Level) rank() int {
	switch l {
	case LevelExtended:
		return 1
	case LevelPrivileged:
		return 2
	}
	return 0
}

type argRule int

const (
	argNone argRule = iota // takes no argument
	argOne                 // exactly one
	argMany                // one to maxGrantArgs
)

const (
	maxGrantArgs    = 16
	maxCapabilities = 64
)

type grantSpec struct {
	level  Level
	args   argRule
	family bool // the name is a prefix: name.<segment> is the grant
}

// grantCatalogue is the closed set of grant names. The first segment is the
// enforcement domain: "ui." is checked by the front-end bridge, "desk." by the
// frame check. Names are provisional (plan section 6, B1); a name not here is
// refused, so a typo or an invented grant never silently grants nothing.
var grantCatalogue = map[string]grantSpec{
	// front-end bridge
	"ui.captures.read": {LevelDefault, argNone, false},
	"ui.read.kind":     {LevelExtended, argOne, false},
	"ui.net.egress":    {LevelExtended, argOne, false},
	"ui.shell":         {LevelPrivileged, argNone, false},

	// desk frame: default
	"desk.act":           {LevelDefault, argNone, false},
	"desk.act.cancel":    {LevelDefault, argNone, false},
	"desk.captures.read": {LevelDefault, argNone, false},
	"desk.intake.read":   {LevelDefault, argNone, false},
	"desk.request":       {LevelDefault, argMany, false},

	// desk frame: extended
	"desk.texture.search": {LevelExtended, argNone, false},
	"desk.mail.read":      {LevelExtended, argOne, false},
	"desk.net.egress":     {LevelExtended, argOne, false},
	"desk.read.kind":      {LevelExtended, argOne, false},

	// desk frame: privileged (first-party only)
	"desk.capsule.mutate":    {LevelPrivileged, argNone, false},
	"desk.selfdev":           {LevelPrivileged, argNone, true},
	"desk.platform.update":   {LevelPrivileged, argNone, false},
	"desk.product.api":       {LevelPrivileged, argNone, false},
	"desk.commit":            {LevelPrivileged, argOne, false},
	"desk.delegate.task":     {LevelPrivileged, argNone, false},
	"desk.admit.assignment":  {LevelPrivileged, argNone, false},
	"desk.world.read":        {LevelPrivileged, argNone, false},
	"desk.cancel.assignment": {LevelPrivileged, argNone, false},
	"desk.intake.dispose":    {LevelPrivileged, argNone, false},
	"desk.precommit.rule":    {LevelPrivileged, argMany, false},
	"desk.run.deadline":      {LevelPrivileged, argOne, false},
}

var (
	familyTailRe = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)
	grantArgRe   = regexp.MustCompile(`^[a-z0-9][a-z0-9._:-]{0,127}$`)
)

// lookupGrant resolves a grant name against the catalogue.
func lookupGrant(name string) (grantSpec, bool) {
	if s, ok := grantCatalogue[name]; ok && !s.family {
		return s, true
	}
	if i := strings.LastIndex(name, "."); i > 0 {
		if s, ok := grantCatalogue[name[:i]]; ok && s.family && familyTailRe.MatchString(name[i+1:]) {
			return s, true
		}
	}
	return grantSpec{}, false
}

// GrantLevel is the catalogue level of a grant name.
func GrantLevel(name string) (Level, bool) {
	s, ok := lookupGrant(name)
	return s.level, ok
}

func (m Manifest) validateCapabilities() error {
	if len(m.Capabilities) > maxCapabilities {
		return invalid("capabilities: at most %d", maxCapabilities)
	}
	hasDesk := len(m.Contributes.Desks) > 0
	seen := map[string]bool{}
	for i, c := range m.Capabilities {
		spec, ok := lookupGrant(c.Grant)
		if !ok {
			return invalid("capabilities[%d]: %q is not a grant in the catalogue", i, c.Grant)
		}
		switch domain := c.Grant[:strings.Index(c.Grant, ".")]; {
		case domain == "ui" && m.App == nil:
			return invalid("capabilities[%d]: %s is a front-end grant and the package has no app", i, c.Grant)
		case domain == "desk" && !hasDesk:
			return invalid("capabilities[%d]: %s is a desk grant and the package contributes no desk", i, c.Grant)
		}
		switch {
		case spec.args == argNone && len(c.Args) != 0:
			return invalid("capabilities[%d]: %s takes no argument", i, c.Grant)
		case spec.args == argOne && len(c.Args) != 1:
			return invalid("capabilities[%d]: %s takes exactly one argument", i, c.Grant)
		case spec.args == argMany && (len(c.Args) == 0 || len(c.Args) > maxGrantArgs):
			return invalid("capabilities[%d]: %s takes one to %d arguments", i, c.Grant, maxGrantArgs)
		}
		for _, a := range c.Args {
			if !grantArgRe.MatchString(a) {
				return invalid("capabilities[%d]: argument %q of %s is not a plain name, host or kind", i, a, c.Grant)
			}
		}
		key := c.key()
		if seen[key] {
			return invalid("capabilities[%d]: %s is requested twice", i, key)
		}
		seen[key] = true
	}
	return nil
}

func (c Capability) key() string {
	args := append([]string{}, c.Args...)
	sort.Strings(args)
	return fmt.Sprintf("%s(%s)", c.Grant, strings.Join(args, ","))
}

// Level is the package's publishing level: the highest of its grants'.
func (m Manifest) Level() Level {
	level := LevelDefault
	for _, c := range m.Capabilities {
		if l, ok := GrantLevel(c.Grant); ok && l.rank() > level.rank() {
			level = l
		}
	}
	return level
}

// ReviewRequired reports whether a platform review precedes the listing: an
// extended or privileged level, or any desk (a desk's prompt is behaviour, so
// a desk is never classed below default with review).
func (m Manifest) ReviewRequired() bool {
	return m.Level() != LevelDefault || len(m.Contributes.Desks) > 0
}

// Class is what the package is, inferred from what it contains. A manifest
// cannot declare it (ErrPublisherDeclared).
type Class string

const (
	ClassData    Class = "data"     // kinds only: runs no code
	ClassApp     Class = "app"      // a front-end window
	ClassDesk    Class = "desk"     // desks, no window
	ClassAppDesk Class = "app+desk" // both
)

// Class infers the package class.
func (m Manifest) Class() Class {
	switch hasDesk := len(m.Contributes.Desks) > 0; {
	case m.App != nil && hasDesk:
		return ClassAppDesk
	case hasDesk:
		return ClassDesk
	case m.App != nil:
		return ClassApp
	}
	return ClassData
}

// CodeBearing reports whether the package runs code on a computer: a window or
// a desk. Data-only packages run none (plan section 12.1).
func (m Manifest) CodeBearing() bool { return m.Class() != ClassData }

// GrantRow is one row of the grant catalogue as other packages read it (the
// trust impact table maps every row to an impact class).
type GrantRow struct {
	Name   string
	Level  Level
	Family bool
}

// GrantCatalogueRows lists the catalogue, sorted by name. A Family row is a
// prefix: name.<segment> is the grant.
func GrantCatalogueRows() []GrantRow {
	rows := make([]GrantRow, 0, len(grantCatalogue))
	for name, s := range grantCatalogue {
		rows = append(rows, GrantRow{Name: name, Level: s.level, Family: s.family})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	return rows
}
