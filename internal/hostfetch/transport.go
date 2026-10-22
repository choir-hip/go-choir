package hostfetch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/hostlink"
)

// Transport is the guest side: a RoundTripper that sends each hop to the
// gateway's fetch verb. It is installed as sourcefetch's route, so every
// guarded client on the computer uses it and keeps its own redirect check.
// It never dials the target itself.
type Transport struct {
	// BaseURL is the gateway (RUNTIME_GATEWAY_URL).
	BaseURL string
	// Token returns the computer's gateway bearer, read per call so a
	// renewed credential is picked up.
	Token func() string
	// Base carries the call to the gateway; nil means hostlink, which
	// resends a refused dial while the host restarts.
	Base http.RoundTripper
}

func closeBody(req *http.Request) {
	if req != nil && req.Body != nil {
		_ = req.Body.Close()
	}
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	token := ""
	if t.Token != nil {
		token = strings.TrimSpace(t.Token())
	}
	if strings.TrimSpace(t.BaseURL) == "" || token == "" {
		closeBody(req)
		return nil, fmt.Errorf("host fetch: no gateway route on this computer")
	}
	var body []byte
	if req.Body != nil {
		b, err := io.ReadAll(io.LimitReader(req.Body, MaxRequestBody+1))
		closeBody(req)
		if err != nil {
			return nil, fmt.Errorf("host fetch: read request body: %w", err)
		}
		if len(b) > MaxRequestBody {
			return nil, fmt.Errorf("host fetch: request body too large")
		}
		body = b
	}
	envelope, err := json.Marshal(Request{Method: req.Method, URL: req.URL.String(), Header: req.Header, Body: body})
	if err != nil {
		return nil, fmt.Errorf("host fetch: encode request: %w", err)
	}
	out, err := http.NewRequestWithContext(req.Context(), http.MethodPost, strings.TrimRight(t.BaseURL, "/")+Path, bytes.NewReader(envelope))
	if err != nil {
		return nil, fmt.Errorf("host fetch: %w", err)
	}
	out.Header.Set("Content-Type", "application/json")
	out.Header.Set("Authorization", "Bearer "+token)
	base := t.Base
	if base == nil {
		base = &hostlink.Transport{}
	}
	resp, err := base.RoundTrip(out)
	if err != nil {
		return nil, fmt.Errorf("host fetch: gateway: %w", err)
	}
	statusText := resp.Header.Get(StatusHeader)
	if statusText == "" {
		defer resp.Body.Close()
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		var e struct {
			Error string `json:"error"`
		}
		msg := strings.TrimSpace(string(raw))
		if json.Unmarshal(raw, &e) == nil && strings.TrimSpace(e.Error) != "" {
			msg = e.Error
		}
		return nil, fmt.Errorf("host fetch: %s (gateway status %d)", msg, resp.StatusCode)
	}
	code, err := strconv.Atoi(statusText)
	if err != nil || code < 100 || code > 999 {
		resp.Body.Close()
		return nil, fmt.Errorf("host fetch: invalid upstream status %q", statusText)
	}
	header, err := DecodeHeaders(resp.Header.Get(HeadersHeader))
	if err != nil {
		resp.Body.Close()
		return nil, err
	}
	return &http.Response{
		Status:        fmt.Sprintf("%d %s", code, http.StatusText(code)),
		StatusCode:    code,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        header,
		Body:          resp.Body,
		ContentLength: resp.ContentLength,
		Request:       req,
	}, nil
}

type refusing string

// Refusing is the route of a granted computer that cannot reach its
// gateway: every fetch fails, none dials directly.
func Refusing(reason string) http.RoundTripper { return refusing(reason) }

func (r refusing) RoundTrip(req *http.Request) (*http.Response, error) {
	closeBody(req)
	return nil, fmt.Errorf("host fetch: %s", string(r))
}
