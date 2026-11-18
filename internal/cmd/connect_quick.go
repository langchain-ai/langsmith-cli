package cmd

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os/signal"
	"sort"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-go/lib/connector"
)

// connectionsPath is the connections collection. Like the connector's carrier
// path, it is served at the API origin rather than under /api/v1.
const connectionsPath = "/v1/connections"

const (
	// quickTunnelCreateAttempts is the first create and up to three retries
	// with a new name after a name conflict.
	quickTunnelCreateAttempts = 4
	quickTunnelDescription    = "Quick tunnel from langsmith tunnel connect --quick"
	quickTunnelPollInterval   = 500 * time.Millisecond
	// The targets endpoint's largest page.
	quickTunnelTargetPageSize = 100
	quickTunnelReadyTimeout   = 30 * time.Second
	quickTunnelDeleteTimeout  = 10 * time.Second
)

// The server deletes a quick tunnel quickTunnelTTL after its last heartbeat,
// so one whose CLI dies without cleaning up does not linger.
const quickTunnelTTL = 5 * time.Minute

// quickTunnelHeartbeatInterval is a variable so tests can shorten it.
var quickTunnelHeartbeatInterval = time.Minute

var errQuickTunnelExpired = errors.New("the quick tunnel expired or was deleted")

type quickTunnelConnection struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type quickTunnelTarget struct {
	Name      string `json:"name"`
	Available bool   `json:"is_available"`
	TunnelURL string `json:"tunnel_url"`
}

type quickTunnelReport struct {
	ConnectionID string                 `json:"connection_id"`
	Name         string                 `json:"name"`
	Targets      []quickTunnelReportURL `json:"targets"`
}

type quickTunnelReportURL struct {
	Name      string `json:"name"`
	TunnelURL string `json:"tunnel_url"`
}

// runQuickTunnel creates a personal tunnel connection, serves it until ctx is
// cancelled, the process is interrupted or the connector fails, and deletes
// the connection on the way out.
func runQuickTunnel(ctx context.Context, w io.Writer, targets map[string]string, logLevel string) error {
	// Validate the connector settings before creating anything, with a
	// placeholder ID replaced once the connection exists.
	config, err := newConnectorConfig(connector.Settings{ConnectionID: uuid.NewString(), Targets: targets})
	if err != nil {
		return err
	}
	c, err := getClient()
	if err != nil {
		return err
	}
	logger := newTunnelLogger(logLevel)

	conn, err := createQuickTunnel(ctx, c)
	if err != nil {
		return err
	}
	defer deleteQuickTunnel(c, conn.ID, logger)
	if config.ConnectionID, err = uuid.Parse(conn.ID); err != nil {
		return fmt.Errorf("LangSmith returned an invalid connection ID %q", conn.ID)
	}

	ctx, stop := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	var background sync.WaitGroup
	background.Add(2)
	go func() {
		defer background.Done()
		reportQuickTunnel(ctx, c, w, conn, targets, logger)
	}()
	go func() {
		defer background.Done()
		heartbeatQuickTunnel(ctx, cancel, c, conn.ID, quickTunnelHeartbeatInterval, logger)
	}()
	err = serveConnector(ctx, config, logLevel)
	if cause := context.Cause(ctx); errors.Is(cause, errQuickTunnelExpired) {
		err = cause
	}
	cancel(nil)
	background.Wait()
	return err
}

// heartbeatQuickTunnel moves the tunnel's expiry forward every interval until
// ctx is done. It cancels ctx with errQuickTunnelExpired once the tunnel is
// gone, and keeps trying through other errors while the expiry allows.
func heartbeatQuickTunnel(ctx context.Context, cancel context.CancelCauseFunc, c *client.Client, id string, interval time.Duration, logger *slog.Logger) {
	path := connectionsPath + "/" + url.PathEscape(id)
	body := map[string]any{"expires_in_seconds": int(quickTunnelTTL / time.Second)}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		err := c.RawPatch(ctx, path, body, nil)
		switch {
		case err == nil:
		case client.IsNotFound(err):
			cancel(errQuickTunnelExpired)
			return
		case ctx.Err() == nil:
			logger.Warn("could not extend the quick tunnel's expiry; retrying", "connection_id", id, "error", err)
		}
	}
}

