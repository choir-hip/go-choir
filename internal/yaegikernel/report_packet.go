package yaegikernel

import (
	"encoding/json"
	"fmt"
	"strings"

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
// of after it (docs/problems/research-report-packets-rejected-by-schema-2026-10-09.md).
func checkReportPacket(packetJSON string) (string, error) {
	var packet types.CoagentSourcePacketPayload
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
