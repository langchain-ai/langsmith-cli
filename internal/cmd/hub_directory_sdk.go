package cmd

import (
	"fmt"

	langsmith "github.com/langchain-ai/langsmith-go"
)

func hubFilesToSDKFiles(files map[string]hubFileEntry) map[string]langsmith.RepoDirectoryCommitParamsFilesUnion {
	out := make(map[string]langsmith.RepoDirectoryCommitParamsFilesUnion, len(files))
	for path, entry := range files {
		out[path] = langsmith.FileEntryParam{
			Type:    langsmith.F(langsmith.FileEntryType(entry.Type)),
			Content: langsmith.F(entry.Content),
		}
	}
	return out
}

func sdkFilesToHubFiles(files map[string]langsmith.RepoDirectoryListResponseFile) (map[string]hubFileEntry, error) {
	out := make(map[string]hubFileEntry, len(files))
	for path, raw := range files {
		entryType := string(raw.Type)
		if entryType == "" {
			return nil, fmt.Errorf("invalid entry type for %q", path)
		}

		out[path] = hubFileEntry{
			Type:       entryType,
			Content:    raw.Content,
			RepoHandle: raw.RepoHandle,
			Owner:      raw.Owner,
			CommitHash: raw.CommitHash,
		}
	}
	return out, nil
}
