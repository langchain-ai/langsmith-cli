package deploy

// Build and publish customer-managed Agent Server images
// (`deploy/image.rs`).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/mda/errs"
	"github.com/langchain-ai/langsmith-cli/internal/mda/hostapi"
	"github.com/langchain-ai/langsmith-cli/internal/mda/proc"
	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
)

const digestPrefix = "@sha256:"

// ImageSourceKind is where a deployment's image comes from.
type ImageSourceKind int

// The kinds.
const (
	// ImageSourceInternalSource: the host backend builds the image from the
	// uploaded source.
	ImageSourceInternalSource ImageSourceKind = iota
	// ImageSourceCustomerRegistry: the CLI builds and pushes the image to the
	// customer's registry ([ImageSource.Target]).
	ImageSourceCustomerRegistry
)

// ImageSource is where a deployment's image comes from.
type ImageSource struct {
	Kind ImageSourceKind
	// Target is the push target of [ImageSourceCustomerRegistry].
	Target PushTarget
}

// PushTarget is a customer-managed registry repository and the tagged image
// pushed to it.
type PushTarget struct {
	repository  string
	taggedImage string
}

// commandSpec is a program and its arguments.
type commandSpec struct {
	program string
	args    []string
}

// hostBuildsImagesEnv is set to `1` when the host backend builds images from
// the uploaded source even though it is served below `/api-host`, as the
// local stack in `tools/local-stack` does. `mda-local env` sets it.
const hostBuildsImagesEnv = "MDA_HOST_BUILDS_IMAGES"

// requiresExternalImage reports whether the host backend at hostURL needs a
// customer-built image.
func requiresExternalImage(hostURL string) bool {
	value, ok := os.LookupEnv(hostBuildsImagesEnv)
	var hostBuildsImages *string
	if ok {
		hostBuildsImages = &value
	}
	return requiresExternalImageWith(hostURL, hostBuildsImages)
}

func requiresExternalImageWith(hostURL string, hostBuildsImages *string) bool {
	optedOut := false
	if hostBuildsImages != nil {
		switch strings.ToLower(strings.TrimSpace(*hostBuildsImages)) {
		case "1", "true":
			optedOut = true
		}
	}
	return !optedOut && hostapi.IsSelfHostedHostBackendURL(hostURL)
}

// selectImageSource picks the image source; pushTo is `--push-to`, nil for
// none.
func selectImageSource(selfHosted bool, pushTo *string) (ImageSource, error) {
	switch {
	case pushTo != nil && selfHosted:
		target, err := parsePushTarget(*pushTo)
		if err != nil {
			return ImageSource{}, err
		}
		return ImageSource{Kind: ImageSourceCustomerRegistry, Target: target}, nil
	case pushTo != nil:
		return ImageSource{}, errors.New("--push-to is only supported for self-hosted LangSmith deployments")
	case selfHosted:
		return ImageSource{}, errors.New("self-hosted LangSmith requires a customer-managed image; pass --push-to <repository[:tag]>")
	default:
		return ImageSource{Kind: ImageSourceInternalSource}, nil
	}
}

func parsePushTarget(value string) (PushTarget, error) {
	value = strings.TrimSpace(value)
	if strings.Contains(value, "@") || len(value) > 255 {
		return PushTarget{}, errors.New("--push-to must be a Docker repository with an optional tag, such as registry.example.com/team/agent:latest")
	}

	lastSlash := strings.LastIndex(value, "/")
	tagSeparator := strings.LastIndex(value, ":")
	if tagSeparator <= lastSlash {
		tagSeparator = -1
	}
	repository, taggedImage := value, value+":latest"
	if tagSeparator >= 0 {
		repository, taggedImage = value[:tagSeparator], value
	}
	if !validRepository(repository) || (tagSeparator >= 0 && !validTag(value[tagSeparator+1:])) {
		return PushTarget{}, errors.New("--push-to must be a valid Docker repository with an optional tag")
	}
	return PushTarget{repository: repository, taggedImage: taggedImage}, nil
}

func validRepository(value string) bool {
	if value == "" || strings.Contains(value, "//") {
		return false
	}
	for index := range len(value) {
		b := value[index]
		if !isASCIILower(b) && !isASCIIDigit(b) && !strings.ContainsRune("._-/:", rune(b)) {
			return false
		}
	}
	name := value
	if slash := strings.LastIndex(value, "/"); slash >= 0 {
		name = value[slash+1:]
	}
	return !strings.Contains(name, ":") &&
		isASCIIAlphanumeric(value[0]) &&
		isASCIIAlphanumeric(value[len(value)-1])
}

