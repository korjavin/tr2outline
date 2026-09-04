package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds the application configuration loaded from environment variables.
type Config struct {
	Port                  int
	AnarlogWebhookSecrets []string
	OutlineURL            string
	OutlineAPIKey         string
	OutlineCollectionID   string
}

// LoadConfig loads configuration from the environment and optional .env file.
func LoadConfig() (*Config, error) {
	// Try loading .env file if present, ignore error if it doesn't exist
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		log.Printf("Notice: .env file not loaded: %v", err)
	}

	port := 3000
	if portStr := os.Getenv("PORT"); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil && p > 0 {
			port = p
		} else {
			log.Printf("Warning: invalid PORT %q, using default %d", portStr, port)
		}
	}

	rawSecrets := os.Getenv("ANARLOG_WEBHOOK_SECRET")
	var secrets []string
	for _, s := range strings.Split(rawSecrets, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			secrets = append(secrets, s)
		}
	}

	cfg := &Config{
		Port:                  port,
		AnarlogWebhookSecrets: secrets,
		OutlineURL:            strings.TrimRight(os.Getenv("OUTLINE_URL"), "/"),
		OutlineAPIKey:         os.Getenv("OUTLINE_API_KEY"),
		OutlineCollectionID:   os.Getenv("OUTLINE_COLLECTION_ID"),
	}

	var missing []string
	if len(cfg.AnarlogWebhookSecrets) == 0 {
		missing = append(missing, "ANARLOG_WEBHOOK_SECRET")
	}
	if cfg.OutlineURL == "" {
		missing = append(missing, "OUTLINE_URL")
	}
	if cfg.OutlineAPIKey == "" {
		missing = append(missing, "OUTLINE_API_KEY")
	}
	if cfg.OutlineCollectionID == "" {
		missing = append(missing, "OUTLINE_COLLECTION_ID")
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	return cfg, nil
}
