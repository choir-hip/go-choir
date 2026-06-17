package promptspec

import (
	"fmt"
	"strings"
	"text/template"

	"gopkg.in/yaml.v2"
)

// Document is a versioned agent prompt with metadata and multiline body text.
type Document struct {
	Version  int               `yaml:"version"`
	Role     string            `yaml:"role,omitempty"`
	Flags    map[string]bool   `yaml:"flags,omitempty"`
	Comments []string          `yaml:"comments,omitempty"`
	Body     string            `yaml:"body"`
}

// Parse decodes a prompt YAML document and normalizes body text.
func Parse(data []byte) (Document, error) {
	var doc Document
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return Document{}, fmt.Errorf("parse prompt yaml: %w", err)
	}
	doc.Body = strings.TrimSpace(doc.Body)
	if doc.Version <= 0 {
		return Document{}, fmt.Errorf("parse prompt yaml: version must be positive")
	}
	if strings.TrimSpace(doc.Body) == "" {
		return Document{}, fmt.Errorf("parse prompt yaml: body is required")
	}
	return doc, nil
}

// BodyText returns the prompt body suitable for runtime injection.
func (d Document) BodyText() string {
	return d.Body
}

// Render executes the prompt body as a Go text/template against data.
func (d Document) Render(data any) (string, error) {
	tmpl, err := template.New("prompt").Option("missingkey=zero").Parse(d.Body)
	if err != nil {
		return "", fmt.Errorf("parse prompt template: %w", err)
	}
	var b strings.Builder
	if err := tmpl.Execute(&b, data); err != nil {
		return "", fmt.Errorf("execute prompt template: %w", err)
	}
	return strings.TrimSpace(b.String()), nil
}

// MustRender executes a template and panics on failure.
func (d Document) MustRender(data any) string {
	out, err := d.Render(data)
	if err != nil {
		panic(err)
	}
	return out
}

// ParseAndRender decodes YAML and renders the body template in one step.
func ParseAndRender(data []byte, vars any) (string, error) {
	doc, err := Parse(data)
	if err != nil {
		return "", err
	}
	return doc.Render(vars)
}
