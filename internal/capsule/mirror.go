package capsule

import (
	"fmt"
	"net/netip"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/mirrorgrant"
)

// Capsule package mirrors, guest side (station FP10,
// docs/definitions/choir-launch-fp10-net02-mirrors-2026-10-21.md). On a
// computer the host granted (mirrorgrant, on the credential disk), an L2
// capsule gets one more egress route, MirrorRouteHost, to go-choir-pkgmirror
// on the tap peer, and its tools are pointed at it for the granted
// ecosystems only. With mirrors only, the registries those mirrors replace
// leave the capsule's CONNECT allowlist. Everywhere else nothing changes.

// MirrorRouteHost is the name capsule workloads use for the host package
// mirror. Like GoProxyRouteHost it is an egress route, not a DNS name.
const MirrorRouteHost = "mirror.choir.internal"

// MirrorHostPort is go-choir-pkgmirror's port on the guest's tap peer.
const MirrorHostPort = 8089

// MirrorBrokerEnv carries the granted ecosystems from the executor to a
// capsule broker, which turns them into client settings (MirrorClientEnv).
const MirrorBrokerEnv = "CHOIR_CAPSULE_MIRRORS"

// MirrorAddr is where the executor routes MirrorRouteHost: the explicit
// address when one is configured, else the gateway URL's host (the guest's
// tap peer) on MirrorHostPort.
func MirrorAddr(explicit, gatewayURL string) (netip.AddrPort, bool, error) {
	return hostServiceAddr(explicit, gatewayURL, MirrorHostPort, "package mirror")
}

// mirroredHosts are the allowlist entries each mirror replaces. Hosts no
// mirror replaces (githubusercontent.com, conda, PyTorch, Hugging Face,
// papers) stay on CONNECT.
var mirroredHosts = map[string][]string{
	mirrorgrant.NPM:    {"registry.npmjs.org", "registry.yarnpkg.com", "repo.yarnpkg.com"},
	mirrorgrant.PyPI:   {"pypi.org", "pythonhosted.org"},
	mirrorgrant.Crates: {"crates.io"},
	mirrorgrant.Git:    {"github.com", "gitlab.com", "codeberg.org"},
}

// goProxiedHosts are replaced by the Go module proxy route.
var goProxiedHosts = []string{"proxy.golang.org", "sum.golang.org"}

// mirrorForges are the forges the git mirror serves.
var mirrorForges = []string{"github.com", "gitlab.com", "codeberg.org"}

// applyCapsuleMirrors adds the mirror route for a granted computer with a
// route address and, under mirrors only, removes the mirrored registries
// (and the Go hosts, when the Go proxy route exists) from the allowlist.
func applyCapsuleMirrors(policy EgressPolicy, st mirrorgrant.Status, addr netip.AddrPort, goRoute bool) EgressPolicy {
	if !st.Granted || len(st.Ecosystems) == 0 || !addr.IsValid() {
		return policy
	}
	routes := map[string]netip.AddrPort{MirrorRouteHost: addr}
	for k, v := range policy.Routes {
		routes[k] = v
	}
	policy.Routes = routes
	if !st.MirrorsOnly {
		return policy
	}
	drop := map[string]bool{}
	for _, eco := range st.Ecosystems {
		for _, h := range mirroredHosts[eco] {
			drop[h] = true
		}
	}
	if goRoute {
		for _, h := range goProxiedHosts {
			drop[h] = true
		}
	}
	kept := make([]string, 0, len(policy.Hosts))
	for _, h := range policy.Hosts {
		if !drop[h] {
			kept = append(kept, h)
		}
	}
	policy.Hosts = kept
	return policy
}

// MirrorClientEnv is what a broker adds to an L2 workload's environment for
// the ecosystems the executor granted it (MirrorBrokerEnv), and nothing
// when it granted none. Each tool reaches the route through the capsule
// proxy (HTTP_PROXY), as plain HTTP.
func MirrorClientEnv(lookup func(string) (string, bool)) []string {
	raw, _ := lookup(MirrorBrokerEnv)
	base := "http://" + MirrorRouteHost
	var env []string
	for _, eco := range mirrorgrant.ParseEcosystems(raw) {
		switch eco {
		case mirrorgrant.NPM:
			// npm fetches a tarball the packument names on
			// registry.npmjs.org from this registry (replace-registry-host).
			env = append(env,
				"npm_config_registry="+base+"/npm/", "npm_config_replace_registry_host=npmjs",
				"npm_config_audit=false", "npm_config_fund=false", "npm_config_update_notifier=false")
		case mirrorgrant.PyPI:
			index := base + "/pypi/simple/"
			env = append(env, "PIP_INDEX_URL="+index, "PIP_TRUSTED_HOST="+MirrorRouteHost,
				"UV_DEFAULT_INDEX="+index, "UV_INSECURE_HOST="+MirrorRouteHost)
		case mirrorgrant.Git:
			env = append(env, fmt.Sprintf("GIT_CONFIG_COUNT=%d", len(mirrorForges)))
			for i, forge := range mirrorForges {
				env = append(env,
					fmt.Sprintf("GIT_CONFIG_KEY_%d=url.%s/git/%s/.insteadOf", i, base, forge),
					fmt.Sprintf("GIT_CONFIG_VALUE_%d=https://%s/", i, forge))
			}
		case mirrorgrant.Crates:
			// cargo is not in the guest image; its source replacement is a
			// config file, written when cargo ships (station residual).
		}
	}
	return env
}

// mirrorBrokerEnv is the broker variable for a capsule's granted ecosystems.
func mirrorBrokerEnv(ecosystems []string) []string {
	if len(ecosystems) == 0 {
		return nil
	}
	return []string{MirrorBrokerEnv + "=" + strings.Join(ecosystems, ",")}
}
