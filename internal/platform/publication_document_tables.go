package platform

import "strings"

func isMarkdownTableStart(lines []string, i int) bool {
	if i+1 >= len(lines) {
		return false
	}
	return strings.Contains(lines[i], "|") && markdownTableSeparator(lines[i+1])
}

func markdownTableSeparator(line string) bool {
	clean := strings.TrimSpace(strings.Trim(line, "|"))
	if clean == "" {
		return false
	}
	for _, part := range strings.Split(clean, "|") {
		part = strings.TrimSpace(part)
		if len(part) < 3 {
			return false
		}
		for _, r := range part {
			if r != '-' && r != ':' {
				return false
			}
		}
	}
	return true
}

func splitMarkdownTableRow(line string) []string {
	line = strings.TrimSpace(strings.Trim(line, "|"))
	parts := strings.Split(line, "|")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		out = append(out, strings.TrimSpace(strings.ReplaceAll(part, `\|`, "|")))
	}
	return out
}
