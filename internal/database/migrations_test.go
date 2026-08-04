// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package database

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func openMigrationTestDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "migration.db")
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatalf("open SQLite database: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestMigrateFreshDatabaseIncludesLanguageColumns(t *testing.T) {
	db := openMigrationTestDB(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate fresh database: %v", err)
	}

	assertSchemaVersion(t, db, schemaVersion)
	assertColumn(t, db, "feeds", "language", "TEXT", true, "''")
	assertColumn(t, db, "entries", "language", "TEXT", true, "''")
}

func TestMigrateV3DatabaseAddsLanguageColumnsAndPreservesRows(t *testing.T) {
	db := openMigrationTestDB(t)

	// Recreate the exact pre-language schema by applying migrations v1-v3.
	for version := 1; version <= 3; version++ {
		tx, err := db.Begin()
		if err != nil {
			t.Fatalf("begin migration v%d: %v", version, err)
		}
		if err := migrations[version-1](tx); err != nil {
			tx.Rollback()
			t.Fatalf("apply migration v%d: %v", version, err)
		}
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", version)); err != nil {
			tx.Rollback()
			t.Fatalf("set user_version=%d: %v", version, err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit migration v%d: %v", version, err)
		}
	}

	if _, err := db.Exec(`INSERT INTO users (username) VALUES ('alice')`); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO categories (user_id, title) VALUES (1, 'All')`); err != nil {
		t.Fatalf("seed category: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO feeds (user_id, category_id, feed_url, site_url, title) VALUES (1, 1, 'https://example.org/feed', 'https://example.org/', 'Example')`); err != nil {
		t.Fatalf("seed feed: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO entries (user_id, feed_id, hash, title, url, content, published_at, changed_at) VALUES (1, 1, 'hash', 'Entry', 'https://example.org/entry', 'Body', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`); err != nil {
		t.Fatalf("seed entry: %v", err)
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("upgrade v3 database: %v", err)
	}

	assertSchemaVersion(t, db, schemaVersion)
	var feedCount, entryCount int
	if err := db.QueryRow(`SELECT count(*) FROM feeds`).Scan(&feedCount); err != nil {
		t.Fatalf("count feeds: %v", err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM entries`).Scan(&entryCount); err != nil {
		t.Fatalf("count entries: %v", err)
	}
	if feedCount != 1 || entryCount != 1 {
		t.Fatalf("row counts changed during migration: feeds=%d entries=%d", feedCount, entryCount)
	}

	var feedLanguage, entryLanguage string
	if err := db.QueryRow(`SELECT language FROM feeds WHERE id=1`).Scan(&feedLanguage); err != nil {
		t.Fatalf("read feed language: %v", err)
	}
	if err := db.QueryRow(`SELECT language FROM entries WHERE id=1`).Scan(&entryLanguage); err != nil {
		t.Fatalf("read entry language: %v", err)
	}
	if feedLanguage != "" || entryLanguage != "" {
		t.Fatalf("existing rows should default to empty language: feed=%q entry=%q", feedLanguage, entryLanguage)
	}
}

func assertSchemaVersion(t *testing.T, db *sql.DB, expected int) {
	t.Helper()
	var got int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&got); err != nil {
		t.Fatalf("read schema version: %v", err)
	}
	if got != expected {
		t.Fatalf("schema version: got %d, want %d", got, expected)
	}
}

func assertColumn(t *testing.T, db *sql.DB, table, column, wantType string, wantNotNull bool, wantDefault string) {
	t.Helper()
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		t.Fatalf("table_info(%s): %v", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notNull, pk int
		var name, typ string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			t.Fatalf("scan table_info(%s): %v", table, err)
		}
		if name == column {
			if typ != wantType || (notNull != 0) != wantNotNull || defaultValue.String != wantDefault {
				t.Fatalf("%s.%s: type=%q notnull=%d default=%q", table, column, typ, notNull, defaultValue.String)
			}
			return
		}
	}
	t.Fatalf("missing column %s.%s", table, column)
}
