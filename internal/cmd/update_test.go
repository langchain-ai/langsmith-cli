package cmd

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// ---------- compareVersions ----------

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"0.1.0", "0.2.0", -1},
		{"0.2.0", "0.1.0", 1},
		{"1.0.0", "0.9.9", 1},
		{"0.0.1", "0.0.2", -1},
		{"1.2.3", "1.2.3", 0},
		{"10.0.0", "9.9.9", 1},
		{"0.10.0", "0.9.0", 1},
		{"0.0.10", "0.0.9", 1},
		{"2.0.0", "1.99.99", 1},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_vs_%s", tt.a, tt.b), func(t *testing.T) {
			got := compareVersions(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("compareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// ---------- buildArchiveName ----------

func TestBuildArchiveName(t *testing.T) {
	name := buildArchiveName()

	// We can't control runtime.GOOS/GOARCH in tests, but we can verify the pattern
	expectedExt := "tar.gz"
	if runtime.GOOS == "windows" {
		expectedExt = "zip"
	}
	expected := fmt.Sprintf("langsmith_%s_%s.%s", runtime.GOOS, runtime.GOARCH, expectedExt)
	if name != expected {
		t.Errorf("got %q, want %q", name, expected)
	}
}

// ---------- verifyChecksum ----------

func TestVerifyChecksum_Correct(t *testing.T) {
	// Create a temp file with known content
	content := []byte("hello world")
	archiveFile, err := os.CreateTemp("", "test-archive-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(archiveFile.Name())
	archiveFile.Write(content)
	archiveFile.Close()

	// Compute its hash
	h := sha256.Sum256(content)
	hash := fmt.Sprintf("%x", h)

	// Create checksums file
	checksumFile, err := os.CreateTemp("", "test-checksums-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(checksumFile.Name())
	fmt.Fprintf(checksumFile, "%s  test-archive.tar.gz\n", hash)
	checksumFile.Close()

	err = verifyChecksum(archiveFile.Name(), checksumFile.Name(), "test-archive.tar.gz")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestVerifyChecksum_Mismatch(t *testing.T) {
	content := []byte("hello world")
	archiveFile, err := os.CreateTemp("", "test-archive-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(archiveFile.Name())
	archiveFile.Write(content)
	archiveFile.Close()

	checksumFile, err := os.CreateTemp("", "test-checksums-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(checksumFile.Name())
	fmt.Fprintf(checksumFile, "%s  test-archive.tar.gz\n", "0000000000000000000000000000000000000000000000000000000000000000")
	checksumFile.Close()

	err = verifyChecksum(archiveFile.Name(), checksumFile.Name(), "test-archive.tar.gz")
	if err == nil {
		t.Error("expected checksum mismatch error")
	}
}

func TestVerifyChecksum_MissingEntry(t *testing.T) {
	content := []byte("hello world")
	archiveFile, err := os.CreateTemp("", "test-archive-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(archiveFile.Name())
	archiveFile.Write(content)
	archiveFile.Close()

	checksumFile, err := os.CreateTemp("", "test-checksums-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(checksumFile.Name())
	fmt.Fprintf(checksumFile, "abc123  other-file.tar.gz\n")
	checksumFile.Close()

	err = verifyChecksum(archiveFile.Name(), checksumFile.Name(), "test-archive.tar.gz")
	if err == nil {
		t.Error("expected error for missing checksum entry")
	}
}

// ---------- fetchLatestVersion ----------

func TestFetchLatestVersion(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/langchain-ai/langsmith-cli/releases/latest" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{
			"tag_name": "v0.2.0",
		})
	}))
	defer ts.Close()

	old := githubReleasesBaseURL
	githubReleasesBaseURL = ts.URL
	defer func() { githubReleasesBaseURL = old }()

	version, err := fetchLatestVersion(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if version != "0.2.0" {
		t.Errorf("expected 0.2.0, got %s", version)
	}
}

func TestFetchLatestVersion_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	old := githubReleasesBaseURL
	githubReleasesBaseURL = ts.URL
	defer func() { githubReleasesBaseURL = old }()

	_, err := fetchLatestVersion(context.Background())
	if err == nil {
		t.Error("expected error for API failure")
	}
}

// ---------- runUpdate ----------

func TestRunUpdate_AlreadyUpToDate(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{
			"tag_name": "v0.1.7",
		})
	}))
	defer ts.Close()

	old := githubReleasesBaseURL
	githubReleasesBaseURL = ts.URL
	defer func() { githubReleasesBaseURL = old }()

	oldFmt := flagOutputFormat
	flagOutputFormat = "json"
	defer func() { flagOutputFormat = oldFmt }()

	output := captureStdout(t, func() {
		err := runUpdate(context.Background(), "0.1.7", false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	var result map[string]string
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON output: %v, output: %q", err, output)
	}
	if result["status"] != "up-to-date" {
		t.Errorf("expected status up-to-date, got %q", result["status"])
	}
}

func TestRunUpdate_DryRun(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{
			"tag_name": "v0.2.0",
		})
	}))
	defer ts.Close()

	old := githubReleasesBaseURL
	githubReleasesBaseURL = ts.URL
	defer func() { githubReleasesBaseURL = old }()

	oldFmt := flagOutputFormat
	flagOutputFormat = "json"
	defer func() { flagOutputFormat = oldFmt }()

	output := captureStdout(t, func() {
		err := runUpdate(context.Background(), "0.1.7", true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	var result map[string]string
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON output: %v, output: %q", err, output)
	}
	if result["status"] != "update-available" {
		t.Errorf("expected status update-available, got %q", result["status"])
	}
	if result["latest_version"] != "0.2.0" {
		t.Errorf("expected latest_version 0.2.0, got %q", result["latest_version"])
	}
}

func TestRunUpdate_DevBuild(t *testing.T) {
	err := runUpdate(context.Background(), "dev", false)
	if err == nil {
		t.Error("expected error for dev build")
	}
	if err.Error() != "cannot update a development build; install from a release" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

// ---------- extractBinary ----------

func TestExtractBinary(t *testing.T) {
	// Create a tar.gz with a fake binary
	archivePath := filepath.Join(t.TempDir(), "test.tar.gz")
	binaryContent := []byte("#!/bin/sh\necho hello")

	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	tw.WriteHeader(&tar.Header{
		Name:     "langsmith",
		Mode:     0755,
		Size:     int64(len(binaryContent)),
		Typeflag: tar.TypeReg,
	})
	tw.Write(binaryContent)
	tw.Close()
	gw.Close()
	f.Close()

	extractedPath, err := extractBinary(archivePath, "langsmith")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer os.Remove(extractedPath)

	got, err := os.ReadFile(extractedPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(binaryContent) {
		t.Errorf("extracted content mismatch")
	}
}

func TestExtractBinary_NotFound(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "test.tar.gz")
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	tw.Close()
	gw.Close()
	f.Close()

	_, err = extractBinary(archivePath, "langsmith")
	if err == nil {
		t.Error("expected error for missing binary in archive")
	}
}

// ---------- updateCmd flag ----------

func TestUpdateCmd_HasDryRunFlag(t *testing.T) {
	cmd := newUpdateCmd("0.1.0")
	f := cmd.Flags().Lookup("dry-run")
	if f == nil {
		t.Fatal("--dry-run flag not found")
	}
	if f.DefValue != "false" {
		t.Errorf("expected default false, got %q", f.DefValue)
	}
}
