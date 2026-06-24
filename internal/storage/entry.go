// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package storage // import "miniflux.app/v2/internal/storage"

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"miniflux.app/v2/internal/crypto"
	"miniflux.app/v2/internal/model"
)

// ErrEntryTombstoned is returned when an entry cannot be created because its
// (feed_id, hash) pair has a tombstone recording a prior deletion.
var ErrEntryTombstoned = errors.New("store: entry is tombstoned")

// CountAllEntries returns the number of entries for each status in the database.
func (s *Storage) CountAllEntries() (map[string]int64, error) {
	rows, err := s.db.Query(`SELECT status, count(*) FROM entries GROUP BY status`)
	if err != nil {
		return nil, fmt.Errorf("storage: unable to count entries: %w", err)
	}
	defer rows.Close()

	results := make(map[string]int64)
	results[model.EntryStatusUnread] = 0
	results[model.EntryStatusRead] = 0

	for rows.Next() {
		var status string
		var count int64

		if err := rows.Scan(&status, &count); err != nil {
			continue
		}

		results[status] = count
	}

	results["total"] = results[model.EntryStatusUnread] + results[model.EntryStatusRead]
	return results, nil
}

// UpdateEntryTitleAndContent updates entry title and content.
func (s *Storage) UpdateEntryTitleAndContent(entry *model.Entry) error {
	query := `
		UPDATE
			entries
		SET
			title=$1,
			content=$2,
			reading_time=$3
		WHERE
			id=$4 AND user_id=$5
	`

	if _, err := s.db.Exec(
		query,
		entry.Title,
		entry.Content,
		entry.ReadingTime,
		entry.ID,
		entry.UserID); err != nil {
		return fmt.Errorf(`store: unable to update entry #%d: %v`, entry.ID, err)
	}

	return nil
}

// createEntry add a new entry.
func (s *Storage) createEntry(tx *sql.Tx, entry *model.Entry) error {
	// The WHERE NOT EXISTS guard makes the tombstone check atomic with the insert, so a
	// concurrent archive committing between an earlier existence check and this statement
	// cannot bring a deleted entry back as unread.
	//
	// Full-text indexing is maintained automatically by the entries_fts AFTER triggers,
	// so there is no document_vectors column to populate here.
	query := `
		INSERT INTO entries
			(
				title,
				hash,
				url,
				comments_url,
				published_at,
				content,
				author,
				user_id,
				feed_id,
				reading_time,
				changed_at,
				tags
			)
		SELECT
			$1,
			$2,
			$3,
			$4,
			$5,
			$6,
			$7,
			$8,
			$9,
			$10,
			CURRENT_TIMESTAMP,
			$11
		WHERE NOT EXISTS (
			SELECT 1 FROM entry_tombstones WHERE feed_id=$9 AND hash=$2
		)
		RETURNING
			id, status, created_at, changed_at
	`
	err := tx.QueryRow(
		query,
		entry.Title,
		entry.Hash,
		entry.URL,
		entry.CommentsURL,
		entry.Date,
		entry.Content,
		entry.Author,
		entry.UserID,
		entry.FeedID,
		entry.ReadingTime,
		jsonStringArray(entry.Tags),
	).Scan(
		&entry.ID,
		&entry.Status,
		&entry.CreatedAt,
		&entry.ChangedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrEntryTombstoned
	}
	if err != nil {
		return fmt.Errorf(`store: unable to create entry %q (feed #%d): %v`, entry.URL, entry.FeedID, err)
	}

	for _, enclosure := range entry.Enclosures {
		enclosure.EntryID = entry.ID
		enclosure.UserID = entry.UserID
		err := s.createEnclosure(tx, enclosure)
		if err != nil {
			return err
		}
	}

	return nil
}

// updateEntry updates an entry when a feed is refreshed.
// Note: we do not update the published date because some feeds do not contains any date,
// it default to time.Now() which could change the order of items on the history page.
func (s *Storage) updateEntry(tx *sql.Tx, entry *model.Entry) error {
	query := `
		UPDATE
			entries
		SET
			title=$1,
			url=$2,
			comments_url=$3,
			content=$4,
			author=$5,
			reading_time=$6,
			tags=$7
		WHERE
			user_id=$8 AND feed_id=$9 AND hash=$10
		RETURNING
			id
	`
	err := tx.QueryRow(
		query,
		entry.Title,
		entry.URL,
		entry.CommentsURL,
		entry.Content,
		entry.Author,
		entry.ReadingTime,
		jsonStringArray(entry.Tags),
		entry.UserID,
		entry.FeedID,
		entry.Hash,
	).Scan(&entry.ID)
	if err != nil {
		return fmt.Errorf(`store: unable to update entry %q: %v`, entry.URL, err)
	}

	for _, enclosure := range entry.Enclosures {
		enclosure.UserID = entry.UserID
		enclosure.EntryID = entry.ID
	}

	return s.updateEnclosures(tx, entry)
}

