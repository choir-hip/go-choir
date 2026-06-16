package runtime

import (
	"encoding/json"
	"strings"
	"unicode"

	diffmatchpatch "github.com/sergi/go-diff/diffmatchpatch"

	"github.com/yusefmosiah/go-choir/internal/markdownstructure"
)

func carryForwardDurableVTextMetadata(raw, parentRaw json.RawMessage) json.RawMessage {
	parentMeta := decodeRevisionMetadata(parentRaw)
	meta := decodeRevisionMetadata(raw)
	if meta == nil {
		meta = map[string]any{}
	}
	changed := false
	if promoteCanonicalTextureSourcePath(meta, meta) {
		changed = true
	}
	if len(parentMeta) == 0 {
		if !changed {
			return raw
		}
		data, err := json.Marshal(meta)
		if err != nil {
			return raw
		}
		return data
	}
	for _, key := range durableMetadataKeys {
		if hasNonEmptyVTextMetadataValue(meta[key]) {
			continue
		}
		val, ok := parentMeta[key]
		if !ok || !hasNonEmptyVTextMetadataValue(val) {
			continue
		}
		meta[key] = val
		changed = true
	}
	if promoteCanonicalTextureSourcePath(meta, parentMeta) {
		changed = true
	}
	if !changed {
		return raw
	}
	data, err := json.Marshal(meta)
	if err != nil {
		return raw
	}
	return data
}

func promoteCanonicalTextureSourcePath(meta map[string]any, source map[string]any) bool {
	if meta == nil || source == nil || hasNonEmptyVTextMetadataValue(meta[canonicalTextureSourcePathMetadataKey]) {
		return false
	}
	if val, ok := canonicalTextureSourcePathMetadataValue(source); ok {
		meta[canonicalTextureSourcePathMetadataKey] = val
		return true
	}
	return false
}

func canonicalTextureSourcePathMetadataValue(source map[string]any) (any, bool) {
	if source == nil {
		return nil, false
	}
	if val, ok := source[canonicalTextureSourcePathMetadataKey]; ok && hasNonEmptyVTextMetadataValue(val) {
		return val, true
	}
	if val, ok := source[legacyCanonicalVTextSourcePathKey]; ok && hasNonEmptyVTextMetadataValue(val) {
		return val, true
	}
	return nil, false
}

func hasNonEmptyVTextMetadataValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(typed) != ""
	case []any:
		return len(typed) > 0
	case []vtextSourceEntity:
		return len(typed) > 0
	case []vtextMediaSourceRef:
		return len(typed) > 0
	default:
		return true
	}
}

func rebaseUserDraftContent(baseContent, headContent, userContent, staleParentID string) (string, string, bool) {
	if userContent == baseContent {
		return headContent, "no_user_change", true
	}
	if headContent == baseContent {
		return userContent, "head_unchanged", true
	}
	dmp := diffmatchpatch.New()
	patches := dmp.PatchMake(baseContent, userContent)
	merged, applied := dmp.PatchApply(patches, headContent)
	clean := len(applied) > 0
	for _, ok := range applied {
		if !ok {
			clean = false
			break
		}
	}
	if clean {
		return merged, "diff_match_patch", true
	}
	recovered := strings.TrimRight(headContent, "\n") +
		"\n\n---\n\nRecovered user draft based on revision " + staleParentID + ":\n\n" +
		strings.TrimSpace(userContent) + "\n"
	return recovered, "append_recovered_draft", false
}

type markdownTableBlock struct {
	Text      string
	Cells     []string
	StartLine int
	EndLine   int
}

func stabilizeVTextUserMarkdownStructures(parentContent, userContent string) (string, bool) {
	parentTables := extractMarkdownTableBlocks(parentContent)
	if len(parentTables) == 0 {
		return userContent, false
	}
	userTables := extractMarkdownTableBlocks(userContent)
	if len(userTables) >= len(parentTables) {
		return markdownstructure.NormalizeTableShapedRows(userContent)
	}
	out := userContent
	changed := false
	for _, table := range parentTables {
		if strings.Contains(out, table.Text) {
			continue
		}
		next, ok := replaceCollapsedMarkdownTable(out, table)
		if ok {
			out = next
			changed = true
			continue
		}
		next, ok = restoreOmittedParentMarkdownTable(parentContent, out, table)
		if ok {
			out = next
			changed = true
		}
	}
	next, normalized := markdownstructure.NormalizeTableShapedRows(out)
	if normalized {
		out = next
		changed = true
	}
	return out, changed
}

func extractMarkdownTableBlocks(content string) []markdownTableBlock {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	var blocks []markdownTableBlock
	for i := 0; i < len(lines); {
		if markdownstructure.TableRowCells(lines[i]) == nil {
			i++
			continue
		}
		start := i
		for i < len(lines) && markdownstructure.TableRowCells(lines[i]) != nil {
			i++
		}
		tableLines := lines[start:i]
		if len(tableLines) < 3 {
			continue
		}
		separator := markdownstructure.TableRowCells(tableLines[1])
		if !markdownstructure.IsTableSeparatorCells(separator) {
			continue
		}
		var cells []string
		for _, line := range tableLines {
			rowCells := markdownstructure.TableRowCells(line)
			if markdownstructure.IsTableSeparatorCells(rowCells) {
				continue
			}
			for _, cell := range rowCells {
				cell = strings.TrimSpace(cell)
				if cell != "" {
					cells = append(cells, cell)
				}
			}
		}
		blocks = append(blocks, markdownTableBlock{
			Text:      strings.Join(tableLines, "\n"),
			Cells:     cells,
			StartLine: start,
			EndLine:   i - 1,
		})
	}
	return blocks
}

