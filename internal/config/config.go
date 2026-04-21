package config

import "os"

const (
	defaultAppName     = "kubernetes-cost-guard"
	defaultEnvironment = "local"
	defaultHTTPAddr    = ":8080"
	defaultVersion     = "dev"
)

// Config contains environment-driven settings for the API.
// Phase 1 intentionally keeps this small; integrations are added in later phases.
type Config struct {
	AppName     string
	Environment string
	HTTPAddr    string
	Version     string
	Kubeconfig  string
}

func Load() Config {
	return Config{
		AppName:     getEnv("KCG_APP_NAME", defaultAppName),
		Environment: getEnv("KCG_ENV", defaultEnvironment),
		HTTPAddr:    getEnv("KCG_HTTP_ADDR", defaultHTTPAddr),
		Version:     getEnv("KCG_VERSION", defaultVersion),
		Kubeconfig:  getEnv("KCG_KUBECONFIG", os.Getenv("KUBECONFIG")),
	}
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
