// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package storage // import "miniflux.app/v2/internal/storage"

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Storage handles all operations related to the database.
type Storage struct {
	db *sql.DB
}

// NewStorage returns a new Storage.
func NewStorage(db *sql.DB) *Storage {
	return &Storage{db}
}

// DatabaseVersion returns the version of the database which is in use.
func (s *Storage) DatabaseVersion() string {
	var dbVersion string
	err := s.db.QueryRow(`SELECT sqlite_version()`).Scan(&dbVersion)
	if err != nil {
		return err.Error()
	}

	return "SQLite " + dbVersion
}

// quoteIdentifier safely quotes a SQL identifier for SQLite (double quotes,
// internal double quotes doubled). Replaces github.com/lib/pq's QuoteIdentifier.
func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// jsonIntArray serializes a slice of int64 identifiers into a JSON array so it
// can be expanded with SQLite's json_each(). Replaces PostgreSQL arrays passed
// through pq.Array for "= ANY(...)" style queries.
func jsonIntArray(values []int64) string {
	if len(values) == 0 {
		return "[]"
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return "[]"
	}
	return string(encoded)
}

// jsonStringArray serializes a slice of strings into a JSON array for use with
// SQLite's json_each().
func jsonStringArray(values []string) string {
	if len(values) == 0 {
		return "[]"
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return "[]"
	}
	return string(encoded)
}

// jsonStringSliceScanner scans a JSON-array TEXT column (e.g. entries.tags) into
// a Go []string. It replaces github.com/lib/pq's Array scanner.
type jsonStringSliceScanner struct {
	target *[]string
}

func jsonScanStringSlice(target *[]string) jsonStringSliceScanner {
	return jsonStringSliceScanner{target: target}
}

func (j jsonStringSliceScanner) Scan(src any) error {
	*j.target = nil
	if src == nil {
		return nil
	}

	var data []byte
	switch v := src.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return fmt.Errorf("storage: cannot scan %T into []string", src)
	}

	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, j.target)
}

// toFTS5Query converts free-form user search input into a safe FTS5 MATCH
// expression. Each whitespace-separated token is double-quoted (with internal
// quotes escaped) and joined with implicit AND, which mirrors the default
// behaviour of PostgreSQL's websearch_to_tsquery while avoiding FTS5 syntax
// errors from punctuation in the raw query.
func toFTS5Query(input string) string {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return `""`
	}
	quoted := make([]string, 0, len(fields))
	for _, f := range fields {
		quoted = append(quoted, `"`+strings.ReplaceAll(f, `"`, `""`)+`"`)
	}
	return strings.Join(quoted, " ")
}

// Ping checks if the database connection works.
func (s *Storage) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return s.db.PingContext(ctx)
}

// DBStats returns database statistics.
func (s *Storage) DBStats() sql.DBStats {
	return s.db.Stats()
}

// DBSize returns how much size the database is using in a pretty way.
func (s *Storage) DBSize() (string, error) {
	var pageCount, pageSize int64
	if err := s.db.QueryRow(`PRAGMA page_count`).Scan(&pageCount); err != nil {
		return "", err
	}
	if err := s.db.QueryRow(`PRAGMA page_size`).Scan(&pageSize); err != nil {
		return "", err
	}
	return humanizeBytes(pageCount * pageSize), nil
}

// humanizeBytes renders a byte count using binary units (mirrors the previous
// pg_size_pretty output style).
func humanizeBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d bytes", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
