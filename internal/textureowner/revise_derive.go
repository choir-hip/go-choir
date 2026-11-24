package textureowner

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/texturedoc"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type derivedOwnerBody struct {
	bodyDoc, sourceEntities json.RawMessage
	content                 string
}

// deriveOwnerBody is `derive_from_markdown`: the owner's typed Markdown and the parent revision (none for a document with no revision yet) become the new body_doc, its source entities and the content they
// project to. texturedoc.MergeMarkdown keeps every node of the parent the text did not touch; the store then checks the result as it checks any body (prepareTextureRevision).
func deriveOwnerBody(parent types.Revision, hasParent bool, markdown string) (derivedOwnerBody, error) {
	var parentDoc texturedoc.StructuredTextureDoc
	var parentEntities []texturedoc.SourceEntity
	parentContent := ""
	if hasParent {
		parentContent = parent.Content
		if len(strings.TrimSpace(string(parent.BodyDoc))) > 0 {
			if err := json.Unmarshal(parent.BodyDoc, &parentDoc); err != nil {
				return derivedOwnerBody{}, fmt.Errorf("the parent revision's body_doc cannot be read: %v", err)
			}
		}
		if raw := strings.TrimSpace(string(parent.SourceEntities)); raw != "" && raw != "null" {
			if err := json.Unmarshal(parent.SourceEntities, &parentEntities); err != nil {
				return derivedOwnerBody{}, fmt.Errorf("the parent revision's source_entities cannot be read: %v", err)
			}
		}
	}
	doc, entities := texturedoc.MergeMarkdown(parentDoc, parentEntities, parentContent, markdown)
	projection, err := texturedoc.Project(doc, entities)
	if err != nil {
		return derivedOwnerBody{}, fmt.Errorf("invalid texture revision: %v", err)
	}
	bodyDoc, err := json.Marshal(doc)
	if err != nil {
		return derivedOwnerBody{}, err
	}
	out := derivedOwnerBody{bodyDoc: bodyDoc, content: projection.Text}
	if len(entities) > 0 {
		if out.sourceEntities, err = json.Marshal(entities); err != nil {
			return derivedOwnerBody{}, err
		}
	}
	return out, nil
}
