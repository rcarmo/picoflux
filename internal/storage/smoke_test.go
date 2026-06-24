// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"testing"

	"miniflux.app/v2/internal/model"
)

func TestUserLifecycle(t *testing.T) {
	store := newTestStore(t)

	user := createTestUser(t, store, "Alice")
	if user.ID == 0 {
		t.Fatal("expected non-zero user ID")
	}
	if user.Username != "alice" {
		t.Errorf("username should be lowercased, got %q", user.Username)
	}

	fetched, err := store.UserByID(user.ID)
	if err != nil {
		t.Fatalf("UserByID: %v", err)
	}
	if fetched == nil || fetched.Username != "alice" {
		t.Fatalf("expected to fetch alice, got %+v", fetched)
	}

	if !store.UserExists("alice") {
		t.Error("UserExists should report true for alice")
	}
	if store.UserExists("bob") {
		t.Error("UserExists should report false for bob")
	}
}

func TestDefaultCategoryAndFeed(t *testing.T) {
	store := newTestStore(t)
	user := createTestUser(t, store, "alice")

	category, err := store.FirstCategory(user.ID)
	if err != nil {
		t.Fatalf("FirstCategory: %v", err)
	}
	if category.Title != "All" {
		t.Errorf("expected default category 'All', got %q", category.Title)
	}

	feed := createTestFeed(t, store, user, "https://example.org/feed.xml")
	if feed.ID == 0 {
		t.Fatal("expected non-zero feed ID")
	}

	got, err := store.FeedByID(user.ID, feed.ID)
	if err != nil {
		t.Fatalf("FeedByID: %v", err)
	}
	if got == nil || got.FeedURL != "https://example.org/feed.xml" {
		t.Fatalf("unexpected feed: %+v", got)
	}
}

func TestEntryCreateAndStatus(t *testing.T) {
	store := newTestStore(t)
	user := createTestUser(t, store, "alice")
	feed := createTestFeed(t, store, user, "https://example.org/feed.xml")

	entry := model.NewEntry()
	entry.Title = "Hello SQLite"
	entry.URL = "https://example.org/post-1"
	entry.Hash = "hash-1"
	entry.Content = "A first post about golang and sqlite."
	entry.Tags = []string{"golang", "databases"}

	created, err := store.InsertEntryForFeed(user.ID, feed.ID, entry)
	if err != nil {
		t.Fatalf("InsertEntryForFeed: %v", err)
	}
	if !created {
		t.Fatal("expected a new entry to be created")
	}
	if entry.ID == 0 {
		t.Fatal("expected non-zero entry ID")
	}

	// Read it back through the query builder and verify tags round-trip.
	fetched, err := store.NewEntryQueryBuilder(user.ID).WithEntryIDs(entry.ID).GetEntry()
	if err != nil {
		t.Fatalf("GetEntry: %v", err)
	}
	if fetched == nil {
		t.Fatal("expected to fetch the entry")
	}
	if len(fetched.Tags) != 2 || fetched.Tags[0] != "golang" {
		t.Errorf("tags did not round-trip, got %#v", fetched.Tags)
	}

	// Mark as read and verify.
	if err := store.SetEntriesStatus(user.ID, []int64{entry.ID}, model.EntryStatusRead); err != nil {
		t.Fatalf("SetEntriesStatus: %v", err)
	}
	reread, err := store.NewEntryQueryBuilder(user.ID).WithEntryIDs(entry.ID).GetEntry()
	if err != nil {
		t.Fatalf("GetEntry after status: %v", err)
	}
	if reread.Status != model.EntryStatusRead {
		t.Errorf("expected status read, got %q", reread.Status)
	}
}
