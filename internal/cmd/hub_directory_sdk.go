package cmd

import (
	"fmt"
)

func hubFilesToSDKFiles(files map[string]hubFileEntry) map[string]interface{} {
	out := make(map[string]interface{}, len(files))
	for path, entry := range files {
		out[path] = map[string]interface{}{
			"type":    entry.Type,
			"content": entry.Content,
		}
	}
	return out
}

func sdkFilesToHubFiles(files map[string]interface{}) (map[string]hubFileEntry, error) {
	out := make(map[string]hubFileEntry, len(files))
	for path, raw := range files {
		entryMap, ok := raw.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid file entry for %q", path)
		}

		rawType, ok := entryMap["type"]
		if !ok {
			return nil, fmt.Errorf("missing entry type for %q", path)
		}
		entryType, ok := rawType.(string)
		if !ok || entryType == "" {
			return nil, fmt.Errorf("invalid entry type for %q", path)
		}

		entry := hubFileEntry{Type: entryType}
		if content, ok := entryMap["content"].(string); ok {
			entry.Content = content
		}
		if repoHandle, ok := entryMap["repo_handle"].(string); ok {
			entry.RepoHandle = repoHandle
		}
		if owner, ok := entryMap["owner"].(string); ok {
			entry.Owner = owner
		}
		if commitHash, ok := entryMap["commit_hash"].(string); ok {
			entry.CommitHash = commitHash
		}

		out[path] = entry
	}
	return out, nil
}
