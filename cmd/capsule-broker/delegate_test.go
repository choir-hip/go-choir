//go:build linux

package main

import (
	"os"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/claudedelegate"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// docs/design/claude-delegate-slice1-2026-10-14.md F4: the broker holds the
// delegate token, so every environment it builds for anything but the claude
// process must leave it out, and the broker must drop it from its own
// environment before it starts any child.

const brokerFakeToken = "sk-ant-oat01-FAKE-broker-token-0123456789abcdefgh"

func TestBrokerTakesTheDelegateTokenOutOfItsEnvironment(t *testing.T) {
	t.Setenv(capsule.ClaudeDelegateBrokerTokenEnv, brokerFakeToken)
	t.Setenv(capsule.ClaudeDelegateBrokerBinEnv, "/nix/store/x-claude-code/bin/claude")
	token, bin := takeDelegateEnv()
	if token != brokerFakeToken || bin != "/nix/store/x-claude-code/bin/claude" {
		t.Fatalf("took %v %q", token != "", bin)
	}
	if strings.Contains(strings.Join(os.Environ(), "\n"), brokerFakeToken) {
		t.Fatal("the token is still in the broker's environment")
	}
}

func TestNoBrokerChildEnvironmentCarriesTheDelegateToken(t *testing.T) {
	env := map[string]string{
		"PATH": "/run/current-system/sw/bin", yaegikernel.BuildToolchainPathEnv: "/nix/store/gcc/bin", "CGO_CFLAGS": "-I/x",
		capsule.ClaudeDelegateBrokerTokenEnv: brokerFakeToken, claudedelegate.TokenEnv: brokerFakeToken,
	}
	lookup := func(k string) (string, bool) { v, ok := env[k]; return v, ok }
	overrides := []string{claudedelegate.TokenEnv + "=" + brokerFakeToken, "HOME=/tmp"}
	for name, got := range map[string][]string{
		"session worker env": sessionWorkerEnv(true, lookup),
		"direct exec env":    directExecEnvFrom(lookup, overrides),
		"delegate base env":  delegateBaseEnv(lookup),
	} {
		if strings.Contains(strings.Join(got, "\n"), brokerFakeToken) {
			t.Errorf("%s carries the delegate token", name)
		}
	}
	base := strings.Join(delegateBaseEnv(lookup), "\n")
	for _, want := range []string{"GOPROXY=", "HTTPS_PROXY=", "CGO_CFLAGS=", "/nix/store/gcc/bin"} {
		if !strings.Contains(base, want) {
			t.Errorf("delegate base env lacks %s", want)
		}
	}
}