// entryExists checks if an entry already exists based on its hash when refreshing a feed.
func (s *Storage) entryExists(tx *sql.Tx, entry *model.Entry) (bool, error) {
	var result bool

	// Note: This query uses entries_feed_id_hash_key index (filtering on user_id is not necessary).
	err := tx.QueryRow(`SELECT true FROM entries WHERE feed_id=$1 AND hash=$2 LIMIT 1`, entry.FeedID, entry.Hash).Scan(&result)

	if err != nil && err != sql.ErrNoRows {
		return result, fmt.Errorf(`store: unable to check if entry exists: %v`, err)
	}

	return result, nil
}

func (s *Storage) getEntryIDByHash(tx *sql.Tx, feedID int64, entryHash string) (int64, error) {
	var entryID int64

	err := tx.QueryRow(
		`SELECT id FROM entries WHERE feed_id=$1 AND hash=$2 LIMIT 1`,
		feedID,
		entryHash,
	).Scan(&entryID)

	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf(`store: unable to fetch entry ID: %v`, err)
	}

	return entryID, nil
}

// InsertEntryForFeed inserts a single entry into a feed, optionally updating if it already exists.
// Returns true if a new entry was created, false if an existing one was reused.
func (s *Storage) InsertEntryForFeed(userID, feedID int64, entry *model.Entry) (bool, error) {
	entry.UserID = userID
	entry.FeedID = feedID

	tx, err := s.db.Begin()
	if err != nil {
		return false, fmt.Errorf("store: unable to start transaction: %v", err)
	}
	defer tx.Rollback()

	entryID, err := s.getEntryIDByHash(tx, entry.FeedID, entry.Hash)
	if err != nil {
		return false, err
	}
	alreadyExistingEntry := entryID > 0

	if alreadyExistingEntry {
		entry.ID = entryID
	} else {
		if err := s.createEntry(tx, entry); err != nil {
			return false, err
		}
	}

	if err := tx.Commit(); err != nil {
		return false, err
	}

	return !alreadyExistingEntry, nil
}

func (s *Storage) IsNewEntry(feedID int64, entryHash string) bool {
	// An entry is new only if it is neither stored nor tombstoned; otherwise
	// callers (such as the crawler) would do expensive work on every refresh
	// for items that will be discarded.
	query := `
		SELECT
			EXISTS (
				SELECT 1 FROM entries WHERE feed_id=$1 AND hash=$2
			) OR EXISTS (
				SELECT 1 FROM entry_tombstones WHERE feed_id=$1 AND hash=$2
			)
	`
	var known bool
	s.db.QueryRow(query, feedID, entryHash).Scan(&known)
	return !known
}

func (s *Storage) GetReadTime(feedID int64, entryHash string) int {
	var result int

	// Note: This query uses entries_feed_id_hash_key index
	s.db.QueryRow(
		`SELECT
			reading_time
		FROM
			entries
		WHERE
			feed_id=$1 AND
			hash=$2
		`,
		feedID,
		entryHash,
	).Scan(&result)
	return result
}

// RefreshFeedEntries updates feed entries while refreshing a feed.
func (s *Storage) RefreshFeedEntries(userID, feedID int64, entries model.Entries, updateExistingEntries bool) (newEntries model.Entries, err error) {
	for _, entry := range entries {
		entry.UserID = userID
		entry.FeedID = feedID

		tx, err := s.db.Begin()
		if err != nil {
			return nil, fmt.Errorf(`store: unable to start transaction: %v`, err)
		}

		entryExists, err := s.entryExists(tx, entry)
		if err != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				return nil, fmt.Errorf(`store: unable to rollback transaction: %v (rolled back due to: %v)`, rollbackErr, err)
			}
			return nil, err
		}

		if entryExists {
			if updateExistingEntries {
				err = s.updateEntry(tx, entry)
			}
		} else {
			err = s.createEntry(tx, entry)
			switch {
			case errors.Is(err, ErrEntryTombstoned):
				err = nil
			case err == nil:
				newEntries = append(newEntries, entry)
			}
		}

		if err != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				return nil, fmt.Errorf(`store: unable to rollback transaction: %v (rolled back due to: %v)`, rollbackErr, err)
			}
			return nil, err
		}

		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf(`store: unable to commit transaction: %v`, err)
		}
	}

	return newEntries, nil
}

