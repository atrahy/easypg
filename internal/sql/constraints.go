package sql

const constraintQuery = `
	SELECT
		con.conname AS name,
		CASE con.contype
			WHEN 'c' THEN 'check'
			WHEN 'f' THEN 'foreign key'
			WHEN 'p' THEN 'primary key'
			WHEN 'u' THEN 'unique'
			WHEN 't' THEN 'trigger'
			WHEN 'x' THEN 'exclusion'
			ELSE con.contype::text
		END AS type,
		pg_catalog.pg_get_constraintdef(con.oid) AS definition
	FROM pg_catalog.pg_constraint con
	WHERE con.conrelid = $1
	ORDER BY con.conname;
`

type ConstraintAttr struct {
	Name       string `db:"name"`
	Type       string `db:"type"`
	Definition string `db:"definition"`
}

func (c *DBConnection) QueryTableConstraints(tableOID string) ([]ConstraintAttr, error) {
	return makeQueryAndCollectRows[ConstraintAttr](c, constraintQuery, tableOID)
}
