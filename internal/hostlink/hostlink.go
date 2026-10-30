// Package hostlink is the HTTP transport for a guest's calls to its host
// services: the event chain (corpusd), the model gateway, credential
// renewal and search. A host deploy stops those services for up to a couple
// of minutes while busy guests keep running; a guest client that sent once
// failed whatever turn was in flight, and could not even record that failure
// (docs/archive/problems/host-deploy-restart-fails-in-flight-guest-turns-2026-10-10.md).
//
// A refused dial never reached the host, so resending it is safe for any
// method. Transport resends only that, with backoff, for up to RestartWait
// and never past the request's context. Anything that reached the host,
// including a connection reset mid-call, is returned as it was.
package hostlink

import (
	"context"
	"errors"
	"io"
	"net/http"
	"syscall"
	"time"
)

// RestartWait bounds how long a call keeps resending while the host refuses
// connections. The observed deploy gap was 1 min 48 s.
const RestartWait = 3 * time.Minute

// NewClient returns a client whose attempts each get attemptTimeout (the
// whole-request bound the caller used to set on http.Client.Timeout) while
// refused dials are resent across RestartWait.
func NewClient(attemptTimeout time.Duration) *http.Client {
	return &http.Client{Transport: &Transport{AttemptTimeout: attemptTimeout}}
}

// Transport resends requests whose dial was refused.
type Transport struct {
	// Base performs each attempt; nil means http.DefaultTransport.
	Base http.RoundTripper
	// AttemptTimeout bounds one attempt including reading its body; zero
	// means no per-attempt bound beyond the request's context.
	AttemptTimeout time.Duration
	// Wait bounds the resend window; zero means RestartWait.
	Wait time.Duration
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	wait := t.Wait
	if wait <= 0 {
		wait = RestartWait
	}
	deadline := time.Now().Add(wait)
	backoff := 250 * time.Millisecond
	attempt := req
	for {
		resp, err := t.roundTripOnce(base, attempt)
		if err == nil || !errors.Is(err, syscall.ECONNREFUSED) {
			return resp, err
		}
		replayable := req.Body == nil || req.Body == http.NoBody || req.GetBody != nil
		if !replayable || time.Now().Add(backoff).After(deadline) {
			return nil, err
		}
		timer := time.NewTimer(backoff)
		select {
		case <-req.Context().Done():
			timer.Stop()
			return nil, err
		case <-timer.C:
		}
		attempt = req.Clone(req.Context())
		if req.GetBody != nil {
			body, bodyErr := req.GetBody()
			if bodyErr != nil {
				return nil, err
			}
			attempt.Body = body
		}
		if backoff < 5*time.Second {
			backoff *= 2
		}
	}
}

func (t *Transport) roundTripOnce(base http.RoundTripper, req *http.Request) (*http.Response, error) {
	if t.AttemptTimeout <= 0 {
		return base.RoundTrip(req)
	}
	ctx, cancel := context.WithTimeout(req.Context(), t.AttemptTimeout)
	resp, err := base.RoundTrip(req.WithContext(ctx))
	if err != nil {
		cancel()
		return nil, err
	}
	resp.Body = &cancelOnClose{ReadCloser: resp.Body, cancel: cancel}
	return resp, nil
}

// cancelOnClose keeps an attempt's context alive until its body is closed,
// so the per-attempt bound covers reading the body, as http.Client.Timeout
// does.
type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c *cancelOnClose) Close() error {
	err := c.ReadCloser.Close()
	c.cancel()
	return err
}
