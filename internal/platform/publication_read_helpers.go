package platform

import (
	"fmt"
	"strings"
)

func renderBlocks(content string, spans []RetrievalSpan) []RenderBlock {
	blocks := []RenderBlock{}
	lines := strings.SplitAfter(content, "\n")
	var paragraph strings.Builder
	start := 0
	cursor := 0
	flush := func(end int) {
		text := strings.TrimSpace(paragraph.String())
		if text == "" {
			paragraph.Reset()
			return
		}
		kind := "paragraph"
		if strings.HasPrefix(text, "#") {
			kind = "heading"
		} else if strings.HasPrefix(text, "- ") || strings.HasPrefix(text, "* ") {
			kind = "list"
		}
		spanID := ""
		textHash := ""
		if len(spans) > 0 {
			spanID = spans[0].ID
			textHash = sha256Hex([]byte(text))
		}
		blocks = append(blocks, RenderBlock{
			ID:       fmt.Sprintf("block-%d", len(blocks)+1),
			Kind:     kind,
			Text:     text,
			Start:    start,
			End:      end,
			SpanID:   spanID,
			TextHash: textHash,
		})
		paragraph.Reset()
	}
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			flush(cursor)
			cursor += len([]rune(line))
			start = cursor
			continue
		}
		if paragraph.Len() == 0 {
			start = cursor
		}
		paragraph.WriteString(line)
		cursor += len([]rune(line))
	}
	flush(cursor)
	if len(blocks) == 0 && strings.TrimSpace(content) != "" {
		blocks = append(blocks, RenderBlock{
			ID:       "block-1",
			Kind:     "paragraph",
			Text:     strings.TrimSpace(content),
			Start:    0,
			End:      len([]rune(content)),
			TextHash: sha256Hex([]byte(strings.TrimSpace(content))),
		})
	}
	return blocks
}

func snippet(content, query string) string {
	content = strings.TrimSpace(content)
	if len([]rune(content)) <= 260 {
		return content
	}
	lowerContent := strings.ToLower(content)
	lowerQuery := strings.ToLower(strings.TrimSpace(query))
	start := 0
	if lowerQuery != "" {
		if idx := strings.Index(lowerContent, lowerQuery); idx >= 0 {
			start = idx - 90
			if start < 0 {
				start = 0
			}
		}
	}
	runes := []rune(content)
	if start > len(runes) {
		start = 0
	}
	end := start + 240
	if end > len(runes) {
		end = len(runes)
	}
	prefix := ""
	suffix := ""
	if start > 0 {
		prefix = "..."
	}
	if end < len(runes) {
		suffix = "..."
	}
	return prefix + strings.TrimSpace(string(runes[start:end])) + suffix
}
