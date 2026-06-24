package texturedoc

import (
	"fmt"
	"strings"
)

type Projection struct {
	Text       string
	SourceRefs []ProjectedSourceRef
}

type ProjectedSourceRef struct {
	Number         int
	NodeID         string
	SourceEntityID string
	DisplayMode    string
	Label          string
	Title          string
	TargetKind     string
}

func Project(doc StructuredTextureDoc, entities []SourceEntity, unusedSourceEntityIDs ...string) (Projection, error) {
	if err := Validate(doc, entities, unusedSourceEntityIDs...); err != nil {
		return Projection{}, err
	}
	entityByID := make(map[string]SourceEntity, len(entities))
	for _, entity := range entities {
		entityByID[entity.SourceEntityID] = entity
	}
	projector := projector{
		entities: entityByID,
		numbers:  make(map[string]int),
	}
	for i := range doc.Doc.Content {
		projector.renderBlock(doc.Doc.Content[i], 0)
		if i < len(doc.Doc.Content)-1 {
			projector.writeParagraphBreak()
		}
	}
	return Projection{
		Text:       strings.TrimRight(projector.builder.String(), "\n"),
		SourceRefs: projector.refs,
	}, nil
}

type projector struct {
	builder  strings.Builder
	entities map[string]SourceEntity
	numbers  map[string]int
	refs     []ProjectedSourceRef
}

func (p *projector) renderBlock(node Node, depth int) {
	switch node.Type {
	case "paragraph":
		p.renderInlineContent(node.Content)
	case "heading":
		level, _ := intAttr(node, "level")
		p.builder.WriteString(strings.Repeat("#", level))
		p.builder.WriteByte(' ')
		p.renderInlineContent(node.Content)
	case "bullet_list":
		for i, item := range node.Content {
			p.builder.WriteString(strings.Repeat("  ", depth))
			p.builder.WriteString("- ")
			p.renderListItem(item, depth)
			if i < len(node.Content)-1 {
				p.builder.WriteByte('\n')
			}
		}
	case "ordered_list":
		start, ok := intAttr(node, "start")
		if !ok {
			start = 1
		}
		for i, item := range node.Content {
			p.builder.WriteString(strings.Repeat("  ", depth))
			p.builder.WriteString(fmt.Sprintf("%d. ", start+i))
			p.renderListItem(item, depth)
			if i < len(node.Content)-1 {
				p.builder.WriteByte('\n')
			}
		}
	case "list_item":
		p.renderListItem(node, depth)
	case "blockquote":
		var nested projector
		nested.entities = p.entities
		nested.numbers = p.numbers
		for i := range node.Content {
			nested.renderBlock(node.Content[i], depth)
			if i < len(node.Content)-1 {
				nested.writeParagraphBreak()
			}
		}
		lines := strings.Split(strings.TrimRight(nested.builder.String(), "\n"), "\n")
		for i, line := range lines {
			p.builder.WriteString("> ")
			p.builder.WriteString(line)
			if i < len(lines)-1 {
				p.builder.WriteByte('\n')
			}
		}
		p.refs = append(p.refs, nested.refs...)
	case "code_block":
		language := stringAttr(node, "language")
		p.builder.WriteString("```")
		p.builder.WriteString(language)
		p.builder.WriteByte('\n')
		for i, child := range node.Content {
			p.builder.WriteString(child.Text)
			if i < len(node.Content)-1 {
				p.builder.WriteByte('\n')
			}
		}
		p.builder.WriteString("\n```")
	case "horizontal_rule":
		p.builder.WriteString("---")
	}
}

func (p *projector) renderListItem(node Node, depth int) {
	for i := range node.Content {
		if i > 0 {
			p.builder.WriteByte('\n')
			p.builder.WriteString(strings.Repeat("  ", depth+1))
		}
		p.renderBlock(node.Content[i], depth+1)
	}
}

func (p *projector) renderInlineContent(nodes []Node) {
	for i := range nodes {
		node := nodes[i]
		switch node.Type {
		case "text":
			p.builder.WriteString(node.Text)
		case "hard_break":
			p.builder.WriteByte('\n')
		case "source_ref":
			sourceEntityID := stringAttr(node, "source_entity_id")
			entity := p.entities[sourceEntityID]
			number := p.numberFor(sourceEntityID)
			displayMode := stringAttr(node, "display_mode")
			if displayMode == "expanded_ref" {
				title := sourceTitle(entity)
				p.builder.WriteString(fmt.Sprintf("[source %d: %s]", number, title))
			} else {
				p.builder.WriteString(fmt.Sprintf("[%d]", number))
			}
			p.refs = append(p.refs, ProjectedSourceRef{
				Number:         number,
				NodeID:         nodeID(node),
				SourceEntityID: sourceEntityID,
				DisplayMode:    displayMode,
				Label:          stringAttr(node, "label"),
				Title:          sourceTitle(entity),
				TargetKind:     entity.Target.Kind,
			})
		}
	}
}

func (p *projector) writeParagraphBreak() {
	p.builder.WriteString("\n\n")
}

func (p *projector) numberFor(sourceEntityID string) int {
	if existing := p.numbers[sourceEntityID]; existing != 0 {
		return existing
	}
	number := len(p.numbers) + 1
	p.numbers[sourceEntityID] = number
	return number
}

func sourceTitle(entity SourceEntity) string {
	if entity.Display.Title != "" {
		return entity.Display.Title
	}
	if entity.Display.Label != "" {
		return entity.Display.Label
	}
	if entity.Target.ID != "" {
		return entity.Target.ID
	}
	if entity.Target.URI != "" {
		return entity.Target.URI
	}
	return entity.SourceEntityID
}
