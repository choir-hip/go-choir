package main

// One command registry generates both of Choir's agent surfaces: the CLI's
// flag parsing, positional arguments, validation and help, and the MCP
// server's tools/list (input schemas, annotations, scopes) and tools/call.
// A command is declared once, with typed parameters and one handler that
// calls the HTTP API and returns structured output; the CLI prints that
// output as JSON and the MCP server returns it as a tool result. A command
// that cannot be a tool says why (cliOnly), and so does every parameter
// that exists on one surface only. registry_test.go fails on drift.
// Design and inventory: docs/reports/choir-cli-mcp-parity-2026-10-14.md.

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type kind int

const (
	kString   kind = iota
	kInt           // int64
	kNumber        // float64; a CLI string flag parsed on use
	kBool          // bool
	kDuration      // CLI duration flag; MCP string such as "15m"
	kStrings       // []string; a repeatable CLI flag, an MCP array
	kJSON          // a JSON value; an MCP object
)

type posMode int

const (
	posNone posMode = iota
	posOne          // one positional argument
	posJoin         // every remaining positional argument, joined by spaces
	posRest         // every remaining positional argument, appended to a list
)

// param is one typed input. Its name is the MCP property; its CLI flag is
// the kebab-case name unless flag says otherwise. A positional parameter
// has no flag unless flag names one.
type param struct {
	name     string
	kind     kind
	desc     string
	required bool
	enum     []string
	def      any    // default: string, int64, float64, bool, time.Duration or []string
	env      string // non-secret environment default (strings only)
	flag     string
	pos      posMode
	split    bool // kStrings: CLI values are comma-separated too
	min, max *float64
	missing  string // CLI message when this parameter is missing or invalid
	cliOnly  string // reason this parameter exists only on the CLI
	mcpOnly  string // reason this parameter exists only on MCP
}

func pStr(name, desc string) param      { return param{name: name, kind: kString, desc: desc} }
func pInt(name, desc string) param      { return param{name: name, kind: kInt, desc: desc} }
func pNum(name, desc string) param      { return param{name: name, kind: kNumber, desc: desc} }
func pBool(name, desc string) param     { return param{name: name, kind: kBool, desc: desc} }
func pDuration(name, desc string) param { return param{name: name, kind: kDuration, desc: desc} }
func pList(name, desc string) param     { return param{name: name, kind: kStrings, desc: desc} }
func pJSON(name, desc string) param     { return param{name: name, kind: kJSON, desc: desc} }

func (p param) req() param                  { p.required = true; return p }
func (p param) flagName(f string) param     { p.flag = f; return p }
func (p param) dflt(v any) param            { p.def = v; return p }
func (p param) fromEnv(env string) param    { p.env = env; return p }
func (p param) one() param                  { p.pos = posOne; return p }
func (p param) join() param                 { p.pos = posJoin; return p }
func (p param) rest() param                 { p.pos = posRest; return p }
func (p param) commas() param               { p.split = true; return p }
func (p param) oneOf(v ...string) param     { p.enum = v; return p }
func (p param) atLeast(n float64) param     { p.min = &n; return p }
func (p param) between(a, b float64) param  { p.min, p.max = &a, &b; return p }
func (p param) orSay(msg string) param      { p.missing = msg; return p }
func (p param) onlyCLI(reason string) param { p.cliOnly = reason; return p }
func (p param) onlyMCP(reason string) param { p.mcpOnly = reason; return p }

func (p param) cliFlag() string {
	if p.flag != "" {
		return p.flag
	}
	if p.pos != posNone {
		return ""
	}
	return strings.ReplaceAll(p.name, "_", "-")
}

func (p param) onCLI() bool { return p.mcpOnly == "" }
func (p param) onMCP() bool { return p.cliOnly == "" }

// command is one CLI command and, unless cliOnly, one MCP tool.
type command struct {
	path        []string
	tool        string // MCP tool name; default: path joined with "_"
	summary     string
	desc        string // longer description; default summary
	params      []param
	scopes      []string // API key scopes; "authenticated" means any valid key, "public" none
	readOnly    bool
	destructive bool
	idempotent  bool
	openWorld   bool
	usage       string // CLI message for a missing or invalid flag parameter
	cliOnly     string // reason this command is not an MCP tool
	handler     func(*call) (any, error)
	render      func(w io.Writer, result any, x *call) int // CLI output override
	lead        func(result any) string                    // MCP: a text block before the JSON
	legacy      func(args []string, stdout, stderr io.Writer) int
}

