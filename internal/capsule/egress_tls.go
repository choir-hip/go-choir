package capsule

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"time"
)

// Tunnel identity (docs/problems/egress-proxy-tunnel-identity-2026-10-21.md).
//
// A CONNECT is admitted by the host name in the request, after which the
// proxy relays opaque bytes. Nothing tied what the tunnel carried to that
// name, so an allowlisted front door could be used to reach another service
// behind the same front end. The proxy therefore reads the first TLS record
// the client sends, requires it to be a ClientHello that names exactly the
// CONNECT host in its server_name extension, and refuses everything else:
// no TLS, no name, two names, a name in another form, a hidden name
// (Encrypted Client Hello), a hello split across records, or a malformed
// one. A refusal closes the tunnel before a byte reaches the upstream.
//
// Fail-closed notes. A client that sends a GREASE Encrypted Client Hello is
// refused along with a real one, because the proxy cannot tell them apart;
// none of the toolchains an L2 capsule runs (go, pip, npm, git, curl,
// huggingface_hub) does so by default. A client that does not send SNI is
// refused; every TLS library sends it for a host name.

const (
	// maxTunnelRecord is the largest TLS record the protocol allows (2^14 of
	// plaintext). The first record of a tunnel is a plaintext ClientHello.
	maxTunnelRecord = 1 << 14
	// tunnelBufferSize holds a record header plus a full record behind the
	// CONNECT request without a second read.
	tunnelBufferSize = maxTunnelRecord + 5 + 512
	// defaultHelloTimeout bounds how long a tunnel may stay silent.
	defaultHelloTimeout = 10 * time.Second

	tlsRecordHandshake  = 0x16
	tlsHandshakeHello   = 0x01
	tlsExtServerName    = 0x0000
	tlsExtECH           = 0xfe0d // encrypted_client_hello
	tlsExtESNIDraft     = 0xffce // encrypted_server_name (draft)
	tlsServerNameHost   = 0
	maxServerNameLength = 253
)

// verifyTunnelClientHello checks one complete TLS record (header and payload)
// against the host the CONNECT named. It returns nil only for a single-record
// ClientHello whose only server_name is host.
func verifyTunnelClientHello(record []byte, host string) error {
	if len(record) < 5 {
		return fmt.Errorf("tunnel did not start with a TLS record")
	}
	if record[0] != tlsRecordHandshake {
		return fmt.Errorf("tunnel does not start with a TLS handshake record")
	}
	// The legacy record version of a ClientHello is 3.1 to 3.3.
	if record[1] != 0x03 || record[2] < 0x01 || record[2] > 0x03 {
		return fmt.Errorf("tunnel's first record has an unsupported TLS version")
	}
	length := int(binary.BigEndian.Uint16(record[3:5]))
	if length == 0 || length > maxTunnelRecord {
		return fmt.Errorf("tunnel's first TLS record has an invalid length")
	}
	if len(record) != 5+length {
		return fmt.Errorf("tunnel's first TLS record is incomplete or has trailing bytes")
	}
	hs := record[5:]
	if len(hs) < 4 || hs[0] != tlsHandshakeHello {
		return fmt.Errorf("tunnel's first handshake message is not a ClientHello")
	}
	helloLen := int(hs[1])<<16 | int(hs[2])<<8 | int(hs[3])
	if helloLen != len(hs)-4 {
		return fmt.Errorf("ClientHello is split across records or has trailing bytes")
	}
	body := hs[4:]

	// legacy_version(2) random(32) session_id<0..32> cipher_suites<2..2^16-2>
	// compression_methods<1..2^8-1> extensions<8..2^16-1>
	r := hello{b: body}
	r.skip(2 + 32)
	r.skip(r.u8())
	r.skip(r.u16())
	r.skip(r.u8())
	if r.bad {
		return fmt.Errorf("ClientHello is malformed")
	}
	if len(r.b) == 0 {
		return fmt.Errorf("ClientHello carries no SNI")
	}
	extLen := r.u16()
	ext := r.take(extLen)
	if r.bad || len(r.b) != 0 {
		return fmt.Errorf("ClientHello extension block is malformed")
	}

	var names []string
	seen := map[int]bool{}
	for len(ext.b) > 0 {
		typ := ext.u16()
		data := ext.take(ext.u16())
		if ext.bad {
			return fmt.Errorf("ClientHello extension is malformed")
		}
		if seen[typ] {
			return fmt.Errorf("ClientHello repeats an extension")
		}
		seen[typ] = true
		switch typ {
		case tlsExtECH, tlsExtESNIDraft:
			return fmt.Errorf("ClientHello hides its server name (Encrypted Client Hello)")
		case tlsExtServerName:
			list := data.take(data.u16())
			if data.bad || len(data.b) != 0 {
				return fmt.Errorf("ClientHello SNI extension is malformed")
			}
			for len(list.b) > 0 {
				nameType := list.u8()
				name := list.take(list.u16())
				if list.bad {
					return fmt.Errorf("ClientHello SNI list is malformed")
				}
				if nameType != tlsServerNameHost {
					return fmt.Errorf("ClientHello SNI is not a host name")
				}
				names = append(names, string(name.b))
			}
		}
	}
	if len(names) == 0 {
		return fmt.Errorf("ClientHello carries no SNI")
	}
	if len(names) != 1 {
		return fmt.Errorf("ClientHello names more than one SNI host")
	}
	sni := names[0]
	if !plainHostName(sni) {
		return fmt.Errorf("ClientHello SNI %q is not a plain host name", clip(sni))
	}
	if !strings.EqualFold(strings.TrimSuffix(sni, "."), strings.TrimSuffix(host, ".")) {
		return fmt.Errorf("ClientHello SNI %q is not the CONNECT host %q", clip(sni), clip(host))
	}
	return nil
}

