package main

import "os"

// Config holds the runtime configuration for the deployer service.
type Config struct {
	Port           string // PORT, default "8082"
	Namespace      string // K8S_NAMESPACE, default "platform-apps"
	IngressClass   string // INGRESS_CLASS, default "nginx"
	AppsDomain     string // APPS_DOMAIN, default "localhost"
	KubeconfigPath string // KUBECONFIG, default "" (use in-cluster config)
}

// LoadConfig reads configuration from environment variables, applying defaults
// for any variables that are not set.
func LoadConfig() *Config {
	return &Config{
		Port:           getEnv("PORT", "8082"),
		Namespace:      getEnv("K8S_NAMESPACE", "platform-apps"),
		IngressClass:   getEnv("INGRESS_CLASS", "nginx"),
		AppsDomain:     getEnv("APPS_DOMAIN", "localhost"),
		KubeconfigPath: getEnv("KUBECONFIG", ""),
	}
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