func (cmd *command) name() string { return strings.Join(cmd.path, " ") }

func (cmd *command) toolName() string {
	if cmd.tool != "" {
		return cmd.tool
	}
	return strings.ReplaceAll(strings.Join(cmd.path, "_"), "-", "_")
}

func (cmd *command) description() string {
	if cmd.desc != "" {
		return cmd.desc
	}
	return cmd.summary
}

func (cmd *command) hasParam(name string) bool {
	for _, p := range cmd.params {
		if p.name == name {
			return true
		}
	}
	return false
}

// mcpParams are the tool's inputs: the command's MCP parameters plus the
// shared computer target, the one client setting a model may choose (the
// key, host and timeout are the server's configuration, never arguments).
func (cmd *command) mcpParams() []param {
	var out []param
	for _, p := range cmd.params {
		if p.onMCP() {
			out = append(out, p)
		}
	}
	if !cmd.hasParam("computer") {
		out = append(out, sharedComputerParam)
	}
	return out
}

var sharedComputerParam = pStr("computer", "Stable ComputerID to target (sent as X-Choir-Computer); defaults to the configured computer")

// clientConfigFlags are the CLI's shared client flags that are server
// configuration on MCP. The computer flag is not among them: it is a tool
// argument (sharedComputerParam).
var clientConfigFlags = map[string]string{
	"api-key-file": "the key is the MCP server's configuration ($CHOIR_API_KEY or $CHOIR_API_KEY_FILE; hosted: the OAuth token), never a model-written argument",
	"host":         "the host is the MCP server's configuration; a model-chosen host could send the key anywhere",
	"timeout":      "the request timeout is the MCP server's configuration",
}

var (
	registryOnce sync.Once
	registryList []*command
)

// commands returns the registry, built once (lazily, so handlers may call
// printUsage, which reads the registry, without an initialization cycle).
func commands() []*command {
	registryOnce.Do(func() { registryList = buildRegistry() })
	return registryList
}

// ---- calls ----

type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

func usagef(format string, args ...any) error { return usageError{fmt.Sprintf(format, args...)} }

// fileBytes is a raw byte result: the CLI writes it as is; MCP returns text
// or a base64 blob.
type fileBytes []byte

type call struct {
	cmd     *command
	c       *client
	vals    map[string]any
	given   map[string]bool
	stdin   io.Reader // the CLI's stdin; nil on MCP
	surface string    // "cli" or "mcp"
}

func (x *call) str(name string) string    { v, _ := x.vals[name].(string); return v }
func (x *call) trim(name string) string   { return strings.TrimSpace(x.str(name)) }
func (x *call) num(name string) int64     { v, _ := x.vals[name].(int64); return v }
func (x *call) flag(name string) bool     { v, _ := x.vals[name].(bool); return v }
func (x *call) list(name string) []string { v, _ := x.vals[name].([]string); return v }
func (x *call) dur(name string) time.Duration {
	v, _ := x.vals[name].(time.Duration)
	return v
}
func (x *call) float(name string) (float64, bool) {
	v, ok := x.vals[name].(float64)
	return v, ok && x.given[name]
}
func (x *call) json(name string) json.RawMessage { v, _ := x.vals[name].(json.RawMessage); return v }

// defaultValue is a parameter's value when nothing sets it.
func (p param) defaultValue() any {
	if p.env != "" {
		def, _ := p.def.(string)
		return envOr(p.env, def)
	}
	switch p.kind {
	case kString:
		v, _ := p.def.(string)
		return v
	case kInt:
		switch v := p.def.(type) {
		case int:
			return int64(v)
		case int64:
			return v
		}
		return int64(0)
	case kNumber:
		v, _ := p.def.(float64)
		return v
	case kBool:
		v, _ := p.def.(bool)
		return v
	case kDuration:
		v, _ := p.def.(time.Duration)
		return v
	case kStrings:
		v, _ := p.def.([]string)
		return append([]string(nil), v...)
	}
	return nil
}

