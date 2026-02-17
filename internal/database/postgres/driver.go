package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joacominatel/minadb/internal/database"
)

// Driver implements the database.Driver interface for PostgreSQL.
type Driver struct {
	pool   *pgxpool.Pool
	dbName string
}

// New creates a new PostgreSQL driver.
func New() *Driver {
	return &Driver{}
}

// Connect establishes a connection pool to PostgreSQL.
func (d *Driver) Connect(ctx context.Context, dsn string) error {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return fmt.Errorf("parse dsn: %w", err)
	}

	cfg.MaxConns = 5
	cfg.MinConns = 1

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return fmt.Errorf("ping: %w", err)
	}

	d.pool = pool
	d.dbName = cfg.ConnConfig.Database
	return nil
}

// Close closes the connection pool.
func (d *Driver) Close() error {
	if d.pool != nil {
		d.pool.Close()
	}
	return nil
}

// Ping checks if the connection is alive.
func (d *Driver) Ping(ctx context.Context) error {
	if d.pool == nil {
		return fmt.Errorf("not connected")
	}
	return d.pool.Ping(ctx)
}

// ListSchemas returns all user-created schemas.
func (d *Driver) ListSchemas(ctx context.Context) ([]string, error) {
	rows, err := d.pool.Query(ctx, queryListSchemas)
	if err != nil {
		return nil, fmt.Errorf("list schemas: %w", err)
	}
	defer rows.Close()

	var schemas []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan schema: %w", err)
		}
		schemas = append(schemas, name)
	}
	return schemas, rows.Err()
}

// ListTables returns all table names in a schema.
func (d *Driver) ListTables(ctx context.Context, schema string) ([]string, error) {
	rows, err := d.pool.Query(ctx, queryListTables, schema)
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan table: %w", err)
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}

// GetColumns returns column metadata for a table.
func (d *Driver) GetColumns(ctx context.Context, schema, table string) ([]database.Column, error) {
	rows, err := d.pool.Query(ctx, queryGetColumns, schema, table)
	if err != nil {
		return nil, fmt.Errorf("get columns: %w", err)
	}
	defer rows.Close()

	var columns []database.Column
	for rows.Next() {
		var col database.Column
		var nullable string
		if err := rows.Scan(&col.Name, &col.DataType, &nullable, &col.Default, &col.OrdinalPos, &col.IsPrimary); err != nil {
			return nil, fmt.Errorf("scan column: %w", err)
		}
		col.IsNullable = nullable == "YES"
		columns = append(columns, col)
	}
	return columns, rows.Err()
}

// GetTableRowCount returns the approximate row count using pg_class statistics.
func (d *Driver) GetTableRowCount(ctx context.Context, schema, table string) (int64, error) {
	var count int64
	err := d.pool.QueryRow(ctx, queryTableRowCount, table, schema).Scan(&count)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, nil
		}
		return 0, fmt.Errorf("row count: %w", err)
	}
	if count < 0 {
		count = 0
	}
	return count, nil
}

// ExecuteQuery runs a SQL query and returns the results.
func (d *Driver) ExecuteQuery(ctx context.Context, query string) (*database.QueryResult, error) {
	start := time.Now()

	rows, err := d.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("execute: %w", err)
	}
	defer rows.Close()

	// Get column names
	fields := rows.FieldDescriptions()
	columns := make([]string, len(fields))
	for i, f := range fields {
		columns[i] = f.Name
	}

	// Collect rows
	var resultRows [][]string
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, fmt.Errorf("read row: %w", err)
		}
		row := make([]string, len(values))
		for i, v := range values {
			row[i] = formatCell(v)
		}
		resultRows = append(resultRows, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}

	return &database.QueryResult{
		Columns:  columns,
		Rows:     resultRows,
		RowCount: len(resultRows),
		Duration: time.Since(start),
	}, nil
}

// DatabaseName returns the name of the connected database.
func (d *Driver) DatabaseName() string {
	return d.dbName
}

func formatCell(v any) string {
	if v == nil {
		return "null"
	}

	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return formatBytes(val)
	case time.Time:
		return val.Format("2006-01-02 15:04:05")
	case pgtype.Timestamp:
		return formatTimestamp(val.Time, val.Valid, val.InfinityModifier)
	case *pgtype.Timestamp:
		if val == nil {
			return "null"
		}
		return formatTimestamp(val.Time, val.Valid, val.InfinityModifier)
	case pgtype.Timestamptz:
		return formatTimestamp(val.Time, val.Valid, val.InfinityModifier)
	case *pgtype.Timestamptz:
		if val == nil {
			return "null"
		}
		return formatTimestamp(val.Time, val.Valid, val.InfinityModifier)
	case pgtype.Numeric:
		return formatNumeric(val)
	case *pgtype.Numeric:
		if val == nil {
			return "null"
		}
		return formatNumeric(*val)
	case map[string]any, []any:
		if b, err := json.Marshal(val); err == nil {
			return string(b)
		}
	}

	if b, err := json.Marshal(v); err == nil {
		if json.Valid(b) && (strings.HasPrefix(string(b), "[") || strings.HasPrefix(string(b), "{")) {
			return string(b)
		}
	}

	return fmt.Sprintf("%v", v)
}

func formatBytes(b []byte) string {
	if len(b) == 0 {
		return ""
	}

	if len(b) == 16 {
		if u, err := uuid.FromBytes(b); err == nil {
			return u.String()
		}
	}

	if json.Valid(b) {
		var compact bytes.Buffer
		if err := json.Compact(&compact, b); err == nil {
			return compact.String()
		}
	}

	if utf8.Valid(b) {
		return string(b)
	}

	return fmt.Sprintf("0x%x", b)
}

func formatTimestamp(t time.Time, valid bool, inf pgtype.InfinityModifier) string {
	if !valid {
		return "null"
	}
	if inf == pgtype.Infinity {
		return "infinity"
	}
	if inf == pgtype.NegativeInfinity {
		return "-infinity"
	}
	return t.Format("2006-01-02 15:04:05")
}

func formatNumeric(n pgtype.Numeric) string {
	if !n.Valid {
		return "null"
	}
	if n.NaN {
		return "NaN"
	}
	if n.InfinityModifier == pgtype.Infinity {
		return "infinity"
	}
	if n.InfinityModifier == pgtype.NegativeInfinity {
		return "-infinity"
	}

	f8, err := n.Float64Value()
	if err == nil && f8.Valid {
		return fmt.Sprintf("%.2f", f8.Float64)
	}

	b, err := n.MarshalJSON()
	if err == nil {
		s := strings.Trim(string(b), "\"")
		if s != "" && s != "null" {
			return s
		}
	}

	return "0.00"
}