// ArchiveEntries deletes entries older than the given interval and records tombstones so they are not re-ingested.
func (s *Storage) ArchiveEntries(status string, interval time.Duration, limit int) (int64, error) {
	if interval < 0 || limit <= 0 {
		return 0, nil
	}

	days := max(int(interval/(24*time.Hour)), 1)
	cutoff := fmt.Sprintf("-%d days", days)

	// SQLite has no data-modifying CTEs, DELETE ... USING, or FOR UPDATE SKIP LOCKED.
	// With a single writer we select the archivable rows, record tombstones, then
	// delete them inside one transaction.
	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf(`store: unable to start transaction: %v`, err)
	}
	defer tx.Rollback()

	rows, err := tx.Query(`
		SELECT id, feed_id, hash
		FROM entries
		WHERE
			status=$1 AND
			starred=0 AND
			share_code='' AND
			created_at < datetime('now', $2)
		ORDER BY created_at ASC
		LIMIT $3
	`, status, cutoff, limit)
	if err != nil {
		return 0, fmt.Errorf(`store: unable to select archivable %s entries: %v`, status, err)
	}

	type archivable struct {
		id     int64
		feedID int64
		hash   string
	}
	var batch []archivable
	for rows.Next() {
		var a archivable
		if err := rows.Scan(&a.id, &a.feedID, &a.hash); err != nil {
			rows.Close()
			return 0, fmt.Errorf(`store: unable to scan archivable entry: %v`, err)
		}
		batch = append(batch, a)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf(`store: unable to iterate archivable entries: %v`, err)
	}

	var count int64
	for _, a := range batch {
		if a.hash != "" {
			if _, err := tx.Exec(
				`INSERT INTO entry_tombstones (feed_id, hash) VALUES ($1, $2) ON CONFLICT (feed_id, hash) DO NOTHING`,
				a.feedID, a.hash,
			); err != nil {
				return 0, fmt.Errorf(`store: unable to record tombstone: %v`, err)
			}
		}
		res, err := tx.Exec(`DELETE FROM entries WHERE id=$1`, a.id)
		if err != nil {
			return 0, fmt.Errorf(`store: unable to archive entry #%d: %v`, a.id, err)
		}
		n, _ := res.RowsAffected()
		count += n
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf(`store: unable to commit archive of %s entries: %v`, status, err)
	}

	return count, nil
}

// SetEntriesStatus update the status of the given list of entries.
func (s *Storage) SetEntriesStatus(userID int64, entryIDs []int64, status string) error {
	query := `
		UPDATE
			entries
		SET
			status=$1,
			changed_at=CURRENT_TIMESTAMP
		WHERE
			user_id=$2 AND
			id IN (SELECT value FROM json_each($3))
		`
	if _, err := s.db.Exec(query, status, userID, jsonIntArray(entryIDs)); err != nil {
		return fmt.Errorf(`store: unable to update entries statuses %v: %v`, entryIDs, err)
	}

	return nil
}

