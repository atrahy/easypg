package sql

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5"
)

const tableQuery = `
	SELECT
		n.nspname as "schema",
		c.relname as "name",
		CASE c.relkind WHEN 'r' THEN 'table' WHEN 'v' THEN 'view' WHEN 'm' THEN 'materialized view' WHEN 'i' THEN 'index' WHEN 'S' THEN 'sequence' WHEN 't' THEN 'TOAST table' WHEN 'f' THEN 'foreign table' WHEN 'p' THEN 'partitioned table' WHEN 'I' THEN 'partitioned index' END as "type",
		pg_catalog.pg_get_userbyid(c.relowner) as "owner"
	FROM pg_catalog.pg_class c
		LEFT JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		LEFT JOIN pg_catalog.pg_am am ON am.oid = c.relam
	WHERE c.relkind IN ('r','p','v','m','S','f','')
		AND n.nspname = $1
	ORDER BY 1,2;`

type Table struct {
	Schema string  `db:"schema"`
	Name   string  `db:"name"`
	Type   string  `db:"type"`
	Owner  *string `db:"owner"`
}

func (c *DBConnection) QueryTablesForSchema(schema string) ([]Table, error) {
	rows, err := c.conn.Query(context.Background(), tableQuery, schema)
	if err != nil {
		log.Printf("Query failed: %v\n", err)
		return nil, err
	}

	result, err := pgx.CollectRows(rows, pgx.RowToStructByName[Table])
	if err != nil {
		log.Printf("Collect failed: %v\n", err)
		return nil, err
	}

	return result, nil
}
