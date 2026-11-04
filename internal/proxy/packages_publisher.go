package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
)

// Publisher address on listed releases (owner, 2026-11-04: an app store shows
// who made the app by their numbered Choir address, <NNN>@<primary domain>,
// the account's public address, today always its numbered Choir address; every account has one, test accounts
// included, and the account's sign-in email is never shown). maild owns the
// numbers; the proxy asks its host-internal, read-only lookup
// (maild.PublicAddressesPath) for the owners of the releases in the answer
// it is about to relay and adds publisher_address (and own) to them. It is
// display-only: corpusd and the record never see it, and nothing acts on it.
// A request parameter is never resolved: a history's echoed track names no
// publisher.
//
// There is no fallback. If maild cannot answer, the catalog read fails with
// 502 publisher_address_unavailable; a listed release whose publisher has no
// number is a defect, logged and named in a 502 publisher_address_missing.
// Publishing requires the publisher to have a number
// (requirePublisherAddress), so a listed release always has one. See
// docs/design/library-app-store-2026-11-04.md.

const (
	publicAddressesPath = "/internal/maild/public-addresses"
	publisherAddressTTL = time.Minute
	publisherAddressMax = 100
)

type publisherAddressCache struct {
	mu      sync.Mutex
	entries map[string]publisherAddressEntry
}

type publisherAddressEntry struct {
	address string // "" is a cached "no number"
	until   time.Time
}

func (c *publisherAddressCache) get(owner string, now time.Time) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[owner]
	if !ok || now.After(e.until) {
		return "", false
	}
	return e.address, true
}

func (c *publisherAddressCache) put(owner, address string, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil || len(c.entries) > 4096 {
		c.entries = map[string]publisherAddressEntry{}
	}
	c.entries[owner] = publisherAddressEntry{address: address, until: now.Add(publisherAddressTTL)}
}

var errPublisherAddressUnavailable = errors.New("the address service did not answer")

// publisherAddressesFor resolves owners to numbered addresses; an owner with
// no number is absent from the result. A maild failure is an error, never an
// empty answer. Only found addresses are cached, so a number assigned a
// moment later is seen at the next read.
func (h *Handler) publisherAddressesFor(ctx context.Context, owners []string) (map[string]string, error) {
	out := map[string]string{}
	now := time.Now()
	var missing []string
	for _, owner := range owners {
		if addr, ok := h.publisherAddresses.get(owner, now); ok && addr != "" {
			out[owner] = addr
			continue
		}
		missing = append(missing, owner)
	}
	for len(missing) > 0 {
		batch := missing[:min(publisherAddressMax, len(missing))]
		missing = missing[len(batch):]
		found, err := h.askMaildForAddresses(ctx, batch)
		if err != nil {
			log.Printf("proxy: publisher addresses: %v", err)
			return nil, errPublisherAddressUnavailable
		}
		for _, owner := range batch {
			if addr := strings.TrimSpace(found[owner]); addr != "" {
				h.publisherAddresses.put(owner, addr, now)
				out[owner] = addr
			}
		}
	}
	return out, nil
}

