package proxy

import (
	"encoding/json"
	"strings"
)

// Publisher email on listed releases (owner, 2026-11-04: an app store shows
// who made the app, by the account's email address). The owner asked for the
// address to be public on listed apps; publishing a release makes its
// publisher's account email visible to every signed-in user
// (docs/design/library-app-store-2026-11-04.md). The address is resolved
// here, at the proxy, from the auth store it already reads, and added to the
// catalog answers as a display-only field: corpusd never sees it, no record
// or listing holds it, and nothing acts on it. An owner the store cannot
// resolve gets no field, and the app shows no address.

// decoratePublisherEmail adds publisher_email (and own) to a catalog answer:
// each package entry by its track, a history by its track, a release by its
// record's publisher. The raw JSON is returned unchanged when it does not
// parse as an object.
func (h *Handler) decoratePublisherEmail(raw []byte, reader string) []byte {
	if h.apiKeyValidator == nil {
		return raw
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		return raw
	}
	cache := map[string]string{}
	email := func(owner string) string {
		owner = strings.TrimSpace(owner)
		if owner == "" {
			return ""
		}
		if v, ok := cache[owner]; ok {
			return v
		}
		v := ""
		if user, err := h.apiKeyValidator.GetUserByID(owner); err == nil && user != nil {
			v = strings.TrimSpace(user.Email)
		}
		cache[owner] = v
		return v
	}
	put := func(m map[string]any, owner string) {
		if e := email(owner); e != "" {
			m["publisher_email"] = e
		}
		m["own"] = owner == reader
	}
	changed := false
	if packages, ok := doc["packages"]; ok {
		var entries []map[string]any
		if json.Unmarshal(packages, &entries) == nil {
			for _, entry := range entries {
				owner, _ := entry["track"].(string)
				put(entry, owner)
			}
			doc["packages"], _ = json.Marshal(entries)
			changed = true
		}
	}
	if trackRaw, ok := doc["track"]; ok {
		var track string
		if json.Unmarshal(trackRaw, &track) == nil {
			top := map[string]any{}
			put(top, track)
			for k, v := range top {
				doc[k], _ = json.Marshal(v)
			}
			changed = true
		}
	}
	if recordRaw, ok := doc["record"]; ok {
		var record struct {
			Publisher struct {
				OwnerID string `json:"owner_id"`
			} `json:"publisher"`
		}
		if json.Unmarshal(recordRaw, &record) == nil && record.Publisher.OwnerID != "" {
			top := map[string]any{}
			put(top, record.Publisher.OwnerID)
			for k, v := range top {
				doc[k], _ = json.Marshal(v)
			}
			changed = true
		}
	}
	if !changed {
		return raw
	}
	out, err := json.Marshal(doc)
	if err != nil {
		return raw
	}
	return out
}
