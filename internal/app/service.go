package app

import (
	"context"

	"github.com/joacominatel/minadb/internal/database"
)

// SchemaTree represents the loaded schema hierarchy for the explorer.
type SchemaTree struct {
	Database string
	Schemas  []SchemaNode
}

// SchemaNode holds a schema name and its tables.
type SchemaNode struct {
	Name   string
	Tables []string
}

// Service coordinates application-level operations between the TUI and database.
type Service struct {
	driver database.Driver
	dsn    string
}

// NewService creates a new application service.
func NewService(driver database.Driver) *Service {
	return &Service{driver: driver}
}

// Connect establishes a database connection.
func (s *Service) Connect(ctx context.Context, dsn string) error {
	if err := s.driver.Connect(ctx, dsn); err != nil {
		return &ErrConnection{Cause: err}
	}
	s.dsn = dsn
	return nil
}

// Disconnect closes the database connection.
func (s *Service) Disconnect() error {
	return s.driver.Close()
}

// LoadSchemaTree fetches schemas and their tables for the connected database.
func (s *Service) LoadSchemaTree(ctx context.Context) (*SchemaTree, error) {
	schemas, err := s.driver.ListSchemas(ctx)
	if err != nil {
		return nil, err
	}

	tree := &SchemaTree{
		Database: s.driver.DatabaseName(),
	}

	for _, schema := range schemas {
		tables, err := s.driver.ListTables(ctx, schema)
		if err != nil {
			return nil, err
		}
		tree.Schemas = append(tree.Schemas, SchemaNode{
			Name:   schema,
			Tables: tables,
		})
	}

	return tree, nil
}

// LoadColumns fetches column metadata for a specific table.
func (s *Service) LoadColumns(ctx context.Context, schema, table string) ([]database.Column, error) {
	return s.driver.GetColumns(ctx, schema, table)
}

// GetTableRowCount returns the approximate row count for a table.
func (s *Service) GetTableRowCount(ctx context.Context, schema, table string) (int64, error) {
	return s.driver.GetTableRowCount(ctx, schema, table)
}

// ExecuteQuery runs a SQL query and returns the results.
func (s *Service) ExecuteQuery(ctx context.Context, query string) (*database.QueryResult, error) {
	result, err := s.driver.ExecuteQuery(ctx, query)
	if err != nil {
		return nil, &ErrQuery{Query: query, Cause: err}
	}
	return result, nil
}

// DatabaseName returns the current database name.
func (s *Service) DatabaseName() string {
	return s.driver.DatabaseName()
}
