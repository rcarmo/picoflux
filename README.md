picoflux
========

**picoflux is a fork of [Miniflux 2](https://github.com/miniflux/v2) that replaces PostgreSQL with an embedded, pure-Go SQLite database.**

The goal is to keep everything that makes Miniflux great — a minimalist, fast,
opinionated feed reader — while removing the only piece of operational
overhead it had: the external database server. picoflux stores everything in a
single SQLite file, so the whole application is one static binary plus one
database file.

> [!NOTE]
> This is an unofficial, community-maintained fork. It is **not** affiliated
> with or endorsed by the upstream Miniflux project. For the original,
> PostgreSQL-backed application and its official support, documentation, and
> hosting, please use [Miniflux](https://miniflux.app).

How picoflux differs from Miniflux
----------------------------------

- **Embedded SQLite instead of PostgreSQL.** The storage layer uses the
  pure-Go [`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite) driver
  (no CGo, no `libpq`), so there is no database server to install, configure,
  back up, or keep running. Point `DATABASE_URL` at a file path and go.
- **WAL by default.** Connections are opened in WAL mode with a busy timeout
  and foreign-key enforcement, tuned for a single-writer, server-style
  workload.
- **Full-text search via SQLite FTS5 + BM25.** Search is powered by an FTS5
  virtual table with weighted `bm25()` ranking (title weighted above body)
  and a recency boost, maintained automatically by triggers — replacing the
  Postgres `tsvector`/`ts_rank` machinery.
- **Single consolidated schema.** The 132 incremental Postgres migrations are
  collapsed into one SQLite baseline, versioned with `PRAGMA user_version`.
- **Trivial cross-compilation.** Because the build is CGo-free, every release
  artifact is produced from the same source with `CGO_ENABLED=0`.

Everything else — the UI, the REST/Fever/Google Reader APIs, integrations,
scrapers, authentication, and configuration — is inherited from Miniflux and
behaves the same way.

### Migrating from Miniflux

There is **no automatic migration** from an existing PostgreSQL Miniflux
database to picoflux. picoflux starts a fresh SQLite database; re-import your
feeds via OPML.

Installation
------------

picoflux is configured exactly like Miniflux, except `DATABASE_URL` is a path
to a SQLite file instead of a Postgres connection string.

```bash
# Run migrations and create the first admin user, then start the server.
DATABASE_URL=/var/lib/picoflux/picoflux.db \
RUN_MIGRATIONS=1 \
CREATE_ADMIN=1 \
ADMIN_USERNAME=admin \
ADMIN_PASSWORD=changeme \
picoflux
```

Container images and binaries are published from this repository (see
[Releases & CI](#releases--ci)). A minimal Compose setup is just the app —
there is no database service:

```yaml
services:
  picoflux:
    image: ghcr.io/rcarmo/picoflux:latest
    ports:
      - "80:8080"
    environment:
      - DATABASE_URL=/data/picoflux.db
      - RUN_MIGRATIONS=1
      - CREATE_ADMIN=1
      - ADMIN_USERNAME=admin
      - ADMIN_PASSWORD=changeme
    volumes:
      - picoflux-data:/data
volumes:
  picoflux-data:
```

Releases & CI
-------------

Release automation lives in `.github/workflows/` and is triggered **only by
`vX.X.X` version tags** (e.g. `v2.3.0`):

- **`release.yml`** cross-compiles static binaries for `linux/amd64`,
  `linux/arm64`, `darwin/amd64`, and `darwin/arm64`, attaches them (with
  SHA-256 checksums) to the GitHub release.
- **`docker.yml`** builds and publishes multi-arch container images to the
  GitHub Container Registry at `ghcr.io/rcarmo/picoflux`:
    - **Alpine:** `amd64, 386, arm/v6, arm/v7, arm64, riscv64, ppc64le, s390x`
    - **Distroless** (`-distroless` tag): `amd64, arm/v7, arm64, ppc64le, riscv64, s390x`

Features
--------

These are inherited from Miniflux.

### Feed Reader

- Supported feed formats: Atom 0.3/1.0, RSS 1.0/2.0, and JSON Feed 1.0/1.1.
- [OPML](https://en.wikipedia.org/wiki/OPML) file import/export and URL import.
- Supports multiple attachments (podcasts, videos, music, and images enclosures).
- Plays videos from YouTube directly inside the reader.
- Organizes articles using categories and bookmarks.
- Share individual articles publicly.
- Fetches website icons (favicons).
- Saves articles to third-party services.
- Provides full-text search (powered by SQLite FTS5 with BM25 ranking).
- Available in 20 languages: Portuguese (Brazilian), Chinese (Simplified and Traditional), Dutch, English (US), Finnish, French, German, Greek, Hindi, Indonesian, Italian, Japanese, Polish, Romanian, Russian, Taiwanese POJ, Ukrainian, Spanish, and Turkish.

### Privacy and Security

- Removes pixel trackers.
- Strips tracking parameters from URLs (e.g., `utm_source`, `utm_medium`, `utm_campaign`, `fbclid`, etc.).
- Retrieves original links when feeds are sourced from FeedBurner.
- Opens external links with attributes `rel="noopener noreferrer" referrerpolicy="no-referrer"` for improved security.
- Implements the HTTP header `Referrer-Policy: no-referrer` to prevent referrer leakage.
- Provides a media proxy to avoid tracking and resolve mixed content warnings when using HTTPS.
- Plays YouTube videos via the privacy-focused domain `youtube-nocookie.com`.
- Supports alternative YouTube video players such as [Invidious](https://invidio.us).
- Blocks external JavaScript to prevent tracking and enhance security.
- Sanitizes external content before rendering it.
- Enforces a [Content Security](https://developer.mozilla.org/en-US/docs/Web/HTTP/CSP) and a [Trusted Types Policy](https://developer.mozilla.org/en-US/docs/Web/API/Trusted_Types_API) to only application JavaScript and blocks inline scripts and styles.

### Bot Protection Bypass Mechanisms

- Optionally disable HTTP/2 to mitigate fingerprinting.
- Allows configuration of a custom user agent.
- Supports adding custom cookies for specific use cases.
- Enables the use of proxies for enhanced privacy or bypassing restrictions.

### Content Manipulation

- Fetches the original article and extracts only the relevant content using a local Readability parser.
- Allows custom scraper rules based on <abbr title="Cascading Style Sheets">CSS</abbr> selectors.
- Supports custom rewriting rules for content manipulation.
- Provides a regex filter to include or exclude articles based on specific patterns.
- Optionally permits self-signed or invalid certificates (disabled by default).
- Scrapes YouTube's website to retrieve video duration as read time or uses the YouTube API (disabled by default).

### User Interface

- Optimized stylesheet for readability.
- Responsive design that adapts seamlessly to desktop, tablet, and mobile devices.
- Minimalistic and distraction-free user interface.
- No requirement to download an app from Apple App Store or Google Play Store.
- Can be added directly to the home screen for quick access.
- Supports a wide range of keyboard shortcuts for efficient navigation.
- Optional touch gesture support for navigation on mobile devices.
- Custom stylesheets and JavaScript to personalize the user interface to your preferences.
- Themes:
    - Light (Sans-Serif)
    - Light (Serif)
    - Dark (Sans-Serif)
    - Dark (Serif)
    - System (Sans-Serif) – Automatically switches between Dark and Light themes based on system preferences.
    - System (Serif)

### Integrations

- 25+ integrations with third-party services: [Apprise](https://github.com/caronc/apprise), [Betula](https://sr.ht/~bouncepaw/betula/), [Cubox](https://cubox.cc/), [Discord](https://discord.com/), [Espial](https://github.com/jonschoning/espial), [Instapaper](https://www.instapaper.com/), [LinkAce](https://www.linkace.org/), [Linkding](https://github.com/sissbruecker/linkding), [LinkTaco](https://linktaco.com), [LinkWarden](https://linkwarden.app/), [Matrix](https://matrix.org), [Notion](https://www.notion.com/), [Ntfy](https://ntfy.sh/), [Nunux Keeper](https://keeper.nunux.org/), [Pinboard](https://pinboard.in/), [Pushover](https://pushover.net), [RainDrop](https://raindrop.io/), [Readeck](https://readeck.org/en/), [Readwise Reader](https://readwise.io/read), [RssBridge](https://rss-bridge.org/), [Shaarli](https://github.com/shaarli/Shaarli), [Shiori](https://github.com/go-shiori/shiori), [Slack](https://slack.com/), [Telegram](https://telegram.org), [Wallabag](https://www.wallabag.org/), etc.
- Bookmarklet for subscribing to websites directly from any web browser.
- Webhooks for real-time notifications or custom integrations.
- Compatibility with existing mobile applications using the Fever or Google Reader API.
- REST API with client libraries available in [Go](https://github.com/miniflux/v2/tree/main/client) and [Python](https://github.com/miniflux/python-client).

### Authentication

- Local username and password.
- Passkeys ([WebAuthn](https://en.wikipedia.org/wiki/WebAuthn)).
- Google (OAuth2).
- Generic OpenID Connect.
- Reverse-Proxy authentication.

### Technical Stuff

- Written in [Go (Golang)](https://golang.org/).
- Single binary compiled statically without dependency.
- Uses an embedded, pure-Go [SQLite](https://www.sqlite.org/) database (via `modernc.org/sqlite`) — no external database server required.
- Does not use any ORM or any complicated frameworks.
- Uses modern vanilla JavaScript only when necessary.
- All static files are bundled into the application binary using the Go `embed` package.
- Supports the Systemd `sd_notify` protocol for process monitoring.
- Configures HTTPS automatically with Let's Encrypt.
- Allows the use of custom <abbr title="Secure Sockets Layer">SSL</abbr> certificates.
- Supports [HTTP/2](https://en.wikipedia.org/wiki/HTTP/2) when TLS is enabled.
- Updates feeds in the background using an internal scheduler or a traditional cron job.
- Uses native lazy loading for images and iframes.
- Compatible only with modern browsers.
- Adheres to the [Twelve-Factor App](https://12factor.net/) methodology.
- Publishes multi-arch Docker images to the GitHub Container Registry, plus pre-built binaries, with ARM, RISC-V, ppc64le and s390x architecture support.
- Uses a limited amount of third-party go dependencies.
- Has a comprehensive testsuite, with both unit tests and integration tests.
- Only uses a couple of MB of memory and a negligible amount of CPU, even with several hundreds of feeds.
- Respects/sends Last-Modified, If-Modified-Since, If-None-Match, Cache-Control, Expires and ETags headers, and has a default polling interval of 1h.

Documentation
-------------

Because picoflux is API- and configuration-compatible with Miniflux (apart
from `DATABASE_URL`), the upstream Miniflux documentation applies:
<https://miniflux.app/docs/> ([Man page](https://miniflux.app/miniflux.1.html))

- [Opinionated?](https://miniflux.app/opinionated.html)
- [Features](https://miniflux.app/features.html)
- [Configuration](https://miniflux.app/docs/configuration.html) — note `DATABASE_URL` is a SQLite file path here
- [Command Line Usage](https://miniflux.app/docs/cli.html)
- [User Interface Usage](https://miniflux.app/docs/ui.html)
- [Keyboard Shortcuts](https://miniflux.app/docs/keyboard_shortcuts.html)
- [Integration with External Services](https://miniflux.app/docs/#integrations)
- [Rewrite and Scraper Rules](https://miniflux.app/docs/rules.html)
- [API Reference](https://miniflux.app/docs/api.html)
- [Frequently Asked Questions](https://miniflux.app/faq.html)

Screenshots
-----------

Default theme:

![Default theme](https://miniflux.app/images/overview.png)

Dark theme when using keyboard navigation:

![Dark theme](https://miniflux.app/images/item-selection-black-theme.png)

Credits
-------

picoflux stands entirely on the shoulders of Miniflux.

- **Upstream Miniflux** — created by Frédéric Guillot and
  [contributors](https://github.com/miniflux/v2/graphs/contributors).
  All of the feed-reading application is their work.
- **picoflux fork** (SQLite backend, CI/CD) — maintained by Rui Carmo and
  contributors.
- Distributed under the Apache 2.0 License, the same as upstream Miniflux.
