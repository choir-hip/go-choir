package frontimportance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/gateway"
	"github.com/yusefmosiah/go-choir/internal/hostauth"
)

// SEC-16: the scorer reaches Jev through the gateway (host route
// /provider/v1/host/judgments), so the gateway is the one holder and meter of
// the OpenRouter key and this job's unit loads no provider environment.

const (
	// defaultGatewayJudgmentsURL is the gateway's host route on loopback.
	defaultGatewayJudgmentsURL = "http://127.0.0.1:8084/provider/v1/host/judgments"
	// hostCallerName is this job's name in the gateway's host-caller table.
	hostCallerName = "front-importance"
	// gatewayAnswerLimit bounds the answer read from the gateway (the
	// gateway's own transport limit is 1 MiB).
	gatewayAnswerLimit = 1 << 20
)

// gatewayDecider is the decider that calls the gateway's host route.
type gatewayDecider struct {
	url    string
	client *http.Client
}

// Decide posts one judgment request and returns the gateway's exact answer.
// The computer label is the gateway's own for this caller; the request never
// carries one.
func (d *gatewayDecider) Decide(ctx context.Context, _ string, request gateway.JudgmentRequest) ([]byte, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("encode judgment request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build gateway request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Choir-Host-Caller", hostCallerName)
	hostauth.MarkInternal(req)
	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call gateway: %w", err)
	}
	defer resp.Body.Close()
	answer, err := io.ReadAll(io.LimitReader(resp.Body, gatewayAnswerLimit+1))
	if err != nil {
		return nil, fmt.Errorf("read gateway answer: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		detail := strings.TrimSpace(string(answer))
		if len(detail) > 200 {
			detail = detail[:200] + "..."
		}
		return nil, fmt.Errorf("gateway answered %d: %s", resp.StatusCode, detail)
	}
	if len(answer) > gatewayAnswerLimit {
		return nil, fmt.Errorf("gateway answer exceeds the transport limit")
	}
	return answer, nil
}

// newDecider picks how the scorer reaches Jev: through the gateway's host
// route (SEC-16; the old direct OpenRouter path, FRONT_IMPORTANCE_DIRECT=1,
// was deleted 2026-10-26). The gateway URL must be loopback: the
// host-internal credential is never sent off the host.
func newDecider(getenv func(string) string, client *http.Client) (decider, string, error) {
	target := strings.TrimSpace(getenv("FRONT_IMPORTANCE_GATEWAY_URL"))
	if target == "" {
		target = defaultGatewayJudgmentsURL
	}
	u, err := url.Parse(target)
	if err != nil || u.Scheme != "http" || u.Hostname() == "" {
		return nil, "gateway", fmt.Errorf("gateway URL %q is not an http loopback URL", target)
	}
	if ip := net.ParseIP(u.Hostname()); ip == nil || !ip.IsLoopback() {
		return nil, "gateway", fmt.Errorf("gateway URL %q is not loopback; the host credential stays on the host", target)
	}
	return &gatewayDecider{url: target, client: client}, "gateway", nil
}
