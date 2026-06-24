// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"path/filepath"
	"testing"
	"time"

	"miniflux.app/v2/internal/database"
	"miniflux.app/v2/internal/model"
)

// newTestStore creates an isolated, schema-migrated SQLite database backed by a
// temporary file and returns a *Storage wired to it. The database is removed
// automatically when the test finishes.
func newTestStore(t *testing.T) *Storage {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "picoflux-test.db")
	db, err := database.NewConnectionPool(dbPath, 1, 1, time.Minute)
	if err != nil {
		t.Fatalf("unable to open test database: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := database.Migrate(db); err != nil {
		t.Fatalf("unable to migrate test database: %v", err)
	}

	return NewStorage(db)
}

// createTestUser inserts a user and returns it.
func createTestUser(t *testing.T, store *Storage, username string) *model.User {
	t.Helper()

	user, err := store.CreateUser(&model.UserCreationRequest{
		Username: username,
		Password: "test-password",
	})
	if err != nil {
		t.Fatalf("unable to create user %q: %v", username, err)
	}
	return user
}

// createTestFeed inserts a feed (and category) for the given user and returns it.
func createTestFeed(t *testing.T, store *Storage, user *model.User, feedURL string) *model.Feed {
	t.Helper()

	category, err := store.FirstCategory(user.ID)
	if err != nil {
		t.Fatalf("unable to fetch default category: %v", err)
	}

	feed := &model.Feed{
		UserID:   user.ID,
		Category: category,
		Title:    "Test Feed",
		FeedURL:  feedURL,
		SiteURL:  "https://example.org",
	}
	if err := store.CreateFeed(feed); err != nil {
		t.Fatalf("unable to create feed: %v", err)
	}
	return feed
}
