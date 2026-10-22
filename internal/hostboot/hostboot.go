// Package hostboot reports whether the host runs what it last activated:
// the installed-but-not-booted detector
// (docs/reports/patch-lanes-plan-2026-10-15.md, slice 1;
// docs/reports/patch-pipeline-time-to-patch-2026-10-15.md, section 4.2).
//
// NixOS keeps the facts as files. /run/booted-system is the system the host
// booted; /run/current-system is the one last activated. A deploy moves the
// second and never the first, so a kernel, its modules, its command line and
// the KVM module parameters can be installed for months without running.
//
// The same contract is implemented in Python by
// scripts/verify-host-hardening.py (--boot-state-json), which runs over ssh
// as `python3 -` and cannot import this package. TestPythonParity runs both on
// the same fixture trees.
package hostboot

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// SchemaVersion is the version of the State JSON shape.
const SchemaVersion = 1

// OverdueAfter is the staleness floor for a pending reboot (patch pipeline
// report, section 7.2: reboot_pending never older than 7 days).
const OverdueAfter = 7 * 24 * time.Hour

// Reason names, stable for consumers.
const (
	ReasonKernel        = "kernel"
	ReasonKernelModules = "kernel_modules"
	ReasonKernelParams  = "kernel_params"
	ReasonModuleParam   = "module_param"
)

// ModuleParam is one declared KVM module parameter and its live value.
type ModuleParam struct {
	Module   string `json:"module"`
	Param    string `json:"param"`
	Declared string `json:"declared"`
	Source   string `json:"source"` // "kernel-params" or "modprobe.d"
	Live     string `json:"live,omitempty"`
	Loaded   bool   `json:"loaded"`
	Match    bool   `json:"match"`
}

// State is the boot-state fact. Reasons is empty when nothing is pending.
type State struct {
	SchemaVersion        int           `json:"schema_version"`
	CheckedAt            string        `json:"checked_at"`
	Available            bool          `json:"available"`
	Error                string        `json:"error,omitempty"`
	RunningKernelRelease string        `json:"running_kernel_release,omitempty"`
	BootedAt             string        `json:"booted_at,omitempty"`
	BootedSystem         string        `json:"booted_system,omitempty"`
	CurrentSystem        string        `json:"current_system,omitempty"`
	BootedKernel         string        `json:"booted_kernel,omitempty"`
	CurrentKernel        string        `json:"current_kernel,omitempty"`
	BootedKernelModules  string        `json:"booted_kernel_modules,omitempty"`
	CurrentKernelModules string        `json:"current_kernel_modules,omitempty"`
	ParamsNotOnCmdline   []string      `json:"kernel_params_not_on_cmdline"`
	ModuleParams         []ModuleParam `json:"module_params"`
	RebootPending        bool          `json:"reboot_pending"`
	Reasons              []string      `json:"reasons"`
	PendingSince         string        `json:"pending_since,omitempty"`
	PendingOverdue       bool          `json:"pending_overdue"`
}

