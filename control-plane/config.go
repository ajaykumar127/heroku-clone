package main

import (
	"os"
)

type Config struct {
	DatabaseURL    string
	Port           string
	Environment    string
	ReposDir       string
	BuilderScript  string
	DeployerScript string
	RegistryURL    string
	GitServerHost  string
	GitServerPort  string
}

func LoadConfig() *Config {
	return &Config{
		DatabaseURL:    getEnv("DATABASE_URL", "./platform.db"),
		Port:           getEnv("PORT", "8080"),
		Environment:    getEnv("ENVIRONMENT", "development"),
		ReposDir:       getEnv("REPOS_DIR", "../git-server/repos"),
		BuilderScript:  getEnv("BUILDER_SCRIPT", "../builder/builder.sh"),
		DeployerScript: getEnv("DEPLOYER_SCRIPT", "../deployer/deploy.sh"),
		RegistryURL:    getEnv("REGISTRY_URL", "localhost:5000"),
		GitServerHost:  getEnv("GIT_SERVER_HOST", "localhost"),
		GitServerPort:  getEnv("GIT_SERVER_PORT", "2222"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func (c *Config) IsProduction() bool {
	return c.Environment == "production"
}

func (c *Config) IsPostgres() bool {
	return len(c.DatabaseURL) > 8 && c.DatabaseURL[:8] == "postgres"
}
