package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

func RegisterReadOnlyFileTools(registry *toolregistry.ToolRegistry, cwd string) error {
	for _, tool := range []toolregistry.Tool{
		newReadFileTool(cwd),
		newGlobTool(cwd),
		newGrepTool(cwd),
	} {
		if err := registry.Register(tool); err != nil {
			return err
		}
	}
	return nil
}

func newReadFileTool(cwd string) toolregistry.Tool {
	type args struct {
		Path string `json:"path"`
	}
	return toolregistry.Tool{Name: "read_file",
		Description: "Read a file from disk relative to the sandbox working directory.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"path": map[string]any{"type": "string"},
		}, []string{"path"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode read_file args: %w", err)
			}
			baseCWD := effectiveToolCWD(ctx, cwd)
			resolved, err := resolveToolPath(baseCWD, in.Path)
			if err != nil {
				return "", err
			}
			data, err := os.ReadFile(resolved)
			if err != nil {
				return "", err
			}
			content := string(data)
			if len(content) > 100*1024 {
				content = content[:100*1024] + fmt.Sprintf("\n\n[file truncated — %d bytes total, showing first 100KB]", len(data))
			}
			return toolregistry.ResultJSON(map[string]any{
				"path":    resolved,
				"content": content,
			})
		}}
}

func newGlobTool(cwd string) toolregistry.Tool {
	type args struct {
		Pattern string `json:"pattern"`
		Limit   int    `json:"limit,omitempty"`
	}
	return toolregistry.Tool{Name: "glob",
		Description: "Find files by glob-like pattern relative to the sandbox working directory.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"pattern": map[string]any{"type": "string"},
			"limit":   map[string]any{"type": "integer"},
		}, []string{"pattern"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode glob args: %w", err)
			}
			matcher, err := globPatternToRegexp(in.Pattern)
			if err != nil {
				return "", err
			}
			limit := in.Limit
			if limit <= 0 {
				limit = 200
			}
			baseCWD := effectiveToolCWD(ctx, cwd)
			matches := make([]string, 0, minInt(limit, 32))
			err = filepath.WalkDir(baseCWD, func(current string, d fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if current == baseCWD {
					return nil
				}
				rel, err := filepath.Rel(baseCWD, current)
				if err != nil {
					return err
				}
				rel = filepath.ToSlash(rel)
				if d.IsDir() {
					if rel == ".git" || strings.HasPrefix(rel, ".git/") {
						return filepath.SkipDir
					}
					return nil
				}
				if matcher.MatchString(rel) {
					matches = append(matches, rel)
					if len(matches) >= limit {
						return errToolLimitReached
					}
				}
				return nil
			})
			if err != nil && err != errToolLimitReached {
				return "", err
			}
			sort.Strings(matches)
			return toolregistry.ResultJSON(map[string]any{
				"pattern": in.Pattern,
				"matches": matches,
			})
		}}
}

func newGrepTool(cwd string) toolregistry.Tool {
	type args struct {
		Pattern         string `json:"pattern"`
		Path            string `json:"path,omitempty"`
		Limit           int    `json:"limit,omitempty"`
		CaseInsensitive bool   `json:"case_insensitive,omitempty"`
	}
	return toolregistry.Tool{Name: "grep",
		Description: "Search file contents for a regular expression.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"pattern":          map[string]any{"type": "string"},
			"path":             map[string]any{"type": "string"},
			"limit":            map[string]any{"type": "integer"},
			"case_insensitive": map[string]any{"type": "boolean"},
		}, []string{"pattern"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode grep args: %w", err)
			}
			pattern := in.Pattern
			if in.CaseInsensitive {
				pattern = "(?i)" + pattern
			}
			re, err := regexp.Compile(pattern)
			if err != nil {
				return "", err
			}
			baseCWD := effectiveToolCWD(ctx, cwd)
			searchRoot := baseCWD
			if strings.TrimSpace(in.Path) != "" {
				searchRoot, err = resolveToolPath(baseCWD, in.Path)
				if err != nil {
					return "", err
				}
			}
			limit := in.Limit
			if limit <= 0 {
				limit = 100
			}
			var matches []map[string]any
			err = filepath.Walk(searchRoot, func(path string, info os.FileInfo, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if info.IsDir() {
					if info.Name() == ".git" {
						return filepath.SkipDir
					}
					return nil
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return nil
				}
				lines := strings.Split(string(data), "\n")
				for idx, line := range lines {
					if re.MatchString(line) {
						rel, _ := filepath.Rel(baseCWD, path)
						matches = append(matches, map[string]any{
							"path": filepath.ToSlash(rel),
							"line": idx + 1,
							"text": line,
						})
						if len(matches) >= limit {
							return errToolLimitReached
						}
					}
				}
				return nil
			})
			if err != nil && err != errToolLimitReached {
				return "", err
			}
			return toolregistry.ResultJSON(map[string]any{
				"pattern": in.Pattern,
				"matches": matches,
			})
		}}
}

func effectiveToolCWD(ctx context.Context, defaultCWD string) string {
	if ctx != nil {
		if override := toolregistry.ExecutionContextFrom(ctx).WorkingDir; override != "" {
			if filepath.IsAbs(override) {
				return filepath.Clean(override)
			}
			return filepath.Clean(filepath.Join(defaultCWD, override))
		}
	}
	return filepath.Clean(defaultCWD)
}

func resolveToolPath(cwd, userPath string) (string, error) {
	if strings.TrimSpace(userPath) == "" {
		return "", fmt.Errorf("path must not be empty")
	}
	base := filepath.Clean(cwd)
	if !filepath.IsAbs(userPath) {
		userPath = filepath.Join(base, userPath)
	}
	resolved := filepath.Clean(userPath)
	rel, err := filepath.Rel(base, resolved)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path %q escapes working directory", userPath)
	}
	return resolved, nil
}

func globPatternToRegexp(pattern string) (*regexp.Regexp, error) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return nil, fmt.Errorf("pattern must not be empty")
	}
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				b.WriteString(".*")
				i++
			} else {
				b.WriteString(`[^/]*`)
			}
		case '?':
			b.WriteString(".")
		case '.', '+', '(', ')', '[', ']', '{', '}', '^', '$', '|', '\\':
			b.WriteByte('\\')
			b.WriteByte(pattern[i])
		default:
			b.WriteByte(pattern[i])
		}
	}
	b.WriteString("$")
	return regexp.Compile(b.String())
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

var errToolLimitReached = fmt.Errorf("tool limit reached")
