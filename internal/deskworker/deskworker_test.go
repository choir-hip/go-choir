package deskworker

import (
	"errors"
	"net"
	"reflect"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

func recorded(order *[]string, confineErr error) steps {
	c1, c2 := net.Pipe()
	_ = c2
	rec := func(n string) { *order = append(*order, n) }
	return steps{
		config: func() yaegikernel.SessionWorkerConfig {
			return yaegikernel.SessionWorkerConfig{AllowedPackages: []string{"fmt"}}
		},
		sockFD:      func() (int, net.Conn, error) { return 3, c1, nil },
		volunteer:   func() error { rec("volunteer"); return nil },
		memoryLimit: func(uint64) error { rec("rlimit"); return nil },
		goMemLimit:  func() { rec("gomemlimit") },
		confine:     func() error { rec("confine"); return confineErr },
		selfTest:    func() { rec("selftest") },
		serve:       func(net.Conn, yaegikernel.SessionWorkerConfig) { rec("serve") },
	}
}

func TestRunConfinesBeforeServing(t *testing.T) {
	var order []string
	if code := run(recorded(&order, nil)); code != 0 {
		t.Fatalf("exit %d", code)
	}
	want := []string{"volunteer", "rlimit", "gomemlimit", "confine", "selftest", "serve"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("startup order %v, want %v", order, want)
	}
}

func TestRunDoesNotServeWhenConfinementFails(t *testing.T) {
	var order []string
	if code := run(recorded(&order, errors.New("landlock"))); code != 3 {
		t.Fatalf("exit %d, want 3", code)
	}
	for _, n := range order {
		if n == "serve" || n == "selftest" {
			t.Fatalf("worker reached %q without confinement: %v", n, order)
		}
	}
}

// Run must wire the real confinement, not a stub.
func TestRealStepsWireConfinement(t *testing.T) {
	s := realSteps()
	if s.confine == nil || s.serve == nil || s.goMemLimit == nil || s.selfTest == nil {
		t.Fatal("realSteps leaves a startup step unwired")
	}
}
