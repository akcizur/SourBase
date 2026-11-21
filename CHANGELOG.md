# Changelog

## 1.6.0 · Research implementation

- Encrypt the complete local desktop state with Windows DPAPI current-user scope.
- Move Xtream password and private SKTorrent RSS URL into a separate DPAPI-protected store.
- Migrate 1.5.0 plaintext local state automatically and remove the old plaintext state after a successful migration.
- Stop saving desktop state into browser localStorage; clear the old browser snapshot after a successful protected write.
- Add a version marker to persisted state and compatibility handling for older state payloads.
- Add tests for secret separation, protected storage round-trips and plaintext-state migration.
- Document architecture, storage boundaries, user responsibilities and the remaining SQLite migration.

## AVMCMP+ · TV-first media center

- Rebrand the visible application as AVMCMP+ while retaining the existing WAVE local storage directory and browser storage key for upgrade compatibility.
- Add a central source hub for M3U/M3U8 URLs and files, Xtream Codes, XMLTV EPG, local media, RSS and qBittorrent.
- Introduce responsive desktop / TV, tablet and mobile sizing, clearer focus states and arrow-key spatial navigation for remote-style controls.
- Keep the existing Go local host, player, EPG, library, Xtream, RSS and qBittorrent modules intact.
