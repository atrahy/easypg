package sql

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5"
)

type DBConnection struct {
	conn *pgx.Conn
}

func Connect(connectionString string) (*DBConnection, error) {
	conn, err := pgx.Connect(
		context.Background(),
		connectionString,
	)
	if err != nil {
		return nil, err
	}

	return &DBConnection{conn: conn}, nil
}

func (c *DBConnection) Close() error {
	return c.conn.Close(context.Background())
}

func makeQueryAndCollectRows[T any](c *DBConnection, queryString string, params ...any) ([]T, error) {
	rows, err := c.conn.Query(context.Background(), queryString, params...)
	if err != nil {
		log.Printf("Query failed: %v\n", err)
		return nil, err
	}

	result, err := pgx.CollectRows(rows, pgx.RowToStructByName[T])
	if err != nil {
		log.Printf("Collect failed: %v\n", err)
		return nil, err
	}

	return result, nil
}
