// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package storage // import "miniflux.app/v2/internal/storage"

import (
	"log/slog"

	"miniflux.app/v2/internal/config"
)

type NavMetadata struct {
	CountUnread     int
	CountErrorFeeds int
	HasSaveEntry    bool
}

// GetNavMetadata returns the navigation metadata for the given user in a
// single SQL query.
func (s *Storage) GetNavMetadata(userID int64) (NavMetadata, error) {
	query := `
		SELECT
			(SELECT count(*)
			   FROM entries e
			   JOIN feeds f ON f.id = e.feed_id
			   JOIN categories c ON c.id = f.category_id
			  WHERE e.user_id = $1
			    AND e.status = 'unread'
			    AND f.hide_globally = 0
			    AND c.hide_globally = 0
			) AS count_unread,
			(SELECT EXISTS(
				SELECT 1
				  FROM integrations
				 WHERE user_id = $1
				   AND (
					pinboard_enabled=1 OR
					instapaper_enabled=1 OR
					wallabag_enabled=1 OR
					notion_enabled=1 OR
					nunux_keeper_enabled=1 OR
					espial_enabled=1 OR
					readwise_enabled=1 OR
					linkace_enabled=1 OR
					linkding_enabled=1 OR
					linktaco_enabled=1 OR
					linkwarden_enabled=1 OR
					apprise_enabled=1 OR
					shiori_enabled=1 OR
					readeck_enabled=1 OR
					shaarli_enabled=1 OR
					webhook_enabled=1 OR
					omnivore_enabled=1 OR
					karakeep_enabled=1 OR
					raindrop_enabled=1 OR
					betula_enabled=1 OR
					cubox_enabled=1 OR
					discord_enabled=1 OR
					slack_enabled=1 OR
					archiveorg_enabled=1
				   )
			)) AS has_save_entry,
	`
	if config.Opts.PollingParsingErrorLimit() == 0 {
		// zero means unlimited amount of accepted errors
		query += `(SELECT $2) AS count_error_feeds`
	} else {
		query += `(SELECT count(*)
			   FROM feeds
			  WHERE user_id = $1
			    AND parsing_error_count >= $2
			) AS count_error_feeds
			 `
	}

	var countUnread, countErrorFeeds int
	var hasSaveEntry bool

	err := s.db.QueryRow(query, userID, config.Opts.PollingParsingErrorLimit()).Scan(
		&countUnread,
		&hasSaveEntry,
		&countErrorFeeds,
	)
	if err != nil {
		slog.Error("Unable to fetch navigation metadata",
			slog.Int64("user_id", userID),
			slog.Any("error", err),
		)
		return NavMetadata{}, err
	}

	return NavMetadata{
		CountUnread:     countUnread,
		CountErrorFeeds: countErrorFeeds,
		HasSaveEntry:    hasSaveEntry,
	}, nil
}
