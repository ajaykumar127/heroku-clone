package main

import "os"

// Config holds all configuration values loaded from environment variables.
type Config struct {
	Port           string // PORT, default "8081"
	RegistryURL    string // REGISTRY_URL, default "localhost:5000"
	BuildDir       string // BUILD_DIR, default "/tmp/builds"
	DefaultBuilder string // CNB_BUILDER, default "paketobuildpacks/builder-jammy-base"
}

// LoadConfig reads configuration from environment variables, applying defaults
// for any value that is not set.
func LoadConfig() *Config {
	cfg := &Config{
		Port:           getEnv("PORT", "8081"),
		RegistryURL:    getEnv("REGISTRY_URL", "localhost:5000"),
		BuildDir:       getEnv("BUILD_DIR", "/tmp/builds"),
		DefaultBuilder: getEnv("CNB_BUILDER", "paketobuildpacks/builder-jammy-base"),
	}
	return cfg
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
