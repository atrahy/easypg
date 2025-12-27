package sql

import (
	"context"

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
