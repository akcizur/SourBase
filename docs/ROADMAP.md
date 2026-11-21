# WAVE roadmap

## Phase 0 · Foundation
- [x] Local Go host and embedded web UI.
- [x] Loopback-only local API with per-run token.
- [x] M3U/M3U8 import, channel groups and metadata.
- [x] Playlist management, favourites and local persistence.
- [x] Windows DPAPI storage and Windows x64 build pipeline.

## Phase 1 · Personal library and player
- [ ] Dedicated player workspace, playback history and resume.
- [ ] Local media indexing and file metadata.
- [ ] VLC/MPV hand-off and codec capability detection.
- [ ] XMLTV EPG timeline and channel matching.

## Phase 2 · Catalog
- [ ] Movie/series/season/episode data model.
- [ ] TMDB/OMDb metadata provider, artwork cache and matching.
- [ ] Collections, watch progress and richer search.

## Phase 3 · Source adapters
- [ ] Xtream Live/VOD/series adapter.
- [ ] Official SKTorrent user RSS/API adapter, subject to tracker rules.
- [ ] qBittorrent localhost Web API for user-authorized downloads.
- [ ] Provider health, caching and sync logs.

## Phase 4 · Release quality
- [ ] SQLite migration and versioned schema.
- [ ] Per-platform secret vault, backup and restore.
- [ ] Signed installer, auto-update channel and real Windows integration tests.
