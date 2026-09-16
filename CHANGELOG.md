# Changelog

## v0.2.0 — 2026-09-16

First public release.

- Config-driven resolution: `--shim-config` / `CLAWSHIM_CONFIG` / `clawshim.json` (exe dir) / `%APPDATA%\clawshim\config.json` / built-in autodetection; `%VAR%`, `$VAR`, `${VAR}` expansion.
- Real `--version` probing with an mtime/size-keyed disk cache; `--shim-refresh-version` to force a re-probe.
- Management commands: `--shim-info`, `--shim-doctor`, `--shim-setup` (presets: `npm`, `autoclaw`, `custom`).
- Windows hardening: kill-on-close job object (killing the shim terminates the CLI tree); windowless child when the shim has no console.
- Zero third-party dependencies (standard library only).
- Integration test suite (12 KB / 30 KB args, quoting/unicode round-trips, exit codes, env injection, cache behavior) + CI on windows-latest.
- Install / uninstall scripts; release workflow producing windows/amd64 + arm64 zips and checksums.
- Docs: README (EN + zh-CN), CONFIG, TROUBLESHOOTING, SECURITY.

## v0.1.0 — internal prototype

- Single-file forwarder (hardcoded paths) that first solved Multica + embedded OpenClaw long-prompt failures on one machine. Reference copy: `docs/v0.1/main.go`.
