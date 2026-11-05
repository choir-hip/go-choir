package apppackage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// What a package contributes to the shell beside its window (system
// modularity design section 2.2). The front end's services own the runtime
// rules (frontend/src/lib/services/{commands,links,theme}.ts) and
// frontend/scripts/check-packages.mjs checks in-tree manifests; this is the
// same check for any manifest, so an installed package is held to it too.
//
// Failure modes (each has a test in contributes_test.go):
//
//	X1 a command id outside the package's namespace, repeated, or with an
//	   action the shell does not know: refused;
//	X2 a keybinding on a locked key, or naming another package's command:
//	   refused (a package binds a key and never reads keys);
//	X3 a theme id that is not the package's own (it would shadow a built-in
//	   preset) or a theme file that leaves the package: refused;
//	X4 a link for another package's app, a shell-owned parameter, or an
//	   expression where a literal belongs: refused;
//	X5 a handler command, a link or an open without an app to run it: refused;
//	X6 a list or string past its bound: refused (a manifest is untrusted input);
//	X7 a package that contributes nothing at all: refused.

const (
	maxCommands    = 64
	maxKeybindings = 64
	maxThemes      = 16
	maxLinks       = 16
	maxLinkParams  = 16
	maxStringField = 512
	// MaxThemeFileBytes bounds a contributed theme file.
	MaxThemeFileBytes = 64 << 10
)

// Command is one contributed command: { id, title, scope, run, when? }. Run is
// a declarative action (openApp, openFile, copyText, setTheme, toast) or
// { "handler": true }, meaning the package's code registers it.
type Command struct {
	ID    string          `json:"id"`
	Title string          `json:"title"`
	Scope string          `json:"scope"`
	Run   json.RawMessage `json:"run"`
	When  string          `json:"when,omitempty"`
}

// Keybinding binds a key to one of the package's own commands.
type Keybinding struct {
	Key     string `json:"key"`
	Command string `json:"command"`
	When    string `json:"when,omitempty"`
}

// ThemeFile names a theme the package carries as data: Id is the theme's id,
// File a schema-2 theme JSON inside the package.
type ThemeFile struct {
	ID   string `json:"id"`
	File string `json:"file"`
}

// Link opens the package's app from a URL (?app=<app>&<param>=...).
type Link struct {
	App    string      `json:"app"`
	Params []LinkParam `json:"params"`
	Launch *LinkLaunch `json:"launch,omitempty"`
}

// LinkParam is one URL parameter of a link. Context names the window context
// key it fills; a parameter without one is consumed and nothing more.
type LinkParam struct {
	Name     string `json:"name"`
	Required bool   `json:"required,omitempty"`
	Context  string `json:"context,omitempty"`
}

// LinkLaunch is the window a link opens. Context values are literals or
// "param:<name>"; there are no expressions.
type LinkLaunch struct {
	WindowTitle string         `json:"windowTitle,omitempty"`
	RequireAny  []string       `json:"requireAny,omitempty"`
	Context     map[string]any `json:"context,omitempty"`
}

var (
	commandIDRe     = regexp.MustCompile(`^[a-z][a-z0-9-]*\.[A-Za-z][A-Za-z0-9._-]*$`)
	commandScopeRe  = regexp.MustCompile(`^(global|app|target:[a-z][a-z0-9_-]*)$`)
	themeIDRe       = regexp.MustCompile(`^[a-z][a-z0-9-]*(\.[a-z0-9-]+)*$`)
	themeFileRe     = regexp.MustCompile(`^[A-Za-z0-9._-]+(/[A-Za-z0-9._-]+)*\.json$`)
	linkNameRe      = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)
	linkContextKey  = regexp.MustCompile(`^[a-z][A-Za-z0-9_]{0,31}$`)
	linkReserved    = map[string]bool{"app": true, "title": true, "doc": true, "doc_id": true}
	lockedKeys      = map[string]bool{"Mod+K": true}
	commandActions  = map[string][]string{"openApp": {"app"}, "openFile": {"path"}, "copyText": {"text"}, "setTheme": {"theme"}, "toast": {"message"}}
	commandOptional = map[string][]string{"copyText": {"label"}}
)

