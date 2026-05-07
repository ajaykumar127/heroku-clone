package main

import (
	"os"
)

type Config struct {
	DatabaseURL string
	Port        string
	Environment string
}

func LoadConfig() *Config {
	return &Config{
		DatabaseURL: getEnv("DATABASE_URL", "./platform.db"),
		Port:        getEnv("PORT", "8080"),
		Environment: getEnv("ENVIRONMENT", "development"),
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
