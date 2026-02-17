package config

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
		dsn += ":" + itoa(c.Port)
	}
	dsn += "/" + c.Database
	if c.SSLMode != "" {
		dsn += "?sslmode=" + c.SSLMode
	}
	return dsn
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	s := ""
	for i > 0 {
		s = string(rune('0'+i%10)) + s
		i /= 10
	}
	return s
}
