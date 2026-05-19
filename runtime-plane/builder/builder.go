package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// BuildRequest contains the parameters for a single build job.
type BuildRequest struct {
	AppName   string `json:"app_name"`
	Commit    string `json:"commit"`
	RepoURL   string `json:"repo_url"`
	ImageName string `json:"image_name"`
}

// BuildResult is the outcome of a completed (or failed) build.
type BuildResult struct {
	Status string `json:"status"`
	Image  string `json:"image"`
	Output string `json:"output"`
}

// Build executes the full build pipeline:
//  1. Creates a temporary working directory.
//  2. Clones the source repository.
//  3. Detects the application type.
//  4. Builds the container image (docker build or pack).
//  5. Pushes the image to the registry.
//  6. Cleans up the working directory.
func Build(cfg *Config, req BuildRequest) BuildResult {
	var logs bytes.Buffer

	logf := func(format string, args ...any) {
		line := fmt.Sprintf(format+"\n", args...)
		logs.WriteString(line)
	}

	fail := func(msg string) BuildResult {
		return BuildResult{
			Status: "failed",
			Image:  req.ImageName,
			Output: logs.String() + msg + "\n",
		}
	}

	// 1. Create build directory.
	buildDir := filepath.Join(cfg.BuildDir, fmt.Sprintf("%s-%s", req.AppName, req.Commit))
	logf("==> Creating build directory: %s", buildDir)
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		return fail(fmt.Sprintf("failed to create build directory: %v", err))
	}
	defer func() {
		logf("==> Cleaning up build directory: %s", buildDir)
		os.RemoveAll(buildDir)
	}()

	// 2. Clone repository.
	logf("==> Cloning %s", req.RepoURL)
	cloneOut, err := runCommand("git", "clone", req.RepoURL, buildDir)
	logs.WriteString(cloneOut)
	if err != nil {
		return fail(fmt.Sprintf("git clone failed: %v", err))
	}

	// 3. Detect application type.
	appType := Detect(buildDir)
	logf("==> Detected app type: %s", appType)

	// 4+5. Build and push the image.
	if appType == AppTypeDockerfile {
		logf("==> Building image with Docker: %s", req.ImageName)
		buildOut, err := runCommand("docker", "build", "-t", req.ImageName, buildDir)
		logs.WriteString(buildOut)
		if err != nil {
			return fail(fmt.Sprintf("docker build failed: %v", err))
		}

		logf("==> Pushing image: %s", req.ImageName)
		pushOut, err := runCommand("docker", "push", req.ImageName)
		logs.WriteString(pushOut)
		if err != nil {
			return fail(fmt.Sprintf("docker push failed: %v", err))
		}
	} else {
		// Verify that pack is available before attempting to use it.
		if _, err := exec.LookPath("pack"); err != nil {
			if errors.Is(err, exec.ErrNotFound) {
				return fail("pack CLI not found, install it")
			}
			return fail(fmt.Sprintf("could not locate pack CLI: %v", err))
		}

		logf("==> Building image with Cloud Native Buildpacks (pack): %s", req.ImageName)
		packOut, err := runCommand(
			"pack", "build", req.ImageName,
			"--builder", cfg.DefaultBuilder,
			"--path", buildDir,
			"--publish",
		)
		logs.WriteString(packOut)
		if err != nil {
			return fail(fmt.Sprintf("pack build failed: %v", err))
		}
	}

	logf("==> Build succeeded: %s", req.ImageName)
	return BuildResult{
		Status: "succeeded",
		Image:  req.ImageName,
		Output: logs.String(),
	}
}

// runCommand executes an external command and returns its combined stdout/stderr
// output along with any error.
func runCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
