// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"errors"
	"testing"
	"time"

	"miniflux.app/v2/internal/model"
)

func TestArchiveEntriesAndTombstones(t *testing.T) {
	store := newTestStore(t)
	user := createTestUser(t, store, "alice")
	feed := createTestFeed(t, store, user, "https://example.org/feed.xml")

	now := time.Now().UTC()
	old := insertEntry(t, store, user, feed, "old", "Old", "old body", now.Add(-30*24*time.Hour))
	insertEntry(t, store, user, feed, "fresh", "Fresh", "fresh body", now)

	// Mark the old entry read so it is archivable.
	if err := store.SetEntriesStatus(user.ID, []int64{old.ID}, model.EntryStatusRead); err != nil {
		t.Fatalf("SetEntriesStatus: %v", err)
	}

	// created_at is set to "now" at insert time, so archive by a tiny interval to
	// catch the read entry. ArchiveEntries filters on created_at, so force it back.
	if _, err := store.db.Exec(`UPDATE entries SET created_at = datetime('now','-30 days') WHERE id=$1`, old.ID); err != nil {
		t.Fatalf("backdate: %v", err)
	}

	count, err := store.ArchiveEntries(model.EntryStatusRead, 15*24*time.Hour, 100)
	if err != nil {
		t.Fatalf("ArchiveEntries: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 archived entry, got %d", count)
	}

	// A tombstone must now exist, preventing re-ingestion of the same hash.
	if store.IsNewEntry(feed.ID, "old") {
		t.Error("tombstoned hash should not be considered a new entry")
	}

	reinsert := model.NewEntry()
	reinsert.Title = "Old"
	reinsert.Content = "old body"
	reinsert.Hash = "old"
	reinsert.URL = "https://example.org/old"
	reinsert.Date = now
	_, err = store.InsertEntryForFeed(user.ID, feed.ID, reinsert)
	if !errors.Is(err, ErrEntryTombstoned) {
		t.Errorf("expected ErrEntryTombstoned re-inserting a tombstoned hash, got %v", err)
	}
}

func TestFlushHistoryKeepsStarred(t *testing.T) {
	store := newTestStore(t)
	user := createTestUser(t, store, "alice")
	feed := createTestFeed(t, store, user, "https://example.org/feed.xml")

	now := time.Now().UTC()
	read := insertEntry(t, store, user, feed, "read", "Read", "body", now)
	starred := insertEntry(t, store, user, feed, "starred", "Starred", "body", now)

	if err := store.SetEntriesStatus(user.ID, []int64{read.ID, starred.ID}, model.EntryStatusRead); err != nil {
		t.Fatalf("SetEntriesStatus: %v", err)
	}
	if err := store.SetEntriesStarredState(user.ID, []int64{starred.ID}, true); err != nil {
		t.Fatalf("SetEntriesStarredState: %v", err)
	}

	if err := store.FlushHistory(user.ID); err != nil {
		t.Fatalf("FlushHistory: %v", err)
	}

	// The read, non-starred entry is gone; the starred one remains.
	if got := store.entryExistsByID(t, read.ID); got {
		t.Error("read entry should have been flushed")
	}
	if got := store.entryExistsByID(t, starred.ID); !got {
		t.Error("starred entry should be kept")
	}
}

