# WAVE architecture · foundation

## Runtime
- Go standard-library host and embedded web UI.
- Binds to an ephemeral port on IPv4 loopback only.
- Random per-run bearer token protects private API routes.
- State is persisted atomically; Windows uses DPAPI current-user scope, non-Windows currently uses restrictive file permissions.

## Layers
```
Web UI -> Local HTTP API -> domain modules -> local state store
```
The M3U parser is deterministic and never fetches URLs during import.

## Provider contracts planned
- `CatalogProvider`: normalized movies, series, seasons and episodes.
- `LiveProvider`: channels, groups and playback URL resolution.
- `GuideProvider`: XMLTV programme ingestion and channel matching.
- `DownloadProvider`: authorized qBittorrent task management and completed-file lookup.
- `MetadataProvider`: TMDB/OMDb enrichment and poster cache.

Providers must not mutate UI state directly. They should declare network hosts, credentials and capabilities explicitly. Third-party executable plugins are not loaded in the initial release.

## Playback
The initial UI uses HTML5 video and supports only codecs/formats available in the host browser. VLC/MPV hand-off and a native player engine are later milestones.
