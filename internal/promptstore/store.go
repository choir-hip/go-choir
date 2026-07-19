package promptstore

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/promptspec"
	"github.com/yusefmosiah/go-choir/internal/textureprompts"
)

//go:embed defaults/*.yaml
var promptDefaultsFS embed.FS

type Descriptor struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Source  string `json:"source"`
	Path    string `json:"path"`
}

type Store struct {
	root string
}

func New(root string) *Store {
	return &Store{root: root}
}

func promptRoles() []string {
	return []string{
		agentprofile.Conductor,
		agentprofile.Texture,
		agentprofile.Researcher,
		agentprofile.Processor,
		agentprofile.Reconciler,
		agentprofile.Super,
		agentprofile.CoSuper,
	}
}

func promptDefaultFiles() []string {
	return append([]string{"core"}, promptRoles()...)
}

func (ps *Store) LoadCore() (string, error) {
	if err := ps.ensureDefaults(); err != nil {
		return "", err
	}
	content, err := os.ReadFile(ps.defaultPromptPath("core"))
	if err != nil {
		return "", fmt.Errorf("read core prompt: %w", err)
	}
	return strings.TrimSpace(string(content)), nil
}

func (ps *Store) List(ownerID string) ([]Descriptor, error) {
	if err := ps.ensureDefaults(); err != nil {
		return nil, err
	}
	prompts := make([]Descriptor, 0, len(promptRoles()))
	for _, role := range promptRoles() {
		prompt, err := ps.Load(ownerID, role)
		if err != nil {
			return nil, err
		}
		prompts = append(prompts, prompt)
	}
	sort.Slice(prompts, func(i, j int) bool {
		return prompts[i].Role < prompts[j].Role
	})
	return prompts, nil
}

func (ps *Store) Load(ownerID, role string) (Descriptor, error) {
	if err := ps.ensureDefaults(); err != nil {
		return Descriptor{}, err
	}
	role, err := normalizePromptRole(role)
	if err != nil {
		return Descriptor{}, err
	}
	userPath := ps.userPromptPath(ownerID, role)
	if ownerID != "" {
		if content, err := os.ReadFile(userPath); err == nil {
			return Descriptor{
				Role:    role,
				Content: strings.TrimSpace(string(content)),
				Source:  "user",
				Path:    userPath,
			}, nil
		} else if !os.IsNotExist(err) {
			return Descriptor{}, fmt.Errorf("read user prompt %s: %w", role, err)
		}
	}
	defaultPath := ps.defaultPromptPath(role)
	content, err := os.ReadFile(defaultPath)
	if err != nil {
		return Descriptor{}, fmt.Errorf("read default prompt %s: %w", role, err)
	}
	return Descriptor{
		Role:    role,
		Content: strings.TrimSpace(string(content)),
		Source:  "default",
		Path:    defaultPath,
	}, nil
}

func (ps *Store) Save(ownerID, role, content string) (Descriptor, error) {
	if strings.TrimSpace(ownerID) == "" {
		return Descriptor{}, fmt.Errorf("owner is required")
	}
	if err := ps.ensureDefaults(); err != nil {
		return Descriptor{}, err
	}
	role, err := normalizePromptRole(role)
	if err != nil {
		return Descriptor{}, err
	}
	if strings.TrimSpace(content) == "" {
		return Descriptor{}, fmt.Errorf("prompt content is required")
	}
	path := ps.userPromptPath(ownerID, role)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return Descriptor{}, fmt.Errorf("create prompt directory: %w", err)
	}
	normalized := strings.TrimSpace(content) + "\n"
	if err := os.WriteFile(path, []byte(normalized), 0o644); err != nil {
		return Descriptor{}, fmt.Errorf("write prompt override: %w", err)
	}
	return Descriptor{
		Role:    role,
		Content: strings.TrimSpace(content),
		Source:  "user",
		Path:    path,
	}, nil
}

func (ps *Store) Reset(ownerID, role string) (Descriptor, error) {
	if strings.TrimSpace(ownerID) == "" {
		return Descriptor{}, fmt.Errorf("owner is required")
	}
	role, err := normalizePromptRole(role)
	if err != nil {
		return Descriptor{}, err
	}
	path := ps.userPromptPath(ownerID, role)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return Descriptor{}, fmt.Errorf("remove prompt override: %w", err)
	}
	return ps.Load(ownerID, role)
}

func normalizePromptRole(role string) (string, error) {
	role = strings.TrimSpace(role)
	for _, allowed := range promptRoles() {
		if role == allowed {
			return role, nil
		}
	}
	return "", fmt.Errorf("unsupported prompt role %q", role)
}

func (ps *Store) ensureDefaults() error {
	if strings.TrimSpace(ps.root) == "" {
		return fmt.Errorf("prompt root is not configured")
	}
	if err := os.MkdirAll(filepath.Join(ps.root, "defaults"), 0o755); err != nil {
		return fmt.Errorf("create prompt defaults directory: %w", err)
	}
	for _, name := range promptDefaultFiles() {
		path := ps.defaultPromptPath(name)
		var content []byte
		if name == agentprofile.Texture {
			content = []byte(textureprompts.DefaultSystemPrompt() + "\n")
		} else {
			raw, readErr := fs.ReadFile(promptDefaultsFS, filepath.ToSlash(filepath.Join("defaults", name+".yaml")))
			if readErr != nil {
				return fmt.Errorf("load embedded prompt default %s: %w", name, readErr)
			}
			doc, parseErr := promptspec.Parse(raw)
			if parseErr != nil {
				return fmt.Errorf("parse embedded prompt default %s: %w", name, parseErr)
			}
			content = []byte(doc.BodyText() + "\n")
		}
		if current, err := os.ReadFile(path); err == nil && string(current) == string(content) {
			continue
		} else if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("read prompt default %s: %w", name, err)
		}
		if err := os.WriteFile(path, content, 0o644); err != nil {
			return fmt.Errorf("seed prompt default %s: %w", name, err)
		}
	}
	return nil
}

func (ps *Store) defaultPromptPath(role string) string {
	return filepath.Join(ps.root, "defaults", role+".md")
}

func (ps *Store) userPromptPath(ownerID, role string) string {
	return filepath.Join(ps.root, "users", sanitizePromptPath(ownerID), role+".md")
}

func sanitizePromptPath(value string) string {
	if strings.TrimSpace(value) == "" {
		return "anonymous"
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case strings.ContainsRune("-_.@", r):
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	if b.Len() == 0 {
		return "anonymous"
	}
	return b.String()
}
