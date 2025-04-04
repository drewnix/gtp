package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	configContent := `
server:
  port: 9090
  host: "127.0.0.1"
  readTimeoutSec: 30
processing:
  maxConcurrent: 20
  defaultTimeoutSec: 120
logging:
  level: 2
  format: "text"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	// Test loading from file
	t.Run("Load from file", func(t *testing.T) {
		cfg, err := LoadConfig(configPath)
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		// Check values from file
		if cfg.Server.Port != 9090 {
			t.Errorf("Expected port 9090, got %d", cfg.Server.Port)
		}
		if cfg.Server.Host != "127.0.0.1" {
			t.Errorf("Expected host 127.0.0.1, got %s", cfg.Server.Host)
		}
		if cfg.Server.ReadTimeoutSec != 30 {
			t.Errorf("Expected read timeout 30, got %d", cfg.Server.ReadTimeoutSec)
		}
		if cfg.Processing.MaxConcurrent != 20 {
			t.Errorf("Expected max concurrent 20, got %d", cfg.Processing.MaxConcurrent)
		}
		if cfg.Logging.Level != 2 {
			t.Errorf("Expected log level 2, got %d", cfg.Logging.Level)
		}
		if cfg.Logging.Format != "text" {
			t.Errorf("Expected log format text, got %s", cfg.Logging.Format)
		}
	})

	// Test loading defaults
	t.Run("Load defaults", func(t *testing.T) {
		cfg, err := LoadConfig("")
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		// Check default values
		if cfg.Server.Port != 8080 {
			t.Errorf("Expected default port 8080, got %d", cfg.Server.Port)
		}
		if cfg.Server.Host != "0.0.0.0" {
			t.Errorf("Expected default host 0.0.0.0, got %s", cfg.Server.Host)
		}
		if cfg.Processing.MaxConcurrent != 10 {
			t.Errorf("Expected default max concurrent 10, got %d", cfg.Processing.MaxConcurrent)
		}
	})

	// Test environment variable overrides
	t.Run("Environment variable override", func(t *testing.T) {
		// Set environment variables
		os.Setenv("SERVER_PORT", "7070")
		os.Setenv("PROCESSING_MAX_CONCURRENT", "30")
		defer func() {
			os.Unsetenv("SERVER_PORT")
			os.Unsetenv("PROCESSING_MAX_CONCURRENT")
		}()

		cfg, err := LoadConfig(configPath)
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		// Check that env vars override file values
		if cfg.Server.Port != 7070 {
			t.Errorf("Expected port 7070 from env var, got %d", cfg.Server.Port)
		}
		if cfg.Processing.MaxConcurrent != 30 {
			t.Errorf("Expected max concurrent 30 from env var, got %d", cfg.Processing.MaxConcurrent)
		}

		// Check that other values from file are preserved
		if cfg.Server.Host != "127.0.0.1" {
			t.Errorf("Expected host 127.0.0.1 from file, got %s", cfg.Server.Host)
		}
		if cfg.Logging.Format != "text" {
			t.Errorf("Expected log format text from file, got %s", cfg.Logging.Format)
		}
	})
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name      string
		config    *ServerConfig
		expectErr bool
	}{
		{
			name: "Valid config",
			config: &ServerConfig{
				Server: struct {
					Port            int    `yaml:"port" env:"SERVER_PORT"`
					Host            string `yaml:"host" env:"SERVER_HOST"`
					ReadTimeoutSec  int    `yaml:"readTimeoutSec" env:"SERVER_READ_TIMEOUT"`
					WriteTimeoutSec int    `yaml:"writeTimeoutSec" env:"SERVER_WRITE_TIMEOUT"`
					IdleTimeoutSec  int    `yaml:"idleTimeoutSec" env:"SERVER_IDLE_TIMEOUT"`
				}{
					Port: 8080,
				},
				Processing: struct {
					MaxConcurrent  int `yaml:"maxConcurrent" env:"PROCESSING_MAX_CONCURRENT"`
					DefaultTimeout int `yaml:"defaultTimeoutSec" env:"PROCESSING_DEFAULT_TIMEOUT"`
					MaxTimeout     int `yaml:"maxTimeoutSec" env:"PROCESSING_MAX_TIMEOUT"`
				}{
					MaxConcurrent:  10,
					DefaultTimeout: 60,
					MaxTimeout:     300,
				},
			},
			expectErr: false,
		},
		{
			name: "Invalid port",
			config: &ServerConfig{
				Server: struct {
					Port            int    `yaml:"port" env:"SERVER_PORT"`
					Host            string `yaml:"host" env:"SERVER_HOST"`
					ReadTimeoutSec  int    `yaml:"readTimeoutSec" env:"SERVER_READ_TIMEOUT"`
					WriteTimeoutSec int    `yaml:"writeTimeoutSec" env:"SERVER_WRITE_TIMEOUT"`
					IdleTimeoutSec  int    `yaml:"idleTimeoutSec" env:"SERVER_IDLE_TIMEOUT"`
				}{
					Port: 70000,
				},
				Processing: struct {
					MaxConcurrent  int `yaml:"maxConcurrent" env:"PROCESSING_MAX_CONCURRENT"`
					DefaultTimeout int `yaml:"defaultTimeoutSec" env:"PROCESSING_DEFAULT_TIMEOUT"`
					MaxTimeout     int `yaml:"maxTimeoutSec" env:"PROCESSING_MAX_TIMEOUT"`
				}{
					MaxConcurrent: 10,
				},
			},
			expectErr: true,
		},
		{
			name: "Invalid concurrency",
			config: &ServerConfig{
				Server: struct {
					Port            int    `yaml:"port" env:"SERVER_PORT"`
					Host            string `yaml:"host" env:"SERVER_HOST"`
					ReadTimeoutSec  int    `yaml:"readTimeoutSec" env:"SERVER_READ_TIMEOUT"`
					WriteTimeoutSec int    `yaml:"writeTimeoutSec" env:"SERVER_WRITE_TIMEOUT"`
					IdleTimeoutSec  int    `yaml:"idleTimeoutSec" env:"SERVER_IDLE_TIMEOUT"`
				}{
					Port: 8080,
				},
				Processing: struct {
					MaxConcurrent  int `yaml:"maxConcurrent" env:"PROCESSING_MAX_CONCURRENT"`
					DefaultTimeout int `yaml:"defaultTimeoutSec" env:"PROCESSING_DEFAULT_TIMEOUT"`
					MaxTimeout     int `yaml:"maxTimeoutSec" env:"PROCESSING_MAX_TIMEOUT"`
				}{
					MaxConcurrent: 0,
				},
			},
			expectErr: true,
		},
		{
			name: "Default timeout greater than max",
			config: &ServerConfig{
				Server: struct {
					Port            int    `yaml:"port" env:"SERVER_PORT"`
					Host            string `yaml:"host" env:"SERVER_HOST"`
					ReadTimeoutSec  int    `yaml:"readTimeoutSec" env:"SERVER_READ_TIMEOUT"`
					WriteTimeoutSec int    `yaml:"writeTimeoutSec" env:"SERVER_WRITE_TIMEOUT"`
					IdleTimeoutSec  int    `yaml:"idleTimeoutSec" env:"SERVER_IDLE_TIMEOUT"`
				}{
					Port: 8080,
				},
				Processing: struct {
					MaxConcurrent  int `yaml:"maxConcurrent" env:"PROCESSING_MAX_CONCURRENT"`
					DefaultTimeout int `yaml:"defaultTimeoutSec" env:"PROCESSING_DEFAULT_TIMEOUT"`
					MaxTimeout     int `yaml:"maxTimeoutSec" env:"PROCESSING_MAX_TIMEOUT"`
				}{
					MaxConcurrent:  10,
					DefaultTimeout: 500,
					MaxTimeout:     300,
				},
			},
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate()
			if tc.expectErr && err == nil {
				t.Error("Expected validation error but got none")
			}
			if !tc.expectErr && err != nil {
				t.Errorf("Expected no validation error but got: %v", err)
			}
		})
	}
}