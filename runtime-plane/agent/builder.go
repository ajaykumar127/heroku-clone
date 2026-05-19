package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Build clones the repo, runs a CNB build, pushes the image to the registry.
// Returns combined stdout+stderr output as a string.
func Build(cfg *Config, job Job) (string, error) {
	var out bytes.Buffer

	buildDir := filepath.Join("/tmp/build", fmt.Sprintf("%s-%s", job.AppName, job.Commit))

	// Ensure clean build directory
	if err := os.RemoveAll(buildDir); err != nil {
		return "", fmt.Errorf("clean build dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(buildDir), 0o755); err != nil {
		return "", fmt.Errorf("create build parent dir: %w", err)
	}

	// Defer cleanup of build directory
	defer func() {
		_ = os.RemoveAll(buildDir)
	}()

	// Step 1: clone repository
	fmt.Fprintf(&out, "-----> Cloning %s\n", job.RepoURL)
	cloneCmd := exec.Command("git", "clone", job.RepoURL, buildDir)
	cloneCmd.Stdout = &out
	cloneCmd.Stderr = &out
	if err := cloneCmd.Run(); err != nil {
		return out.String(), fmt.Errorf("git clone: %w", err)
	}

	// Step 2: determine build method
	if packPath, err := exec.LookPath("pack"); err == nil {
		// pack CLI is available — use Cloud Native Buildpacks directly
		fmt.Fprintf(&out, "-----> Building with pack CLI (%s)\n", packPath)
		packCmd := exec.Command(
			"pack", "build", job.ImageName,
			"--builder", "paketobuildpacks/builder:base",
			"--path", buildDir,
			"--publish",
		)
		packCmd.Stdout = &out
		packCmd.Stderr = &out
		if err := packCmd.Run(); err != nil {
			return out.String(), fmt.Errorf("pack build: %w", err)
		}
	} else {
		// Fall back to the builder shell script
		fmt.Fprintf(&out, "-----> pack not found; running builder script %s\n", cfg.BuilderScript)
		scriptCmd := exec.Command("bash", cfg.BuilderScript, job.AppName, job.Commit, buildDir)
		scriptCmd.Stdout = &out
		scriptCmd.Stderr = &out
		if err := scriptCmd.Run(); err != nil {
			return out.String(), fmt.Errorf("builder script: %w", err)
		}
	}

	fmt.Fprintf(&out, "=====> Build complete: %s\n", job.ImageName)
	return out.String(), nil
}
