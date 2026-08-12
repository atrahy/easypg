package sql

import (
	"context"
	"errors"
	"log"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DBConnection wraps a single *pgx.Conn. A single connection cannot service
// concurrent queries (pgx reports "conn busy" / statement deallocation errors),
// and Bubble Tea runs the Cmds of a batch concurrently, so every query is
// serialized behind mu. A connection pool would lift this restriction later.
type DBConnection struct {
	mu   sync.Mutex
	conn *pgx.Conn
}

// Connect opens a connection to connectionString, applying password out of band
// rather than through the string itself: the DSN comes from a config file that
// must never hold a secret, and setting the field on the parsed config sidesteps
// the escaping question entirely (a "@" or a "/" in a password, URL-encoded or
// libpq-quoted) while keeping the secret out of anything that gets logged.
//
// An empty password means "set nothing", which is what the pgpass, env and none
// authentication modes need: pgx then reads ~/.pgpass or $PGPASSWORD itself.
func Connect(connectionString, password string) (*DBConnection, error) {
	config, err := pgx.ParseConfig(connectionString)
	if err != nil {
		return nil, err
	}

	if password != "" {
		config.Password = password
	}

	conn, err := pgx.ConnectConfig(context.Background(), config)
	if err != nil {
		return nil, err
	}

	return &DBConnection{conn: conn}, nil
}

// IsAuthError reports whether the server rejected the credentials, as opposed to
// being unreachable or refusing the database. The caller uses it to tell the one
// failure a stored password can cause from every other kind — without it, a wrong
// secret in the vault is a dead end: nothing would ever ask for it again.
//
// 28P01 is invalid_password, 28000 invalid_authorization_specification.
func IsAuthError(err error) bool {
	var pgErr *pgconn.PgError

	if !errors.As(err, &pgErr) {
		return false
	}

	return pgErr.Code == "28P01" || pgErr.Code == "28000"
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