// validate checks one parameter's value for the surface: required, enum and
// range. It returns a short reason, or "".
func (p param) validate(v any, given bool) string {
	missing := false
	switch p.kind {
	case kString:
		missing = strings.TrimSpace(fmt.Sprint(v)) == "" || v == nil
	case kStrings:
		l, _ := v.([]string)
		missing = len(l) == 0
	case kJSON:
		raw, _ := v.(json.RawMessage)
		missing = len(bytes.TrimSpace(raw)) == 0 || string(bytes.TrimSpace(raw)) == "null"
	default:
		missing = !given
	}
	if missing {
		if p.required {
			return "required"
		}
		return ""
	}
	if s, ok := v.(string); ok && len(p.enum) > 0 && strings.TrimSpace(s) != "" {
		found := false
		for _, e := range p.enum {
			found = found || e == strings.TrimSpace(s)
		}
		if !found {
			return "must be one of " + strings.Join(p.enum, ", ")
		}
	}
	var n float64
	switch val := v.(type) {
	case int64:
		n = float64(val)
	case float64:
		n = val
	default:
		return ""
	}
	if p.min != nil && p.max != nil && (n < *p.min || n > *p.max) {
		return fmt.Sprintf("must be a number between %g and %g", *p.min, *p.max)
	}
	if p.min != nil && n < *p.min {
		return fmt.Sprintf("must be at least %g", *p.min)
	}
	if p.max != nil && n > *p.max {
		return fmt.Sprintf("must be at most %g", *p.max)
	}
	return ""
}

// ---- CLI ----

// listFlag is a repeatable string flag. A default is replaced by the first
// value set; split also accepts comma-separated values.
type listFlag struct {
	vals      []string
	split     bool
	isDefault bool
}

func (l *listFlag) String() string {
	if l == nil {
		return ""
	}
	return strings.Join(l.vals, ",")
}

func (l *listFlag) Set(v string) error {
	if l.isDefault {
		l.vals, l.isDefault = nil, false
	}
	if !l.split {
		if v = strings.TrimSpace(v); v != "" {
			l.vals = append(l.vals, v)
		}
		return nil
	}
	for _, part := range strings.Split(v, ",") {
		if part = strings.TrimSpace(part); part != "" {
			l.vals = append(l.vals, part)
		}
	}
	return nil
}

// clientFlags defines the shared client flags on fs and returns the
// function that builds the client once fs is parsed. A command that owns a
// computer flag defines it first; the client shares it.
func clientFlags(fs *flag.FlagSet) func() (*client, error) {
	apiKeyFile := fs.String("api-key-file", "", "Read API key from a mode-0600 file; '-' reads stdin; defaults to $"+apiKeyEnvVar)
	host := fs.String("host", envOr(hostEnvVar, defaultHost), "Choir host")
	if fs.Lookup("computer") == nil {
		fs.String("computer", envOr("CHOIR_COMPUTER", ""), "Stable ComputerID targeting for owner-wide API keys (sent as X-Choir-Computer)")
	}
	timeout := fs.String("timeout", "", "Request timeout (for example 75s or 2m)")
	return func() (*client, error) {
		key := strings.TrimSpace(os.Getenv(apiKeyEnvVar))
		if strings.TrimSpace(*apiKeyFile) != "" {
			var err error
			key, err = readCLISecretFile(*apiKeyFile, cliStdin)
			if err != nil {
				return nil, err
			}
		}
		if key == "" {
			return nil, fmt.Errorf("api key required: set --api-key-file or $%s", apiKeyEnvVar)
		}
		if !strings.HasPrefix(key, apiKeyPrefix) {
			return nil, fmt.Errorf("api key must start with %q", apiKeyPrefix)
		}
		h := strings.TrimRight(strings.TrimSpace(*host), "/")
		if h == "" {
			h = defaultHost
		}
		requestTimeout, err := resolveTimeout(*timeout, os.Getenv(timeoutEnvVar))
		if err != nil {
			return nil, err
		}
		return &client{
			host:       h,
			apiKey:     key,
			computerID: strings.TrimSpace(fs.Lookup("computer").Value.String()),
			http:       newHTTPClient(requestTimeout),
		}, nil
	}
}

// cliParse parses args into flags and positional arguments. Commands whose
// text is every remaining argument stop at the first positional argument
// (so the text may contain dashes); every other command accepts flags
// before and after its positional arguments.
func (cmd *command) cliParse(fs *flag.FlagSet, args []string) ([]string, error) {
	interleaved := true
	for _, p := range cmd.params {
		if p.pos == posJoin {
			interleaved = false
		}
	}
	if !interleaved {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		return fs.Args(), nil
	}
	var positional []string
	rest := args
	for {
		if err := fs.Parse(rest); err != nil {
			return nil, err
		}
		rest = fs.Args()
		if len(rest) == 0 {
			return positional, nil
		}
		positional = append(positional, rest[0])
		rest = rest[1:]
	}
}

