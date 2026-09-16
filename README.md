# clawshim

A tiny native Windows forwarder that lets the [OpenClaw](https://github.com/openclaw/openclaw) CLI run with **long command lines** — past the `cmd.exe` 8191-character limit that breaks npm-style `.cmd` shims.

> **Scope notice**
> clawshim is primarily built for, and tested against, the OpenClaw **bundled with AutoClaw** (embedded deployment).
> Other setups — an npm-installed OpenClaw, a standalone install, another Node CLI — follow the same design and may work, but they have **not** been verified by the maintainer. **Test on your own before relying on it.**
>
> 中文说明见 [README.zh-CN.md](README.zh-CN.md)。

---

## The problem it solves

On Windows, `openclaw` installed via npm is a `.cmd` batch shim. Everything that launches it — including orchestrators like [Multica](https://github.com/multica-ai/multica) — goes through `cmd.exe`, whose command line caps at **8191 characters**. When a task prompt (system prompt + issue context) is passed via `--message`, tasks fail with:

```
The command line is too long.
openclaw returned no parseable output
```

clawshim is a real PE executable. It spawns `node.exe` + `openclaw.mjs` **directly through CreateProcess** — no `cmd.exe` anywhere — and passes argv, stdin/stdout/stderr, and the exit code through untouched. The effective command-line limit becomes CreateProcess's **32767 characters**.

```
orchestrator ──spawn──▶ openclaw.exe (clawshim) ──CreateProcess──▶ node.exe ──▶ openclaw.mjs
                         argv/stdio/exit-code passthrough, env injection
```

Same idea as npm's shim, minus the cmd.exe middleman — plus a few things a plain wrapper lacks:

- **Config-driven** — works with any layout via `clawshim.json` (no hardcoded paths).
- **Real `--version`, cached** — daemons probe versions frequently; clawshim answers from a live first probe plus a disk cache keyed by the entry file's mtime/size, so upgrades invalidate it automatically.
- **Kill-on-close job object** — if the shim is force-killed (task cancel/timeout), the CLI process tree terminates with it.
- **`--shim-doctor`** — one command to check runtime, entry, version probe and cache status.

## Install

### Option A — AutoClaw users (primary target)

```powershell
# 1. Get a build (or build from source, see below)
# 2. Put openclaw.exe somewhere stable, e.g. %LOCALAPPDATA%\clawshim\bin
# 3. Create the config (preset fills the known AutoClaw layout):
openclaw.exe --shim-setup --preset autoclaw --autoclaw-root "D:\AI\AutoClaw"
# 4. Verify
openclaw.exe --shim-doctor
```

### Option B — npm-installed OpenClaw

```powershell
openclaw.exe --shim-setup --preset npm
openclaw.exe --shim-doctor
```

### Release download

```powershell
pwsh -File scripts/install.ps1 -Preset autoclaw -AutoClawRoot "D:\AI\AutoClaw"
```

If you just want the file: grab `clawshim_<ver>_windows_amd64.zip` from Releases, put `openclaw.exe` on `PATH` (and `clawshim.json` next to it).

## Wiring into Multica

Two ways. Use whichever fits your setup:

**Env method (recommended, most explicit):**

```powershell
[Environment]::SetEnvironmentVariable('MULTICA_OPENCLAW_PATH', 'C:\path\to\openclaw.exe', 'User')
multica daemon restart
```

**PATH method:** put the shim's directory on `PATH` **before** `%APPDATA%\npm` (or before any other `openclaw`), then `multica daemon restart`.

Verify in the daemon log — you should see the shim adopted with a real version:

```
INF adopted resolved agent executable provider=openclaw command=openclaw
    new_path=...\openclaw.exe version="OpenClaw 2026.9.x"
```

## Configuration

Resolution order (first hit wins):

1. `--shim-config <path>` argument
2. `CLAWSHIM_CONFIG` environment variable
3. `clawshim.json` next to `openclaw.exe`
4. `%APPDATA%\clawshim\config.json`
5. Built-in autodetection (runtime `node`, npm global layout)

```jsonc
{
  "runtime": "node",                                   // "node" (PATH) or full path to node.exe
  "entry": "%APPDATA%\\npm\\node_modules\\openclaw\\openclaw.mjs",
  "env": { "OPENCLAW_STATE_DIR": "C:\\Users\\me\\.openclaw-autoclaw" },
  "versionCache": { "enabled": true },                 // optional; enabled by default
  "logFile": ""                                        // optional debug log (never stdout)
}
```

`%VAR%`, `$VAR` and `${VAR}` are expanded in all path values. Extra `env` entries are set **only if not already present** — the caller's environment always wins. See [docs/CONFIG.md](docs/CONFIG.md) for the full reference.

## Commands

| Command | Purpose |
|---|---|
| `openclaw <args…>` | Forward to the real CLI. Everything passes through. |
| `openclaw --version` | Live probe + cache (daemons probe this often; cached after first call). |
| `openclaw --shim-info` | Show resolved config: runtime, entry, env, cache state. |
| `openclaw --shim-doctor` | Diagnose: runtime reachable, entry exists, version probe, cache status. |
| `openclaw --shim-setup` | Write a `clawshim.json`: `--preset npm \| autoclaw \| custom`. |
| `openclaw --shim-refresh-version` | Force a version re-probe (after upgrading OpenClaw). |

Only a first argument starting with `--shim` is intercepted; everything else, including future OpenClaw flags, is forwarded.

## Build from source

```bash
GOOS=windows GOARCH=amd64 go build -trimpath \
  -ldflags "-s -w -X github.com/KleinM2025/clawshim/internal/buildinfo.Version=v0.2.0" \
  -o dist/openclaw.exe ./cmd/clawshim
```

No third-party dependencies (standard library only), so builds are auditable and reproducible.

## Tests

```bash
go test ./...                       # unit tests (any OS)
pwsh -File scripts/test-integration.ps1   # integration suite (Windows + node)
```

The integration suite exercises the exact failure this tool exists for: 12 KB and 30 KB arguments, quoting/backslash/CJK/emoji round-trips, exit-code passthrough, env injection, version probe + cache behavior, and stdout purity.

## FAQ

**Is this an OpenClaw product?** No. It's an independent community workaround, not affiliated with OpenClaw, AutoClaw, or Multica.

**Why not just wait for an upstream fix?** Multica issue [#6032](https://github.com/multica-ai/multica/issues/6032) tracks the `.cmd`-shim variant of this problem (a related fix PR was closed unmerged). clawshim works regardless of upstream, and also covers embedded deployments where no `openclaw` entry exists on `PATH` at all.

**What's the real limit?** 32767 characters for the whole command line (a Windows CreateProcess limit that no executable can exceed). If your prompts routinely exceed ~30 KB, the transport itself needs to change (stdin/file) — that's an orchestrator-side fix.

**Does it need admin rights?** No. User-level install and user-level config.

**How do I uninstall?** Delete `openclaw.exe` + `clawshim.json`, remove any `MULTICA_OPENCLAW_PATH`, `multica daemon restart`. `scripts/uninstall.ps1` does it for you.

## License

MIT — see [LICENSE](LICENSE).
