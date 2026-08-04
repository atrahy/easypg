package sql

import (
	"context"
	"log"
	"sync"

	"github.com/jackc/pgx/v5"
)

// DBConnection wraps a single *pgx.Conn. A single connection cannot service
// concurrent queries (pgx reports "conn busy" / statement deallocation errors),
// and Bubble Tea runs the Cmds of a batch concurrently, so every query is
// serialized behind mu. A connection pool would lift this restriction later.
type DBConnection struct {
	mu   sync.Mutex
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
	// Hold the lock across Query + CollectRows: the rows must be fully read
	// (and thus the conn freed) before another query may run.
	c.mu.Lock()
	defer c.mu.Unlock()

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
