package database

import "time"

// Column represents a table column with its metadata.
type Column struct {
	Name       string
	DataType   string
	IsNullable bool
	IsPrimary  bool
	Default    string
	OrdinalPos int
}

// QueryResult holds the result of a SQL query execution.
type QueryResult struct {
	Columns  []string
	Rows     [][]string
	RowCount int
	Duration time.Duration
}
