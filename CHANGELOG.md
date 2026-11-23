# Changelog

All notable changes to this project are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses [Semantic Versioning](https://semver.org/). The reasoning behind each change is in [`docs/specs/`](./docs/specs/).

## [Unreleased]

### Added

- Read-only JSON API under `/api/v1` (spec 0005): `GET /api/v1/events` with filters and cursor pagination, `GET /api/v1/events/{id}`, `GET /api/v1/metrics/error-frequency` (empty days filled, UTC), `GET /api/v1/metrics/severity-counts`, `GET /api/v1/metrics/data-range` (with a suggested default period) and `GET /api/v1/applications`, `/environments`, `/sources`.
- Strict validation on the read API: invalid, unknown or repeated parameters answer `400` with `code` and `field`; a missing event answers `404`; unknown paths under `/api/` answer JSON `404`.
- `HTTP_HOST` setting for the address `tracelog serve` listens on.
- Reference responses in `internal/adapters/http/testdata/api/`, checked by the handler tests.
- Migration `000004`: index on `(severity, occurred_at DESC, id DESC)` for severity-filtered reads (measured: filtered list from ~11 ms to ~0.3 ms, severity counts from ~35 ms to ~20 ms on 80k events). Creating it locks writes on `technical_event` while it builds (190 ms for 80k rows).

### Changed

- **Breaking:** `tracelog serve` listens on `127.0.0.1` by default instead of every interface, because the read API serves event payloads without authentication. Set `HTTP_HOST=0.0.0.0` to receive events from other machines. The Docker image and the Compose stack set it inside the container, so `docker compose up` behaves as before.
- `tracelog events list` orders events with the same `occurred_at` by `id`, so the order is stable.
- `tracelog db list` reads through the same repository port as the API (same output).

## [0.1.0] - 2026-11-21

First release meant to be run by others: `cp .env.example .env && make demo` brings everything up.

### Added

- `tracelog import logs`: imports JSON, `timestamp level message` and free-text log lines, with categorization and a quality summary.
- Docker image (multi-stage, non-root) and a Compose stack that starts PostgreSQL, applies migrations, creates the default setup and starts the API with no manual steps.
- `tracelog migrate`: migrations embedded in the binary and recorded in `schema_migrations`, applied once each under an advisory lock. Replaces `scripts/run-migrations.sh`.
- `tracelog init --defaults` for non-interactive setup.
- `GET /ready` (database check), separate from `GET /health` (liveness).
- Deduplication by `(application_id, external_id)`: resending an event returns `200` with `"duplicate": true`; re-importing a log file does not duplicate lines.
- `HTTP_MAX_BODY_BYTES` (default 1 MiB): larger bodies get `413`.
- Synthetic example data in `examples/logs/`, the deterministic generator `tools/loggen` and `scripts/demo.sh` / `make demo`.
- Import summary lists failed lines (`file:line: reason`, up to 10).
- CI: gofmt, `go vet`, race-enabled tests against PostgreSQL 16, build and Docker image build.
- `Makefile`, `CHANGELOG.md`, `AGENTS.md`, project guide and specs 0001–0004.

### Changed

- **Breaking:** `metrics errors` output has `ERRORS`, `FATAL` and `TOTAL` columns, counts `fatal` events and groups days in UTC.
- **Breaking:** `POST /events` answers `415` when a `Content-Type` other than JSON is sent (a missing header is still accepted).
- **Breaking:** a client can no longer set the severity through `payload.level` on HTTP; it still applies to imported log lines.
- Severity keywords match whole words, so `terror` or `crashpad` no longer count as errors.
- Internal errors return a generic `500` body; details go to the server log.
- Oversized `external_id`, `trace_id` and `correlation_id` are truncated instead of failing the insert.
- `--to YYYY-MM-DD` includes the whole day.
- The HTTP server has read/write/idle timeouts and shuts down gracefully on SIGINT/SIGTERM; the database pool is capped at 10 connections.
- Import connections use `synchronous_commit=off` (about 10x faster on the test machine).
- Go 1.26.11; Compose publishes ports on `127.0.0.1` only.
- Documentation consolidated into the README and the project guide.

### Fixed

- Duplicate `external_id` returned `500` and collided across applications.
- Request bodies had no size limit.
- Database passwords with spaces or quotes broke the connection string.
- UUID v6/v7/v8 references were treated as invalid and replaced by the fallback.
- Failed logins and `unauthorized` lines were categorized as `audit` instead of `security`.
- Bracketed timestamps (`[06/Mar/2026:02:26:01 +0000]`) were parsed as broken JSON.
- Lines with unreadable timestamps were reported as `normalized`.
- Blank log lines were stored as events.
- The raw request body was copied into every event with a normalization warning.

### Removed

- `tracelog seed`, which only printed a message.
- `scripts/run-migrations.sh`.
- The outdated documents `docs/http-api.md`, `docs/cli.md`, `docs/domain.md`, `docs/architecture.md`, `docs/development.md` and `docs/getting-started.md`.

## [0.0.1] - 2026-03-09

Initial MVP on PostgreSQL: `POST /events`, `GET /health` and the commands `serve`, `init`, `events list`, `metrics errors`, `db list` and `seed` (a stub).

[Unreleased]: https://github.com/svitorz/tracelog/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/svitorz/tracelog/compare/v0.0.1...v0.1.0
[0.0.1]: https://github.com/svitorz/tracelog/releases/tag/v0.0.1