// NormalizeKey is the shell's key spelling (commands.ts normalizeKey): Mod,
// Alt and Shift in that order, then the key. It returns "" for a spelling
// that names no key.
func NormalizeKey(key string) string {
	mods := map[string]bool{}
	main := ""
	for _, part := range strings.Split(key, "+") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		switch strings.ToLower(part) {
		case "mod", "cmd", "ctrl", "meta":
			mods["Mod"] = true
		case "alt", "option":
			mods["Alt"] = true
		case "shift":
			mods["Shift"] = true
		default:
			if utf8.RuneCountInString(part) == 1 {
				main = strings.ToUpper(part)
			} else {
				main = strings.ToUpper(part[:1]) + part[1:]
			}
		}
	}
	if main == "" {
		return ""
	}
	out := []string{}
	for _, m := range []string{"Mod", "Alt", "Shift"} {
		if mods[m] {
			out = append(out, m)
		}
	}
	return strings.Join(append(out, main), "+")
}

func tooLong(s string, n int) bool { return utf8.RuneCountInString(s) > n }

func (m Manifest) contributesNothingElse() bool {
	c := m.Contributes
	return len(c.Commands) == 0 && len(c.Keybindings) == 0 && len(c.Themes) == 0 && len(c.Links) == 0 && len(c.Settings) == 0
}

func (m Manifest) validateContributes() error {
	if err := m.validateCommands(); err != nil {
		return err
	}
	if err := m.validateThemes(); err != nil {
		return err
	}
	if err := m.validateLinks(); err != nil {
		return err
	}
	return m.validateSettings()
}

func (m Manifest) validateCommands() error {
	c := m.Contributes
	if len(c.Commands) > maxCommands {
		return invalid("contributes.commands: at most %d", maxCommands)
	}
	ids := map[string]bool{}
	for i, cmd := range c.Commands {
		w := fmt.Sprintf("contributes.commands[%d]", i)
		if !commandIDRe.MatchString(cmd.ID) || tooLong(cmd.ID, 96) {
			return invalid("%s.id %q must be <package>.<name>", w, cmd.ID)
		}
		if strings.SplitN(cmd.ID, ".", 2)[0] != m.Name {
			return invalid("%s.id %q is outside the namespace of %q", w, cmd.ID, m.Name)
		}
		if ids[cmd.ID] {
			return invalid("%s.id %q is declared twice", w, cmd.ID)
		}
		ids[cmd.ID] = true
		if strings.TrimSpace(cmd.Title) == "" || tooLong(cmd.Title, 80) {
			return invalid("%s.title is required and at most 80 characters", w)
		}
		if !commandScopeRe.MatchString(cmd.Scope) {
			return invalid("%s.scope %q must be global, app or target:<kind>", w, cmd.Scope)
		}
		if tooLong(cmd.When, 200) {
			return invalid("%s.when is at most 200 characters", w)
		}
		if err := m.validateRun(w, cmd.Run); err != nil {
			return err
		}
	}
	if len(c.Keybindings) > maxKeybindings {
		return invalid("contributes.keybindings: at most %d", maxKeybindings)
	}
	bound := map[string]bool{}
	for i, kb := range c.Keybindings {
		w := fmt.Sprintf("contributes.keybindings[%d]", i)
		key := NormalizeKey(kb.Key)
		switch {
		case key == "" || tooLong(kb.Key, 40):
			return invalid("%s.key %q must name a key, for example \"Mod+Shift+P\"", w, kb.Key)
		case lockedKeys[key]:
			return invalid("%s.key %s is locked to the shell", w, key)
		case !strings.HasPrefix(kb.Command, m.Name+"."):
			return invalid("%s.command %q must be one of the package's own (%s.<name>)", w, kb.Command, m.Name)
		case !ids[kb.Command]:
			return invalid("%s.command %q is not in this package's commands", w, kb.Command)
		case tooLong(kb.When, 200):
			return invalid("%s.when is at most 200 characters", w)
		}
		slot := key + "\x00" + kb.When
		if bound[slot] {
			return invalid("%s: key %s is bound twice", w, key)
		}
		bound[slot] = true
	}
	return nil
}

