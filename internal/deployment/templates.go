package deployment

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Template represents a project template.
type Template struct {
	Description string
	Python      string
	JS          string
}

// Templates is the list of available project templates.
var Templates = map[string]Template{
	"Deep Agent": {
		Description: "An agent that breaks down complex tasks into smaller steps and delegates work to sub-agents.",
		Python:      "https://github.com/langchain-ai/deepagents/archive/refs/heads/main.zip",
	},
	"Agent": {
		Description: "A simple agent that can be flexibly extended to many tools.",
		Python:      "https://github.com/langchain-ai/simple-agent-template/archive/refs/heads/main.zip",
	},
	"New LangGraph Project": {
		Description: "A simple, minimal chatbot with memory.",
		Python:      "https://github.com/langchain-ai/new-langgraph-project/archive/refs/heads/main.zip",
		JS:          "https://github.com/langchain-ai/new-langgraphjs-project/archive/refs/heads/main.zip",
	},
	"ReAct Agent": {
		Description: "A simple agent that can be flexibly extended to many tools.",
		Python:      "https://github.com/langchain-ai/react-agent/archive/refs/heads/main.zip",
		JS:          "https://github.com/langchain-ai/react-agent-js/archive/refs/heads/main.zip",
	},
}

// TemplateIDConfig maps template IDs to their config.
type TemplateIDConfig struct {
	Name string
	Lang string
	URL  string
}

// BuildTemplateIDs generates the template ID to config mapping.
func BuildTemplateIDs() map[string]TemplateIDConfig {
	result := make(map[string]TemplateIDConfig)
	for name, tmpl := range Templates {
		id := strings.ToLower(strings.ReplaceAll(name, " ", "-"))
		if tmpl.Python != "" {
			result[id+"-python"] = TemplateIDConfig{Name: name, Lang: "python", URL: tmpl.Python}
		}
		if tmpl.JS != "" {
			result[id+"-js"] = TemplateIDConfig{Name: name, Lang: "js", URL: tmpl.JS}
		}
	}
	return result
}

// TemplateHelpString generates the help text for the --template flag.
func TemplateHelpString() string {
	ids := BuildTemplateIDs()
	var lines []string
	lines = append(lines, "Available templates:")
	for id := range ids {
		lines = append(lines, "  "+id)
	}
	return strings.Join(lines, "\n")
}

// CreateNew creates a new project at the given path from a template.
func CreateNew(path, templateID string) error {
	if path == "" {
		path = "."
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}

	// Check if path exists and is not empty
	entries, err := os.ReadDir(absPath)
	if err == nil && len(entries) > 0 {
		return fmt.Errorf("directory %s already exists and is not empty", absPath)
	}

	ids := BuildTemplateIDs()
	tmplConfig, ok := ids[templateID]
	if !ok {
		var available []string
		for id := range ids {
			available = append(available, id)
		}
		return fmt.Errorf("template '%s' not found. Available: %s", templateID, strings.Join(available, ", "))
	}

	if err := downloadAndExtract(tmplConfig.URL, absPath); err != nil {
		return fmt.Errorf("downloading template: %w", err)
	}

	fmt.Fprintf(os.Stderr, "New project created at %s\n", absPath)
	return nil
}

func downloadAndExtract(url, destPath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading %s: HTTP %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	zipReader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return fmt.Errorf("opening zip: %w", err)
	}

	// Create destination directory
	if err := os.MkdirAll(destPath, 0755); err != nil {
		return err
	}

	// Find the common prefix (usually "repo-name-main/")
	var prefix string
	if len(zipReader.File) > 0 {
		name := zipReader.File[0].Name
		if idx := strings.Index(name, "/"); idx >= 0 {
			prefix = name[:idx+1]
		}
	}

	for _, f := range zipReader.File {
		// Strip the prefix
		name := f.Name
		if prefix != "" {
			name = strings.TrimPrefix(name, prefix)
		}
		if name == "" {
			continue
		}

		// Zip Slip protection: reject paths that try to escape
		cleanName := filepath.Clean(name)
		if strings.HasPrefix(cleanName, "..") {
			return fmt.Errorf("illegal file path in archive: %s", name)
		}

		targetPath := filepath.Join(destPath, cleanName)

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return err
			}
			continue
		}

		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}

		outFile, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			_ = outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		_ = rc.Close()
		_ = outFile.Close()
		if err != nil {
			return err
		}
	}

	return nil
}