// createQuickTunnel creates a personal tunnel connection with a random name,
// trying another name when one is taken.
func createQuickTunnel(ctx context.Context, c *client.Client) (quickTunnelConnection, error) {
	for attempt := 1; ; attempt++ {
		name, err := quickTunnelName()
		if err != nil {
			return quickTunnelConnection{}, err
		}
		var conn quickTunnelConnection
		err = c.RawPost(ctx, connectionsPath, map[string]any{
			"name":               name,
			"kind":               "tunnel",
			"personal":           true,
			"description":        quickTunnelDescription,
			"expires_in_seconds": int(quickTunnelTTL / time.Second),
		}, &conn)
		if err == nil {
			if conn.Name == "" {
				conn.Name = name
			}
			return conn, nil
		}
		if !client.IsConflict(err) || attempt == quickTunnelCreateAttempts {
			return quickTunnelConnection{}, fmt.Errorf("creating a quick tunnel: %w", err)
		}
	}
}

func quickTunnelName() (string, error) {
	suffix := make([]byte, 4)
	if _, err := rand.Read(suffix); err != nil {
		return "", fmt.Errorf("generating a tunnel name: %w", err)
	}
	return "quick-" + hex.EncodeToString(suffix), nil
}

// reportQuickTunnel waits for every target to be available at the edge, then
// prints the tunnel URL of each HTTP target.
func reportQuickTunnel(ctx context.Context, c *client.Client, w io.Writer, conn quickTunnelConnection, want map[string]string, logger *slog.Logger) {
	ctx, cancel := context.WithTimeout(ctx, quickTunnelReadyTimeout)
	defer cancel()
	path := connectionsPath + "/" + url.PathEscape(conn.ID) + "/targets"
	ticker := time.NewTicker(quickTunnelPollInterval)
	defer ticker.Stop()
	for {
		items, err := listQuickTunnelTargets(ctx, c, path)
		if err == nil && quickTunnelTargetsReady(items, want) {
			if err := printQuickTunnel(w, conn, items); err != nil {
				logger.Error("could not print the tunnel URLs", "error", err)
			}
			return
		}
		if err != nil && ctx.Err() == nil {
			logger.Debug("tunnel targets are not ready yet", "error", err)
		}
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				logger.Warn("tunnel targets did not become available; the connector keeps trying",
					"connection_id", conn.ID, "waited", quickTunnelReadyTimeout)
			}
			return
		case <-ticker.C:
		}
	}
}

// listQuickTunnelTargets reads every page of the connection's targets.
func listQuickTunnelTargets(ctx context.Context, c *client.Client, path string) ([]quickTunnelTarget, error) {
	var items []quickTunnelTarget
	cursor := ""
	for {
		query := url.Values{"page_size": {strconv.Itoa(quickTunnelTargetPageSize)}}
		if cursor != "" {
			query.Set("cursor", cursor)
		}
		var page struct {
			Items      []quickTunnelTarget `json:"items"`
			NextCursor *string             `json:"next_cursor"`
		}
		if err := c.RawGet(ctx, path+"?"+query.Encode(), &page); err != nil {
			return nil, err
		}
		items = append(items, page.Items...)
		if page.NextCursor == nil || *page.NextCursor == "" {
			return items, nil
		}
		cursor = *page.NextCursor
	}
}

func quickTunnelTargetsReady(items []quickTunnelTarget, want map[string]string) bool {
	available := make(map[string]bool, len(items))
	for _, item := range items {
		available[item.Name] = item.Available
	}
	for name := range want {
		if !available[name] {
			return false
		}
	}
	return true
}

func printQuickTunnel(w io.Writer, conn quickTunnelConnection, items []quickTunnelTarget) error {
	report := quickTunnelReport{ConnectionID: conn.ID, Name: conn.Name, Targets: []quickTunnelReportURL{}}
	for _, item := range items {
		if item.TunnelURL != "" {
			report.Targets = append(report.Targets, quickTunnelReportURL{Name: item.Name, TunnelURL: item.TunnelURL})
		}
	}
	sort.Slice(report.Targets, func(i, j int) bool { return report.Targets[i].Name < report.Targets[j].Name })

	if GetFormat() == "json" {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(w, string(data))
		return err
	}
	if len(report.Targets) == 0 {
		_, err := fmt.Fprintf(w, "Tunnel ready: %s\n", conn.Name)
		return err
	}
	for _, target := range report.Targets {
		if _, err := fmt.Fprintf(w, "Tunnel ready: %s → %s\n", target.Name, target.TunnelURL); err != nil {
			return err
		}
	}
	return nil
}

// deleteQuickTunnel deletes the connection with a fresh context, since the
// command's own context is cancelled by the time it runs.
func deleteQuickTunnel(c *client.Client, id string, logger *slog.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), quickTunnelDeleteTimeout)
	defer cancel()
	path := connectionsPath + "/" + url.PathEscape(id)
	if err := c.RawDelete(ctx, path, nil); err != nil && !client.IsNotFound(err) {
		logger.Error(fmt.Sprintf("could not delete the quick tunnel; delete it with: langsmith api %s -X DELETE", path),
			"connection_id", id, "error", err)
	}
}