func (cmd *command) synopsis() string {
	var parts []string
	for _, p := range cmd.params {
		if !p.onCLI() {
			continue
		}
		label := strings.ReplaceAll(p.name, "_", "-")
		switch p.pos {
		case posOne:
			parts = append(parts, "<"+label+">")
		case posJoin:
			parts = append(parts, "<"+label+"...>")
		case posRest:
			parts = append(parts, "["+label+"...]")
		}
	}
	return strings.Join(parts, " ")
}

func (cmd *command) cliHelp(w io.Writer, fs *flag.FlagSet) {
	fmt.Fprintf(w, "Usage: choir %s [flags] %s\n\n%s\n", cmd.name(), cmd.synopsis(), cmd.description())
	if len(cmd.scopes) > 0 {
		fmt.Fprintf(w, "\nScopes: %s\n", strings.Join(cmd.scopes, ", "))
	}
	if cmd.cliOnly != "" {
		fmt.Fprintf(w, "CLI only: %s\n", cmd.cliOnly)
	} else {
		fmt.Fprintf(w, "MCP tool: %s (%s)\n", cmd.toolName(), cmd.hintSummary())
	}
	fmt.Fprintln(w, "\nFlags:")
	fs.PrintDefaults()
}

func (cmd *command) hintSummary() string {
	switch {
	case cmd.readOnly:
		return "read-only"
	case cmd.destructive:
		return "destructive write"
	default:
		return "write"
	}
}

// cliFlagSet is the generated flag set for one command: its parameters'
// flags, then the shared client flags.
func (cmd *command) cliFlagSet(stderr io.Writer) (*flag.FlagSet, map[string]func() (any, bool), func() (*client, error)) {
	fs := flag.NewFlagSet("choir "+cmd.name(), flag.ContinueOnError)
	fs.SetOutput(stderr)
	getters := map[string]func() (any, bool){}
	for _, p := range cmd.params {
		if !p.onCLI() || p.cliFlag() == "" {
			continue
		}
		getters[p.name] = defineFlag(fs, p)
	}
	fs.Usage = func() { cmd.cliHelp(fs.Output(), fs) }
	return fs, getters, clientFlags(fs)
}

// runCLI is the generated CLI for one command.
func (cmd *command) runCLI(args []string, stdout, stderr io.Writer) int {
	if cmd.legacy != nil {
		return cmd.legacy(args, stdout, stderr)
	}
	label := "choir " + cmd.name()
	fs, getters, build := cmd.cliFlagSet(stderr)
	positional, err := cmd.cliParse(fs, args)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", label, err)
		return 2
	}
	c, err := build()
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", label, err)
		return 2
	}
	c.stdout, c.stderr = stdout, stderr
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	vals, given := map[string]any{}, map[string]bool{}
	for _, p := range cmd.params {
		if !p.onCLI() {
			continue
		}
		if get := getters[p.name]; get != nil {
			v, ok := get()
			if !ok {
				fmt.Fprintf(stderr, "%s: --%s %s\n", label, p.cliFlag(), p.validateKindMessage())
				return 2
			}
			vals[p.name] = v
			given[p.name] = set[p.cliFlag()] || (p.kind == kString && v != "")
			if p.kind == kNumber {
				given[p.name] = set[p.cliFlag()] && fs.Lookup(p.cliFlag()).Value.String() != ""
			}
		} else {
			vals[p.name] = p.defaultValue()
		}
	}
	// Positional arguments, in declaration order.
	for _, p := range cmd.params {
		if !p.onCLI() {
			continue
		}
		switch p.pos {
		case posOne:
			if len(positional) > 0 {
				vals[p.name], given[p.name] = positional[0], true
				positional = positional[1:]
			}
		case posJoin:
			text := strings.TrimSpace(strings.Join(positional, " "))
			positional = nil
			if strings.TrimSpace(fmt.Sprint(vals[p.name])) == "" || vals[p.name] == nil {
				vals[p.name], given[p.name] = text, text != ""
			}
		case posRest:
			l, _ := vals[p.name].([]string)
			for _, arg := range positional {
				if arg = strings.TrimSpace(arg); arg != "" {
					l = append(l, arg)
				}
			}
			positional = nil
			vals[p.name], given[p.name] = l, len(l) > 0
		}
	}
	// Positional failures first (as the legacy commands checked them), then
	// flags.
	for _, positionalPass := range []bool{true, false} {
		for _, p := range cmd.params {
			if !p.onCLI() || (p.pos != posNone && p.cliFlag() == "") != positionalPass {
				continue
			}
			reason := p.validate(vals[p.name], given[p.name])
			if reason == "" {
				continue
			}
			msg := ""
			switch {
			case p.missing != "":
				msg = p.missing
			case positionalPass:
				msg = strings.ReplaceAll(p.name, "_", " ") + " " + reason
			case cmd.usage != "":
				msg = cmd.usage
			default:
				msg = "--" + p.cliFlag()
				if p.env != "" {
					msg += " (or $" + p.env + ")"
				}
				msg += " " + reasonIs(reason)
			}
			fmt.Fprintf(stderr, "%s: %s\n", label, msg)
			return 2
		}
	}
	if len(positional) > 0 {
		fmt.Fprintf(stderr, "%s: unexpected positional arguments %q\n", label, positional)
		return 2
	}
	x := &call{cmd: cmd, c: c, vals: vals, given: given, stdin: cliStdin, surface: "cli"}
	result, err := cmd.handler(x)
	if err != nil {
		var usage usageError
		if id := cmd.singleID(x); id != "" && !errors.As(err, &usage) {
			label += " " + id // as the legacy commands named the object: "choir run status run-1: ..."
		}
		fmt.Fprintf(stderr, "%s: %v\n", label, err)
		if errors.As(err, &usage) {
			return 2
		}
		return 1
	}
	if cmd.render != nil {
		return cmd.render(stdout, result, x)
	}
	switch v := result.(type) {
	case nil:
		return 0
	case fileBytes:
		if _, err := stdout.Write(v); err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", label, err)
			return 1
		}
		return 0
	}
	return writeJSON(stdout, result)
}

