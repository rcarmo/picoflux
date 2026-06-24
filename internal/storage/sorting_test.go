// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"testing"
	"time"

	"miniflux.app/v2/internal/model"
)

// TestGetEntriesSortingNoAmbiguousColumn guards against a regression where the
// unread view (and any view sorting by "id"/"title") produced
// "ambiguous column name: id" on SQLite, because the joined query
// (entries/feeds/categories/icons/users) has several "id" columns and the bare
// ORDER BY column did not resolve to the SELECT-list alias the way PostgreSQL
// did. Every valid entry sort column must execute against the real joined query.
func TestGetEntriesSortingNoAmbiguousColumn(t *testing.T) {
	store := newTestStore(t)
	user := createTestUser(t, store, "alice")
	feed := createTestFeed(t, store, user, "https://example.org/feed.xml")
	insertEntry(t, store, user, feed, "h1", "Title One", "body one", time.Now().UTC())
	insertEntry(t, store, user, feed, "h2", "Title Two", "body two", time.Now().UTC())

	// These are exactly the columns ValidateEntryOrder accepts, plus the
	// secondary "id" tiebreaker the UI appends after the primary order.
	orders := []string{
		"id", "status", "changed_at", "published_at", "created_at",
		"category_title", "category_id", "title", "author",
	}

	for _, order := range orders {
		for _, dir := range []string{"ASC", "DESC"} {
			// Mirror the UI: primary user order + "id" tiebreaker.
			entries, err := store.NewEntryQueryBuilder(user.ID).
				WithStatuses([]string{model.EntryStatusUnread}...).
				WithSorting(order, dir).
				WithSorting("id", dir).
				GetEntries()
			if err != nil {
				t.Fatalf("GetEntries sorted by %q %s failed: %v", order, dir, err)
			}
			if len(entries) != 2 {
				t.Errorf("expected 2 entries sorted by %q %s, got %d", order, dir, len(entries))
			}
		}
	}
}
