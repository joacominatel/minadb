package config

import (
	"fmt"
	"net/url"
	"strconv"
)

// Config represents the application configuration.
type Config struct {
	Connections []Connection `mapstructure:"connections" yaml:"connections"`
	Preferences Preferences  `mapstructure:"preferences" yaml:"preferences"`
}

// Connection represents a saved database connection profile.
type Connection struct {
	Name     string `mapstructure:"name" yaml:"name"`
	Driver   string `mapstructure:"driver" yaml:"driver"`
	Host     string `mapstructure:"host" yaml:"host"`
	Port     int    `mapstructure:"port" yaml:"port"`
	Database string `mapstructure:"database" yaml:"database"`
	Username string `mapstructure:"username" yaml:"username"`
	Password string `mapstructure:"password" yaml:"password,omitempty"`
	SSLMode  string `mapstructure:"sslmode" yaml:"sslmode"`
}

// Preferences holds user preferences.
type Preferences struct {
	Theme             string `mapstructure:"theme" yaml:"theme"`
	DefaultConnection string `mapstructure:"default_connection" yaml:"default_connection"`
}

// DSN builds a PostgreSQL connection string from the connection profile.
func (c Connection) DSN() string {
	dsn := "postgresql://"
	if c.Username != "" {
		dsn += c.Username
		if c.Password != "" {
			dsn += ":" + c.Password
		}
		dsn += "@"
	}
	dsn += c.Host
	if c.Port > 0 {
		dsn += ":" + strconv.Itoa(c.Port)
	}
	dsn += "/" + c.Database
	if c.SSLMode != "" {
		dsn += "?sslmode=" + c.SSLMode
	}
	return dsn
}

// DisplayString returns a human-readable summary of the connection.
func (c Connection) DisplayString() string {
	s := c.Host
	if c.Port > 0 {
		s += ":" + strconv.Itoa(c.Port)
	}
	s += "/" + c.Database
	if c.Username != "" {
		s = c.Username + "@" + s
	}
	return s
}

// ParseDSN parses a PostgreSQL connection string into a Connection.
func ParseDSN(dsn string) (Connection, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return Connection{}, fmt.Errorf("invalid DSN: %w", err)
	}

	conn := Connection{
		Driver:   "postgres",
		Host:     u.Hostname(),
		Database: trimPrefix(u.Path, "/"),
		SSLMode:  u.Query().Get("sslmode"),
	}

	if u.User != nil {
		conn.Username = u.User.Username()
		if p, ok := u.User.Password(); ok {
			conn.Password = p
		}
	}

	if portStr := u.Port(); portStr != "" {
		conn.Port, _ = strconv.Atoi(portStr)
	}
	if conn.Port == 0 {
		conn.Port = 5432
	}

	// Auto-generate a name
	conn.Name = fmt.Sprintf("postgres-%s-%d-%s", conn.Host, conn.Port, conn.Database)

	return conn, nil
}

// HasConnection checks if a connection with the given name already exists.
func (cfg *Config) HasConnection(name string) bool {
	for _, c := range cfg.Connections {
		if c.Name == name {
			return true
		}
	}
	return false
}

// AddConnection appends a connection if it doesn't already exist.
func (cfg *Config) AddConnection(conn Connection) {
	if !cfg.HasConnection(conn.Name) {
		cfg.Connections = append(cfg.Connections, conn)
	}
}

func trimPrefix(s, prefix string) string {
	if len(s) >= len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):]
	}
	return s
}