// validateRun checks a command's run: a declarative action with exactly its
// own string fields, or exactly { "handler": true } (code registers it).
func (m Manifest) validateRun(w string, raw json.RawMessage) error {
	var run map[string]json.RawMessage
	if err := json.Unmarshal(raw, &run); err != nil || run == nil {
		return invalid("%s.run must be an action or { \"handler\": true }", w)
	}
	if h, ok := run["handler"]; ok {
		if len(run) != 1 || !bytes.Equal(bytes.TrimSpace(h), []byte("true")) {
			return invalid("%s.run.handler must be exactly { \"handler\": true }", w)
		}
		if m.App == nil {
			return invalid("%s.run: a handler command needs an app to register it", w)
		}
		return nil
	}
	var action string
	if json.Unmarshal(run["action"], &action) != nil || commandActions[action] == nil {
		return invalid("%s.run.action must be one of openApp, openFile, copyText, setTheme, toast", w)
	}
	allowed := map[string]bool{"action": true}
	for _, f := range commandActions[action] {
		allowed[f] = true
		var v string
		if json.Unmarshal(run[f], &v) != nil || strings.TrimSpace(v) == "" {
			return invalid("%s.run.%s is required for %s and must be a string", w, f, action)
		}
	}
	for _, f := range commandOptional[action] {
		allowed[f] = true
	}
	limits := map[string]int{"app": 40, "path": 512, "text": 2000, "label": 60, "theme": 96, "message": 200}
	for k, v := range run {
		if !allowed[k] {
			return invalid("%s.run: %q is not a field of %s", w, k, action)
		}
		if k == "action" {
			continue
		}
		var s string
		if json.Unmarshal(v, &s) != nil {
			return invalid("%s.run.%s must be a string", w, k)
		}
		if tooLong(s, limits[k]) {
			return invalid("%s.run.%s is at most %d characters", w, k, limits[k])
		}
		if k == "app" && !aliasPattern.MatchString(s) {
			return invalid("%s.run.app %q is not an app id", w, s)
		}
	}
	return nil
}

func (m Manifest) validateThemes() error {
	themes := m.Contributes.Themes
	if len(themes) > maxThemes {
		return invalid("contributes.themes: at most %d", maxThemes)
	}
	ids, files := map[string]bool{}, map[string]bool{}
	for i, t := range themes {
		w := fmt.Sprintf("contributes.themes[%d]", i)
		if !themeIDRe.MatchString(t.ID) || tooLong(t.ID, 96) {
			return invalid("%s.id %q must be lowercase letters, digits, dashes and dots", w, t.ID)
		}
		// A theme id is the package's own namespace, so a pack can never take
		// a built-in preset's id.
		if t.ID != m.Name && !strings.HasPrefix(t.ID, m.Name+".") {
			return invalid("%s.id %q must be %q or start with %q", w, t.ID, m.Name, m.Name+".")
		}
		if ids[t.ID] {
			return invalid("%s.id %q is declared twice", w, t.ID)
		}
		ids[t.ID] = true
		if !themeFileRe.MatchString(t.File) || strings.Contains(t.File, "..") || len(strings.Split(t.File, "/")) > 8 || tooLong(t.File, 256) {
			return invalid("%s.file %q must be a relative .json path inside the package", w, t.File)
		}
		if files[t.File] {
			return invalid("%s.file %q is named twice", w, t.File)
		}
		files[t.File] = true
	}
	return nil
}

