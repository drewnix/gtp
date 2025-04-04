package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

// ServerConfig holds all configuration for the task processor service
type ServerConfig struct {
	// Server settings
	Server struct {
		Port            int    `yaml:"port" env:"SERVER_PORT"`
		Host            string `yaml:"host" env:"SERVER_HOST"`
		ReadTimeoutSec  int    `yaml:"readTimeoutSec" env:"SERVER_READ_TIMEOUT"`
		WriteTimeoutSec int    `yaml:"writeTimeoutSec" env:"SERVER_WRITE_TIMEOUT"`
		IdleTimeoutSec  int    `yaml:"idleTimeoutSec" env:"SERVER_IDLE_TIMEOUT"`
	} `yaml:"server"`

	// Processing settings
	Processing struct {
		MaxConcurrent  int `yaml:"maxConcurrent" env:"PROCESSING_MAX_CONCURRENT"`
		DefaultTimeout int `yaml:"defaultTimeoutSec" env:"PROCESSING_DEFAULT_TIMEOUT"`
		MaxTimeout     int `yaml:"maxTimeoutSec" env:"PROCESSING_MAX_TIMEOUT"`
	} `yaml:"processing"`

	// Logging settings
	Logging struct {
		Level  int    `yaml:"level" env:"LOG_LEVEL"`
		Format string `yaml:"format" env:"LOG_FORMAT"`
	} `yaml:"logging"`
}

// LoadConfig loads configuration from a YAML file and environment variables
// Environment variables take precedence over file configuration
func LoadConfig(configPath string) (*ServerConfig, error) {
	// Default configuration
	config := &ServerConfig{}
	
	// Set defaults
	config.Server.Port = 8080
	config.Server.Host = "0.0.0.0"
	config.Server.ReadTimeoutSec = 10
	config.Server.WriteTimeoutSec = 10
	config.Server.IdleTimeoutSec = 30
	
	config.Processing.MaxConcurrent = 10
	config.Processing.DefaultTimeout = 60
	config.Processing.MaxTimeout = 300
	
	config.Logging.Level = 0
	config.Logging.Format = "json"

	// Load from file if specified
	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read config file")
		}

		err = yaml.Unmarshal(data, config)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse config YAML")
		}
	}

	// Override with environment variables
	if port := os.Getenv("SERVER_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			config.Server.Port = p
		}
	}
	
	if host := os.Getenv("SERVER_HOST"); host != "" {
		config.Server.Host = host
	}
	
	if timeout := os.Getenv("SERVER_READ_TIMEOUT"); timeout != "" {
		if t, err := strconv.Atoi(timeout); err == nil {
			config.Server.ReadTimeoutSec = t
		}
	}
	
	if timeout := os.Getenv("SERVER_WRITE_TIMEOUT"); timeout != "" {
		if t, err := strconv.Atoi(timeout); err == nil {
			config.Server.WriteTimeoutSec = t
		}
	}
	
	if timeout := os.Getenv("SERVER_IDLE_TIMEOUT"); timeout != "" {
		if t, err := strconv.Atoi(timeout); err == nil {
			config.Server.IdleTimeoutSec = t
		}
	}
	
	if concurrent := os.Getenv("PROCESSING_MAX_CONCURRENT"); concurrent != "" {
		if c, err := strconv.Atoi(concurrent); err == nil {
			config.Processing.MaxConcurrent = c
		}
	}
	
	if timeout := os.Getenv("PROCESSING_DEFAULT_TIMEOUT"); timeout != "" {
		if t, err := strconv.Atoi(timeout); err == nil {
			config.Processing.DefaultTimeout = t
		}
	}
	
	if timeout := os.Getenv("PROCESSING_MAX_TIMEOUT"); timeout != "" {
		if t, err := strconv.Atoi(timeout); err == nil {
			config.Processing.MaxTimeout = t
		}
	}
	
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		if l, err := strconv.Atoi(level); err == nil {
			config.Logging.Level = l
		}
	}
	
	if format := os.Getenv("LOG_FORMAT"); format != "" {
		config.Logging.Format = format
	}
	
	return config, nil
}

// Validate checks if the configuration is valid
func (c *ServerConfig) Validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("server port must be between 1 and 65535")
	}
	
	if c.Processing.MaxConcurrent < 1 {
		return fmt.Errorf("max concurrent tasks must be at least 1")
	}
	
	if c.Processing.MaxTimeout > 0 && c.Processing.DefaultTimeout > c.Processing.MaxTimeout {
		return fmt.Errorf("default timeout cannot be greater than max timeout")
	}
	
	return nil
}