func restoreOmittedParentMarkdownTable(parentContent, userContent string, table markdownTableBlock) (string, bool) {
	if strings.Contains(userContent, table.Text) || comparableMarkdownBlockProjection(parentWithoutMarkdownTable(parentContent, table)) == comparableMarkdownBlockProjection(userContent) {
		return userContent, false
	}
	parentLines := strings.Split(strings.ReplaceAll(parentContent, "\r\n", "\n"), "\n")
	userLines := strings.Split(strings.ReplaceAll(userContent, "\r\n", "\n"), "\n")
	beforeAnchor := nearestNonEmptyLine(parentLines, table.StartLine-1, -1)
	afterAnchor := nearestNonEmptyLine(parentLines, table.EndLine+1, 1)
	insertAt := -1
	if afterAnchor != "" {
		if idx := indexLine(userLines, afterAnchor); idx >= 0 {
			insertAt = idx
		}
	}
	if insertAt < 0 && beforeAnchor != "" {
		if idx := indexLine(userLines, beforeAnchor); idx >= 0 {
			insertAt = idx + 1
		}
	}
	if insertAt < 0 {
		return userContent, false
	}
	tableLines := strings.Split(table.Text, "\n")
	out := make([]string, 0, len(userLines)+len(tableLines)+2)
	out = append(out, userLines[:insertAt]...)
	if len(out) > 0 && strings.TrimSpace(out[len(out)-1]) != "" {
		out = append(out, "")
	}
	out = append(out, tableLines...)
	if insertAt < len(userLines) && strings.TrimSpace(userLines[insertAt]) != "" {
		out = append(out, "")
	}
	out = append(out, userLines[insertAt:]...)
	return strings.Join(out, "\n"), true
}

func parentWithoutMarkdownTable(parentContent string, table markdownTableBlock) string {
	lines := strings.Split(strings.ReplaceAll(parentContent, "\r\n", "\n"), "\n")
	if table.StartLine < 0 || table.EndLine < table.StartLine || table.EndLine >= len(lines) {
		return parentContent
	}
	out := make([]string, 0, len(lines)-(table.EndLine-table.StartLine+1))
	out = append(out, lines[:table.StartLine]...)
	out = append(out, lines[table.EndLine+1:]...)
	return strings.Join(out, "\n")
}

func nearestNonEmptyLine(lines []string, start, step int) string {
	for i := start; i >= 0 && i < len(lines); i += step {
		if text := strings.TrimSpace(lines[i]); text != "" {
			return lines[i]
		}
	}
	return ""
}

func indexLine(lines []string, target string) int {
	for i, line := range lines {
		if line == target {
			return i
		}
	}
	return -1
}

func comparableMarkdownBlockProjection(content string) string {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(lines))
	previousBlank := true
	for _, line := range lines {
		text := strings.TrimSpace(line)
		if text == "" {
			if !previousBlank {
				out = append(out, "")
			}
			previousBlank = true
			continue
		}
		out = append(out, text)
		previousBlank = false
	}
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return strings.Join(out, "\n")
}

func replaceCollapsedMarkdownTable(content string, table markdownTableBlock) (string, bool) {
	if len(table.Cells) < 4 {
		return content, false
	}
	startNeedle := collapsedTableNeedle(table.Cells[:minInt(4, len(table.Cells))])
	if startNeedle == "" {
		return content, false
	}
	start := strings.Index(collapsedComparableText(content), startNeedle)
	if start < 0 {
		return content, false
	}
	originalStart, ok := comparableBoundaryToOriginalIndex(content, start)
	if !ok {
		return content, false
	}
	lastCells := table.Cells
	if len(lastCells) > 4 {
		lastCells = lastCells[len(lastCells)-4:]
	}
	endNeedle := collapsedTableNeedle(lastCells)
	endComparable := collapsedComparableText(content[originalStart:])
	end := strings.Index(endComparable, endNeedle)
	if end < 0 {
		return content, false
	}
	originalEndRel, ok := comparableBoundaryToOriginalIndex(content[originalStart:], end+len(endNeedle))
	if !ok {
		return content, false
	}
	originalEnd := originalStart + originalEndRel
	return strings.TrimRight(content[:originalStart], " \t") + table.Text + strings.TrimLeft(content[originalEnd:], " \t"), true
}

func collapsedTableNeedle(cells []string) string {
	return collapsedComparableText(strings.Join(cells, ""))
}

func collapsedComparableText(value string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func comparableBoundaryToOriginalIndex(value string, comparableBoundary int) (int, bool) {
	if comparableBoundary <= 0 {
		return 0, true
	}
	seen := 0
	for index, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if seen == comparableBoundary {
				return index, true
			}
			seen++
			if seen == comparableBoundary {
				return index + len(string(r)), true
			}
		}
	}
	return len(value), seen == comparableBoundary
}
