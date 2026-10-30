package capsule

import (
	"strings"

	"github.com/yusefmosiah/go-choir/internal/visualcapture"
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

// captureBrokerEnv is what the executor adds to a new broker's environment:
// the Chromium and fontconfig paths when the host granted visual capture to
// this computer and the image names an executable Chromium, and nothing
// otherwise (V1). The broker hands them to jobs alone (CaptureJobEnv).
func captureBrokerEnv(grantDir, browser, fontconfig string) []string {
	st := visualcapture.ReadGrant(grantDir, browser)
	if !st.Granted {
		return nil
	}
	env := []string{visualcapture.BrowserEnv + "=" + st.Browser}
	if fontconfig = strings.TrimSpace(fontconfig); fontconfig != "" {
		env = append(env, visualcapture.FontconfigEnv+"="+fontconfig)
	}
	return env
}

// CaptureJobEnv is what a broker adds to its jobs' environment for visual
// capture: the Chromium and fontconfig paths the executor gave it, and only
// those, when it gave them (a granted computer;
// docs/design/visual-evidence-slice-2026-10-15.md V1). Cells and direct exec
// build their environments without them.
func CaptureJobEnv(lookup func(string) (string, bool)) []string {
	var env []string
	for _, key := range []string{visualcapture.BrowserEnv, visualcapture.FontconfigEnv} {
		if v, ok := lookup(key); ok && strings.TrimSpace(v) != "" && !strings.ContainsAny(v, "\x00\n") {
			env = append(env, key+"="+strings.TrimSpace(v))
		}
	}
	return env
}
