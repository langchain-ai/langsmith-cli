package cmd

import (
	"bytes"
	"cmp"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	deployPlatform     = "linux/amd64"
	deployPushAttempts = 3
	registryTokenUser  = "oauth2accesstoken"
)

type imageReference struct {
	repository string
	tag        string
}

func parseImageReference(ref string) (imageReference, error) {
	if strings.Contains(ref, "@") {
		return imageReference{}, fmt.Errorf("%q carries a digest and cannot be tagged", ref)
	}
	pathStart := strings.LastIndex(ref, "/") + 1
	name, tag, ok := strings.Cut(ref[pathStart:], ":")
	if !ok {
		return imageReference{repository: ref}, nil
	}
	return imageReference{repository: ref[:pathStart] + name, tag: tag}, nil
}

func (r imageReference) String() string {
	if r.tag == "" {
		return r.repository
	}
	return r.repository + ":" + r.tag
}

func (r imageReference) matchesDigest(repoDigest string) bool {
	return strings.HasPrefix(repoDigest, r.repository+"@sha256:")
}

type docker struct {
	configDir string
	verbose   bool
	stderr    io.Writer
}

func requireDocker() error {
	if _, err := exec.LookPath("docker"); err != nil {
		return errors.New("docker is required for --image and --push-to but was not found on PATH; install Docker Desktop: https://docs.docker.com/get-docker/")
	}
	return nil
}

func (d docker) run(ctx context.Context, stdin string, args ...string) (string, error) {
	if d.configDir != "" {
		args = append([]string{"--config", d.configDir}, args...)
	}
	cmd := exec.CommandContext(ctx, "docker", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if d.verbose {
		cmd.Stderr = io.MultiWriter(&stderr, d.stderr)
	}
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("docker %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

func (d docker) validatePrebuiltImage(ctx context.Context, image string) error {
	out, err := d.run(ctx, "", "image", "inspect", "--format", "{{.Os}}/{{.Architecture}}", image)
	if err != nil {
		return fmt.Errorf("docker image %q was not found locally; build or pull it before deploying with --image: %w", image, err)
	}
	if platform := strings.TrimSpace(out); platform != deployPlatform {
		return fmt.Errorf("docker image %q targets %s, but LangSmith Deployment requires %s; rebuild or pull the image for %s",
			image, cmp.Or(platform, "unknown"), deployPlatform, deployPlatform)
	}
	return nil
}

func (d docker) push(ctx context.Context, image string, p *deployProgress) error {
	var err error
	for attempt := 1; attempt <= deployPushAttempts; attempt++ {
		if _, err = d.run(ctx, "", "push", image); err == nil {
			return nil
		}
		if attempt < deployPushAttempts {
			p.Info("Push failed, retrying (attempt %d of %d)...", attempt+1, deployPushAttempts)
		}
	}
	return err
}

// pushedDigest pins the registry's manifest, falling back to the tag.
func (d docker) pushedDigest(ctx context.Context, image string, p *deployProgress) string {
	ref, err := parseImageReference(image)
	if err != nil {
		return image
	}
	out, err := d.run(ctx, "", "image", "inspect", "--format", "{{json .RepoDigests}}", image)
	var digests []string
	if err == nil && json.Unmarshal([]byte(out), &digests) != nil {
		digests = nil
	}
	for _, digest := range digests {
		if ref.matchesDigest(digest) {
			return digest
		}
	}
	p.Info("Could not resolve image digest for %s; falling back to the tag-based reference.", image)
	return image
}

// tokenDockerConfig keeps credential helpers out of the push.
func tokenDockerConfig(registryHost, token string) (string, error) {
	dir, err := os.MkdirTemp("", "langsmith-deploy-docker-")
	if err != nil {
		return "", err
	}
	auth := base64.StdEncoding.EncodeToString([]byte(registryTokenUser + ":" + token))
	data, err := json.Marshal(map[string]any{"auths": map[string]any{registryHost: map[string]string{"auth": auth}}})
	if err == nil {
		err = os.WriteFile(filepath.Join(dir, "config.json"), data, 0o600)
	}
	if err != nil {
		os.RemoveAll(dir)
		return "", err
	}
	return dir, nil
}
