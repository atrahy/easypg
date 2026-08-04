package sql

import (
	"context"
	"fmt"
	"strings"
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
	WHERE c.relkind IN ('r','p','v','m')
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
	return makeQueryAndCollectRows[Table](c, tableQuery, schema)
}

type TableAttr struct {
	Columns     []ColumnAttr
	Indexes     []IndexAttr
	Constraints []ConstraintAttr
	// DDL is a ready-to-display SQL definition of the object: a reconstructed
	// CREATE TABLE (+ constraints + indexes) for tables, or a CREATE VIEW built
	// from pg_get_viewdef for views/materialized views.
	DDL string
}

const viewDefQuery = `SELECT pg_catalog.pg_get_viewdef($1::oid, true);`

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

func (c *DBConnection) QueryTableAttr(table Table) (*TableAttr, error) {
	columnsAttr, err := makeQueryAndCollectRows[ColumnAttr](c, tableColumnsAttrQuery, table.OID)
	if err != nil {
		return nil, err
	}

	indexes, err := c.QueryTableIndexes(table.OID)
	if err != nil {
		return nil, err
	}

	constraints, err := c.QueryTableConstraints(table.OID)
	if err != nil {
		return nil, err
	}

	ddl, err := c.buildDDL(table, columnsAttr, constraints, indexes)
	if err != nil {
		return nil, err
	}

	return &TableAttr{
		Columns:     columnsAttr,
		Indexes:     indexes,
		Constraints: constraints,
		DDL:         ddl,
	}, nil
}

// buildDDL returns the SQL definition of an object. Views delegate to
// pg_get_viewdef; everything else is reconstructed from its columns,
// constraints and indexes via buildTableDDL.
func (c *DBConnection) buildDDL(table Table, columns []ColumnAttr, constraints []ConstraintAttr, indexes []IndexAttr) (string, error) {
	if strings.Contains(table.Type, "view") {
		def, err := c.queryViewDef(table.OID)
		if err != nil {
			return "", err
		}

		verb := "CREATE OR REPLACE VIEW"
		if strings.Contains(table.Type, "materialized") {
			verb = "CREATE MATERIALIZED VIEW"
		}

		return fmt.Sprintf("%s %s AS\n%s", verb, qualifiedName(table), def), nil
	}

	return buildTableDDL(table, columns, constraints, indexes), nil
}

// quoteIdent double-quotes a SQL identifier, escaping embedded double quotes.
func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func qualifiedName(table Table) string {
	return quoteIdent(table.Schema) + "." + quoteIdent(table.Name)
}

func (c *DBConnection) queryViewDef(oid string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var def string

	err := c.conn.QueryRow(context.Background(), viewDefQuery, oid).Scan(&def)
	if err != nil {
		return "", err
	}

	return def, nil
}

// buildTableDDL reconstructs a CREATE TABLE statement followed by ALTER TABLE
// ADD CONSTRAINT and CREATE INDEX statements. Indexes that back a constraint
// (primary key or unique — their index shares the constraint's name) are
// skipped, since the constraint already emits them.
func buildTableDDL(table Table, columns []ColumnAttr, constraints []ConstraintAttr, indexes []IndexAttr) string {
	var b strings.Builder

	qualified := qualifiedName(table)

	constraintNames := make(map[string]bool, len(constraints))
	for _, con := range constraints {
		constraintNames[con.Name] = true
	}

	fmt.Fprintf(&b, "CREATE TABLE %s (\n", qualified)
	for i, col := range columns {
		fmt.Fprintf(&b, "    %s %s", quoteIdent(col.Name), col.Type)
		if col.DefaultExpr != nil {
			fmt.Fprintf(&b, " DEFAULT %s", *col.DefaultExpr)
		}
		if col.NotNullable {
			b.WriteString(" NOT NULL")
		}
		if i < len(columns)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString(");\n")

	for _, con := range constraints {
		fmt.Fprintf(&b, "\nALTER TABLE %s ADD CONSTRAINT %s %s;\n", qualified, quoteIdent(con.Name), con.Definition)
	}

	for _, idx := range indexes {
		if constraintNames[idx.Name] {
			continue
		}
		fmt.Fprintf(&b, "\n%s;\n", idx.Definition)
	}

	return b.String()
}
