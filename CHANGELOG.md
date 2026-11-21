# Changelog

## 1.6.0 · Research implementation

- Encrypt the complete local desktop state with Windows DPAPI current-user scope.
- Move Xtream password and private SKTorrent RSS URL into a separate DPAPI-protected store.
- Migrate 1.5.0 plaintext local state automatically and remove the old plaintext state after a successful migration.
- Stop saving desktop state into browser localStorage; clear the old browser snapshot after a successful protected write.
- Add a version marker to persisted state and compatibility handling for older state payloads.
- Add tests for secret separation, protected storage round-trips and plaintext-state migration.
- Document architecture, storage boundaries, user responsibilities and the remaining SQLite migration.
