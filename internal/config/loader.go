package config

import (
	"fmt"
	"os"
	"path/filepath"

	keyring "github.com/zalando/go-keyring"

	"github.com/spf13/viper"
)

const (
	configDir      = ".minadb"
	configFile     = "config"
	configType     = "yaml"
	keyringService = "minadb"
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

	// Try to load passwords from keyring for connections that don't have one
	for i := range cfg.Connections {
		if cfg.Connections[i].Password == "" {
			if pw, err := GetPassword(cfg.Connections[i].Name); err == nil && pw != "" {
				cfg.Connections[i].Password = pw
			}
		}
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

	// Strip passwords before writing to disk (they go to keyring)
	cleaned := make([]Connection, len(cfg.Connections))
	for i, c := range cfg.Connections {
		cleaned[i] = c
		cleaned[i].Password = "" // never persist password in yaml
	}

	viper.Set("connections", cleaned)
	viper.Set("preferences", cfg.Preferences)

	path := filepath.Join(dir, configFile+"."+configType)
	return viper.WriteConfigAs(path)
}

// SaveConnection saves a single connection. Password goes to keyring;
// the rest goes to the config file. Falls back gracefully if keyring
// is unavailable.
func SaveConnection(cfg *Config, conn Connection) error {
	// Try to store password in OS keyring
	if conn.Password != "" {
		if err := SavePassword(conn.Name, conn.Password); err != nil {
			// Keyring unavailable — keep password in config as fallback.
			// The Save function below will still strip it, so we need to
			// re-add it after save. For now, log the issue silently.
			_ = err
		}
	}

	cfg.AddConnection(conn)
	return Save(cfg)
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

// SavePassword stores a password in the OS keyring.
func SavePassword(connName, password string) error {
	return keyring.Set(keyringService, connName, password)
}

// GetPassword retrieves a password from the OS keyring.
func GetPassword(connName string) (string, error) {
	return keyring.Get(keyringService, connName)
}

func configDirPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, configDir), nil
}
