package cmd

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// githubReleasesBaseURL can be overridden in tests.
var githubReleasesBaseURL = "https://api.github.com"

// githubDownloadBaseURL can be overridden in tests.
var githubDownloadBaseURL = "https://github.com"

func newUpdateCmd(rawVersion string) *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update langsmith to the latest version",
		Long:  "Check for and install the latest version of the langsmith CLI.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(cmd.Context(), rawVersion, dryRun)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Check for updates without installing")

	return cmd
}

func runUpdate(ctx context.Context, currentVersion string, dryRun bool) error {
	if currentVersion == "dev" {
		return fmt.Errorf("cannot update a development build; install from a release")
	}

	latest, err := fetchLatestVersion(ctx)
	if err != nil {
		return fmt.Errorf("checking for updates: %w", err)
	}

	cmp := compareVersions(currentVersion, latest)
	format := getFormat()

	if cmp >= 0 {
		if format == "pretty" {
			fmt.Printf("langsmith is already up to date (v%s)\n", currentVersion)
		} else {
			out, _ := json.Marshal(map[string]string{
				"status":          "up-to-date",
				"current_version": currentVersion,
			})
			fmt.Println(string(out))
		}
		return nil
	}

	if dryRun {
		if format == "pretty" {
			fmt.Printf("Update available: v%s → v%s (dry run, no changes made)\n", currentVersion, latest)
		} else {
			out, _ := json.Marshal(map[string]string{
				"status":          "update-available",
				"current_version": currentVersion,
				"latest_version":  latest,
			})
			fmt.Println(string(out))
		}
		return nil
	}

	// Resolve current binary path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolving executable path: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("resolving symlinks: %w", err)
	}

	archiveName := buildArchiveName()
	archiveURL := fmt.Sprintf("%s/langchain-ai/langsmith-cli/releases/download/v%s/%s", githubDownloadBaseURL, latest, archiveName)
	checksumURL := fmt.Sprintf("%s/langchain-ai/langsmith-cli/releases/download/v%s/checksums.txt", githubDownloadBaseURL, latest)

	archivePath, err := downloadFile(ctx, archiveURL)
	if err != nil {
		return fmt.Errorf("downloading archive: %w", err)
	}
	defer os.Remove(archivePath)

	checksumPath, err := downloadFile(ctx, checksumURL)
	if err != nil {
		return fmt.Errorf("downloading checksums: %w", err)
	}
	defer os.Remove(checksumPath)

	if err := verifyChecksum(archivePath, checksumPath, archiveName); err != nil {
		return err
	}

	binaryName := "langsmith"
	if runtime.GOOS == "windows" {
		binaryName = "langsmith.exe"
	}

	var newBinaryPath string
	if runtime.GOOS == "windows" {
		newBinaryPath, err = extractBinaryFromZip(archivePath, binaryName)
	} else {
		newBinaryPath, err = extractBinary(archivePath, binaryName)
	}
	if err != nil {
		return fmt.Errorf("extracting binary: %w", err)
	}
	defer os.Remove(newBinaryPath)

	if err := replaceBinary(newBinaryPath, execPath); err != nil {
		return err
	}

	if format == "pretty" {
		fmt.Printf("Updated langsmith from v%s to v%s\n", currentVersion, latest)
	} else {
		out, _ := json.Marshal(map[string]string{
			"status":           "updated",
			"previous_version": currentVersion,
			"new_version":      latest,
		})
		fmt.Println(string(out))
	}
	return nil
}

