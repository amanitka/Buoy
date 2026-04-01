package main

import (
	"os"

	"k8s.io/apimachinery/pkg/util/yaml"
)

type Config struct {
	DefaultPollInterval string `json:"defaultPollInterval"`
}

func LoadConfig() *Config {
	config := &Config{
		DefaultPollInterval: "@daily",
	}

	if data, err := os.ReadFile("config.yaml"); err == nil {
		_ = yaml.Unmarshal(data, config)
	}

	if envInterval := os.Getenv("BUOY_POLL_INTERVAL"); envInterval != "" {
		config.DefaultPollInterval = envInterval
	}

	return config
}
