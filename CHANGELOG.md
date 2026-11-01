# Changelog

All notable changes to this project appear in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- Added champion and position autodetection through LCU champ select session reads.
- Added detected selection fields to `SyncResult` (`DetectedChampionID`, `DetectedPosition`, `DetectedQueueID`).
- Added item set apply flow with managed set upsert in LCU.
- Added staged item set block apply in LCU with ordered block support.
- Added summoner spell apply flow with Flash slot preservation behavior.
- Added rune page apply flow that reuses an `AutoBuild` page or creates one in LCU.
- Added watch orchestration (`Service.Watch`) and `lol-autobuild watch` command.
- Added LCU websocket event stream support with reconnect flow.
- Added browser-assisted Coachless token capture.
- Added persisted `sync` config settings for the local UI.
- Added local settings UI served from `127.0.0.1`.
- Added LCU connection status checks for the local UI.

### Changed

- `SyncRequest` no longer accepts manual champion or position input. Sync now relies on LCU detection.
- `ApplyItemSetRequest` now accepts staged `Blocks` and no longer accepts legacy flat `ItemIDs`.
- `lcu.enabled` now gates champion and position detection plus apply operations.
- LCU connection discovery now checks League process arguments first, then falls back to `lcu.lockfile_path`.
- Position detection now supports queue IDs `400`, `420`, `440`, and `3110`.
- Sync item recommendations now come from a single Coachless `GetRecommendedBuilds` call. The applied item set mirrors the site's `Starter` / `Core build` / `Late Game` blocks (grouped labels like `Starter | Early Game`, `Core Build & Boots (In Order)`, `3rd Item`, `Late Game`, with ` (Choose One)` suffixes) instead of stat-derived blocks.
- Runes and summoner spells now come from the same `GetRecommendedBuilds` response (builds[0], S-tier first). The recommend engine and its `recommendation.*` config keys are gone.
- `lol-autobuild sync` and `lol-autobuild watch` read `sync.dry_run` when callers omit the flag.
- `lol-autobuild watch` no longer syncs at startup. It syncs once per champ select when the session timer enters `FINALIZATION`.
- Free Coachless users now default to the latest non-Premium patch and cannot request the newest Premium patch.
- The default command now opens the local settings UI.
- Release archives now use the `lol-autobuild` binary name and include README, license, and sample config.

### Removed

- Removed `--champion-id` CLI flag.
- Removed `--role` CLI flag.
- Removed the `recommendation` config section (`min_occurrence`, `top_items`, `top_spells`) and the local recommendation engine; Coachless `GetRecommendedBuilds` now provides curated builds directly.

### Fixed

- LCU connection discovery now also offers the League process' own lockfile when process arguments cannot be parsed (some Windows setups return the caller's own command line for every process).
