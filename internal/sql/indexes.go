package sql

const indexQuery = `
	SELECT
		ic.relname AS name,
		pg_catalog.pg_get_indexdef(i.indexrelid) AS definition,
		i.indisprimary AS is_primary,
		i.indisunique AS is_unique
	FROM pg_catalog.pg_index i
		JOIN pg_catalog.pg_class ic ON ic.oid = i.indexrelid
	WHERE i.indrelid = $1
	ORDER BY ic.relname;
`

type IndexAttr struct {
	Name       string `db:"name"`
	Definition string `db:"definition"`
	IsPrimary  bool   `db:"is_primary"`
	IsUnique   bool   `db:"is_unique"`
}

func (c *DBConnection) QueryTableIndexes(tableOID string) ([]IndexAttr, error) {
	return makeQueryAndCollectRows[IndexAttr](c, indexQuery, tableOID)
}
