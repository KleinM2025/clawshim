# clawshim

一个轻量的 Windows 原生转发器：让 [OpenClaw](https://github.com/openclaw/openclaw) CLI 摆脱 `cmd.exe` 8191 字符命令行上限，npm 风格 `.cmd` shim 长 prompt 必挂的问题从此消失。

> **适配范围说明（重要）**
> 本工具主要适配 **AutoClaw 内置的 OpenClaw**（内嵌部署），并以该场景为主要测试对象。
> 其他安装形态——npm 全局安装、独立部署、其他 Node CLI——原理相同、设计兼容，但**未经维护者实测，需要自行测试**后再投入使用。
>
> English: [README.md](README.md)

---

## 它解决什么问题

在 Windows 上，npm 安装的 `openclaw` 是一个 `.cmd` 批处理壳。任何调用它的程序（例如 [Multica](https://github.com/multica-ai/multica)）都要经过 `cmd.exe`，而 cmd.exe 的命令行上限是 **8191 字符**。当任务提示词（系统提示 + issue 上下文）通过 `--message` 传入时，任务会失败：

```
The command line is too long.
openclaw returned no parseable output
```

clawshim 是一个真正的 PE 可执行文件。它通过 CreateProcess **直接启动** `node.exe` + `openclaw.mjs`——全程没有 `cmd.exe`——argv、stdin/stdout/stderr、退出码原样透传。实际上限变为 CreateProcess 的 **32767 字符**。

```
调度器 ──spawn──▶ openclaw.exe (clawshim) ──CreateProcess──▶ node.exe ──▶ openclaw.mjs
                  argv/stdio/退出码直通 + 环境变量注入
```

思路与 npm 的 shim 相同，只是去掉了 cmd.exe 中间层，并额外提供：

- **配置驱动**——`clawshim.json` 适配任意布局（零硬编码路径）；
- **真实 `--version` + 磁盘缓存**——守护进程会频繁探测版本；首次真探测后按 entry 文件 mtime/size 签名缓存，OpenClaw 升级自动失效；
- **Job Object 进程树守护**——shim 被强杀（任务取消/超时）时，CLI 进程树一并终止；
- **`--shim-doctor`**——一条命令体检运行时、入口、版本探测与缓存状态。

## 安装

### 方式 A —— AutoClaw 用户（主要目标场景）

```powershell
# 1. 获取构建产物（或从源码构建，见下文）
# 2. 把 openclaw.exe 放到固定位置，例如 %LOCALAPPDATA%\clawshim\bin
# 3. 生成配置（preset 会填入已知的 AutoClaw 布局）：
openclaw.exe --shim-setup --preset autoclaw --autoclaw-root "D:\AI\AutoClaw"
# 4. 验证
openclaw.exe --shim-doctor
```

### 方式 B —— npm 安装的 OpenClaw

```powershell
openclaw.exe --shim-setup --preset npm
openclaw.exe --shim-doctor
```

### 从 Release 安装

```powershell
pwsh -File scripts/install.ps1 -Preset autoclaw -AutoClawRoot "D:\AI\AutoClaw"
```

只想手动处理：从 Releases 下载 `clawshim_<ver>_windows_amd64.zip`，把 `openclaw.exe` 放上 `PATH`（`clawshim.json` 放在旁边）。

## 接入 Multica

两种方式，任选：

**环境变量法（推荐，最明确）：**

```powershell
[Environment]::SetEnvironmentVariable('MULTICA_OPENCLAW_PATH', 'C:\path\to\openclaw.exe', 'User')
multica daemon restart
```

**PATH 顶替法：** 把 shim 所在目录放到 `%APPDATA%\npm` **之前**（或任何其他 `openclaw` 之前），然后 `multica daemon restart`。

在 daemon 日志中验证——应看到 shim 被采纳并报告真实版本：

```
INF adopted resolved agent executable provider=openclaw command=openclaw
    new_path=...\openclaw.exe version="OpenClaw 2026.9.x"
```

## 配置

查找顺序（先命中先用）：

1. `--shim-config <path>` 参数
2. `CLAWSHIM_CONFIG` 环境变量
3. `openclaw.exe` 同目录的 `clawshim.json`
4. `%APPDATA%\clawshim\config.json`
5. 内置自动探测（runtime `node`、npm 全局布局）

```jsonc
{
  "runtime": "node",                                   // "node"（走 PATH）或 node.exe 完整路径
  "entry": "%APPDATA%\\npm\\node_modules\\openclaw\\openclaw.mjs",
  "env": { "OPENCLAW_STATE_DIR": "C:\\Users\\me\\.openclaw-autoclaw" },
  "versionCache": { "enabled": true },                 // 可选；默认启用
  "logFile": ""                                        // 可选调试日志（绝不写 stdout）
}
```

所有路径值支持 `%VAR%`、`$VAR`、`${VAR}` 展开。`env` 条目**仅在不存在时注入**——调用方的环境变量优先。完整参考见 [docs/CONFIG.md](docs/CONFIG.md)。

## 命令

| 命令 | 用途 |
|---|---|
| `openclaw <args…>` | 转发给真实 CLI，一切原样透传 |
| `openclaw --version` | 真实探测 + 缓存（守护进程频繁探测，首次后走缓存） |
| `openclaw --shim-info` | 显示解析后的配置：runtime、entry、env、缓存状态 |
| `openclaw --shim-doctor` | 体检：运行时可达、入口存在、版本探测、缓存状态 |
| `openclaw --shim-setup` | 生成 `clawshim.json`：`--preset npm \| autoclaw \| custom` |
| `openclaw --shim-refresh-version` | 强制重新探测版本（升级 OpenClaw 后用） |

仅当**第一个参数**以 `--shim` 开头时才由 shim 拦截；其余一切（包括 OpenClaw 未来新增的参数）全部转发。

## 从源码构建

```bash
GOOS=windows GOARCH=amd64 go build -trimpath \
  -ldflags "-s -w -X github.com/KleinM2025/clawshim/internal/buildinfo.Version=v0.2.0" \
  -o dist/openclaw.exe ./cmd/clawshim
```

零第三方依赖（纯标准库），构建可审计、可复现。

## 测试

```bash
go test ./...                              # 单元测试（任意系统）
pwsh -File scripts/test-integration.ps1    # 集成测试（Windows + node）
```

集成测试覆盖本工具存在的意义所在：12KB / 30KB 参数、引号/反斜杠/中文/emoji 往返、退出码透传、环境变量注入、版本探测与缓存、stdout 纯净度。

## FAQ

**这是 OpenClaw 官方产品吗？** 不是。这是独立的社区方案，与 OpenClaw、AutoClaw、Multica 均无隶属关系。

**为什么不直接等上游修复？** Multica 的 issue [#6032](https://github.com/multica-ai/multica/issues/6032) 跟踪 `.cmd` shim 变种问题（相关修复 PR 已关闭未合并）。clawshim 不依赖上游进度，并且覆盖内嵌部署这种 `PATH` 上根本没有 `openclaw` 入口的场景。

**真正的上限是多少？** 整条命令行 32767 字符（Windows CreateProcess 的硬限制，任何可执行文件都绕不过）。如果你的 prompt 经常超过 ~30KB，需要改传输方式（stdin/文件）——那是调度器侧的事。

**需要管理员权限吗？** 不需要。用户级安装、用户级配置。

**怎么卸载？** 删除 `openclaw.exe` + `clawshim.json`，清掉 `MULTICA_OPENCLAW_PATH`，`multica daemon restart`。也可以跑 `scripts/uninstall.ps1`。

## 许可证

MIT——见 [LICENSE](LICENSE)。
