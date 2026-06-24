// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package database // import "miniflux.app/v2/internal/database"

import (
	"database/sql"
	"fmt"
	"log/slog"
)

// schemaVersionPragma reads the SQLite schema version stored in the database
// header via "PRAGMA user_version". This replaces the PostgreSQL
// "schema_version" table used by upstream miniflux.
func currentSchemaVersion(db *sql.DB) (int, error) {
	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return 0, err
	}
	return version, nil
}

// Migrate executes database migrations.
func Migrate(db *sql.DB) error {
	currentVersion, err := currentSchemaVersion(db)
	if err != nil {
		return fmt.Errorf("unable to read schema version: %w", err)
	}

	slog.Info("Running database migrations",
		slog.Int("current_version", currentVersion),
		slog.Int("latest_version", schemaVersion),
	)

	for version := currentVersion; version < schemaVersion; version++ {
		newVersion := version + 1

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("[Migration v%d] %v", newVersion, err)
		}

		if err := migrations[version](tx); err != nil {
			tx.Rollback()
			return fmt.Errorf("[Migration v%d] %v", newVersion, err)
		}

		// PRAGMA user_version does not support bound parameters, so the value
		// is interpolated. newVersion is an integer derived from a slice index,
		// so this is not an injection vector.
		if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, newVersion)); err != nil {
			tx.Rollback()
			return fmt.Errorf("[Migration v%d] %v", newVersion, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("[Migration v%d] %v", newVersion, err)
		}
	}

	return nil
}

// IsSchemaUpToDate checks if the database schema is up to date.
func IsSchemaUpToDate(db *sql.DB) error {
	currentVersion, err := currentSchemaVersion(db)
	if err != nil {
		return err
	}
	if currentVersion < schemaVersion {
		return fmt.Errorf(`the database schema is not up to date: current=v%d expected=v%d`, currentVersion, schemaVersion)
	}
	return nil
}
