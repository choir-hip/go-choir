package hostfetch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
)

// Executor performs one guarded hop on the host. The zero value uses the
// system resolver, a plain dialer and the host's interface addresses.
type Executor struct {
	// Lookup resolves a hostname; nil means net.DefaultResolver.
	Lookup func(context.Context, string) ([]net.IPAddr, error)
	// Dial dials a checked address; nil means a net.Dialer.
	Dial func(context.Context, string, string) (net.Conn, error)
	// Self reports the host's own addresses, refused like private ones; nil
	// means the addresses on the host's interfaces.
	Self func(net.IP) bool
	// MaxBody bounds a relayed body; 0 means MaxResponseBody.
	MaxBody int64

	once   sync.Once
	client *http.Client
}

// Result describes one call for the gateway's log line. It never carries
// the URL's path or query, which can hold tokens.
type Result struct {
	Method  string
	Host    string
	Status  int
	Bytes   int64
	Refused string
	Err     error
}

func (e *Executor) httpClient() *http.Client {
	e.once.Do(func() {
		lookup := e.Lookup
		if lookup == nil {
			lookup = net.DefaultResolver.LookupIPAddr
		}
		dial := e.Dial
		if dial == nil {
			d := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
			dial = d.DialContext
		}
		self := e.Self
		if self == nil {
			self = hostOwnAddress
		}
		c := sourcefetch.GuardedClient(MaxTimeout, lookup, dial, self)
		// One hop: the guest re-checks and resends each redirect.
		c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
		e.client = c
	})
	return e.client
}

// hostOwnAddress reports an address on one of the host's interfaces: a
// fetch to the host's own public address would reach listeners the guest
// floor drops.
func hostOwnAddress(ip net.IP) bool {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return true
	}
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok && n.IP.Equal(ip) {
			return true
		}
	}
	return false
}

func (e *Executor) maxBody() int64 {
	if e.MaxBody > 0 {
		return e.MaxBody
	}
	return MaxResponseBody
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// Serve decodes one Request from r, performs the hop and relays the answer:
// status 200 with StatusHeader and HeadersHeader and the upstream body, or
// a JSON error without StatusHeader. A body that passes MaxBody, or breaks
// off, aborts the relay (panic http.ErrAbortHandler) so the guest never
// reads a cut body as complete.
func (e *Executor) Serve(w http.ResponseWriter, r *http.Request) Result {
	var res Result
	e.ServeTo(w, r, &res)
	return res
}

// ServeTo is Serve filling res as it goes, so a caller that recovers the
// abort can still log what the hop was.
func (e *Executor) ServeTo(w http.ResponseWriter, r *http.Request, res *Result) {
	envelopeLimit := int64(MaxRequestBody)*2 + 64<<10
	raw, err := io.ReadAll(io.LimitReader(r.Body, envelopeLimit+1))
	if err != nil {
		res.Refused = "unreadable request"
		writeError(w, http.StatusBadRequest, "host fetch: unreadable request")
		return
	}
	if int64(len(raw)) > envelopeLimit {
		res.Refused = "request body too large"
		writeError(w, http.StatusRequestEntityTooLarge, "host fetch: request body too large")
		return
	}
	var req Request
	if err := json.Unmarshal(raw, &req); err != nil {
		res.Refused = "invalid request"
		writeError(w, http.StatusBadRequest, "host fetch: invalid request")
		return
	}
	res.Method = req.Method
	if len(req.Body) > MaxRequestBody {
		res.Refused = "request body too large"
		writeError(w, http.StatusRequestEntityTooLarge, "host fetch: request body too large")
		return
	}
	switch req.Method {
	case http.MethodGet, http.MethodHead, http.MethodPost:
	default:
		res.Refused = "method not allowed"
		writeError(w, http.StatusBadRequest, "host fetch: method must be GET, HEAD or POST")
		return
	}
	if err := sourcefetch.ValidateURL(req.URL); err != nil {
		res.Refused = "url: " + err.Error()
		writeError(w, http.StatusForbidden, "host fetch: "+err.Error())
		return
	}
	target, _ := url.Parse(strings.TrimSpace(req.URL))
	res.Host = target.Hostname()

	ctx, cancel := context.WithTimeout(r.Context(), MaxTimeout)
	defer cancel()
	var body io.Reader
	if len(req.Body) > 0 {
		body = bytes.NewReader(req.Body)
	}
	up, err := http.NewRequestWithContext(ctx, req.Method, target.String(), body)
	if err != nil {
		res.Refused = "invalid request"
		writeError(w, http.StatusBadRequest, "host fetch: invalid request")
		return
	}
	up.Header = stripHop(req.Header)

	resp, err := e.httpClient().Do(up)
	if err != nil {
		// url.Error repeats the URL, which the log must not carry.
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		switch {
		case strings.Contains(err.Error(), "forbidden address"):
			res.Refused = "address: the host resolves to a forbidden address"
			writeError(w, http.StatusForbidden, "host fetch: the host resolves to a forbidden address")
		case errors.Is(err, context.DeadlineExceeded):
			res.Err = err
			writeError(w, http.StatusGatewayTimeout, "host fetch: upstream timed out")
		default:
			res.Err = err
			writeError(w, http.StatusBadGateway, "host fetch: upstream unreachable")
		}
		return
	}
	defer resp.Body.Close()
	res.Status = resp.StatusCode
	limit := e.maxBody()
	if resp.ContentLength > limit {
		res.Refused = "upstream body exceeds the host fetch cap"
		writeError(w, http.StatusBadGateway, "host fetch: upstream body exceeds the host fetch cap")
		return
	}
	h := w.Header()
	h.Set(StatusHeader, strconv.Itoa(resp.StatusCode))
	h.Set(HeadersHeader, EncodeHeaders(stripHop(resp.Header)))
	h.Set("Content-Type", "application/octet-stream")
	if resp.ContentLength >= 0 && req.Method != http.MethodHead {
		h.Set("Content-Length", strconv.FormatInt(resp.ContentLength, 10))
	}
	w.WriteHeader(http.StatusOK)
	n, err := io.Copy(w, io.LimitReader(resp.Body, limit+1))
	res.Bytes = n
	if err != nil || n > limit {
		// Status is sent: only aborting the connection tells the guest the
		// body is not whole.
		panic(http.ErrAbortHandler)
	}
}
