package yaegikernel

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/yusefmosiah/go-choir/internal/coagentpacket"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// reportPacketShape is the minimal well-formed packet, returned with every
// refusal so the desk can correct the packet in its next cell.
const reportPacketShape = `packet shape: {"kind":"evidence_update","summary":"...",` +
	`"claims":[{"text":"...","source_ids":["s1"]}],` +
	`"sources":[{"source_id":"s1","kind":"web_source","target":{"uri":"https://...","title":"..."},` +
	`"selectors":[{"kind":"text_quote","quote":"..."}],"excerpt":"..."}],"questions":["..."]}`

// checkReportPacket validates a ReportPacket body at the call with the same
// contract the reducer applies at commit (coagentpacket), and defaults
// schema_version. A malformed packet fails in the cell that wrote it instead
// of after it (docs/archive/problems/research-report-packets-rejected-by-schema-2026-10-09.md).
func checkReportPacket(packetJSON string) (string, error) {
	var packet types.SourcePacket
	decoder := json.NewDecoder(strings.NewReader(packetJSON))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&packet); err != nil {
		return "", fmt.Errorf("choir: ReportPacket: %v; %s", err, reportPacketShape)
	}
	if decoder.More() {
		return "", fmt.Errorf("choir: ReportPacket: trailing data after the packet; %s", reportPacketShape)
	}
	if strings.TrimSpace(packet.SchemaVersion) == "" {
		packet.SchemaVersion = types.CoagentSourcePacketSchemaV1
	}
	if err := coagentpacket.Validate(coagentpacket.Normalize(packet)); err != nil {
		return "", fmt.Errorf("choir: ReportPacket: %v; %s", err, reportPacketShape)
	}
	raw, err := json.Marshal(packet)
	if err != nil {
		return "", fmt.Errorf("choir: ReportPacket: encode packet: %w", err)
	}
	return string(raw), nil
}

// liftPacketAliases reads two claim fields a desk keeps sending as what they
// mean, so the packet is not refused whole for them (docs/problems/texture-
// story-ask-path-writes-no-fast-v1-and-research-delivers-once-2026-11-16.md:
// two refusals, "source_refs" then "status", cost a minute before the first
// packet):
//
//   - claims[].source_refs is claims[].source_ids, when the claim names no
//     source_ids;
//   - claims[].status ("Give each claim one status", the research prompt says)
//     is kept in the claim's text as a leading "(status) ", where Texture
//     reads it, since a claim has no field for it.
//
// Nothing else is accepted: every other unknown field is still refused by the
// strict decode, so a typo (claim for text) never drops data silently and a
// field a stranger invents never reaches the reducer. A source_refs that names
// no source of the packet is still refused by the contract's source check.
func liftPacketAliases(packetJSON string) string {
	dec := json.NewDecoder(strings.NewReader(packetJSON))
	dec.UseNumber()
	var packet map[string]any
	if err := dec.Decode(&packet); err != nil || dec.More() {
		return packetJSON // the strict decode names the problem
	}
	claims, _ := packet["claims"].([]any)
	changed := false
	for _, c := range claims {
		claim, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if refs, has := claim["source_refs"]; has {
			if _, own := claim["source_ids"]; !own {
				claim["source_ids"] = refs
				delete(claim, "source_refs")
				changed = true
			}
		}
		if status, has := claim["status"]; has {
			label, _ := status.(string)
			label = strings.TrimSpace(label)
			text, _ := claim["text"].(string)
			if label != "" && text != "" && len([]rune(label)) <= 40 && !strings.ContainsFunc(label, unicode.IsControl) {
				claim["text"] = "(" + label + ") " + text
				delete(claim, "status")
				changed = true
			}
		}
	}
	if !changed {
		return packetJSON
	}
	raw, err := json.Marshal(packet)
	if err != nil {
		return packetJSON
	}
	return string(raw)
}