// SetEntriesStatusAndCountVisible updates the status of the given entries and returns how many are visible in global views.
func (s *Storage) SetEntriesStatusAndCountVisible(userID int64, entryIDs []int64, status string) (int, error) {
	// SQLite does not support data-modifying CTEs (UPDATE ... RETURNING inside WITH),
	// so the update and the visibility count run as two statements in one transaction.
	ids := jsonIntArray(entryIDs)

	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf(`store: unable to start transaction: %v`, err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
		UPDATE entries
		SET status=$1, changed_at=CURRENT_TIMESTAMP
		WHERE user_id=$2 AND id IN (SELECT value FROM json_each($3))
	`, status, userID, ids); err != nil {
		return 0, fmt.Errorf(`store: unable to update entries status %v: %v`, entryIDs, err)
	}

	var visible int
	if err := tx.QueryRow(`
		SELECT count(*)
		FROM entries e
			JOIN feeds f ON f.id = e.feed_id
			JOIN categories c ON c.id = f.category_id
		WHERE e.user_id=$1
			AND e.id IN (SELECT value FROM json_each($2))
			AND f.hide_globally=0
			AND c.hide_globally=0
	`, userID, ids).Scan(&visible); err != nil {
		return 0, fmt.Errorf(`store: unable to count visible entries %v: %v`, entryIDs, err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf(`store: unable to commit entries status update %v: %v`, entryIDs, err)
	}

	return visible, nil
}

// SetEntriesStarredState updates the starred state for the given list of entries.
func (s *Storage) SetEntriesStarredState(userID int64, entryIDs []int64, starred bool) error {
	query := `UPDATE entries SET starred=$1, changed_at=CURRENT_TIMESTAMP WHERE user_id=$2 AND id IN (SELECT value FROM json_each($3))`
	if _, err := s.db.Exec(query, starred, userID, jsonIntArray(entryIDs)); err != nil {
		return fmt.Errorf(`store: unable to update the starred state %v: %v`, entryIDs, err)
	}

	return nil
}

// ToggleStarred toggles entry starred value.
func (s *Storage) ToggleStarred(userID int64, entryID int64) error {
	query := `UPDATE entries SET starred = NOT starred, changed_at=CURRENT_TIMESTAMP WHERE user_id=$1 AND id=$2`
	result, err := s.db.Exec(query, userID, entryID)
	if err != nil {
		return fmt.Errorf(`store: unable to toggle starred flag for entry #%d: %v`, entryID, err)
	}

	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf(`store: unable to toggle starred flag for entry #%d: %v`, entryID, err)
	}

	if count == 0 {
		return errors.New(`store: nothing has been updated`)
	}

	return nil
}