// fetchLatestVersion queries the GitHub Releases API for the latest release tag.
func fetchLatestVersion(ctx context.Context) (string, error) {
	url := fmt.Sprintf("%s/repos/langchain-ai/langsmith-cli/releases/latest", githubReleasesBaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("requesting latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("parsing release response: %w", err)
	}

	return strings.TrimPrefix(release.TagName, "v"), nil
}

// compareVersions compares two semver strings (MAJOR.MINOR.PATCH).
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
func compareVersions(a, b string) int {
	aParts := parseVersion(a)
	bParts := parseVersion(b)

	for i := 0; i < 3; i++ {
		if aParts[i] < bParts[i] {
			return -1
		}
		if aParts[i] > bParts[i] {
			return 1
		}
	}
	return 0
}

func parseVersion(v string) [3]int {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	var result [3]int
	for i := 0; i < 3 && i < len(parts); i++ {
		result[i], _ = strconv.Atoi(parts[i])
	}
	return result
}

// buildArchiveName returns the expected archive filename for the current platform.
func buildArchiveName() string {
	ext := "tar.gz"
	if runtime.GOOS == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("langsmith_%s_%s.%s", runtime.GOOS, runtime.GOARCH, ext)
}

// downloadFile downloads a URL to a temporary file and returns its path.
func downloadFile(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status %d for %s", resp.StatusCode, url)
	}

	tmp, err := os.CreateTemp("", "langsmith-update-*")
	if err != nil {
		return "", err
	}
	defer tmp.Close()

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	return tmp.Name(), nil
}

// verifyChecksum verifies the SHA256 checksum of archivePath against checksums in checksumPath.
func verifyChecksum(archivePath, checksumPath, archiveName string) error {
	// Compute hash of archive
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("opening archive for checksum: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("computing checksum: %w", err)
	}
	actualHash := hex.EncodeToString(h.Sum(nil))

	// Read expected hash from checksums file
	data, err := os.ReadFile(checksumPath)
	if err != nil {
		return fmt.Errorf("reading checksums file: %w", err)
	}

	expectedHash := ""
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[1] == archiveName {
			expectedHash = parts[0]
			break
		}
	}

	if expectedHash == "" {
		return fmt.Errorf("checksum not found for %s in checksums file", archiveName)
	}

	if actualHash != expectedHash {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	return nil
}

// extractBinary extracts the named binary from a tar.gz archive and returns the path to a temp file.
func extractBinary(archivePath, binaryName string) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", fmt.Errorf("opening gzip reader: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("reading tar: %w", err)
		}

		// Match the binary name (may be at top level or in a subdirectory)
		if filepath.Base(hdr.Name) == binaryName && hdr.Typeflag == tar.TypeReg {
			tmp, err := os.CreateTemp("", "langsmith-bin-*")
			if err != nil {
				return "", err
			}
			defer tmp.Close()

			if _, err := io.Copy(tmp, tr); err != nil {
				os.Remove(tmp.Name())
				return "", err
			}
			return tmp.Name(), nil
		}
	}

	return "", fmt.Errorf("binary %q not found in archive", binaryName)
}

// extractBinaryFromZip extracts the named binary from a zip archive.
func extractBinaryFromZip(archivePath, binaryName string) (string, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", fmt.Errorf("opening zip: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		if filepath.Base(f.Name) == binaryName {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			defer rc.Close()

			tmp, err := os.CreateTemp("", "langsmith-bin-*")
			if err != nil {
				return "", err
			}
			defer tmp.Close()

			if _, err := io.Copy(tmp, rc); err != nil {
				os.Remove(tmp.Name())
				return "", err
			}
			return tmp.Name(), nil
		}
	}

	return "", fmt.Errorf("binary %q not found in zip archive", binaryName)
}

// replaceBinary atomically replaces the target binary with the new one.
func replaceBinary(newPath, targetPath string) error {
	// Get permissions from existing binary
	info, err := os.Stat(targetPath)
	if err != nil {
		return fmt.Errorf("reading target permissions: %w", err)
	}

	// Write to a temp file in the same directory for atomic rename
	targetDir := filepath.Dir(targetPath)
	tmp, err := os.CreateTemp(targetDir, ".langsmith-update-*")
	if err != nil {
		return fmt.Errorf("creating temp file for replacement (try running with appropriate permissions): %w", err)
	}
	tmpName := tmp.Name()

	src, err := os.Open(newPath)
	if err != nil {
		os.Remove(tmpName)
		return err
	}
	defer src.Close()

	if _, err := io.Copy(tmp, src); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	tmp.Close()

	if err := os.Chmod(tmpName, info.Mode()); err != nil {
		os.Remove(tmpName)
		return err
	}

	if err := os.Rename(tmpName, targetPath); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("replacing binary (you may need to run with sudo or install to a user-writable path like ~/.local/bin): %w", err)
	}

	return nil
}