// singleID is the value of a command's only positional parameter, when it
// has exactly one single-value positional parameter.
func (cmd *command) singleID(x *call) string {
	id, n := "", 0
	for _, p := range cmd.params {
		switch p.pos {
		case posOne:
			id, n = x.trim(p.name), n+1
		case posJoin, posRest:
			return ""
		}
	}
	if n != 1 {
		return ""
	}
	return id
}

func reasonIs(reason string) string {
	if reason == "required" {
		return "is required"
	}
	return reason
}

func (p param) validateKindMessage() string {
	if p.kind == kNumber {
		if p.min != nil && p.max != nil {
			return fmt.Sprintf("must be a number between %g and %g", *p.min, *p.max)
		}
		return "must be a number"
	}
	if p.kind == kJSON {
		return "must be JSON"
	}
	return "is invalid"
}

// defineFlag registers one parameter's flag and returns its reader; the
// reader's false means the value does not parse.
func defineFlag(fs *flag.FlagSet, p param) func() (any, bool) {
	name := p.cliFlag()
	switch p.kind {
	case kInt:
		v := fs.Int64(name, p.defaultValue().(int64), p.desc)
		return func() (any, bool) { return *v, true }
	case kBool:
		v := fs.Bool(name, p.defaultValue().(bool), p.desc)
		return func() (any, bool) { return *v, true }
	case kDuration:
		v := fs.Duration(name, p.defaultValue().(time.Duration), p.desc)
		return func() (any, bool) { return *v, true }
	case kStrings:
		l := &listFlag{split: p.split}
		if def := p.defaultValue().([]string); len(def) > 0 {
			l.vals, l.isDefault = def, true
		}
		fs.Var(l, name, p.desc)
		return func() (any, bool) { return append([]string(nil), l.vals...), true }
	case kNumber:
		v := fs.String(name, "", p.desc)
		return func() (any, bool) {
			raw := strings.TrimSpace(*v)
			if raw == "" {
				return p.defaultValue(), true
			}
			n, err := strconv.ParseFloat(raw, 64)
			if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
				return nil, false
			}
			return n, true
		}
	case kJSON:
		v := fs.String(name, "", p.desc)
		return func() (any, bool) {
			raw := strings.TrimSpace(*v)
			if raw == "" {
				return json.RawMessage(nil), true
			}
			if !json.Valid([]byte(raw)) {
				return nil, false
			}
			return json.RawMessage(raw), true
		}
	default:
		def, _ := p.defaultValue().(string)
		v := fs.String(name, def, p.desc)
		return func() (any, bool) { return *v, true }
	}
}

