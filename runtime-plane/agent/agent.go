package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// Job represents a build/deploy job dispatched by the Control Plane.
type Job struct {
	ID        string    `json:"id"`
	RuntimeID string    `json:"runtime_id"`
	AppID     string    `json:"app_id"`
	ReleaseID string    `json:"release_id"`
	AppName   string    `json:"app_name"`
	Commit    string    `json:"commit"`
	RepoURL   string    `json:"repo_url"`   // ssh://git@host:2222/app.git
	ImageName string    `json:"image_name"` // registry/app:commit
	Type      string    `json:"type"`
	Status    string    `json:"status"`
	LogOutput string    `json:"log_output,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Agent is the runtime agent that bridges this K8s cluster with the Control Plane.
type Agent struct {
	config    *Config
	client    *http.Client
	runtimeID string // assigned by control plane on registration
}

// registerRequest is the payload sent to the Control Plane on registration.
type registerRequest struct {
	Name   string `json:"name"`
	Cloud  string `json:"cloud"`
	Region string `json:"region"`
	URL    string `json:"url,omitempty"`
}

// registerResponse is the payload returned by the Control Plane on registration.
type registerResponse struct {
	RuntimeID string `json:"runtime_id"`
	ID        string `json:"id"`
}

// statusUpdate is the payload sent when reporting job status.
type statusUpdate struct {
	Status    string `json:"status"`
	LogOutput string `json:"log_output"`
}

// setInternalSecret adds the X-Internal-Secret header to req if the secret is configured.
func (a *Agent) setInternalSecret(req *http.Request) {
	if a.config.InternalAPISecret != "" {
		req.Header.Set("X-Internal-Secret", a.config.InternalAPISecret)
	}
}

// newInternalRequest creates an HTTP request for an /internal/ endpoint and
// attaches the shared-secret header when configured.
func (a *Agent) newInternalRequest(method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	a.setInternalSecret(req)
	return req, nil
}

// Register POSTs to /internal/runtime/register and stores the returned runtime ID.
// Retries up to 10 times with 5s backoff before giving up.
func (a *Agent) Register() error {
	payload := registerRequest{
		Name:   a.config.RuntimeName,
		Cloud:  a.config.Cloud,
		Region: a.config.Region,
		URL:    a.config.AgentURL,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal register request: %w", err)
	}

	url := fmt.Sprintf("%s/internal/runtime/register", a.config.ControlPlaneURL)

	const maxAttempts = 10
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, err := a.newInternalRequest(http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("create register request: %w", err)
		}
		resp, err := a.client.Do(req)
		if err != nil {
			log.Printf("Register attempt %d/%d failed (network): %v", attempt, maxAttempts, err)
		} else {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
				var reg registerResponse
				if decodeErr := json.NewDecoder(resp.Body).Decode(&reg); decodeErr != nil {
					log.Printf("Register attempt %d/%d: failed to decode response: %v", attempt, maxAttempts, decodeErr)
				} else {
					id := reg.RuntimeID
					if id == "" {
						id = reg.ID
					}
					if id != "" {
						a.runtimeID = id
						return nil
					}
					log.Printf("Register attempt %d/%d: response contained no runtime ID", attempt, maxAttempts)
				}
			} else {
				respBody, _ := io.ReadAll(resp.Body)
				log.Printf("Register attempt %d/%d: unexpected status %d: %s", attempt, maxAttempts, resp.StatusCode, string(respBody))
			}
		}

		if attempt < maxAttempts {
			log.Printf("Retrying registration in 5s...")
			time.Sleep(5 * time.Second)
			// Reset body for retry
			body, _ = json.Marshal(payload)
		}
	}

	return fmt.Errorf("failed to register with control plane after %d attempts", maxAttempts)
}

// Run is the main loop: polls for jobs every config.PollInterval, processes each one.
func (a *Agent) Run(ctx context.Context) {
	log.Printf("Agent run loop started (poll interval: %s)", a.config.PollInterval)
	ticker := time.NewTicker(a.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("Agent shutting down: %v", ctx.Err())
			return
		case <-ticker.C:
			jobs, err := a.poll()
			if err != nil {
				log.Printf("Poll error: %v", err)
				continue
			}
			for _, job := range jobs {
				go a.processJob(job)
			}
		}
	}
}

// poll calls GET /internal/runtime/{id}/jobs and returns the list of pending jobs.
func (a *Agent) poll() ([]Job, error) {
	url := fmt.Sprintf("%s/internal/runtime/%s/jobs", a.config.ControlPlaneURL, a.runtimeID)
	req, err := a.newInternalRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create poll request: %w", err)
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var jobs []Job
	if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
		return nil, fmt.Errorf("decode jobs: %w", err)
	}
	return jobs, nil
}

// processJob handles a single job: runs build then deploy, reports status throughout.
func (a *Agent) processJob(job Job) {
	log.Printf("Processing job %s (app=%s commit=%s type=%s)", job.ID, job.AppName, job.Commit, job.Type)

	// Acknowledge: mark job as running
	if err := a.reportStatus(job.ID, "running", "Job accepted by agent\n"); err != nil {
		log.Printf("Failed to report running status for job %s: %v", job.ID, err)
	}

	var combinedLogs string

	// Build phase
	log.Printf("Starting build for job %s", job.ID)
	buildLogs, buildErr := Build(a.config, job)
	combinedLogs += buildLogs

	if buildErr != nil {
		errMsg := fmt.Sprintf("\nBuild failed: %v\n", buildErr)
		combinedLogs += errMsg
		log.Printf("Build failed for job %s: %v", job.ID, buildErr)
		_ = a.reportStatus(job.ID, "failed", combinedLogs)
		return
	}

	log.Printf("Build succeeded for job %s", job.ID)
	_ = a.reportStatus(job.ID, "building", combinedLogs)

	// Deploy phase
	log.Printf("Starting deploy for job %s", job.ID)
	deployLogs, deployErr := Deploy(a.config, job, 1)
	combinedLogs += deployLogs

	if deployErr != nil {
		errMsg := fmt.Sprintf("\nDeploy failed: %v\n", deployErr)
		combinedLogs += errMsg
		log.Printf("Deploy failed for job %s: %v", job.ID, deployErr)
		_ = a.reportStatus(job.ID, "failed", combinedLogs)
		return
	}

	log.Printf("Deploy succeeded for job %s", job.ID)
	_ = a.reportStatus(job.ID, "succeeded", combinedLogs)
}

// reportStatus POSTs to /internal/runtime/{runtime_id}/jobs/{job_id}/status.
func (a *Agent) reportStatus(jobID, status, logOutput string) error {
	update := statusUpdate{
		Status:    status,
		LogOutput: logOutput,
	}
	body, err := json.Marshal(update)
	if err != nil {
		return fmt.Errorf("marshal status update: %w", err)
	}

	url := fmt.Sprintf("%s/internal/runtime/%s/jobs/%s/status",
		a.config.ControlPlaneURL, a.runtimeID, jobID)

	req, err := a.newInternalRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create status request: %w", err)
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("POST %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// sendHeartbeat PATCHes /internal/runtime/{id}/heartbeat every 30s in a goroutine.
func (a *Agent) sendHeartbeat() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		url := fmt.Sprintf("%s/internal/runtime/%s/heartbeat", a.config.ControlPlaneURL, a.runtimeID)
		req, err := a.newInternalRequest(http.MethodPatch, url, nil)
		if err != nil {
			log.Printf("Heartbeat: create request error: %v", err)
			continue
		}
		resp, err := a.client.Do(req)
		if err != nil {
			log.Printf("Heartbeat: send error: %v", err)
			continue
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
			log.Printf("Heartbeat: unexpected status %d", resp.StatusCode)
		}
	}
}
