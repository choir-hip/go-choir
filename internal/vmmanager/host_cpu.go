package vmmanager

// The host CPU fingerprint (D2 slice S5, item 2; design 1.2 and S5). A
// snapshot's guest memory and Firecracker's saved vCPU state assume the CPUID
// the capturing host exposed. A microcode or BIOS change without a kernel
// change can alter it under a resumed guest, and nothing guarded that: the
// host kernel string does not move. The fingerprint is a sha256 over the
// sorted, de-duplicated set of (vendor_id, model name, flags, microcode) of
// EVERY processor in /proc/cpuinfo, so a change on any single core moves it.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"strings"
)

// cpuInfoPath is replaced by tests.
var cpuInfoPath = "/proc/cpuinfo"

// readHostCPU is the fingerprint reader; tests replace it.
var readHostCPU = hostCPUFingerprint

const (
	snapshotReasonHostCPUMismatch = "host_cpu_mismatch"
	snapshotReasonHostCPUUnknown  = "host_cpu_unknown"
)

// hostCPUFingerprint hashes every processor block of /proc/cpuinfo. A file
// that cannot be read, or has no processor, is an error, never an empty
// fingerprint.
func hostCPUFingerprint() (string, error) {
	data, err := os.ReadFile(cpuInfoPath)
	if err != nil {
		return "", fmt.Errorf("host cpu fingerprint: %w", err)
	}
	set := map[string]struct{}{}
	for _, block := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n\n") {
		fields := map[string]string{}
		for _, line := range strings.Split(block, "\n") {
			key, value, ok := strings.Cut(line, ":")
			if !ok {
				continue
			}
			fields[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
		if _, isProcessor := fields["processor"]; !isProcessor {
			continue
		}
		set[strings.Join([]string{fields["vendor_id"], fields["model name"], fields["flags"], fields["microcode"]}, "|")] = struct{}{}
	}
	if len(set) == 0 {
		return "", fmt.Errorf("host cpu fingerprint: %s lists no processor", cpuInfoPath)
	}
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	sum := sha256.Sum256([]byte(strings.Join(keys, "\n")))
	return hex.EncodeToString(sum[:]), nil
}

// cpuRefusal applies the S5 rules to a meta: a differing fingerprint refuses
// (both set); a release-bound meta with no fingerprint, or a current one that
// cannot be read, refuses as unknown. A legacy meta with an empty value is
// allowed, as before, so S5 voids no existing snapshot.
func cpuRefusal(meta *SnapshotMeta) string {
	cur, err := readHostCPU()
	if err != nil {
		cur = ""
	}
	if meta.GuestRelease != "" && (meta.HostCPU == "" || cur == "") {
		return snapshotReasonHostCPUUnknown
	}
	if meta.HostCPU != "" && cur != "" && meta.HostCPU != cur {
		return snapshotReasonHostCPUMismatch
	}
	return ""
}