// ---- dispatch and usage ----

// resolve finds the longest registered command path that prefixes args.
// With no command, group is the longest registered group prefix.
func resolve(args []string) (cmd *command, rest []string, group []string) {
	best := -1
	for _, c := range commands() {
		if len(c.path) > len(args) || len(c.path) <= best {
			continue
		}
		match := true
		for i, part := range c.path {
			match = match && args[i] == part
		}
		if match {
			cmd, best = c, len(c.path)
		}
	}
	if cmd != nil {
		return cmd, args[best:], nil
	}
	for depth := len(args); depth > 0; depth-- {
		for _, c := range commands() {
			if len(c.path) > depth && strings.Join(c.path[:depth], " ") == strings.Join(args[:depth], " ") {
				return nil, args[depth:], args[:depth]
			}
		}
	}
	return nil, args, nil
}

// subcommands lists the next path element under a group, in registry order.
func subcommands(group []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, c := range commands() {
		if len(c.path) <= len(group) || strings.Join(c.path[:len(group)], " ") != strings.Join(group, " ") {
			continue
		}
		if next := c.path[len(group)]; !seen[next] {
			seen[next] = true
			out = append(out, next)
		}
	}
	return out
}

func dispatch(args []string, stdout, stderr io.Writer) int {
	cmd, rest, group := resolve(args)
	if cmd != nil {
		return cmd.runCLI(rest, stdout, stderr)
	}
	if group == nil {
		fmt.Fprintf(stderr, "choir: unknown command %q\n", args[0])
		printUsage(stderr)
		return 2
	}
	label := "choir " + strings.Join(group, " ")
	subs := strings.Join(subcommands(group), "|")
	if len(rest) == 0 {
		fmt.Fprintf(stderr, "%s: subcommand required (%s)\n", label, subs)
		return 2
	}
	switch rest[0] {
	case "help", "-h", "--help":
		printGroupUsage(stdout, group)
		return 0
	}
	fmt.Fprintf(stderr, "%s: unknown subcommand %q (%s)\n", label, rest[0], subs)
	return 2
}

func usageLine(c *command) (string, string) {
	left := c.name()
	if s := c.synopsis(); s != "" {
		left += " " + s
	}
	return left, c.summary
}

func printCommandLines(w io.Writer, cmds []*command) {
	width := 0
	for _, c := range cmds {
		if left, _ := usageLine(c); len(left) > width {
			width = len(left)
		}
	}
	if width > 44 {
		width = 44
	}
	for _, c := range cmds {
		left, summary := usageLine(c)
		if len(left) > width {
			fmt.Fprintf(w, "  %s\n  %-*s  %s\n", left, width, "", summary)
			continue
		}
		fmt.Fprintf(w, "  %-*s  %s\n", width, left, summary)
	}
}

func printGroupUsage(w io.Writer, group []string) {
	var cmds []*command
	for _, c := range commands() {
		if len(c.path) > len(group) && strings.Join(c.path[:len(group)], " ") == strings.Join(group, " ") {
			cmds = append(cmds, c)
		}
	}
	fmt.Fprintf(w, "Usage: choir %s <subcommand> [flags]\n\n", strings.Join(group, " "))
	printCommandLines(w, cmds)
	fmt.Fprintln(w, "\nRun choir <command> --help for a command's flags, scopes and MCP tool.")
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `choir — headless Choir control surface (the same commands are MCP tools: choir mcp)

Usage:
  choir <command> [flags]

Commands:`)
	printCommandLines(w, commands())
	fmt.Fprintln(w, `
Auth:
  --api-key-file path  Read API key from a mode-0600 file; "-" reads stdin.
                       Defaults to $CHOIR_API_KEY when omitted.
  --host string        Choir host. Defaults to $CHOIR_HOST or https://choir.news.
  --computer id        Target computer for owner-wide keys. Defaults to $CHOIR_COMPUTER.
  --timeout duration   Request timeout. Defaults to $CHOIR_TIMEOUT or 75s.

Run choir <command> --help for a command's flags, scopes and MCP tool.
Output is JSON to stdout; diagnostics and errors go to stderr.`)
}

// ---- MCP ----

