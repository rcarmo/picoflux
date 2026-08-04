// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"testing"
	"time"

	"miniflux.app/v2/internal/model"
)

func TestFeedAndEntryLanguageRoundTripSQLite(t *testing.T) {
	store := newTestStore(t)
	user := createTestUser(t, store, "language-roundtrip")
	category, err := store.FirstCategory(user.ID)
	if err != nil {
		t.Fatalf("fetch default category: %v", err)
	}

	feed := &model.Feed{
		UserID:   user.ID,
		Category: category,
		Title:    "Portuguese Feed",
		FeedURL:  "https://example.org/pt.xml",
		SiteURL:  "https://example.org/pt/",
		Language: "pt-pt",
	}
	if err := store.CreateFeed(feed); err != nil {
		t.Fatalf("create feed: %v", err)
	}

	gotFeed, err := store.FeedByID(user.ID, feed.ID)
	if err != nil {
		t.Fatalf("get feed: %v", err)
	}
	if gotFeed.Language != "pt-pt" {
		t.Fatalf("feed language round-trip: got %q, want %q", gotFeed.Language, "pt-pt")
	}

	entry := model.NewEntry()
	entry.Title = "Olá mundo"
	entry.Content = "Conteúdo em português"
	entry.Hash = "pt-entry"
	entry.URL = "https://example.org/pt/entry"
	entry.Date = time.Now().UTC()
	entry.Language = "pt-br"
	if _, err := store.InsertEntryForFeed(user.ID, feed.ID, entry); err != nil {
		t.Fatalf("insert entry: %v", err)
	}

	gotEntry, err := store.NewEntryQueryBuilder(user.ID).WithEntryIDs(entry.ID).GetEntry()
	if err != nil {
		t.Fatalf("get entry: %v", err)
	}
	if gotEntry.Language != "pt-br" {
		t.Fatalf("entry language round-trip: got %q, want %q", gotEntry.Language, "pt-br")
	}
	if gotEntry.Feed.Language != "pt-pt" {
		t.Fatalf("joined feed language round-trip: got %q, want %q", gotEntry.Feed.Language, "pt-pt")
	}

	// Verify refresh/update paths persist changed language values too.
	gotFeed.Language = "pt"
	if err := store.UpdateFeed(gotFeed); err != nil {
		t.Fatalf("update feed language: %v", err)
	}
	gotEntry.Language = "en"
	tx, err := store.db.Begin()
	if err != nil {
		t.Fatalf("begin entry update: %v", err)
	}
	if err := store.updateEntry(tx, gotEntry); err != nil {
		tx.Rollback()
		t.Fatalf("update entry language: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit entry update: %v", err)
	}

	updatedFeed, err := store.FeedByID(user.ID, feed.ID)
	if err != nil {
		t.Fatalf("get updated feed: %v", err)
	}
	if updatedFeed.Language != "pt" {
		t.Fatalf("updated feed language: got %q, want %q", updatedFeed.Language, "pt")
	}
	updatedEntry, err := store.NewEntryQueryBuilder(user.ID).WithEntryIDs(entry.ID).GetEntry()
	if err != nil {
		t.Fatalf("get updated entry: %v", err)
	}
	if updatedEntry.Language != "en" {
		t.Fatalf("updated entry language: got %q, want %q", updatedEntry.Language, "en")
	}
}
