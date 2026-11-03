//go:build linux

package capsule

import (
	"context"
	"errors"
	"io"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// docs/problems/engineering-cell-never-returns-2026-11-03.md: a plain
// choir.Poll cell read the broker socket for 46 minutes (until the
// activation deadline) because the read deadline was the caller's. Failure
// modes written before the code:
//   - a go_eval read waits past the broker's own cell bound (timeout_ms, or
//     its 60 s default) plus a slack;
//   - a job verb read waits past its own bound;
//   - the error does not say the broker did not answer, or for a job verb
//     does not name the job;
//   - a caller deadline shorter than the bound is not kept.

// silentBroker accepts connections and reads requests but never answers.
func silentBroker(t *testing.T) string {
	t.Helper()
	socket := filepath.Join(t.TempDir(), "broker.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() { defer conn.Close(); _, _ = io.Copy(io.Discard, conn) }()
		}
	}()
	return socket
}

func shortBrokerBounds(t *testing.T) {
	t.Helper()
	slack, job := brokerAnswerSlack, brokerJobAnswerBound
	brokerAnswerSlack, brokerJobAnswerBound = 100*time.Millisecond, 200*time.Millisecond
	t.Cleanup(func() { brokerAnswerSlack, brokerJobAnswerBound = slack, job })
}

func TestBrokerGoEvalReadEndsAtTheBrokersOwnBoundPlusSlack(t *testing.T) {
	shortBrokerBounds(t)
	client := connectedBrokerClient(t, silentBroker(t))
	start := time.Now()
	_, err := client.GoEval(context.Background(), nil, GoEvalRequest{Source: "println(1)", TimeoutMS: 100})
	elapsed := time.Since(start)
	if elapsed > 3*time.Second {
		t.Fatalf("go_eval read waited %s", elapsed)
	}
	if !errors.Is(err, ErrBrokerNoAnswer) || !strings.Contains(err.Error(), "the capsule broker did not answer") {
		t.Fatalf("err = %v", err)
	}
}

func TestBrokerJobStatusReadEndsAtItsBoundAndNamesTheJob(t *testing.T) {
	shortBrokerBounds(t)
	client := connectedBrokerClient(t, silentBroker(t))
	start := time.Now()
	_, err := client.Job(context.Background(), "job_status", nil, JobParams{ID: "job-d807dd9c4be9"})
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("job_status read waited %s", elapsed)
	}
	if !errors.Is(err, ErrBrokerNoAnswer) || !strings.Contains(err.Error(), "job-d807dd9c4be9") || !strings.Contains(err.Error(), "the capsule broker did not answer") {
		t.Fatalf("err = %v", err)
	}
}

func TestBrokerReadKeepsAShorterCallerDeadline(t *testing.T) {
	shortBrokerBounds(t)
	brokerJobAnswerBound = 10 * time.Second
	client := connectedBrokerClient(t, silentBroker(t))
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := client.Job(ctx, "job_status", nil, JobParams{ID: "job-x"})
	if elapsed := time.Since(start); elapsed > 3*time.Second || err == nil {
		t.Fatalf("after %s: err = %v", elapsed, err)
	}
	if errors.Is(err, ErrBrokerNoAnswer) {
		t.Fatalf("the caller's own deadline was reported as a silent broker: %v", err)
	}
}

func TestBrokerGoEvalAnswerBound(t *testing.T) {
	if got := BrokerGoEvalAnswerBound(0); got != 60*time.Second+brokerAnswerSlack {
		t.Fatalf("default: %s", got)
	}
	if got := BrokerGoEvalAnswerBound(5000); got != 5*time.Second+brokerAnswerSlack {
		t.Fatalf("5000 ms: %s", got)
	}
}
