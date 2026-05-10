package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	langsmith "github.com/langchain-ai/langsmith-go"
)

func isHTTPStatus(err error, statusCode int) bool {
	if err == nil {
		return false
	}
	var apiErr *langsmith.Error
	return errors.As(err, &apiErr) && apiErr.StatusCode == statusCode
}

func isHTTP404(err error) bool {
	return isHTTPStatus(err, http.StatusNotFound)
}

func isHTTP409(err error) bool {
	return isHTTPStatus(err, http.StatusConflict)
}

func isRawHTTPStatus(err error, statusCode int) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), fmt.Sprintf("HTTP %d:", statusCode))
}

func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func formatHubTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func sdkRepoToHubRepo(repo langsmith.RepoWithLookups) hubRepo {
	return hubRepo{
		ID:             repo.ID,
		FullName:       repo.FullName,
		Owner:          strPtrOrNil(repo.Owner),
		RepoHandle:     repo.RepoHandle,
		RepoType:       string(repo.RepoType),
		Description:    strPtrOrNil(repo.Description),
		Readme:         strPtrOrNil(repo.Readme),
		IsPublic:       repo.IsPublic,
		IsArchived:     repo.IsArchived,
		Tags:           repo.Tags,
		NumCommits:     int(repo.NumCommits),
		LastCommitHash: strPtrOrNil(repo.LastCommitHash),
		CreatedAt:      formatHubTime(repo.CreatedAt),
		UpdatedAt:      formatHubTime(repo.UpdatedAt),
	}
}

func hubRepoTypeParam(repoType string) (langsmith.RepoNewParamsRepoType, error) {
	switch repoType {
	case "agent":
		return langsmith.RepoNewParamsRepoTypeAgent, nil
	case "skill":
		return langsmith.RepoNewParamsRepoTypeSkill, nil
	default:
		return "", fmt.Errorf("repo type must be 'agent' or 'skill' (got %q)", repoType)
	}
}

func ensureHubRepo(ctx context.Context, c *client.Client, owner, name, repoType string, meta hubRepoMeta) error {
	_, err := c.SDK.Repos.Get(ctx, owner, name)
	if err == nil {
		if meta.Description != nil || meta.Readme != nil || meta.Tags != nil || meta.IsPublic != nil {
			params := langsmith.RepoUpdateParams{}
			if meta.Description != nil {
				params.Description = langsmith.F(*meta.Description)
			}
			if meta.Readme != nil {
				params.Readme = langsmith.F(*meta.Readme)
			}
			if meta.Tags != nil {
				params.Tags = langsmith.F(meta.Tags)
			}
			if meta.IsPublic != nil {
				params.IsPublic = langsmith.F(*meta.IsPublic)
			}
			if _, err := c.SDK.Repos.Update(ctx, owner, name, params); err != nil {
				return fmt.Errorf("updating metadata for %s/%s: %w", owner, name, err)
			}
		}
		return nil
	}
	if !isHTTP404(err) {
		return fmt.Errorf("checking %s/%s: %w", owner, name, err)
	}
	if !hubRepoHandlePattern.MatchString(name) {
		return fmt.Errorf("repo handle %q must match %s", name, hubRepoHandlePattern.String())
	}
	sdkRepoType, err := hubRepoTypeParam(repoType)
	if err != nil {
		return err
	}
	create := map[string]any{
		"repo_handle": name,
		"repo_type":   string(sdkRepoType),
		"is_public":   false,
		"source": "internal",
	}
	if meta.IsPublic != nil {
		create["is_public"] = *meta.IsPublic
	}
	if meta.Description != nil {
		create["description"] = *meta.Description
	}
	if meta.Readme != nil {
		create["readme"] = *meta.Readme
	}
	if meta.Tags != nil {
		create["tags"] = meta.Tags
	}
	if err := c.RawPost(ctx, "/api/v1/repos", create, nil); err != nil {
		if isHTTP409(err) || isRawHTTPStatus(err, http.StatusConflict) {
			return nil
		}
		return fmt.Errorf("creating %s/%s: %w", owner, name, err)
	}
	return nil
}
