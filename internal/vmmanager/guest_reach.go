// Guest-reach decisions of this package, moved here whole from manager.go so the
// security-epoch gate (nix/guest-security-surfaces.txt) sees a change to them. A
// change to this file changes what a guest computer may reach on the host.
// Keep it to those decisions; put anything else in another file.

package vmmanager

func tapReachableHostServicePorts() []string {
	return []string{
		"8082", // proxy / platform publish path
		"8083", // vmctl
		"8084", // gateway
		// :8085 removed 2026-10-04 (S1a): no production host autoputer serves
		// it; the rule only let guests reach a dead port.
		"8086", // corpusd durable Texture verification
		"8087", // maild draft persistence
		// Go module proxy for L2 capsules' goproxy.choir.internal route
		// (docs/reports/capsule-go-module-proxy-design-2026-10-14.md).
		"8088",
		// Package mirror for L2 capsules' mirror.choir.internal route
		// (station FP10, docs/definitions/choir-launch-fp10-net02-mirrors-2026-10-21.md).
		"8089",
	}
}