// plainHostName accepts a DNS name and refuses everything a lenient parser
// might read two ways: empty, over-long, control or non-ASCII bytes, spaces,
// and IP literals (an allowlisted CONNECT host is never one).
func plainHostName(name string) bool {
	if name == "" || len(name) > maxServerNameLength {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '.', c == '_':
		default:
			return false
		}
	}
	if net.ParseIP(strings.TrimSuffix(name, ".")) != nil {
		return false
	}
	return true
}

// clip bounds attacker-chosen text that goes into a log reason.
func clip(s string) string {
	if len(s) > 64 {
		s = s[:64] + "..."
	}
	return strings.ToValidUTF8(s, "?")
}

// hello is a bounds-checked reader: any overrun sets bad and yields zeros, so
// a lying length can never read outside the record.
type hello struct {
	b   []byte
	bad bool
}

func (h *hello) take(n int) hello {
	if n < 0 || n > len(h.b) {
		h.bad = true
		h.b = nil
		return hello{bad: true}
	}
	out := hello{b: h.b[:n]}
	h.b = h.b[n:]
	return out
}

func (h *hello) skip(n int) { h.take(n) }

func (h *hello) u8() int {
	t := h.take(1)
	if t.bad {
		return 0
	}
	return int(t.b[0])
}

func (h *hello) u16() int {
	t := h.take(2)
	if t.bad {
		return 0
	}
	return int(binary.BigEndian.Uint16(t.b))
}

// verifyTunnel reads the first TLS record the client sends through a CONNECT
// tunnel and checks it against the CONNECT host. The record stays in br, so
// it is forwarded to the upstream unchanged once the check passes. A non-empty
// reason means the tunnel must be closed.
func (p *EgressProxy) verifyTunnel(client net.Conn, br *bufio.Reader, host string) string {
	timeout := p.helloTimeout
	if timeout <= 0 {
		timeout = defaultHelloTimeout
	}
	_ = client.SetReadDeadline(time.Now().Add(timeout))
	defer func() { _ = client.SetReadDeadline(time.Time{}) }()

	header, err := br.Peek(5)
	if err != nil {
		return "tunnel sent no TLS ClientHello"
	}
	if header[0] != tlsRecordHandshake {
		return "tunnel does not start with a TLS handshake record"
	}
	length := int(binary.BigEndian.Uint16(header[3:5]))
	if length == 0 || length > maxTunnelRecord {
		return "tunnel's first TLS record has an invalid length"
	}
	record, err := br.Peek(5 + length)
	if err != nil {
		return "tunnel's TLS ClientHello was incomplete"
	}
	if err := verifyTunnelClientHello(record, host); err != nil {
		return err.Error()
	}
	return ""
}
