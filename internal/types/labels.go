package types

import (
	"encoding/json"
	"sort"
	"strings"
)

// Provenance labels (MA3.5, R15; docs/design/modular-architecture-plan-2026-11-04.md
// section 7.2, "Labels"). Every record, object body, capture-derived text and
// model context segment carries a label set. The lattice is a powerset of
// atoms under union: a derived value carries the union of its inputs'
// labels, and nothing ever removes a label.
//
//	owner_private:<kind>   data the owner holds privately, by the kind of
//	                       the object it came from (capture, texture, ...)
//	world_untrusted        text from outside the computer: content the model
//	                       must treat as data, never as instructions
//	platform               data the platform itself authored
//	publisher:<id>         content a named publisher authored
//
// Labels are stamped by the host (the per-run accumulator and the reducer).
// A cell can neither set nor clear one.
const (
	LabelWorldUntrusted     = "world_untrusted"
	LabelPlatform           = "platform"
	LabelOwnerPrivatePrefix = "owner_private:"
	LabelPublisherPrefix    = "publisher:"

	// LabelsMetadataKey is where an object's label set rides in its
	// metadata object. The host overwrites it on every write.
	LabelsMetadataKey = "labels"
)

// OwnerPrivateLabel is the owner_private label of an object kind.
func OwnerPrivateLabel(kind string) string {
	return LabelOwnerPrivatePrefix + strings.TrimSpace(kind)
}

// PublisherLabel is the publisher label of a publisher id.
func PublisherLabel(id string) string {
	return LabelPublisherPrefix + strings.TrimSpace(id)
}

// ValidLabel reports whether l is an atom of the lattice.
func ValidLabel(l string) bool {
	switch {
	case l == LabelWorldUntrusted, l == LabelPlatform:
		return true
	case strings.HasPrefix(l, LabelOwnerPrivatePrefix):
		return validLabelSuffix(l[len(LabelOwnerPrivatePrefix):])
	case strings.HasPrefix(l, LabelPublisherPrefix):
		return validLabelSuffix(l[len(LabelPublisherPrefix):])
	}
	return false
}

func validLabelSuffix(s string) bool {
	if s == "" || len(s) > 128 {
		return false
	}
	for _, r := range s {
		if r <= ' ' || r == 0x7f {
			return false
		}
	}
	return true
}

// LabelSet is a canonical label set: valid atoms only, sorted, no
// duplicates. The zero value is the empty set.
type LabelSet []string

// NewLabelSet builds the canonical set of the valid atoms among labels;
// anything outside the lattice is dropped.
func NewLabelSet(labels ...string) LabelSet {
	seen := make(map[string]struct{}, len(labels))
	out := make(LabelSet, 0, len(labels))
	for _, l := range labels {
		l = strings.TrimSpace(l)
		if !ValidLabel(l) {
			continue
		}
		if _, dup := seen[l]; dup {
			continue
		}
		seen[l] = struct{}{}
		out = append(out, l)
	}
	sort.Strings(out)
	if len(out) == 0 {
		return nil
	}
	return out
}

// Union is the join of s and the others: the label set of a value derived
// from all of them.
func (s LabelSet) Union(others ...LabelSet) LabelSet {
	all := append([]string(nil), s...)
	for _, o := range others {
		all = append(all, o...)
	}
	return NewLabelSet(all...)
}

// Has reports whether l is in the set.
func (s LabelSet) Has(l string) bool {
	i := sort.SearchStrings(s, l)
	return i < len(s) && s[i] == l
}

// Untrusted reports whether any label marks content the model must read as
// data: world_untrusted or a publisher's.
func (s LabelSet) Untrusted() bool {
	for _, l := range s {
		if l == LabelWorldUntrusted || strings.HasPrefix(l, LabelPublisherPrefix) {
			return true
		}
	}
	return false
}

// OwnerPrivate reports whether any label is an owner_private label.
func (s LabelSet) OwnerPrivate() bool {
	for _, l := range s {
		if strings.HasPrefix(l, LabelOwnerPrivatePrefix) {
			return true
		}
	}
	return false
}

// Strings is the set as a plain slice (nil when empty), the wire form.
func (s LabelSet) Strings() []string {
	if len(s) == 0 {
		return nil
	}
	return append([]string(nil), s...)
}

// LabelsFromMetadata reads the label set an object's metadata carries.
func LabelsFromMetadata(raw json.RawMessage) LabelSet {
	if len(raw) == 0 {
		return nil
	}
	var m struct {
		Labels []string `json:"labels"`
	}
	if json.Unmarshal(raw, &m) != nil {
		return nil
	}
	return NewLabelSet(m.Labels...)
}

// StampMetadataLabels returns metadata with its label set replaced by
// labels: whatever the writer supplied under the key is discarded, so a
// cell-supplied label never survives. Metadata that is not a JSON object is
// replaced by one holding the labels alone.
func StampMetadataLabels(raw json.RawMessage, labels LabelSet) json.RawMessage {
	m := map[string]json.RawMessage{}
	if len(raw) > 0 {
		if json.Unmarshal(raw, &m) != nil || m == nil {
			m = map[string]json.RawMessage{}
		}
	}
	_, had := m[LabelsMetadataKey]
	if !had && len(labels) == 0 {
		return raw
	}
	delete(m, LabelsMetadataKey)
	if len(labels) > 0 {
		enc, _ := json.Marshal([]string(labels))
		m[LabelsMetadataKey] = enc
	}
	out, err := json.Marshal(m)
	if err != nil {
		return raw
	}
	return out
}
