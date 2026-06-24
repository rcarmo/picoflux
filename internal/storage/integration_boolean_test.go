// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"miniflux.app/v2/internal/crypto"
	"testing"
)

// TestIntegrationBooleanQueriesSQLite guards against a regression where
// integration queries filtered enabled flags with the PostgreSQL text-boolean
// literal (`..._enabled='t'`). Under SQLite those columns are stored as
// integers (0/1), so `='t'` never matched and silently disabled the Fever and
// Google Reader APIs plus every save/notify integration and the nav indicator.
func TestIntegrationBooleanQueriesSQLite(t *testing.T) {
	store := newTestStore(t)
	user := createTestUser(t, store, "alice")

	// Start from the row created alongside the user so CHECK-constrained
	// defaults (e.g. linktaco_visibility) stay valid.
	integration, err := store.Integration(user.ID)
	if err != nil {
		t.Fatalf("Integration: %v", err)
	}
	integration.UserID = user.ID
	integration.GoogleReaderEnabled = true
	integration.GoogleReaderUsername = "alice"
	hashedPassword, err := crypto.HashPassword("s3cr3t-password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	integration.GoogleReaderPassword = hashedPassword
	integration.PinboardEnabled = true
	integration.PinboardToken = "tok"
	if err := store.UpdateIntegration(integration); err != nil {
		t.Fatalf("UpdateIntegration: %v", err)
	}

	// Google Reader auth path: enabled flag must match under SQLite.
	if err := store.GoogleReaderUserCheckPassword("alice", "s3cr3t-password"); err != nil {
		t.Fatalf("GoogleReaderUserCheckPassword failed (boolean filter regression?): %v", err)
	}
	if err := store.GoogleReaderUserCheckPassword("alice", "wrong"); err == nil {
		t.Fatal("GoogleReaderUserCheckPassword accepted a wrong password")
	}

	// "Has any save integration enabled" detection (runs on new entries).
	if !store.HasSaveEntry(user.ID) {
		t.Fatal("HasSaveEntry returned false despite an enabled integration")
	}

	got, err := store.GoogleReaderUserGetIntegration("alice")
	if err != nil {
		t.Fatalf("GoogleReaderUserGetIntegration: %v", err)
	}
	if !got.GoogleReaderEnabled || got.GoogleReaderUsername != "alice" {
		t.Fatalf("unexpected integration round-trip: %+v", got)
	}
}