type mcpAnnotations struct {
	Title           string `json:"title,omitempty"`
	ReadOnlyHint    bool   `json:"readOnlyHint"`
	DestructiveHint bool   `json:"destructiveHint"`
	IdempotentHint  bool   `json:"idempotentHint"`
	OpenWorldHint   bool   `json:"openWorldHint"`
}

type mcpTool struct {
	Name        string         `json:"name"`
	Title       string         `json:"title,omitempty"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	Annotations mcpAnnotations `json:"annotations"`
	Meta        map[string]any `json:"_meta,omitempty"`
}

func (p param) schema() map[string]any {
	s := map[string]any{}
	switch p.kind {
	case kString:
		s["type"] = "string"
	case kInt:
		s["type"] = "integer"
	case kNumber:
		s["type"] = "number"
	case kBool:
		s["type"] = "boolean"
	case kDuration:
		s["type"] = "string"
		s["pattern"] = `^([0-9]+(\.[0-9]+)?(ns|us|µs|ms|s|m|h))+$`
	case kStrings:
		s["type"] = "array"
		s["items"] = map[string]any{"type": "string"}
		if p.required {
			s["minItems"] = 1
		}
	case kJSON:
		s["type"] = "object"
	}
	desc := p.desc
	if p.env != "" {
		desc += " (defaults to the server's $" + p.env + ")"
	}
	if p.kind == kDuration {
		desc += " (a Go duration such as 15m or 90s)"
	}
	s["description"] = desc
	if len(p.enum) > 0 {
		s["enum"] = p.enum
	}
	if p.min != nil {
		s["minimum"] = *p.min
	}
	if p.max != nil {
		s["maximum"] = *p.max
	}
	if p.env == "" && p.def != nil {
		switch d := p.def.(type) {
		case time.Duration:
			s["default"] = d.String()
		default:
			s["default"] = d
		}
	}
	return s
}

func (cmd *command) inputSchema() map[string]any {
	props := map[string]any{}
	var required []string
	for _, p := range cmd.mcpParams() {
		props[p.name] = p.schema()
		if p.required {
			required = append(required, p.name)
		}
	}
	s := map[string]any{"type": "object", "properties": props, "additionalProperties": false}
	if len(required) > 0 {
		sort.Strings(required)
		s["required"] = required
	}
	return s
}

func (cmd *command) mcpTool() mcpTool {
	ann := mcpAnnotations{
		Title:           "choir " + cmd.name(),
		ReadOnlyHint:    cmd.readOnly,
		DestructiveHint: !cmd.readOnly && cmd.destructive,
		IdempotentHint:  cmd.readOnly || cmd.idempotent,
		OpenWorldHint:   cmd.openWorld,
	}
	return mcpTool{
		Name:        cmd.toolName(),
		Title:       cmd.summary,
		Description: cmd.description(),
		InputSchema: cmd.inputSchema(),
		Annotations: ann,
		Meta: map[string]any{
			"choir.news/scopes": cmd.scopes,
			"choir.news/cli":    "choir " + cmd.name(),
		},
	}
}

// mcpCommands are the commands that are tools, in registry order.
func mcpCommands() []*command {
	var out []*command
	for _, c := range commands() {
		if c.cliOnly == "" {
			out = append(out, c)
		}
	}
	return out
}

func mcpCommand(tool string) *command {
	for _, c := range mcpCommands() {
		if c.toolName() == tool {
			return c
		}
	}
	return nil
}

// mcpArgs decodes and validates tool arguments against the command's
// parameters: unknown names, wrong types, missing required values, enums
// and ranges are refused before any request is made.
func (cmd *command) mcpArgs(raw json.RawMessage) (map[string]any, map[string]bool, error) {
	in := map[string]json.RawMessage{}
	if trimmed := bytes.TrimSpace(raw); len(trimmed) > 0 && string(trimmed) != "null" {
		dec := json.NewDecoder(bytes.NewReader(trimmed))
		dec.UseNumber()
		if err := dec.Decode(&in); err != nil {
			return nil, nil, fmt.Errorf("invalid arguments: %w", err)
		}
	}
	params := cmd.mcpParams()
	known := map[string]bool{}
	for _, p := range params {
		known[p.name] = true
	}
	var unknown []string
	for name := range in {
		if !known[name] {
			unknown = append(unknown, name)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return nil, nil, fmt.Errorf("invalid arguments: unknown argument %s", strings.Join(unknown, ", "))
	}
	vals, given := map[string]any{}, map[string]bool{}
	for _, p := range params {
		vals[p.name] = p.defaultValue()
		rawVal, ok := in[p.name]
		if ok && string(bytes.TrimSpace(rawVal)) != "null" {
			v, err := p.decodeMCP(rawVal)
			if err != nil {
				return nil, nil, fmt.Errorf("invalid arguments: %s: %w", p.name, err)
			}
			vals[p.name], given[p.name] = v, true
		}
		if reason := p.validate(vals[p.name], given[p.name]); reason != "" {
			msg := p.name + " " + reasonIs(reason)
			if reason == "required" && p.env != "" {
				msg += " (or set $" + p.env + " in the MCP server's environment)"
			}
			return nil, nil, errors.New(msg)
		}
	}
	return vals, given, nil
}

func (p param) decodeMCP(raw json.RawMessage) (any, error) {
	switch p.kind {
	case kString:
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, errors.New("must be a string")
		}
		return s, nil
	case kInt:
		var n json.Number
		if t := bytes.TrimSpace(raw); len(t) > 0 && t[0] == '"' {
			return nil, errors.New("must be an integer")
		}
		if err := json.Unmarshal(raw, &n); err != nil {
			return nil, errors.New("must be an integer")
		}
		v, err := n.Int64()
		if err != nil {
			return nil, errors.New("must be an integer")
		}
		return v, nil
	case kNumber:
		var n float64
		if err := json.Unmarshal(raw, &n); err != nil {
			return nil, errors.New("must be a number")
		}
		return n, nil
	case kBool:
		var b bool
		if err := json.Unmarshal(raw, &b); err != nil {
			return nil, errors.New("must be a boolean")
		}
		return b, nil
	case kDuration:
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, errors.New("must be a duration string such as 15m")
		}
		d, err := time.ParseDuration(strings.TrimSpace(s))
		if err != nil {
			return nil, errors.New("must be a duration string such as 15m")
		}
		return d, nil
	case kStrings:
		var l []string
		if err := json.Unmarshal(raw, &l); err != nil {
			return nil, errors.New("must be an array of strings")
		}
		var out []string
		for _, s := range l {
			if s = strings.TrimSpace(s); s != "" {
				out = append(out, s)
			}
		}
		return out, nil
	case kJSON:
		if t := bytes.TrimSpace(raw); len(t) == 0 || t[0] != '{' {
			return nil, errors.New("must be an object")
		}
		return json.RawMessage(bytes.TrimSpace(raw)), nil
	}
	return nil, errors.New("unsupported parameter kind")
}

type mcpResource struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"`
}

