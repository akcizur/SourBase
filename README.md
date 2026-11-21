# WAVE IPTV

**WAVE is a local-first personal media workspace.** This repository starts a fresh, source-controlled foundation for a single Windows-friendly application that brings IPTV playlists, channel browsing, a personal library and playback into one coherent interface.

> **Foundation status: 0.1.0.** Implemented now: local M3U/M3U8 file import, channel/group parsing, playlist management, favourites, a local library snapshot and a browser-based player for formats supported by the system browser. Xtream, XMLTV, film/series metadata, SKTorrent and qBittorrent are planned adapters, not claimed as finished features.

## Run and build

Requires Go 1.23+.

```bash
go test ./...
go vet ./...
go run .
```

The app binds to an available port on `127.0.0.1` only and opens its local UI. Use `WAVE_NO_BROWSER=1` to disable automatic browser launch.

On Windows, build a portable GUI executable:

```bat
build-windows.bat
```

End users do not need Node.js, a hosted backend or a cloud account. The UI is embedded in the Go executable.

## Current capabilities

- Import local `.m3u` and `.m3u8` files.
- Parse `tvg-id`, `tvg-logo`, `group-title` and channel names.
- Browse/search channels and filter by playlist/category.
- Save favourites and switch/remove playlists.
- Play browser-compatible stream URLs.
- Persist the local library; Windows builds protect the snapshot with current-user DPAPI.
- Local API protected by a random per-run token.

## Architecture and roadmap

- [Architecture](docs/ARCHITECTURE.md)
- [Security model](docs/SECURITY.md)
- [Roadmap](docs/ROADMAP.md)

The next milestones are a dedicated player/library, XMLTV EPG, Xtream adapter, metadata provider, authorised SKTorrent RSS and local qBittorrent adapter, then SQLite and signed releases.

## Playback and legal boundaries

HTML5 playback depends on browser codecs and HLS support. The foundation does not include DRM circumvention, a universal codec engine or a torrent streaming engine. Provider and tracker integrations will use user-authorised sources and explicit permissions. Only import and play content you are allowed to access.
