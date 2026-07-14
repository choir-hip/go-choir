package agentcore

import (
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"os"
	"path/filepath"
	"strings"
)

const maxSkillPromptExtractBytes = 1200

var runtimePromptSkills = []string{
	"mission-gradient",
	"cognitive-transform-portfolio",
}

func (rt *Runtime) skillContextForProfile(profile string) string {
	if rt == nil || !profileReceivesSkillContext(profile) {
		return ""
	}
	root := strings.TrimSpace(rt.cfg.SkillsRoot)
	if root == "" {
		return ""
	}
	entries := make([]string, 0, len(runtimePromptSkills))
	for _, name := range runtimePromptSkills {
		path := filepath.Join(root, name, "SKILL.md")
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		entry := summarizeRuntimeSkill(name, path, string(data))
		if strings.TrimSpace(entry) != "" {
			entries = append(entries, entry)
		}
	}
	if len(entries) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("Available repo skills (natural-language use; no slash commands):\n")
	b.WriteString(strings.Join(entries, "\n\n"))
	b.WriteString("\n\nUse `mission-gradient` for long-running, staging, self-development, or broad architectural work. Use `cognitive-transform-portfolio` before stopping on a blocker, and again after first correctness when a quality pass can raise the result.")
	return b.String()
}

func profileReceivesSkillContext(profile string) bool {
	switch agentprofile.Canonical(profile) {
	case agentprofile.Conductor, agentprofile.Texture, agentprofile.Super, agentprofile.VSuper, agentprofile.CoSuper:
		return true
	default:
		return false
	}
}

func summarizeRuntimeSkill(defaultName, path, raw string) string {
	name, description, body := parseSkillMarkdown(defaultName, raw)
	body = trimSkillExtract(body, maxSkillPromptExtractBytes)
	var b strings.Builder
	b.WriteString("- ")
	b.WriteString(name)
	if description != "" {
		b.WriteString(": ")
		b.WriteString(description)
	}
	b.WriteString("\n  Source: ")
	b.WriteString(path)
	if body != "" {
		b.WriteString("\n  Extract:\n")
		for _, line := range strings.Split(body, "\n") {
			line = strings.TrimRight(line, " \t")
			if line == "" {
				continue
			}
			b.WriteString("  ")
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func parseSkillMarkdown(defaultName, raw string) (string, string, string) {
	raw = strings.TrimSpace(raw)
	name := strings.TrimSpace(defaultName)
	description := ""
	body := raw
	if strings.HasPrefix(raw, "---\n") {
		rest := strings.TrimPrefix(raw, "---\n")
		if idx := strings.Index(rest, "\n---"); idx >= 0 {
			frontMatter := rest[:idx]
			body = strings.TrimSpace(rest[idx+len("\n---"):])
			for _, line := range strings.Split(frontMatter, "\n") {
				key, value, ok := strings.Cut(line, ":")
				if !ok {
					continue
				}
				key = strings.TrimSpace(key)
				value = strings.TrimSpace(value)
				switch key {
				case "name":
					if value != "" {
						name = value
					}
				case "description":
					description = strings.Trim(value, `"`)
				}
			}
		}
	}
	return firstNonEmpty(name, defaultName), description, strings.TrimSpace(body)
}

func trimSkillExtract(body string, limit int) string {
	body = strings.TrimSpace(body)
	if body == "" || limit <= 0 {
		return ""
	}
	if len(body) <= limit {
		return body
	}
	cut := limit
	if idx := strings.LastIndexAny(body[:limit], "\n. "); idx > limit/2 {
		cut = idx + 1
	}
	return strings.TrimSpace(body[:cut]) + "\n..."
}
