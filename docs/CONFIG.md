# Configuration reference

clawshim resolves its config from the first available source, in this order:

| # | Source | Notes |
|---|---|---|
| 1 | `--shim-config <path>` first argument | explicit override, mainly for testing |
| 2 | `CLAWSHIM_CONFIG` environment variable | path to a config file |
| 3 | `clawshim.json` next to the executable | portable mode — recommended |
| 4 | `%APPDATA%\clawshim\config.json` | global mode |
| 5 | Built-in autodetection | runtime `node`; entry from the npm global layout |

A missing or unparseable explicit config is reported (`--shim-info`, `--shim-doctor`) but never hard-fails the loader; problems surface via `Validate` before execution.

## Fields

| Field | Type | Default | Description |
|---|---|---|---|
| `runtime` | string | `"node"` | Node.js executable. A bare name is resolved through `PATH` on every launch; a path containing `/` or `\` is used as-is (expanded) and must exist. |
| `entry` | string | autodetected | Path to `openclaw.mjs`. `.cmd`/`.bat` entries are rejected (that's the shim problem this tool eliminates). Required for execution. |
| `env` | object | `{}` | Extra environment variables for the child process. Each entry is set **only if not already present** in the parent environment. Values support variable expansion. |
| `versionCache.enabled` | bool | `true` | Enable the `--version` disk cache. |
| `versionCache.path` | string | `%LOCALAPPDATA%\clawshim\version-cache.json` | Cache file location. |
| `versionCache.maxAge` | string | unset | Optional Go duration (e.g. `"720h"`). When set, entries older than maxAge re-probe regardless of file signature. |
| `logFile` | string | unset | Optional debug log path. `CLAWSHIM_DEBUG=1` uses the default path (`%LOCALAPPDATA%\clawshim\debug.log`). Logs never touch stdout. |

All path values support `%VAR%` (Windows style), `$VAR` and `${VAR}` (shell style) expansion. Unset variables are left literal.

## Examples

### AutoClaw embedded (primary target)

```json
{
  "runtime": "D:\\AI\\AutoClaw\\resources\\node\\node.exe",
  "entry": "D:\\AI\\AutoClaw\\resources\\gateway\\openclaw\\openclaw.mjs",
  "env": { "OPENCLAW_STATE_DIR": "C:\\Users\\you\\.openclaw-autoclaw" }
}
```

Generate it: `openclaw.exe --shim-setup --preset autoclaw --autoclaw-root "D:\AI\AutoClaw"`

### npm-installed OpenClaw

```json
{
  "runtime": "node",
  "entry": "%APPDATA%\\npm\\node_modules\\openclaw\\openclaw.mjs"
}
```

Generate it: `openclaw.exe --shim-setup --preset npm`

### Custom layout

```json
{
  "runtime": "C:\\nodejs\\node.exe",
  "entry": "D:\\tools\\openclaw\\openclaw.mjs",
  "env": { "OPENCLAW_STATE_DIR": "D:\\state\\openclaw" }
}
```

Generate it: `openclaw.exe --shim-setup --preset custom --runtime ... --entry ... [--state-dir ...]`

## Version cache

Keyed by (`entry` path, file mtime, file size). Any OpenClaw upgrade touches the file and the next `--version` call re-probes once, then caches again. Manual refresh: `--shim-refresh-version`. Disable entirely with `"versionCache": { "enabled": false }` (every probe becomes live; only do this if you distrust the cache).

## Security notes

- The config file is a trust boundary: it decides which executable and which `openclaw.mjs` get launched. Keep it in user-writable locations only.
- The shim performs no shell interpolation, downloads nothing at runtime, and makes no network calls of its own.
- `env` injection never overrides variables the caller already set.
