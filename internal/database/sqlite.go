// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package database // import "miniflux.app/v2/internal/database"

import (
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// connectionPragmas are applied to every new connection in the pool.
//
// SQLite is configured for a single-writer, server-style workload:
//   - WAL journal mode is enabled by default for better read concurrency.
//   - busy_timeout avoids spurious "database is locked" errors by waiting.
//   - foreign_keys enforces referential integrity (off by default in SQLite).
//   - synchronous=NORMAL is the recommended durability/performance trade-off
//     when running in WAL mode.
//
// They are passed as modernc.org/sqlite "_pragma" DSN parameters so that they
// apply to every physical connection the pool opens, including ones created
// after a connection is recycled. (Applying them once via db.Exec would only
// configure whichever connection happened to run the statement.)
var connectionPragmas = []string{
	"journal_mode(WAL)",
	"busy_timeout(10000)",
	"foreign_keys(ON)",
	"synchronous(NORMAL)",
}

// normalizeDSN turns a user-provided DATABASE_URL into a modernc.org/sqlite
// compatible DSN. It accepts a bare filesystem path (the common case) as well
// as the legacy "sqlite://"/"file:" prefixes, and appends the connection
// pragmas as query parameters.
func normalizeDSN(dsn string) string {
	dsn = strings.TrimSpace(dsn)
	dsn = strings.TrimPrefix(dsn, "sqlite://")
	dsn = strings.TrimPrefix(dsn, "sqlite3://")
	if dsn == "" {
		dsn = "nanoflux.db"
	}

	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}

	// Store time.Time values in a format SQLite's date/time functions (julianday,
	// datetime, strftime) can parse. Without this, modernc defaults to
	// time.Time.String() ("... +0000 UTC"), which SQLite cannot interpret and which
	// would break BM25 recency ranking, archival cutoffs, and weekly counters.
	dsn += separator + "_time_format=sqlite"
	separator = "&"

	for _, pragma := range connectionPragmas {
		dsn += separator + "_pragma=" + url.QueryEscape(pragma)
		separator = "&"
	}
	return dsn
}

// NewConnectionPool configures the SQLite database connection pool.
//
// SQLite only supports a single concurrent writer. We therefore cap the pool
// at a single open connection so the application naturally serializes writes
// and never trips over "database is locked". The minConnections argument is
// kept for API compatibility with the previous PostgreSQL implementation but
// no longer meaningfully applies.
func NewConnectionPool(dsn string, minConnections, maxConnections int, connectionLifetime time.Duration) (*sql.DB, error) {
	db, err := sql.Open("sqlite", normalizeDSN(dsn))
	if err != nil {
		return nil, err
	}

	// Single writer: serialize all access through one connection.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(connectionLifetime)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("database: unable to open SQLite database: %w", err)
	}

	return db, nil
}
