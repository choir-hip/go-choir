package autoputer

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/hostfetch"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
)

// installHostFetchRoute routes this computer's guarded public fetches
// (research Read, content import, podcast and YouTube fetches, source
// polls) through the gateway's fetch verb when vmctl granted it on the
// credential disk (SDC NET-01; internal/hostfetch). Without a grant nothing
// changes. It runs first in Run, before any sourcefetch client is built:
// a client takes the route when it is constructed.
func installHostFetchRoute(computerID string) {
	dir := strings.TrimSpace(os.Getenv("CHOIR_HOST_FETCH_CREDENTIAL_DIR"))
	if dir == "" {
		if tokenFile := strings.TrimSpace(os.Getenv("RUNTIME_GATEWAY_TOKEN_FILE")); tokenFile != "" {
			dir = filepath.Dir(tokenFile)
		}
	}
	gatewayURL := strings.TrimSpace(os.Getenv("RUNTIME_GATEWAY_URL"))
	if gatewayURL == "" {
		gatewayURL = strings.TrimSpace(os.Getenv("PROXY_VMCTL_URL"))
	}
	rt, status := hostfetch.GuestRoute(dir, computerID, gatewayURL, provideriface.GatewayToken)
	if rt != nil {
		sourcefetch.SetRoute(rt)
	}
	log.Printf("autoputer: host fetch route %s", status)
}
