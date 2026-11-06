package hostapi

// LangSmith platform API endpoints for sandbox snapshots.
//
// Used by deploy bake (capture / get / wait) and `mda delete` (list + delete
// recipe snapshots named `{deployment}--setup-{digest}`).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-go/option"

	"github.com/langchain-ai/langsmith-cli/internal/mda/proc"
)

// snapshotPageSize is the page size for snapshot listing.
const snapshotPageSize = 100

// DefaultSnapshotReadyTimeout is the default bound for waiting until a
// captured snapshot is `ready`.
const DefaultSnapshotReadyTimeout = 60 * time.Second

// readyPollInterval is the poll interval while waiting for snapshot readiness.
const readyPollInterval = 2 * time.Second

// SnapshotDelete is the result of deleting a sandbox snapshot.
type SnapshotDelete int

// The results.
const (
	// SnapshotDeleteDeleted: the snapshot was removed.
	SnapshotDeleteDeleted SnapshotDelete = iota
	// SnapshotDeleteMissing: the snapshot was already gone (HTTP 404).
	SnapshotDeleteMissing
	// SnapshotDeleteInUse: LangSmith refused because one or more sandboxes
	// still clone it (HTTP 409).
	SnapshotDeleteInUse
)

// SnapshotSummary is a snapshot as returned by list/get endpoints.
type SnapshotSummary struct {
	// ID is the snapshot UUID.
	ID string `json:"id"`
	// Name is the human-readable name (recipe snapshots use
	// `{deployment}--setup-{digest}`).
	Name string `json:"name"`
	// Status is `building`, `ready`, `failed`, …
	Status string `json:"status"`
	// Tags are the tags Host assigned. A snapshot built from a Docker image
	// inherits the image's tag rather than `latest`, which decides how the
	// runtime must reference the recipe.
	Tags []string `json:"tags"`
}

// UnmarshalJSON requires the id.
func (s *SnapshotSummary) UnmarshalJSON(data []byte) error {
	if err := requireFields(data, "id"); err != nil {
		return err
	}
	type plain SnapshotSummary
	return json.Unmarshal(data, (*plain)(s))
}

type snapshotListPage struct {
	Snapshots []SnapshotSummary `json:"snapshots"`
}

// ListSnapshotsWithPrefix lists snapshots whose name contains prefix, then
// keeps prefix matches.
func (c *Client) ListSnapshotsWithPrefix(ctx context.Context, prefix string) ([]SnapshotSummary, error) {
	found := []SnapshotSummary{}
	offset := 0
	for {
		raw, err := c.sdkRequest(ctx, http.MethodGet, "/api/v2/sandboxes/snapshots", requestOptions{},
			func(ctx context.Context, opts ...option.RequestOption) error {
				_, err := c.ls.SDK.Sandboxes.Snapshots.List(ctx, langsmith.SandboxSnapshotListParams{
					NameContains: langsmith.F(prefix),
					Limit:        langsmith.F(int64(snapshotPageSize)),
					Offset:       langsmith.F(int64(offset)),
				}, opts...)
				return err
			})
		if err != nil {
			return nil, err
		}
		if raw == nil {
			break
		}
		var page snapshotListPage
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, fmt.Errorf("failed to decode the snapshot list response: %w", err)
		}
		for _, snapshot := range page.Snapshots {
			if snapshot.Name != "" && strings.HasPrefix(snapshot.Name, prefix) {
				found = append(found, snapshot)
			}
		}
		if len(page.Snapshots) < snapshotPageSize {
			break
		}
		offset += len(page.Snapshots)
	}
	return found, nil
}

// GetSnapshot fetches a snapshot by id.
func (c *Client) GetSnapshot(ctx context.Context, snapshotID string) (SnapshotSummary, error) {
	raw, err := c.sdkRequest(ctx, http.MethodGet, "/api/v2/sandboxes/snapshots/"+snapshotID, requestOptions{},
		func(ctx context.Context, opts ...option.RequestOption) error {
			_, err := c.ls.SDK.Sandboxes.Snapshots.Get(ctx, snapshotID, opts...)
			return err
		})
	if err != nil {
		return SnapshotSummary{}, err
	}
	return decodeSnapshot(raw, "failed to decode the snapshot response")
}

// FindReadySnapshotByName looks up a snapshot by exact name, if one exists and
// is ready.
func (c *Client) FindReadySnapshotByName(ctx context.Context, name string) (*SnapshotSummary, error) {
	matches, err := c.ListSnapshotsWithPrefix(ctx, name)
	if err != nil {
		return nil, err
	}
	for _, snapshot := range matches {
		if snapshot.Name == name && snapshot.Status == "ready" {
			return &snapshot, nil
		}
	}
	return nil, nil
}

