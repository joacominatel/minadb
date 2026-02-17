package database

import "context"

// Driver defines the interface for database operations.
// All implementations must be safe for concurrent use.
type Driver interface {
	// Connect establishes a connection to the database.
	Connect(ctx context.Context, dsn string) error

	// Close closes the database connection.
	Close() error

	// Ping checks if the connection is alive.
	Ping(ctx context.Context) error

	// ListSchemas returns all user schemas for the current database.
	ListSchemas(ctx context.Context) ([]string, error)

	// ListTables returns all table names in a schema.
	ListTables(ctx context.Context, schema string) ([]string, error)

	// GetColumns returns all columns for a table.
	GetColumns(ctx context.Context, schema, table string) ([]Column, error)

	// GetTableRowCount returns the approximate row count for a table.
	GetTableRowCount(ctx context.Context, schema, table string) (int64, error)

	// ExecuteQuery runs a SQL query and returns results.
	ExecuteQuery(ctx context.Context, query string) (*QueryResult, error)

	// DatabaseName returns the name of the connected database.
	DatabaseName() string
}
