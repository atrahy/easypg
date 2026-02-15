package sql

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5"
)

const tableQuery = `
	SELECT
		c.oid as "oid",
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
	OID    string  `db:"oid"`
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

type TableAttr struct {
	Columns          []ColumnAttr
	Indexes          []string
	CheckConstraints []string
}

type ColumnAttr struct {
	Name        string  `db:"name"`
	Type        string  `db:"type"`
	DefaultExpr *string `db:"get_expr"`
	NotNullable bool    `db:"att_not_null"`
}

const tableColumnsAttrQuery = `
	SELECT
		a.attname AS name,
		pg_catalog.format_type(a.atttypid, a.atttypmod) AS type,
		(
			SELECT pg_catalog.pg_get_expr(d.adbin, d.adrelid, true)
			FROM pg_catalog.pg_attrdef d
			WHERE d.adrelid = a.attrelid AND d.adnum = a.attnum AND a.atthasdef
		) AS get_expr,
		a.attnotnull AS "att_not_null"
	FROM pg_catalog.pg_attribute a
	WHERE
		a.attrelid = $1
		AND a.attnum > 0
		AND NOT a.attisdropped
	ORDER BY a.attnum;
`

func (c *DBConnection) QueryTableAttr(tableOID string) (*TableAttr, error) {
	columnsAttr, err := makeQueryAndCollectRows[ColumnAttr](c, tableColumnsAttrQuery, tableOID)
	if err != nil {
		return nil, err
	}

	return &TableAttr{
		Columns: columnsAttr,
	}, nil
}