// DeleteSnapshot deletes a snapshot by id. Idempotent on 404. A 409 is
// [SnapshotDeleteInUse], not an error: live boxes keep the recipe they were
// created from until idle TTL or `mda delete`.
func (c *Client) DeleteSnapshot(ctx context.Context, snapshotID string) (SnapshotDelete, error) {
	_, found, err := c.sdkRequestOptional(ctx, http.MethodDelete, "/api/v2/sandboxes/snapshots/"+snapshotID,
		func(ctx context.Context, opts ...option.RequestOption) error {
			return c.ls.SDK.Sandboxes.Snapshots.Delete(ctx, snapshotID, opts...)
		})
	switch {
	case snapshotInUse(err):
		return SnapshotDeleteInUse, nil
	case err != nil:
		return 0, err
	case !found:
		return SnapshotDeleteMissing, nil
	default:
		return SnapshotDeleteDeleted, nil
	}
}

// CaptureSnapshot captures a snapshot from a running box and waits until it
// is ready.
func (c *Client) CaptureSnapshot(ctx context.Context, boxName, snapshotName string, timeout time.Duration) (SnapshotSummary, error) {
	box := urlencodingPath(boxName)
	raw, err := c.sdkRequest(ctx, http.MethodPost, "/api/v2/sandboxes/boxes/"+box+"/snapshot", requestOptions{},
		func(ctx context.Context, opts ...option.RequestOption) error {
			_, err := c.ls.SDK.Sandboxes.Boxes.NewSnapshot(ctx, box, langsmith.SandboxBoxNewSnapshotParams{
				Name: langsmith.F(snapshotName),
			}, opts...)
			return err
		})
	if err != nil {
		return SnapshotSummary{}, err
	}
	started, err := decodeSnapshot(raw, "failed to decode capture snapshot response")
	if err != nil {
		return SnapshotSummary{}, err
	}
	return c.WaitForSnapshot(ctx, started.ID, timeout)
}

// CreateSnapshotFromDockerImage builds a snapshot from a Docker image and
// waits until it is ready. registryID is "" for a public image.
func (c *Client) CreateSnapshotFromDockerImage(ctx context.Context, name, dockerImage string, fsCapacityBytes uint64, timeout time.Duration, registryID string) (SnapshotSummary, error) {
	params := langsmith.SandboxSnapshotNewParams{
		Name:            langsmith.F(name),
		DockerImage:     langsmith.F(dockerImage),
		FsCapacityBytes: langsmith.F(int64(fsCapacityBytes)),
	}
	if registryID != "" {
		params.RegistryID = langsmith.F(registryID)
	}
	raw, err := c.sdkRequest(ctx, http.MethodPost, "/api/v2/sandboxes/snapshots", requestOptions{},
		func(ctx context.Context, opts ...option.RequestOption) error {
			_, err := c.ls.SDK.Sandboxes.Snapshots.New(ctx, params, opts...)
			return err
		})
	if err != nil {
		return SnapshotSummary{}, err
	}
	started, err := decodeSnapshot(raw, "failed to decode create snapshot response")
	if err != nil {
		return SnapshotSummary{}, err
	}
	return c.WaitForSnapshot(ctx, started.ID, timeout)
}

// WaitForSnapshot polls until the snapshot is `ready` or `failed`, or timeout
// elapses.
func (c *Client) WaitForSnapshot(ctx context.Context, snapshotID string, timeout time.Duration) (SnapshotSummary, error) {
	deadline := time.Now().Add(timeout)
	for {
		snapshot, err := c.GetSnapshot(ctx, snapshotID)
		if err != nil {
			return SnapshotSummary{}, err
		}
		switch {
		case snapshot.Status == "ready":
			return snapshot, nil
		case snapshot.Status == "failed":
			return SnapshotSummary{}, fmt.Errorf("snapshot %s failed to become ready", snapshotID)
		case !time.Now().Before(deadline):
			return SnapshotSummary{}, fmt.Errorf("snapshot %s did not become ready within %ds", snapshotID, int64(timeout.Seconds()))
		}
		if err := proc.Pause(ctx, readyPollInterval); err != nil {
			return SnapshotSummary{}, err
		}
	}
}

func decodeSnapshot(raw json.RawMessage, message string) (SnapshotSummary, error) {
	var snapshot SnapshotSummary
	if err := json.Unmarshal(nullIfEmpty(raw), &snapshot); err != nil {
		return SnapshotSummary{}, fmt.Errorf("%s: %w", message, err)
	}
	return snapshot, nil
}

// urlencodingPath percent-encodes a path segment. Box, snapshot and registry
// names are deployment-safe ([A-Za-z0-9._-]); encode anyway.
func urlencodingPath(value string) string {
	var out strings.Builder
	for _, b := range []byte(value) {
		switch {
		case b >= 'A' && b <= 'Z', b >= 'a' && b <= 'z', b >= '0' && b <= '9', b == '-', b == '_', b == '.':
			out.WriteByte(b)
		default:
			fmt.Fprintf(&out, "%%%02X", b)
		}
	}
	return out.String()
}

func snapshotInUse(err error) bool {
	var api *APIError
	return errors.As(err, &api) && api.Status == http.StatusConflict
}