// Read computes the State from the filesystem under root ("/" on a host).
func Read(root string, now time.Time) State {
	st := State{
		SchemaVersion:      SchemaVersion,
		CheckedAt:          now.UTC().Format(time.RFC3339),
		ParamsNotOnCmdline: []string{},
		ModuleParams:       []ModuleParam{},
		Reasons:            []string{},
	}
	at := func(p string) string { return filepath.Join(root, p) }

	st.RunningKernelRelease = readTrim(at("/proc/sys/kernel/osrelease"))
	if bt := bootTime(at("/proc/stat")); !bt.IsZero() {
		st.BootedAt = bt.UTC().Format(time.RFC3339)
	}

	// The error text is part of the contract with the Python twin; the cause
	// (absent on a non-NixOS host, unreadable in a sandbox) is not.
	booted := resolve(at("/run/booted-system"))
	if booted == "" {
		st.Error = "booted system unreadable"
		return st
	}
	current := resolve(at("/run/current-system"))
	if current == "" {
		st.Error = "current system unreadable"
		return st
	}
	st.Available = true
	st.BootedSystem, st.CurrentSystem = booted, current
	bootedIn, currentIn := readInputs(booted), readInputs(current)
	st.BootedKernel, st.CurrentKernel = bootedIn.kernel, currentIn.kernel
	st.BootedKernelModules, st.CurrentKernelModules = bootedIn.modules, currentIn.modules

	if bootedIn.kernel != currentIn.kernel {
		st.Reasons = append(st.Reasons, ReasonKernel)
	}
	if bootedIn.modules != currentIn.modules {
		st.Reasons = append(st.Reasons, ReasonKernelModules)
	}

	cmdline := map[string]bool{}
	for _, tok := range strings.Fields(readTrim(at("/proc/cmdline"))) {
		cmdline[tok] = true
	}
	for _, p := range strings.Fields(currentIn.params) {
		if !cmdline[p] {
			st.ParamsNotOnCmdline = append(st.ParamsNotOnCmdline, p)
		}
	}
	if len(st.ParamsNotOnCmdline) > 0 {
		st.Reasons = append(st.Reasons, ReasonKernelParams)
	}

	moduleMismatch := false
	for _, d := range declaredModuleParams(currentIn.params, current) {
		dir := at("/sys/module/" + d.Module)
		if _, err := os.Stat(dir); err == nil {
			d.Loaded = true
			d.Live = readTrim(filepath.Join(dir, "parameters", d.Param))
			d.Match = sameParam(d.Declared, d.Live)
			if !d.Match {
				moduleMismatch = true
			}
		} else {
			// Not loaded: the parameter applies when the module loads.
			d.Match = true
		}
		st.ModuleParams = append(st.ModuleParams, d)
	}
	if moduleMismatch {
		st.Reasons = append(st.Reasons, ReasonModuleParam)
	}

	st.RebootPending = len(st.Reasons) > 0
	if st.RebootPending {
		if since, ok := pendingSince(at("/nix/var/nix/profiles"), booted, bootedIn); ok {
			st.PendingSince = since.UTC().Format(time.RFC3339)
			st.PendingOverdue = now.Sub(since) > OverdueAfter
		}
	}
	return st
}

// bootInputs are the parts of a system that take effect only at boot.
type bootInputs struct {
	kernel, modules, params, modprobe string
}

func readInputs(system string) bootInputs {
	return bootInputs{
		kernel:   resolve(filepath.Join(system, "kernel")),
		modules:  resolve(filepath.Join(system, "kernel-modules")),
		params:   readTrim(filepath.Join(system, "kernel-params")),
		modprobe: readModprobe(system),
	}
}

// sameBoot reports whether two systems boot the same way. Whitespace in the
// parameter and modprobe text is not significant.
func (b bootInputs) sameBoot(o bootInputs) bool {
	return b.kernel == o.kernel && b.modules == o.modules &&
		strings.Join(strings.Fields(b.params), " ") == strings.Join(strings.Fields(o.params), " ") &&
		strings.Join(strings.Fields(b.modprobe), " ") == strings.Join(strings.Fields(o.modprobe), " ")
}

func readModprobe(system string) string {
	dir := filepath.Join(system, "etc", "modprobe.d")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".conf") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	var b strings.Builder
	for _, n := range names {
		b.WriteString(readTrim(filepath.Join(dir, n)))
		b.WriteString("\n")
	}
	return b.String()
}

var kernelParamRE = regexp.MustCompile(`^(kvm[A-Za-z0-9_-]*)\.([A-Za-z0-9_]+)=(.*)$`)

// declaredModuleParams lists KVM module parameters the current system
// declares, from its kernel parameters and its modprobe options. A later
// declaration of the same module parameter wins, kernel parameters last (the
// kernel command line overrides modprobe options).
func declaredModuleParams(params, system string) []ModuleParam {
	var out []ModuleParam
	index := map[string]int{}
	add := func(mod, param, value, source string) {
		mod = strings.ReplaceAll(mod, "-", "_")
		key := mod + "." + param
		mp := ModuleParam{Module: mod, Param: param, Declared: value, Source: source}
		if i, ok := index[key]; ok {
			out[i] = mp
			return
		}
		index[key] = len(out)
		out = append(out, mp)
	}
	for _, line := range strings.Split(readModprobe(system), "\n") {
		f := strings.Fields(line)
		if len(f) < 3 || f[0] != "options" || !strings.HasPrefix(f[1], "kvm") {
			continue
		}
		for _, kv := range f[2:] {
			if k, v, ok := strings.Cut(kv, "="); ok {
				add(f[1], k, v, "modprobe.d")
			}
		}
	}
	for _, tok := range strings.Fields(params) {
		if m := kernelParamRE.FindStringSubmatch(tok); m != nil {
			add(m[1], m[2], m[3], "kernel-params")
		}
	}
	return out
}