func validTag(value string) bool {
	if value == "" || len(value) > 128 || value[0] == '.' || value[0] == '-' {
		return false
	}
	for index := range len(value) {
		b := value[index]
		if !isASCIIAlphanumeric(b) && b != '_' && b != '.' && b != '-' {
			return false
		}
	}
	return true
}

func isASCIILower(b byte) bool { return b >= 'a' && b <= 'z' }
func isASCIIDigit(b byte) bool { return b >= '0' && b <= '9' }
func isASCIIAlphanumeric(b byte) bool {
	return isASCIILower(b) || (b >= 'A' && b <= 'Z') || isASCIIDigit(b)
}

// buildAndPush builds the deployment image from buildDir, pushes it to
// target, and returns its immutable digest reference.
func buildAndPush(ctx context.Context, language project.Language, buildDir string, target PushTarget) (string, error) {
	if _, err := runImageCommand(ctx, buildCommand(language, target), buildDir, "build the deployment image"); err != nil {
		return "", err
	}
	push := commandSpec{program: "docker", args: []string{"push", target.taggedImage}}
	if _, err := runImageCommand(ctx, push, buildDir, "push the deployment image"); err != nil {
		return "", err
	}

	inspect := commandSpec{
		program: "docker",
		args:    []string{"image", "inspect", "--format", "{{json .RepoDigests}}", target.taggedImage},
	}
	output, err := runImageCommand(ctx, inspect, buildDir, "inspect the pushed deployment image")
	if err != nil {
		return "", err
	}
	return resolvePushedDigest(target, output.Stdout)
}

func buildCommand(language project.Language, target PushTarget) commandSpec {
	return buildCommandForArch(language, target, runtime.GOARCH)
}

// buildCommandForArch is the image build command on a host of arch (Go's
// GOARCH names).
func buildCommandForArch(language project.Language, target PushTarget, arch string) commandSpec {
	var spec commandSpec
	switch language {
	case project.Python:
		dep := project.PythonDevServerCLI
		spec = commandSpec{
			program: "uv",
			args: []string{
				"run", "--no-project", "--with", dep.Name + dep.Version,
				"langgraph", "build", "-t", target.taggedImage,
			},
		}
	default:
		dep := project.TypeScriptDevServerCLI
		spec = commandSpec{
			program: "npx",
			args:    []string{"--yes", dep.Name + "@" + dep.Version, "build", "-t", target.taggedImage},
		}
	}
	if arch != "amd64" {
		spec.args = append(spec.args, "--platform", "linux/amd64")
	}
	return spec
}

func runImageCommand(ctx context.Context, spec commandSpec, cwd, action string) (*proc.Captured, error) {
	var cmd *exec.Cmd
	switch spec.program {
	case "npx":
		cmd = proc.Npx(spec.args...)
	case "uv":
		cmd = proc.Uv(spec.args...)
	default:
		cmd = exec.Command(spec.program, spec.args...)
	}
	cmd.Dir = cwd
	output, err := proc.Output(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to %s: could not run %s: %w", action, spec.program, err)
	}
	if !output.Status.Success() {
		return nil, errs.ChildFailed(output, "failed to "+action)
	}
	return output, nil
}

func resolvePushedDigest(target PushTarget, stdout []byte) (string, error) {
	var digests []string
	if err := json.Unmarshal(stdout, &digests); err != nil {
		return "", fmt.Errorf("docker returned an invalid image digest list: %w", err)
	}
	prefix := target.repository + digestPrefix
	for _, digest := range digests {
		if strings.HasPrefix(digest, prefix) {
			return validateImageURI(digest)
		}
	}
	return "", fmt.Errorf("pushed image has no digest for %s", target.repository)
}

func validateImageURI(value string) (string, error) {
	invalid := errors.New("pushed image did not resolve to an immutable sha256 digest")
	index := strings.LastIndex(value, digestPrefix)
	if index < 0 {
		return "", invalid
	}
	repository, digest := value[:index], value[index+len(digestPrefix):]
	if repository == "" || strings.Contains(repository, "@") || len(digest) != 64 {
		return "", invalid
	}
	for index := range len(digest) {
		b := digest[index]
		if !isASCIIDigit(b) && (b < 'a' || b > 'f') && (b < 'A' || b > 'F') {
			return "", invalid
		}
	}
	return value, nil
}
