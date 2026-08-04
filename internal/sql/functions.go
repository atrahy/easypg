package sql

const functionsQuery = `
	SELECT
		p.oid AS oid,
		n.nspname AS schema,
		p.proname AS name,
		pg_catalog.pg_get_function_arguments(p.oid) AS arguments
	FROM pg_catalog.pg_proc p
		JOIN pg_catalog.pg_namespace n ON n.oid = p.pronamespace
	WHERE n.nspname = $1
		AND p.prokind IN ('f', 'p')
	ORDER BY p.proname;
`

type Function struct {
	OID       string `db:"oid"`
	Schema    string `db:"schema"`
	Name      string `db:"name"`
	Arguments string `db:"arguments"`
}

func (c *DBConnection) QueryFunctionsForSchema(schema string) ([]Function, error) {
	return makeQueryAndCollectRows[Function](c, functionsQuery, schema)
}

const functionDefQuery = `SELECT pg_catalog.pg_get_functiondef($1) AS def;`

type functionDefRow struct {
	Def string `db:"def"`
}

func (c *DBConnection) QueryFunctionDef(oid string) (string, error) {
	rows, err := makeQueryAndCollectRows[functionDefRow](c, functionDefQuery, oid)
	if err != nil {
		return "", err
	}

	if len(rows) == 0 {
		return "", nil
	}

	return rows[0].Def, nil
}
