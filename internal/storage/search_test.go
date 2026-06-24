// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"testing"
	"time"

	"miniflux.app/v2/internal/model"
)

// insertEntry is a test helper that inserts an entry with explicit title,
// content and published date.
func insertEntry(t *testing.T, store *Storage, user *model.User, feed *model.Feed, hash, title, content string, published time.Time) *model.Entry {
	t.Helper()

	entry := model.NewEntry()
	entry.Title = title
	entry.Content = content
	entry.Hash = hash
	entry.URL = "https://example.org/" + hash
	entry.Date = published

	if _, err := store.InsertEntryForFeed(user.ID, feed.ID, entry); err != nil {
		t.Fatalf("insert entry %q: %v", hash, err)
	}
	return entry
}

func TestFullTextSearchMatching(t *testing.T) {
	store := newTestStore(t)
	user := createTestUser(t, store, "alice")
	feed := createTestFeed(t, store, user, "https://example.org/feed.xml")

	now := time.Now().UTC()
	insertEntry(t, store, user, feed, "h1", "Introduction to Golang", "The Go programming language is great for backends.", now)
	insertEntry(t, store, user, feed, "h2", "Python tutorial", "Learn data science with Python and pandas.", now)
	insertEntry(t, store, user, feed, "h3", "Rust ownership", "Memory safety without a garbage collector.", now)

	results, err := store.NewEntryQueryBuilder(user.ID).
		WithSearchQuery("golang").
		GetEntries()
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for 'golang', got %d", len(results))
	}
	if results[0].Hash != "h1" {
		t.Errorf("expected entry h1, got %q", results[0].Hash)
	}

	// A term in the content should also match.
	results, err = store.NewEntryQueryBuilder(user.ID).
		WithSearchQuery("pandas").
		GetEntries()
	if err != nil {
		t.Fatalf("search pandas: %v", err)
	}
	if len(results) != 1 || results[0].Hash != "h2" {
		t.Fatalf("expected h2 for 'pandas', got %+v", results)
	}

	// No matches.
	results, err = store.NewEntryQueryBuilder(user.ID).
		WithSearchQuery("kubernetes").
		GetEntries()
	if err != nil {
		t.Fatalf("search kubernetes: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for 'kubernetes', got %d", len(results))
	}
}

func TestFullTextSearchTitleWeightAndRecency(t *testing.T) {
	store := newTestStore(t)
	user := createTestUser(t, store, "alice")
	feed := createTestFeed(t, store, user, "https://example.org/feed.xml")

	now := time.Now().UTC()

	// Both match "sqlite" but the title match should rank above the body-only match.
	insertEntry(t, store, user, feed, "title", "SQLite is fast", "A short note.", now.Add(-48*time.Hour))
	insertEntry(t, store, user, feed, "body", "Database notes", "We migrated to sqlite recently.", now.Add(-48*time.Hour))

	results, err := store.NewEntryQueryBuilder(user.ID).
		WithSearchQuery("sqlite").
		GetEntries()
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Hash != "title" {
		t.Errorf("title match should rank first (BM25 title weight), got order %q,%q", results[0].Hash, results[1].Hash)
	}

	// Recency boost: two equally-weighted title matches, newer ranks first.
	store2 := newTestStore(t)
	u := createTestUser(t, store2, "bob")
	f := createTestFeed(t, store2, u, "https://example.org/feed.xml")
	insertEntry(t, store2, u, f, "old", "kubernetes guide", "old content", now.Add(-72*time.Hour))
	insertEntry(t, store2, u, f, "new", "kubernetes guide", "new content", now)

	results, err = store2.NewEntryQueryBuilder(u.ID).
		WithSearchQuery("kubernetes").
		GetEntries()
	if err != nil {
		t.Fatalf("search recency: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Hash != "new" {
		t.Errorf("newer entry should rank first via recency boost, got %q first", results[0].Hash)
	}
}

func TestFullTextSearchFTSTriggersOnUpdateAndDelete(t *testing.T) {
	store := newTestStore(t)
	user := createTestUser(t, store, "alice")
	feed := createTestFeed(t, store, user, "https://example.org/feed.xml")

	now := time.Now().UTC()
	entry := insertEntry(t, store, user, feed, "h1", "Original Title", "elephant content", now)

	// Update title/content; the FTS update trigger must re-index.
	entry.Title = "Replaced Title"
	entry.Content = "giraffe content"
	if err := store.UpdateEntryTitleAndContent(entry); err != nil {
		t.Fatalf("UpdateEntryTitleAndContent: %v", err)
	}

	if n := countSearch(t, store, user.ID, "elephant"); n != 0 {
		t.Errorf("old content 'elephant' should no longer match, got %d", n)
	}
	if n := countSearch(t, store, user.ID, "giraffe"); n != 1 {
		t.Errorf("new content 'giraffe' should match, got %d", n)
	}

	// Deleting the entry (via FlushHistory) should remove it from the FTS index too.
	if err := store.SetEntriesStatus(user.ID, []int64{entry.ID}, model.EntryStatusRead); err != nil {
		t.Fatalf("SetEntriesStatus: %v", err)
	}
	if err := store.FlushHistory(user.ID); err != nil {
		t.Fatalf("FlushHistory: %v", err)
	}
	if n := countSearch(t, store, user.ID, "giraffe"); n != 0 {
		t.Errorf("deleted entry should not match, got %d", n)
	}
}

func countSearch(t *testing.T, store *Storage, userID int64, query string) int {
	t.Helper()
	results, err := store.NewEntryQueryBuilder(userID).WithSearchQuery(query).GetEntries()
	if err != nil {
		t.Fatalf("search %q: %v", query, err)
	}
	return len(results)
}
