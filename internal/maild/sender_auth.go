package maild

import (
	"strings"
	"unicode"
)

// senderAuthenticatedFor is the inbound sender verdict under the configured
// send gate mode: the aligned verdict, or the pre-E1 rule under the
// rollback switch.
func senderAuthenticatedFor(cfg *Config, headers map[string]string, fromAddress string) bool {
	if cfg.legacySendGate() {
		return hasPassingAuthenticationResultEvidence(headers)
	}
	var trusted []string
	if cfg != nil {
		trusted = cfg.TrustedAuthservIDs
	}
	return senderAuthenticated(headers, fromAddress, trusted)
}

// senderAuthenticated reports whether a received message's From address is
// authenticated (G2, G3; docs/archive/problems/maild-send-gate-defects-2026-10-14.md):
// an Authentication-Results header (RFC 8601) whose authserv-id is one of
// trusted, the receiving server's, that reports
//
//   - dmarc=pass with header.from equal to the From domain, or
//   - dkim=pass for a signing domain aligned with the From domain (equal, or
//     one a subdomain of the other: DMARC relaxed alignment without a
//     public-suffix list, so a bare TLD never aligns),
//
// and no dmarc=fail. SPF alone never counts (it authenticates the envelope,
// not From), and ARC-Authentication-Results never count (an intermediary
// wrote them). A value that looks like several headers joined into one is
// ambiguous and yields no verdict. With no trusted authserv-id, nothing is
// authenticated.
func senderAuthenticated(headers map[string]string, fromAddress string, trusted []string) bool {
	fromDomain := emailDomainOf(fromAddress)
	if fromDomain == "" || len(trusted) == 0 {
		return false
	}
	trustedIDs := map[string]bool{}
	for _, id := range trusted {
		if id = strings.ToLower(strings.TrimSpace(id)); id != "" {
			trustedIDs[id] = true
		}
	}
	for key, value := range headers {
		if !strings.EqualFold(strings.TrimSpace(key), "authentication-results") {
			continue
		}
		authservID, results, ok := parseAuthenticationResults(value)
		if !ok || !trustedIDs[authservID] {
			continue
		}
		pass, dmarcFail := false, false
		for _, res := range results {
			switch res.method {
			case "dmarc":
				if res.result == "fail" {
					dmarcFail = true
				}
				if res.result == "pass" && domainsEqual(res.props["header.from"], fromDomain) {
					pass = true
				}
			case "dkim":
				if res.result != "pass" {
					continue
				}
				signer := res.props["header.d"]
				if signer == "" {
					if i := res.props["header.i"]; strings.Contains(i, "@") {
						signer = i[strings.LastIndex(i, "@")+1:]
					}
				}
				if domainsAligned(signer, fromDomain) {
					pass = true
				}
			}
		}
		if pass && !dmarcFail {
			return true
		}
	}
	return false
}

type authResult struct {
	method string
	result string
	props  map[string]string
}

// parseAuthenticationResults parses one Authentication-Results value into
// its authserv-id and method results. Comments are dropped, quoted strings
// kept whole, and folded lines unfolded. It refuses (ok=false) a value with
// a bare line break or a header name inside it, which is what two headers
// joined into one value look like, and unbalanced quotes or comments.
func parseAuthenticationResults(value string) (string, []authResult, bool) {
	for _, fold := range []string{"\r\n ", "\r\n\t", "\n ", "\n\t"} {
		value = strings.ReplaceAll(value, fold, " ")
	}
	if strings.ContainsAny(value, "\r\n") || strings.Contains(strings.ToLower(value), "authentication-results") {
		return "", nil, false
	}
	segments, ok := tokenizeAuthenticationResults(value)
	if !ok || len(segments) == 0 || len(segments[0]) == 0 {
		return "", nil, false
	}
	authservID := strings.ToLower(segments[0][0])
	var results []authResult
	for _, seg := range segments[1:] {
		if len(seg) == 0 {
			continue
		}
		method, result, found := strings.Cut(seg[0], "=")
		if !found {
			continue // "none"
		}
		method, _, _ = strings.Cut(strings.ToLower(method), "/")
		res := authResult{method: method, result: strings.ToLower(result), props: map[string]string{}}
		for _, tok := range seg[1:] {
			if k, v, found := strings.Cut(tok, "="); found {
				res.props[strings.ToLower(k)] = v
			}
		}
		results = append(results, res)
	}
	return authservID, results, true
}

func tokenizeAuthenticationResults(value string) ([][]string, bool) {
	var (
		segments [][]string
		tokens   []string
		cur      strings.Builder
		depth    int
		inQuote  bool
		escaped  bool
	)
	flushToken := func() {
		if cur.Len() > 0 {
			tokens = append(tokens, cur.String())
			cur.Reset()
		}
	}
	flushSegment := func() {
		flushToken()
		segments = append(segments, tokens)
		tokens = nil
	}
	for _, r := range value {
		switch {
		case escaped:
			if depth == 0 {
				cur.WriteRune(r)
			}
			escaped = false
		case r == '\\' && (inQuote || depth > 0):
			escaped = true
		case inQuote:
			if r == '"' {
				inQuote = false
			} else {
				cur.WriteRune(r)
			}
		case depth > 0:
			if r == '(' {
				depth++
			} else if r == ')' {
				depth--
			}
		case r == '(':
			flushToken()
			depth++
		case r == ')':
			return nil, false
		case r == '"':
			inQuote = true
		case r == ';':
			flushSegment()
		case unicode.IsSpace(r):
			flushToken()
		default:
			cur.WriteRune(r)
		}
	}
	if inQuote || depth != 0 || escaped {
		return nil, false
	}
	flushSegment()
	return segments, true
}

func emailDomainOf(address string) string {
	address = strings.TrimSpace(address)
	at := strings.LastIndex(address, "@")
	if at < 0 || at == len(address)-1 {
		return ""
	}
	return normalizeDomain(address[at+1:])
}

func normalizeDomain(domain string) string {
	return strings.Trim(strings.ToLower(strings.TrimSpace(domain)), ".")
}

func domainsEqual(a, b string) bool {
	a, b = normalizeDomain(a), normalizeDomain(b)
	return a != "" && a == b
}

// domainsAligned is relaxed alignment without a public-suffix list: equal,
// or one a subdomain of the other, and never a single-label domain.
func domainsAligned(signer, from string) bool {
	signer, from = normalizeDomain(signer), normalizeDomain(from)
	if signer == "" || from == "" || !strings.Contains(signer, ".") || !strings.Contains(from, ".") {
		return false
	}
	return signer == from || strings.HasSuffix(from, "."+signer) || strings.HasSuffix(signer, "."+from)
}
