package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"
)

const (
	reposDir   = "./repos"
	apiURL     = "http://localhost:8080"
	listenAddr = ":2222"
)

type GitServer struct {
	apiURL string
}

func NewGitServer(apiURL string) *GitServer {
	return &GitServer{apiURL: apiURL}
}

// Handle SSH sessions for git operations
func (g *GitServer) handleSSH(s ssh.Session) {
	cmd := s.Command()
	if len(cmd) == 0 {
		io.WriteString(s, "Interactive shell not supported. Use git commands only.\n")
		s.Exit(1)
		return
	}

	// Parse git command: git-receive-pack 'repo-name.git'
	gitCmd := cmd[0]
	if !strings.HasPrefix(gitCmd, "git-") {
		io.WriteString(s, "Only git commands are supported\n")
		s.Exit(1)
		return
	}

	// Extract repo name
	var repoName string
	if len(cmd) > 1 {
		repoName = strings.Trim(cmd[1], "'\"")
		repoName = strings.TrimSuffix(repoName, ".git")
		repoName = strings.TrimPrefix(repoName, "/")
	}

	if repoName == "" {
		io.WriteString(s, "Repository name required\n")
		s.Exit(1)
		return
	}

	repoPath := filepath.Join(reposDir, repoName+".git")

	// Initialize repo if it doesn't exist
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		if err := g.initRepo(repoPath); err != nil {
			log.Printf("Failed to init repo %s: %v", repoName, err)
			io.WriteString(s, fmt.Sprintf("Failed to initialize repository: %v\n", err))
			s.Exit(1)
			return
		}
	}

	// Set up git command
	gitCommand := exec.Command("git", strings.TrimPrefix(gitCmd, "git-"), repoPath)
	gitCommand.Dir = repoPath
	gitCommand.Env = append(os.Environ(),
		"GIT_DIR="+repoPath,
		"REPO_NAME="+repoName,
	)

	// Connect stdin/stdout/stderr
	gitCommand.Stdin = s
	gitCommand.Stdout = s
	gitCommand.Stderr = s

	// Run git command
	if err := gitCommand.Run(); err != nil {
		log.Printf("Git command failed for %s: %v", repoName, err)
		s.Exit(1)
		return
	}

	s.Exit(0)
}

func (g *GitServer) initRepo(repoPath string) error {
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		return err
	}

	cmd := exec.Command("git", "init", "--bare", repoPath)
	if err := cmd.Run(); err != nil {
		return err
	}

	// Install post-receive hook
	hookPath := filepath.Join(repoPath, "hooks", "post-receive")
	hookScript := `#!/bin/bash
# Post-receive hook - triggers build

set -e

REPO_NAME=$(basename "$GIT_DIR" .git)

# Read old and new commit from stdin
while read oldrev newrev refname; do
    echo "-----> Receiving push for $REPO_NAME"
    echo "       $oldrev -> $newrev ($refname)"

    # Trigger build via API
    COMMIT=$(git rev-parse --short $newrev)

    echo "-----> Triggering build for commit $COMMIT"

    curl -s -X POST http://localhost:8080/internal/builds \
        -H "Content-Type: application/json" \
        -d "{\"app_name\":\"$REPO_NAME\",\"commit\":\"$COMMIT\"}" \
        > /dev/null 2>&1 || echo "Warning: Failed to trigger build"

    echo "-----> Build queued successfully"
    echo "-----> Deploy in progress..."
done
`

	if err := os.WriteFile(hookPath, []byte(hookScript), 0755); err != nil {
		return err
	}

	log.Printf("Initialized bare repository at %s", repoPath)
	return nil
}

func (g *GitServer) triggerBuild(appName, commit string) error {
	payload := map[string]string{
		"app_name": appName,
		"commit":   commit,
	}
	body, _ := json.Marshal(payload)

	resp, err := http.Post(
		g.apiURL+"/internal/builds",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("build trigger failed with status %d", resp.StatusCode)
	}

	return nil
}

// Simple public key authentication (for POC, accept all keys)
func publicKeyAuth(ctx ssh.Context, key ssh.PublicKey) bool {
	// In production, verify against stored keys in database
	log.Printf("Auth attempt with key: %s", gossh.FingerprintSHA256(key))
	return true // Accept all for POC
}

func main() {
	// Create repos directory
	if err := os.MkdirAll(reposDir, 0755); err != nil {
		log.Fatal("Failed to create repos directory:", err)
	}

	gitServer := NewGitServer(apiURL)

	server := &ssh.Server{
		Addr:             listenAddr,
		Handler:          gitServer.handleSSH,
		PublicKeyHandler: publicKeyAuth,
		Version:          "Platform-Git-Server",
	}

	// Generate host key if not exists
	hostKeyPath := "./ssh_host_key"
	if _, err := os.Stat(hostKeyPath); os.IsNotExist(err) {
		log.Println("Generating host key...")
		cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-f", hostKeyPath, "-N", "")
		if err := cmd.Run(); err != nil {
			log.Fatal("Failed to generate host key:", err)
		}
	}

	// Load host key
	hostKeyBytes, err := os.ReadFile(hostKeyPath)
	if err != nil {
		log.Fatal("Failed to read host key:", err)
	}

	signer, err := gossh.ParsePrivateKey(hostKeyBytes)
	if err != nil {
		log.Fatal("Failed to parse host key:", err)
	}

	server.AddHostKey(signer)

	log.Printf("Git server starting on %s", listenAddr)
	log.Printf("Repos directory: %s", reposDir)
	log.Fatal(server.ListenAndServe())
}