func (h *Handler) askMaildForAddresses(ctx context.Context, owners []string) (map[string]string, error) {
	base, err := joinBasePath(h.cfg.MaildURL, publicAddressesPath)
	if err != nil {
		return nil, err
	}
	body, _ := json.Marshal(map[string]any{"owner_ids": owners})
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, base, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	hostauth.MarkInternal(request)
	client := h.maild
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, errors.New("maild answered status " + http.StatusText(response.StatusCode))
	}
	var answer struct {
		Addresses map[string]string `json:"addresses"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&answer); err != nil {
		return nil, err
	}
	return answer.Addresses, nil
}

// requirePublisherAddress refuses a publish by an account with no numbered
// address, and fails visibly when maild cannot say. It writes the answer and
// reports false in either case.
func (h *Handler) requirePublisherAddress(w http.ResponseWriter, r *http.Request, ownerID string) bool {
	found, err := h.publisherAddressesFor(r.Context(), []string{ownerID})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "publisher_address_unavailable: the address service did not answer, so this release cannot be published yet; try again in a moment", "reason": "publisher_address_unavailable"})
		return false
	}
	if found[ownerID] == "" {
		log.Printf("proxy: publisher address missing for publishing owner %s", ownerID)
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "publisher_address_missing: this account has no Choir address, and a published app shows its maker by that address", "reason": "publisher_address_missing"})
		return false
	}
	return true
}

type publisherAddressFailure struct {
	reason string
	detail string
}

// decoratePublisherAddress adds publisher_address and own to each release in
// a catalog answer: each package entry by its track, each release of a
// history and a single release by its record's publisher. Only owners named
// by what the answer actually holds are looked up. The raw JSON is returned
// unchanged when it does not parse as an object, and a failure (maild down,
// or a listed release whose publisher has no number) is returned for the
// caller to answer as an error.
func (h *Handler) decoratePublisherAddress(ctx context.Context, raw []byte, reader string) ([]byte, *publisherAddressFailure) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		return raw, nil
	}
	var entries, releases []map[string]any
	if packages, ok := doc["packages"]; ok && json.Unmarshal(packages, &entries) != nil {
		entries = nil
	}
	if list, ok := doc["releases"]; ok && json.Unmarshal(list, &releases) != nil {
		releases = nil
	}
	var single map[string]any
	if recordRaw, ok := doc["record"]; ok {
		var record map[string]any
		if json.Unmarshal(recordRaw, &record) == nil {
			single = record
		}
	}
	ownerOf := func(rel map[string]any) string {
		record, _ := rel["record"].(map[string]any)
		return publisherOwner(record)
	}
	var owners []string
	seen := map[string]bool{}
	add := func(owner string) {
		if owner != "" && !seen[owner] {
			seen[owner] = true
			owners = append(owners, owner)
		}
	}
	for _, e := range entries {
		owner, _ := e["track"].(string)
		add(owner)
	}
	for _, rel := range releases {
		add(ownerOf(rel))
	}
	if single != nil {
		add(publisherOwner(single))
	}
	if len(owners) == 0 {
		return raw, nil
	}
	addrs, err := h.publisherAddressesFor(ctx, owners)
	if err != nil {
		return nil, &publisherAddressFailure{reason: "publisher_address_unavailable", detail: "the address service did not answer, so the Library cannot show who made these apps; try again in a moment"}
	}
	var without []string
	for _, owner := range owners {
		if addrs[owner] == "" {
			without = append(without, owner)
		}
	}
	if len(without) > 0 {
		log.Printf("proxy: DEFECT: listed releases whose publisher has no numbered address: %s", strings.Join(without, ", "))
		return nil, &publisherAddressFailure{reason: "publisher_address_missing", detail: "listed releases have a publisher with no Choir address (" + strings.Join(without, ", ") + "); this is a defect, not an empty answer"}
	}
	put := func(m map[string]any, owner string) {
		m["publisher_address"] = addrs[owner]
		m["own"] = owner == reader
	}
	if entries != nil {
		for _, e := range entries {
			owner, _ := e["track"].(string)
			put(e, owner)
		}
		doc["packages"], _ = json.Marshal(entries)
	}
	if releases != nil {
		for _, rel := range releases {
			put(rel, ownerOf(rel))
		}
		doc["releases"], _ = json.Marshal(releases)
	}
	if single != nil {
		top := map[string]any{}
		put(top, publisherOwner(single))
		for k, v := range top {
			doc[k], _ = json.Marshal(v)
		}
	}
	out, err := json.Marshal(doc)
	if err != nil {
		return raw, nil
	}
	return out, nil
}

func publisherOwner(record map[string]any) string {
	publisher, _ := record["publisher"].(map[string]any)
	owner, _ := publisher["owner_id"].(string)
	return strings.TrimSpace(owner)
}
