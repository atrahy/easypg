package sql

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5"
)

const (
	namespaceQuery = `
		SELECT
			n.nspname AS "name",
			pg_catalog.pg_get_userbyid(n.nspowner) AS "owner",
			n.nspacl as "access_privileges",
			pg_catalog.obj_description(n.oid, 'pg_namespace') AS "description"
		FROM
			pg_catalog.pg_namespace n
		WHERE
			n.nspname !~ '^pg_' AND n.nspname <> 'information_schema'
		ORDER BY 1;`
)

type Namespace struct {
	Name             string     `db:"name"`
	Owner            string     `db:"owner"`
	AccessPrivileges *[]*string `db:"access_privileges"`
	Description      *string    `db:"description"`
}

func (c *DBConnection) QueryNamespaces() ([]Namespace, error) {
	rows, err := c.conn.Query(context.Background(), namespaceQuery)
	if err != nil {
		log.Printf("Query failed: %v\n", err)
		return nil, err
	}

	result, err := pgx.CollectRows(rows, pgx.RowToStructByName[Namespace])
	if err != nil {
		log.Printf("Collect failed: %v\n", err)
		return nil, err
	}

	// Always get the public namespace first
	var resultOrdered []Namespace

	for _, value := range result {
		if value.Name == "public" {
			resultOrdered = append([]Namespace{value}, resultOrdered...)
		} else {
			resultOrdered = append(resultOrdered, value)
		}
	}

	return resultOrdered, nil
}
