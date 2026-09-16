# Security

## What clawshim does

clawshim spawns a configured runtime (`node.exe`) with a configured entry (`openclaw.mjs`) and forwards argv/stdio/exit code. It contains no networking code, downloads nothing at runtime, and stores no credentials.

## Trust model

- **The config file is executable-equivalent input.** It decides which binary and script get launched. Keep it in user-writable locations you control (exe directory or `%APPDATA%`); never share or commit a machine's live `clawshim.json`.
- **Environment injection is additive only.** `env` entries in the config are set only if the parent process has not already defined them.
- **No shell interpolation.** Arguments are passed through CreateProcess directly; there is no `.cmd`/shell involved, so prompt content cannot be interpreted as shell syntax.
- **Version cache** stores the version output and the entry file signature; it is not executable input and is parsed leniently.

## Release integrity

- Release archives are accompanied by `checksums.txt` (SHA256). `scripts/install.ps1` verifies automatically.
- Builds are produced by the public GitHub Actions workflow from the tagged commit; `-trimpath` is used for reproducible paths.

## Reporting

Open a GitHub security advisory (Security → Report a vulnerability) or a regular issue for non-sensitive reports.