// entryExistsByID is a small test-only helper.
func (s *Storage) entryExistsByID(t *testing.T, id int64) bool {
	t.Helper()
	var exists bool
	if err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM entries WHERE id=$1)`, id).Scan(&exists); err != nil {
		t.Fatalf("entryExistsByID: %v", err)
	}
	return exists
}

func TestEnclosureDedupAndSync(t *testing.T) {
	store := newTestStore(t)
	user := createTestUser(t, store, "alice")
	feed := createTestFeed(t, store, user, "https://example.org/feed.xml")

	entry := model.NewEntry()
	entry.Title = "With enclosures"
	entry.Hash = "enc"
	entry.URL = "https://example.org/enc"
	entry.Date = time.Now().UTC()
	entry.Enclosures = model.EnclosureList{
		{URL: "https://cdn.example.org/a.mp3", MimeType: "audio/mpeg"},
		{URL: "https://cdn.example.org/b.mp3", MimeType: "audio/mpeg"},
		// Duplicate URL should be deduped by the unique index.
		{URL: "https://cdn.example.org/a.mp3", MimeType: "audio/mpeg"},
	}

	if _, err := store.InsertEntryForFeed(user.ID, feed.ID, entry); err != nil {
		t.Fatalf("InsertEntryForFeed: %v", err)
	}

	got, err := store.EnclosuresByEntryID(entry.ID)
	if err != nil {
		t.Fatalf("EnclosuresByEntryID: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 deduped enclosures, got %d", len(got))
	}

	byIDs, err := store.EnclosuresByEntryIDs([]int64{entry.ID})
	if err != nil {
		t.Fatalf("EnclosuresByEntryIDs: %v", err)
	}
	if len(byIDs[entry.ID]) != 2 {
		t.Errorf("expected 2 enclosures via IDs query, got %d", len(byIDs[entry.ID]))
	}
}

func TestWeeklyFeedEntryCount(t *testing.T) {
	store := newTestStore(t)
	user := createTestUser(t, store, "alice")
	feed := createTestFeed(t, store, user, "https://example.org/feed.xml")

	now := time.Now().UTC()
	// Three entries spaced ~1 day apart over the last week.
	insertEntry(t, store, user, feed, "w1", "W1", "b", now.Add(-72*time.Hour))
	insertEntry(t, store, user, feed, "w2", "W2", "b", now.Add(-48*time.Hour))
	insertEntry(t, store, user, feed, "w3", "W3", "b", now.Add(-24*time.Hour))

	count, err := store.WeeklyFeedEntryCount(user.ID, feed.ID)
	if err != nil {
		t.Fatalf("WeeklyFeedEntryCount: %v", err)
	}
	// ~1 entry/day → roughly 7 per week. Allow a tolerant range.
	if count < 3 || count > 12 {
		t.Errorf("weekly count out of expected range: %d", count)
	}
}

func TestEntryPaginationPrevNext(t *testing.T) {
	store := newTestStore(t)
	user := createTestUser(t, store, "alice")
	feed := createTestFeed(t, store, user, "https://example.org/feed.xml")

	now := time.Now().UTC()
	e1 := insertEntry(t, store, user, feed, "p1", "First", "b", now.Add(-3*time.Hour))
	e2 := insertEntry(t, store, user, feed, "p2", "Second", "b", now.Add(-2*time.Hour))
	e3 := insertEntry(t, store, user, feed, "p3", "Third", "b", now.Add(-1*time.Hour))

	prev, next, err := store.NewEntryPaginationBuilder(user.ID, e2.ID, "published_at", "asc").Entries()
	if err != nil {
		t.Fatalf("pagination: %v", err)
	}
	if prev == nil || prev.ID != e1.ID {
		t.Errorf("expected prev=e1(%d), got %+v", e1.ID, prev)
	}
	if next == nil || next.ID != e3.ID {
		t.Errorf("expected next=e3(%d), got %+v", e3.ID, next)
	}
}

func TestCategoryReplaceByName(t *testing.T) {
	store := newTestStore(t)
	user := createTestUser(t, store, "alice")

	// Create two extra categories beyond the default "All".
	tech, err := store.CreateCategory(user.ID, &model.CategoryCreationRequest{Title: "Tech"})
	if err != nil {
		t.Fatalf("create Tech: %v", err)
	}
	if _, err := store.CreateCategory(user.ID, &model.CategoryCreationRequest{Title: "News"}); err != nil {
		t.Fatalf("create News: %v", err)
	}

	// Put a feed under Tech so the move-to-remaining-category path is exercised.
	feed := &model.Feed{
		UserID:   user.ID,
		Category: tech,
		Title:    "Test",
		FeedURL:  "https://example.org/tech.xml",
		SiteURL:  "https://example.org",
	}
	if err := store.CreateFeed(feed); err != nil {
		t.Fatalf("CreateFeed: %v", err)
	}

	// Delete Tech and News, leaving "All".
	if err := store.RemoveAndReplaceCategoriesByName(user.ID, []string{"Tech", "News"}); err != nil {
		t.Fatalf("RemoveAndReplaceCategoriesByName: %v", err)
	}

	cats, err := store.Categories(user.ID)
	if err != nil {
		t.Fatalf("Categories: %v", err)
	}
	if len(cats) != 1 || cats[0].Title != "All" {
		t.Fatalf("expected only 'All' to remain, got %+v", cats)
	}

	// The feed must have been reassigned to the remaining category.
	movedFeed, err := store.FeedByID(user.ID, feed.ID)
	if err != nil {
		t.Fatalf("FeedByID: %v", err)
	}
	if movedFeed.Category.Title != "All" {
		t.Errorf("feed should be reassigned to 'All', got %q", movedFeed.Category.Title)
	}
}