func (m Manifest) validateLinks() error {
	links := m.Contributes.Links
	if len(links) > maxLinks {
		return invalid("contributes.links: at most %d", maxLinks)
	}
	apps := map[string]bool{}
	for i, l := range links {
		w := fmt.Sprintf("contributes.links[%d]", i)
		if m.App == nil {
			return invalid("%s: a link opens the package's app and the package has none", w)
		}
		if l.App != m.Name {
			return invalid("%s.app %q is not the package's own (%q)", w, l.App, m.Name)
		}
		if apps[l.App] {
			return invalid("%s.app %q is declared twice", w, l.App)
		}
		apps[l.App] = true
		if len(l.Params) == 0 || len(l.Params) > maxLinkParams {
			return invalid("%s.params must list one to %d parameters", w, maxLinkParams)
		}
		names := map[string]bool{}
		for j, p := range l.Params {
			pw := fmt.Sprintf("%s.params[%d]", w, j)
			switch {
			case !linkNameRe.MatchString(p.Name):
				return invalid("%s.name %q must be a lowercase word", pw, p.Name)
			case linkReserved[p.Name]:
				return invalid("%s.name %q is a parameter the shell owns", pw, p.Name)
			case names[p.Name]:
				return invalid("%s.name %q is listed twice", pw, p.Name)
			case p.Context != "" && !linkContextKey.MatchString(p.Context):
				return invalid("%s.context %q must be a key word", pw, p.Context)
			}
			names[p.Name] = true
		}
		if l.Launch == nil {
			continue
		}
		lw := w + ".launch"
		if tooLong(l.Launch.WindowTitle, 80) {
			return invalid("%s.windowTitle is at most 80 characters", lw)
		}
		for _, n := range l.Launch.RequireAny {
			if !names[n] {
				return invalid("%s.requireAny names %q, which is not a declared parameter", lw, n)
			}
		}
		if len(l.Launch.Context) > maxLinkParams {
			return invalid("%s.context: at most %d keys", lw, maxLinkParams)
		}
		for k, v := range l.Launch.Context {
			if !linkContextKey.MatchString(k) {
				return invalid("%s.context key %q is not a key word", lw, k)
			}
			switch val := v.(type) {
			case string:
				if tooLong(val, maxStringField) {
					return invalid("%s.context %s is over %d characters", lw, k, maxStringField)
				}
				if ref, ok := strings.CutPrefix(val, "param:"); ok && !names[ref] {
					return invalid("%s.context %s reads parameter %q, which is not declared", lw, k, ref)
				}
			case float64, bool:
			default:
				return invalid("%s.context %s must be a literal or \"param:<name>\"", lw, k)
			}
		}
	}
	return nil
}

// ThemeFiles lists the theme files the package carries, sorted and unique.
func (m Manifest) ThemeFiles() []string {
	out := make([]string, 0, len(m.Contributes.Themes))
	for _, t := range m.Contributes.Themes {
		out = append(out, t.File)
	}
	sort.Strings(out)
	return out
}

// surfaceContributes is the non-desk contributions as the behaviour-surface
// hash sees them: lists sorted, so authoring order never moves the hash. It is
// nil (and omitted) for a package that contributes none, so every existing
// hash holds.
func (m Manifest) surfaceContributes() (json.RawMessage, error) {
	if m.contributesNothingElse() {
		return nil, nil
	}
	c := m.Contributes
	c.Desks = nil
	c.Commands = append([]Command{}, c.Commands...)
	c.Keybindings = append([]Keybinding{}, c.Keybindings...)
	c.Themes = append([]ThemeFile{}, c.Themes...)
	c.Links = append([]Link{}, c.Links...)
	c.Settings = append([]SettingsSection{}, c.Settings...)
	sort.Slice(c.Commands, func(i, j int) bool { return c.Commands[i].ID < c.Commands[j].ID })
	sort.Slice(c.Keybindings, func(i, j int) bool {
		return c.Keybindings[i].Key+"\x00"+c.Keybindings[i].When < c.Keybindings[j].Key+"\x00"+c.Keybindings[j].When
	})
	sort.Slice(c.Themes, func(i, j int) bool { return c.Themes[i].ID < c.Themes[j].ID })
	sort.Slice(c.Links, func(i, j int) bool { return c.Links[i].App < c.Links[j].App })
	sort.Slice(c.Settings, func(i, j int) bool { return c.Settings[i].Section < c.Settings[j].Section })
	raw, err := json.Marshal(struct {
		Commands    []Command         `json:"commands"`
		Keybindings []Keybinding      `json:"keybindings"`
		Themes      []ThemeFile       `json:"themes"`
		Links       []Link            `json:"links"`
		Settings    []SettingsSection `json:"settings,omitempty"`
	}{c.Commands, c.Keybindings, c.Themes, c.Links, c.Settings})
	if err != nil {
		return nil, err
	}
	return canonicalRaw(raw, "null")
}