// FlushHistory deletes all read entries (non-starred, non-shared) and records tombstones to prevent re-ingestion.
func (s *Storage) FlushHistory(userID int64) error {
	// SQLite has no data-modifying CTEs: record tombstones, then delete, in one transaction.
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf(`store: unable to start transaction: %v`, err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
		INSERT INTO entry_tombstones (feed_id, hash)
		SELECT feed_id, hash FROM entries
		WHERE user_id=$1 AND status=$2 AND starred=0 AND share_code='' AND hash <> ''
		ON CONFLICT (feed_id, hash) DO NOTHING
	`, userID, model.EntryStatusRead); err != nil {
		return fmt.Errorf(`store: unable to record tombstones while flushing history: %v`, err)
	}

	if _, err := tx.Exec(`
		DELETE FROM entries
		WHERE user_id=$1 AND status=$2 AND starred=0 AND share_code=''
	`, userID, model.EntryStatusRead); err != nil {
		return fmt.Errorf(`store: unable to flush history: %v`, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf(`store: unable to commit history flush: %v`, err)
	}

	return nil
}

// MarkAllAsRead updates all user entries to the read status.
func (s *Storage) MarkAllAsRead(userID int64) error {
	query := `UPDATE entries SET status=$1, changed_at=CURRENT_TIMESTAMP WHERE user_id=$2 AND status=$3`
	result, err := s.db.Exec(query, model.EntryStatusRead, userID, model.EntryStatusUnread)
	if err != nil {
		return fmt.Errorf(`store: unable to mark all entries as read: %v`, err)
	}

	count, _ := result.RowsAffected()
	slog.Debug("Marked all entries as read",
		slog.Int64("user_id", userID),
		slog.Int64("nb_entries", count),
	)

	return nil
}

// MarkAllAsReadBeforeDate updates all user entries to the read status before the given date.
func (s *Storage) MarkAllAsReadBeforeDate(userID int64, before time.Time) error {
	query := `
		UPDATE
			entries
		SET
			status=$1,
			changed_at=CURRENT_TIMESTAMP
		WHERE
			user_id=$2 AND status=$3 AND published_at < $4
	`
	result, err := s.db.Exec(query, model.EntryStatusRead, userID, model.EntryStatusUnread, before)
	if err != nil {
		return fmt.Errorf(`store: unable to mark all entries as read before %s: %v`, before.Format(time.RFC3339), err)
	}
	count, _ := result.RowsAffected()
	slog.Debug("Marked all entries as read before date",
		slog.Int64("user_id", userID),
		slog.Int64("nb_entries", count),
		slog.String("before", before.Format(time.RFC3339)),
	)
	return nil
}

// MarkGloballyVisibleFeedsAsRead updates all user entries to the read status.
func (s *Storage) MarkGloballyVisibleFeedsAsRead(userID int64) error {
	query := `
		UPDATE
			entries
		SET
			status=$1,
			changed_at=CURRENT_TIMESTAMP
		FROM
			feeds
		WHERE
			entries.feed_id = feeds.id
			AND entries.user_id=$2
			AND entries.status=$3
			AND feeds.hide_globally=$4
	`
	result, err := s.db.Exec(query, model.EntryStatusRead, userID, model.EntryStatusUnread, false)
	if err != nil {
		return fmt.Errorf(`store: unable to mark globally visible feeds as read: %v`, err)
	}

	count, _ := result.RowsAffected()
	slog.Debug("Marked globally visible feed entries as read",
		slog.Int64("user_id", userID),
		slog.Int64("nb_entries", count),
	)

	return nil
}

// MarkFeedAsRead updates all feed entries to the read status.
func (s *Storage) MarkFeedAsRead(userID, feedID int64, before time.Time) error {
	query := `
		UPDATE
			entries
		SET
			status=$1,
			changed_at=CURRENT_TIMESTAMP
		WHERE
			user_id=$2 AND feed_id=$3 AND status=$4 AND published_at < $5
	`
	result, err := s.db.Exec(query, model.EntryStatusRead, userID, feedID, model.EntryStatusUnread, before)
	if err != nil {
		return fmt.Errorf(`store: unable to mark feed entries as read: %v`, err)
	}

	count, _ := result.RowsAffected()
	slog.Debug("Marked feed entries as read",
		slog.Int64("user_id", userID),
		slog.Int64("feed_id", feedID),
		slog.Int64("nb_entries", count),
		slog.String("before", before.Format(time.RFC3339)),
	)

	return nil
}

// MarkCategoryAsRead updates all category entries to the read status.
func (s *Storage) MarkCategoryAsRead(userID, categoryID int64, before time.Time) error {
	query := `
		UPDATE
			entries
		SET
			status=$1,
			changed_at=CURRENT_TIMESTAMP
		FROM
			feeds
		WHERE
			feed_id=feeds.id
		AND
			feeds.user_id=$2
		AND
			status=$3
		AND
			published_at < $4
		AND
			feeds.category_id=$5
	`
	result, err := s.db.Exec(query, model.EntryStatusRead, userID, model.EntryStatusUnread, before, categoryID)
	if err != nil {
		return fmt.Errorf(`store: unable to mark category entries as read: %v`, err)
	}

	count, _ := result.RowsAffected()
	slog.Debug("Marked category entries as read",
		slog.Int64("user_id", userID),
		slog.Int64("category_id", categoryID),
		slog.Int64("nb_entries", count),
		slog.String("before", before.Format(time.RFC3339)),
	)

	return nil
}

// EntryShareCode returns the share code of the provided entry.
// It generates a new one if not already defined.
func (s *Storage) EntryShareCode(userID int64, entryID int64) (shareCode string, err error) {
	query := `SELECT share_code FROM entries WHERE user_id=$1 AND id=$2`
	err = s.db.QueryRow(query, userID, entryID).Scan(&shareCode)
	if err != nil {
		err = fmt.Errorf(`store: unable to get share code for entry #%d: %v`, entryID, err)
		return
	}

	if shareCode == "" {
		shareCode = crypto.GenerateRandomStringHex(20)

		query = `UPDATE entries SET share_code = $1 WHERE user_id=$2 AND id=$3`
		_, err = s.db.Exec(query, shareCode, userID, entryID)
		if err != nil {
			err = fmt.Errorf(`store: unable to set share code for entry #%d: %v`, entryID, err)
			return
		}
	}

	return
}

// UnshareEntry removes the share code for the given entry.
func (s *Storage) UnshareEntry(userID int64, entryID int64) (err error) {
	query := `UPDATE entries SET share_code='' WHERE user_id=$1 AND id=$2`
	_, err = s.db.Exec(query, userID, entryID)
	if err != nil {
		err = fmt.Errorf(`store: unable to remove share code for entry #%d: %v`, entryID, err)
	}
	return
}
