# Troubleshooting

Run `openclaw.exe --shim-doctor` first — it checks runtime resolution, entry existence, a live version probe, and cache state, and prints actionable hints.

## "openclaw returned no parseable output" / "The command line is too long"

The orchestrator is still reaching a `.cmd` shim, not clawshim.

1. Confirm what the daemon resolved — daemon log should show:
   `INF adopted resolved agent executable ... new_path=...\openclaw.exe`
   If it shows a `.cmd` path, fix resolution:
   - PATH method: put the shim's directory **before** `%APPDATA%\npm`.
   - Or set `MULTICA_OPENCLAW_PATH` to the shim and `multica daemon restart`.
2. `multica daemon restart` after ANY path change (resolution is cached at startup and periodically).

## "entry not found" / "runtime not found"

The config points at something that no longer exists (moved install, renamed user folder).

- Re-run `--shim-setup` with the right root, or edit `clawshim.json` by hand.
- Check `--shim-info` to see which config file is actually in use and where it came from.

## Version reported as too old / "below the minimum supported version"

The version cache may be stale after a manual openclaw swap that preserved file timestamps:

```
openclaw.exe --shim-refresh-version
```

## Tasks hang when cancelled / process tree lingers

clawshim runs the CLI inside a kill-on-close job object; killing the shim terminates the tree. If you still see orphans, the daemon is killing something else entirely (check it resolved the shim, not another wrapper). Combine with daemon-side `multica daemon restart` to reset resolution.

## Console window flashes

The shim launches the child windowless when the shim itself has no console. A flash usually means a *different* wrapper (e.g. npm's `.cmd`) is still in the chain.

## Long prompts above ~30 KB still fail

That's the CreateProcess 32767-character ceiling — a Windows platform limit, not a shim bug. The transport has to change (stdin or a temp file) on the orchestrator side. For Multica this is tracked upstream (issue #6032); prompts in the normal 6–15 KB range are the supported envelope.

## I want to see exactly what happens

```
set CLAWSHIM_DEBUG=1
openclaw.exe --shim-info        & rem shows resolved config
openclaw.exe --shim-doctor      & rem full live checks
rem debug log: %LOCALAPPDATA%\clawshim\debug.log
```

Logs contain argument counts and lengths (never prompt content), config source, probe results, exit codes and durations.

## Rollback

Delete `openclaw.exe` + `clawshim.json` from wherever you installed them; remove `MULTICA_OPENCLAW_PATH` if set; `multica daemon restart`. See `scripts/uninstall.ps1`.
