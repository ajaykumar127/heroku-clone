package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// kubectlArgs returns base kubectl arguments, injecting --kubeconfig when set.
func kubectlArgs(cfg *Config, args []string) *exec.Cmd {
	if cfg.KubeconfigPath != "" {
		full := append([]string{"--kubeconfig", cfg.KubeconfigPath}, args...)
		return exec.Command("kubectl", full...)
	}
	return exec.Command("kubectl", args...)
}

// runCmd executes cmd and returns combined stdout+stderr.
func runCmd(cmd *exec.Cmd) (string, error) {
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}

// ensureNamespace creates the namespace idempotently using the dry-run pipe trick:
//
//	kubectl create namespace <ns> --dry-run=client -o yaml | kubectl apply -f -
func ensureNamespace(cfg *Config) (string, error) {
	pr, pw, err := os.Pipe()
	if err != nil {
		return "", fmt.Errorf("pipe: %w", err)
	}

	cmd1 := kubectlArgs(cfg, []string{"create", "namespace", cfg.Namespace, "--dry-run=client", "-o", "yaml"})
	cmd1.Stdout = pw
	cmd1.Stderr = pw

	cmd2 := kubectlArgs(cfg, []string{"apply", "-f", "-"})
	cmd2.Stdin = pr
	var out2 bytes.Buffer
	cmd2.Stdout = &out2
	cmd2.Stderr = &out2

	if err := cmd1.Start(); err != nil {
		pw.Close()
		pr.Close()
		return "", fmt.Errorf("start cmd1: %w", err)
	}
	if err := cmd2.Start(); err != nil {
		pw.Close()
		pr.Close()
		return "", fmt.Errorf("start cmd2: %w", err)
	}

	// Close the write-end after cmd1 exits so cmd2 sees EOF.
	go func() {
		_ = cmd1.Wait()
		pw.Close()
	}()

	pr.Close()
	err2 := cmd2.Wait()
	return out2.String(), err2
}

// applyManifest pipes YAML to kubectl apply -f -.
func applyManifest(cfg *Config, yaml string) (string, error) {
	cmd := kubectlArgs(cfg, []string{"apply", "-f", "-"})
	cmd.Stdin = strings.NewReader(yaml)
	return runCmd(cmd)
}

// Deploy creates or updates the namespace and applies the manifest, then waits
// for the rollout to complete.
func Deploy(cfg *Config, req DeployRequest) (string, error) {
	var sb strings.Builder

	// Step 1: ensure namespace exists.
	out, err := ensureNamespace(cfg)
	sb.WriteString(out)
	if err != nil {
		return sb.String(), fmt.Errorf("ensure namespace: %w", err)
	}

	// Step 2: generate and apply manifest.
	manifest := GenerateManifest(cfg, req)
	out, err = applyManifest(cfg, manifest)
	sb.WriteString(out)
	if err != nil {
		return sb.String(), fmt.Errorf("apply manifest: %w", err)
	}

	// Step 3: wait for rollout.
	out, err = WaitReady(cfg, req.AppName)
	sb.WriteString(out)
	if err != nil {
		return sb.String(), fmt.Errorf("rollout status: %w", err)
	}

	return sb.String(), nil
}

// Scale updates the replica count for an existing deployment.
func Scale(cfg *Config, appName string, replicas int) (string, error) {
	cmd := kubectlArgs(cfg, []string{
		"scale", "deployment", appName,
		fmt.Sprintf("--replicas=%d", replicas),
		"-n", cfg.Namespace,
	})
	return runCmd(cmd)
}

// Destroy deletes all K8s resources for an app (Deployment, Service, Ingress).
func Destroy(cfg *Config, appName string) (string, error) {
	var sb strings.Builder
	resources := []string{"deployment", "service", "ingress"}
	for _, r := range resources {
		cmd := kubectlArgs(cfg, []string{
			"delete", r, appName,
			"-n", cfg.Namespace,
			"--ignore-not-found",
		})
		out, err := runCmd(cmd)
		sb.WriteString(out)
		if err != nil {
			return sb.String(), fmt.Errorf("delete %s/%s: %w", r, appName, err)
		}
	}
	return sb.String(), nil
}

// WaitReady waits up to 5 minutes for the deployment rollout to complete.
func WaitReady(cfg *Config, appName string) (string, error) {
	cmd := kubectlArgs(cfg, []string{
		"rollout", "status",
		fmt.Sprintf("deployment/%s", appName),
		"-n", cfg.Namespace,
		"--timeout=5m",
	})
	return runCmd(cmd)
}
