package main

import (
	"os"
	"strconv"
	"time"
)

// Config holds all configuration loaded from environment variables.
type Config struct {
	// Identity
	RuntimeName string // RUNTIME_NAME, e.g. "aws-us-east-1"
	Cloud       string // CLOUD, e.g. "aws", "gcp", "azure", "local"
	Region      string // REGION, e.g. "us-east-1"
	AgentURL    string // AGENT_URL — this agent's own HTTP URL (optional, for future push)

	// Control Plane
	ControlPlaneURL string // CONTROL_PLANE_URL, e.g. "http://control-plane:8080"

	// Build config
	RegistryURL    string // REGISTRY_URL, e.g. "123456.dkr.ecr.us-east-1.amazonaws.com"
	BuilderScript  string // BUILDER_SCRIPT, default: "/app/builder/builder.sh"
	DeployerScript string // DEPLOYER_SCRIPT, default: "/app/deployer/deploy.sh"

	// Polling
	PollInterval time.Duration // POLL_INTERVAL, default: 5s

	// Kubernetes namespace for deployed apps
	AppsNamespace string // APPS_NAMESPACE, default: "platform-apps"

	// Shared secret for /internal/ routes on the control plane
	InternalAPISecret string // INTERNAL_API_SECRET
}

// LoadConfig reads configuration from environment variables, applying defaults where appropriate.
func LoadConfig() *Config {
	cfg := &Config{
		RuntimeName:     getEnv("RUNTIME_NAME", "local"),
		Cloud:           getEnv("CLOUD", "local"),
		Region:          getEnv("REGION", "local"),
		AgentURL:        getEnv("AGENT_URL", ""),
		ControlPlaneURL: getEnv("CONTROL_PLANE_URL", "http://localhost:8080"),
		RegistryURL:     getEnv("REGISTRY_URL", "localhost:5000"),
		BuilderScript:   getEnv("BUILDER_SCRIPT", "/app/builder/builder.sh"),
		DeployerScript:  getEnv("DEPLOYER_SCRIPT", "/app/deployer/deploy.sh"),
		AppsNamespace:     getEnv("APPS_NAMESPACE", "platform-apps"),
		PollInterval:      parseDuration(getEnv("POLL_INTERVAL", "5s"), 5*time.Second),
		InternalAPISecret: getEnv("INTERNAL_API_SECRET", ""),
	}
	return cfg
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func parseDuration(s string, defaultVal time.Duration) time.Duration {
	// Try parsing as seconds (integer) first, then as a Go duration string.
	if secs, err := strconv.Atoi(s); err == nil {
		return time.Duration(secs) * time.Second
	}
	if d, err := time.ParseDuration(s); err == nil {
		return d
	}
	return defaultVal
}
