package capsule

import (
	"os"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// BrokerProcessEnv is the environment the executor launches a capsule broker
// with, read from the autoputer's own environment through lookup: the
// autoputer's PATH behind the system profile (so exec.LookPath in the broker
// resolves `sh` as the service PATH intends), HOME and TMPDIR, and the build
// toolchain carry (yaegikernel.BuildToolchainCarry) the broker hands its
// workloads (docs/archive/problems/selfdev-drill-capsule-cannot-build-autoputer-2026-10-14.md,
// option B). Nothing else crosses.
func BrokerProcessEnv(lookup func(string) (string, bool)) []string {
	brokerPath, _ := lookup("PATH")
	if strings.TrimSpace(brokerPath) == "" {
		brokerPath = "/run/current-system/sw/bin:/bin:/usr/bin"
	} else if !strings.Contains(brokerPath, "/run/current-system/sw/bin") {
		brokerPath = "/run/current-system/sw/bin:" + brokerPath
	}
	env := []string{"PATH=" + brokerPath + ":/bin:/usr/bin", "HOME=/root", "TMPDIR=/tmp"}
	return append(env, yaegikernel.BuildToolchainCarry(lookup)...)
}

// The store Chromium (docs/problems/selfdev-screens-are-not-files-2026-11-03.md):
// every computer's image carries it on no PATH, and an engineer's Playwright
// jobs launch it by path. The executor gives each broker the path and the
// fontconfig file (so text renders without /etc/fonts); the broker hands them
// to jobs. No grant: the browser is not a privilege.
const (
	// BrowserEnv names the store Chromium's binary in a job's environment.
	BrowserEnv = "CHOIR_BROWSER"
	// FontconfigEnv is fontconfig's own variable, so Chromium reads it as is.
	FontconfigEnv = "FONTCONFIG_FILE"
)

// browserBrokerEnv is what the executor adds to a new broker's environment:
// the Chromium and fontconfig paths when the image names an executable
// Chromium, and nothing otherwise.
func browserBrokerEnv(browser, fontconfig string) []string {
	browser = strings.TrimSpace(browser)
	if browser == "" {
		return nil
	}
	info, err := os.Stat(browser)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return nil
	}
	env := []string{BrowserEnv + "=" + browser}
	if fontconfig = strings.TrimSpace(fontconfig); fontconfig != "" {
		env = append(env, FontconfigEnv+"="+fontconfig)
	}
	return env
}

// BrowserJobEnv is what a broker adds to its jobs' environment: the Chromium
// and fontconfig paths the executor gave it, and only those. Cells and direct
// exec build their environments without them.
func BrowserJobEnv(lookup func(string) (string, bool)) []string {
	var env []string
	for _, key := range []string{BrowserEnv, FontconfigEnv} {
		if v, ok := lookup(key); ok && strings.TrimSpace(v) != "" && !strings.ContainsAny(v, "\x00\n") {
			env = append(env, key+"="+strings.TrimSpace(v))
		}
	}
	return env
}
