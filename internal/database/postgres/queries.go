package postgres

// SQL queries for PostgreSQL metadata introspection.
const (
	queryListSchemas = `
		SELECT schema_name
		FROM information_schema.schemata
		WHERE schema_name NOT IN ('pg_catalog', 'information_schema', 'pg_toast')
		ORDER BY schema_name`

	queryListTables = `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = $1
		  AND table_type = 'BASE TABLE'
		ORDER BY table_name`

	queryGetColumns = `
		SELECT
			c.column_name,
			c.data_type,
			c.is_nullable,
			COALESCE(c.column_default, ''),
			c.ordinal_position,
			CASE WHEN pk.column_name IS NOT NULL THEN true ELSE false END AS is_primary
		FROM information_schema.columns c
		LEFT JOIN (
			SELECT ku.column_name
			FROM information_schema.table_constraints tc
			JOIN information_schema.key_column_usage ku
				ON tc.constraint_name = ku.constraint_name
				AND tc.table_schema = ku.table_schema
			WHERE tc.constraint_type = 'PRIMARY KEY'
				AND tc.table_schema = $1
				AND tc.table_name = $2
		) pk ON c.column_name = pk.column_name
		WHERE c.table_schema = $1
		  AND c.table_name = $2
		ORDER BY c.ordinal_position`

	queryTableRowCount = `
		SELECT COALESCE(reltuples, 0)::bigint
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relname = $1
		  AND n.nspname = $2`
)