type mcpContent struct {
	Type     string       `json:"type"`
	Text     string       `json:"text,omitempty"`
	Resource *mcpResource `json:"resource,omitempty"`
}

type mcpToolResult struct {
	Content           []mcpContent    `json:"content"`
	StructuredContent json.RawMessage `json:"structuredContent,omitempty"`
	IsError           bool            `json:"isError,omitempty"`
}

func toolError(err error) mcpToolResult {
	return mcpToolResult{Content: []mcpContent{{Type: "text", Text: err.Error()}}, IsError: true}
}

// mcpResult turns a handler's output into a tool result: pretty JSON text,
// and the same JSON as structuredContent when it is an object.
func (cmd *command) mcpResult(result any, x *call) mcpToolResult {
	if b, ok := result.(fileBytes); ok {
		uri := "choir://files/" + strings.TrimPrefix(x.trim("path"), "/")
		if utf8.Valid(b) {
			return mcpToolResult{Content: []mcpContent{{Type: "text", Text: string(b)}}}
		}
		return mcpToolResult{Content: []mcpContent{{Type: "resource", Resource: &mcpResource{
			URI: uri, MimeType: "application/octet-stream", Blob: encodeBase64(b),
		}}}}
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return toolError(fmt.Errorf("encode result: %w", err))
	}
	var pretty bytes.Buffer
	if json.Indent(&pretty, raw, "", "  ") != nil {
		pretty.Reset()
		pretty.Write(raw)
	}
	out := mcpToolResult{Content: []mcpContent{{Type: "text", Text: pretty.String()}}}
	if t := bytes.TrimSpace(raw); len(t) > 0 && t[0] == '{' {
		out.StructuredContent = raw
	}
	if cmd.lead != nil {
		if text := cmd.lead(result); text != "" {
			out.Content = append([]mcpContent{{Type: "text", Text: text}}, out.Content...)
		}
	}
	return out
}
