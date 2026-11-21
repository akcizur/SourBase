# WAVE IPTV security notes

## Protected storage

Windows desktop builds use DPAPI (`CryptProtectData` / `CryptUnprotectData`) with current-user scope for `state.dpapi` and `credentials.dpapi`. Do not copy these files to another Windows account and expect them to decrypt. Legacy plaintext `state.json` is removed after migration succeeds.

## Network boundaries

- The local host binds to IPv4 loopback (`127.0.0.1`), not a LAN or public interface.
- qBittorrent endpoints are restricted to loopback addresses.
- The local UI requires a random per-run token for local API operations.
- SKTorrent is connected through a feed URL explicitly supplied by the account owner. No login bypass is included.

## User responsibilities

Use IPTV, VOD, RSS and torrent sources only where permitted by the provider, copyright holder and applicable law. Do not expose qBittorrent Web UI to the public internet. Keep Windows account access protected.

## Known hardening backlog

- Replace the encrypted JSON snapshot with SQLite plus transactional migrations and indexed history/catalog tables.
- Add a first-class encrypted export/restore flow.
- Add a restrictive, nonce-based Content Security Policy after inline scripts/styles are moved to bundled assets.
- Add a Windows-signed release and reproducible provenance metadata.
- Test DPAPI on a clean Windows 10/11 x64 machine and test recovery behavior after account/profile migration.
