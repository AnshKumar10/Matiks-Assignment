package config

import (
	"log"
	"os"
)

type Config struct {
	AppEnv       string
	HTTPPort     string
	RedisAddr    string
	RedisPassword string
	PostgresDSN  string
}

func Load() *Config {
	cfg := &Config{
		AppEnv:       getEnv("APP_ENV", "development"),
		HTTPPort:    getEnv("HTTP_PORT", "8080"),
		RedisAddr:   getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		PostgresDSN: getEnv("POSTGRES_DSN", ""),
	}

	if cfg.PostgresDSN == "" {
		log.Fatal("POSTGRES_DSN is required")
	}

	return cfg
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
