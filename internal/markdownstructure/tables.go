package markdownstructure

import "strings"

// NormalizeTableShapedRows repairs table rows that keep the leading pipe and
// cell separators but lost the final delimiter while inside a Markdown table.
func NormalizeTableShapedRows(content string) (string, bool) {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	changed := false
	inTable := false
	tableHasSeparator := false
	for i, line := range lines {
		strictCells := TableRowCells(line)
		if strictCells != nil {
			inTable = true
			if IsTableSeparatorCells(strictCells) {
				tableHasSeparator = true
			}
			continue
		}
		if strings.TrimSpace(line) == "" {
			inTable = false
			tableHasSeparator = false
			continue
		}
		if !inTable || !tableHasSeparator {
			continue
		}
		if TableShapedRowCells(line) == nil {
			inTable = false
			tableHasSeparator = false
			continue
		}
		lines[i] = strings.TrimRight(line, " \t") + " |"
		changed = true
	}
	if !changed {
		return content, false
	}
	return strings.Join(lines, "\n"), true
}

func TableRowCells(line string) []string {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "|") || !strings.HasSuffix(trimmed, "|") {
		return nil
	}
	parts := SplitTableCells(strings.Trim(trimmed, "|"))
	if len(parts) < 2 {
		return nil
	}
	cells := make([]string, 0, len(parts))
	for _, part := range parts {
		cells = append(cells, strings.TrimSpace(strings.ReplaceAll(part, `\|`, "|")))
	}
	return cells
}

func TableShapedRowCells(line string) []string {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "|") || strings.HasSuffix(trimmed, "|") {
		return nil
	}
	parts := SplitTableCells(strings.TrimPrefix(trimmed, "|"))
	if len(parts) < 2 {
		return nil
	}
	cells := make([]string, 0, len(parts))
	for _, part := range parts {
		cells = append(cells, strings.TrimSpace(strings.ReplaceAll(part, `\|`, "|")))
	}
	return cells
}

func SplitTableCells(value string) []string {
	var cells []string
	var cell strings.Builder
	escaped := false
	for _, r := range value {
		if escaped {
			cell.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			cell.WriteRune(r)
			escaped = true
			continue
		}
		if r == '|' {
			cells = append(cells, cell.String())
			cell.Reset()
			continue
		}
		cell.WriteRune(r)
	}
	cells = append(cells, cell.String())
	return cells
}

func IsTableSeparatorCells(cells []string) bool {
	if len(cells) == 0 {
		return false
	}
	for _, cell := range cells {
		if !isTableSeparatorCell(strings.TrimSpace(cell)) {
			return false
		}
	}
	return true
}

func isTableSeparatorCell(cell string) bool {
	if len(cell) < 3 {
		return false
	}
	start := 0
	end := len(cell)
	if cell[start] == ':' {
		start++
	}
	if end > start && cell[end-1] == ':' {
		end--
	}
	if end-start < 3 {
		return false
	}
	for _, r := range cell[start:end] {
		if r != '-' {
			return false
		}
	}
	return true
}
