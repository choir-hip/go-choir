package hostauth_test

import (
	"os"
	"regexp"
	"strconv"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
	"github.com/yusefmosiah/go-choir/internal/vmmanager"
)

// MarkInternal attaches the host credential by loopback port number. vmctl
// DNATs 127.0.0.1:<host port> to each guest (vmmanager tapNATSpecs, the
// OUTPUT rule), and host ports count up from HostBasePort. A credential port
// inside that range would send the host secret to a guest (DeepSeek L2,
// bunch 41). Every credential port must sit below the base, in the code
// default and in the deployed VM_HOST_BASE_PORT.
func TestCredentialPortsNeverInGuestHostPortRange(t *testing.T) {
	bases := map[string]int{"vmmanager default": vmmanager.DefaultManagerConfig().HostBasePort}
	nix, err := os.ReadFile("../../nix/node-b.nix")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`"VM_HOST_BASE_PORT=(\d+)"`).FindAllSubmatch(nix, -1)
	if len(m) == 0 {
		t.Fatal("nix/node-b.nix sets no VM_HOST_BASE_PORT")
	}
	for _, sub := range m {
		n, _ := strconv.Atoi(string(sub[1]))
		bases["nix/node-b.nix VM_HOST_BASE_PORT="+string(sub[1])] = n
	}
	if len(hostauth.DefaultPortsForTest) == 0 {
		t.Fatal("no credential ports")
	}
	for _, p := range hostauth.DefaultPortsForTest {
		port, err := strconv.Atoi(p)
		if err != nil {
			t.Fatalf("credential port %q: %v", p, err)
		}
		for name, base := range bases {
			if port >= base {
				t.Errorf("credential port %d is inside the guest host-port range from %d (%s)", port, base, name)
			}
		}
	}
}