// sameParam compares a declared value with a sysfs value. Boolean module
// parameters read back as Y or N whatever was written.
func sameParam(declared, live string) bool {
	if live == "Y" || live == "N" {
		switch strings.ToLower(declared) {
		case "1", "y", "yes", "true", "on":
			return live == "Y"
		case "0", "n", "no", "false", "off":
			return live == "N"
		}
	}
	return declared == live
}

// pendingSince is the activation time of the first system generation after
// the booted one whose boot inputs differ from the booted system's. When the
// booted generation is no longer in the profile, it is the first retained
// generation that differs.
func pendingSince(profiles, booted string, bootedIn bootInputs) (time.Time, bool) {
	entries, err := os.ReadDir(profiles)
	if err != nil {
		return time.Time{}, false
	}
	type gen struct {
		n    int
		link string
	}
	var gens []gen
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "system-") || !strings.HasSuffix(name, "-link") {
			continue
		}
		n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(name, "system-"), "-link"))
		if err != nil {
			continue
		}
		gens = append(gens, gen{n, filepath.Join(profiles, name)})
	}
	sort.Slice(gens, func(i, j int) bool { return gens[i].n < gens[j].n })
	start := 0
	for i, g := range gens {
		if resolve(g.link) == booted {
			start = i + 1
		}
	}
	for _, g := range gens[start:] {
		target := resolve(g.link)
		if target == "" || readInputs(target).sameBoot(bootedIn) {
			continue
		}
		fi, err := os.Lstat(g.link)
		if err != nil {
			return time.Time{}, false
		}
		return fi.ModTime(), true
	}
	return time.Time{}, false
}

func bootTime(statPath string) time.Time {
	for _, line := range strings.Split(readTrim(statPath), "\n") {
		if rest, ok := strings.CutPrefix(line, "btime "); ok {
			if s, err := strconv.ParseInt(strings.TrimSpace(rest), 10, 64); err == nil {
				return time.Unix(s, 0)
			}
		}
	}
	return time.Time{}
}

func resolve(p string) string {
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		return ""
	}
	return r
}

func readTrim(p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// kernelVersionRE finds the version in a NixOS kernel store name:
// <hash>-linux-6.18.37, <hash>-linux-hardened-6.12.9, <hash>-linux-6.18.37-modules.
var kernelVersionRE = regexp.MustCompile(`-linux(?:-[a-z]+)*-([0-9]+\.[0-9]+(?:\.[0-9]+)?(?:-rc[0-9]+)?)(?:[-/]|$)`)

// KernelVersion is the kernel version named by a store path, or "" when the
// path names none.
func KernelVersion(path string) string {
	if m := kernelVersionRE.FindStringSubmatch(path); m != nil {
		return m[1]
	}
	return ""
}

// LogLine is the state as one journal line: the alarm a host writes without
// anyone asking (docs/problems/host-kernel-216-days-unpatched-2026-10-15.md,
// 2026-10-22). A pending reboot starts with WARNING, an overdue one says
// OVERDUE.
func (s State) LogLine() string {
	if !s.Available {
		return "host boot state unavailable: " + s.Error
	}
	orUnknown := func(v string) string {
		if v == "" {
			return "unknown"
		}
		return v
	}
	head := "host boot state: booted is activated"
	if s.RebootPending {
		head = "WARNING host reboot pending: booted system older than the activated one"
		if s.PendingOverdue {
			head = "WARNING host reboot pending, OVERDUE (over 7 days): booted system older than the activated one"
		}
	}
	fields := []string{
		head,
		"booted_system=" + orUnknown(s.BootedSystem),
		"current_system=" + orUnknown(s.CurrentSystem),
		"booted_kernel=" + orUnknown(KernelVersion(s.BootedKernel)),
		"current_kernel=" + orUnknown(KernelVersion(s.CurrentKernel)),
		"running_kernel=" + orUnknown(s.RunningKernelRelease),
		"booted_at=" + orUnknown(s.BootedAt),
	}
	if s.RebootPending {
		fields = append(fields,
			"reasons="+strings.Join(s.Reasons, ","),
			"pending_since="+orUnknown(s.PendingSince),
			"overdue="+strconv.FormatBool(s.PendingOverdue))
	}
	return strings.Join(fields, " ")
}
