package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

const (
	configDir  = ".minadb"
	configFile = "config"
	configType = "yaml"
)

// Load reads the configuration from ~/.minadb/config.yaml.
// Returns an empty config if the file does not exist.
func Load() (*Config, error) {
	dir, err := configDirPath()
	if err != nil {
		return nil, fmt.Errorf("config dir: %w", err)
	}

	viper.SetConfigName(configFile)
	viper.SetConfigType(configType)
	viper.AddConfigPath(dir)

	// Defaults
	viper.SetDefault("preferences.theme", "default")

	cfg := &Config{}

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return cfg, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	if err := viper.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	return cfg, nil
}

// Save writes the configuration to ~/.minadb/config.yaml.
func Save(cfg *Config) error {
	dir, err := configDirPath()
	if err != nil {
		return fmt.Errorf("config dir: %w", err)
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	viper.Set("connections", cfg.Connections)
	viper.Set("preferences", cfg.Preferences)

	path := filepath.Join(dir, configFile+"."+configType)
	return viper.WriteConfigAs(path)
}

// DefaultConnection returns the default connection from config, or the first one.
func DefaultConnection(cfg *Config) *Connection {
	if len(cfg.Connections) == 0 {
		return nil
	}

	if cfg.Preferences.DefaultConnection != "" {
		for i := range cfg.Connections {
			if cfg.Connections[i].Name == cfg.Preferences.DefaultConnection {
				return &cfg.Connections[i]
			}
		}
	}

	return &cfg.Connections[0]
}

func configDirPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, configDir), nil
}
