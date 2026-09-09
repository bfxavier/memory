# Memory

Local-first persistent memory for Codex and Claude Code.

Memory captures bounded agent lifecycle events, stores durable memories in SQLite, and exposes recall through hooks, a CLI, and MCP. Hooks never call a model or the network and always fail open.

## Current scope

The first working slice provides:

- native Windows, macOS, and Linux binaries
- Codex and Claude Code hook packages
- atomic event spooling and a native per-user worker
- SQLite WAL storage with FTS5 search
- explicit remember, search, inspect, correct, forget, recent, and export commands
- a stdio MCP server exposing the same memory operations
- secret redaction, payload clipping, and fail-open hooks
- automatic checkpoint extraction using the logged-in Codex or Claude subscription
- automatic supersession of contradicted project memories
- bounded active-memory digests on session and subagent start
- optional local or remote OpenAI-compatible extraction

Hooks never call a model. The background worker creates one durable job per stop, interrupt, session end, compaction, or subagent stop checkpoint.

## Build

```sh
go build -o bin/memory ./cmd/memory
go test ./...
```

Put the resulting binary on `PATH`. On Windows, build `bin/memory.exe` instead.

Initialize storage and install the background worker:

```sh
memory init
memory service install
memory doctor
```

The worker uses LaunchAgent on macOS, a systemd user service on Linux, and Task Scheduler on Windows. Install it only after the binary is in its permanent location.

## Automatic extraction

Host extraction is enabled by default. Codex events run through `gpt-5.6-luna` using the logged-in Codex subscription. Claude events run through `haiku` using the logged-in Claude subscription. Each extraction is an isolated non-interactive process with customizations suppressed, tools disabled, session persistence disabled, and a recursion guard on memory hooks.

Host extraction consumes the corresponding subscription quota. Change its models or executable paths with:

```sh
memory provider host \
  --codex-model gpt-5.6-luna \
  --claude-model haiku
memory provider status
```

For Ollama or another local OpenAI-compatible server on `127.0.0.1:11434`:

```sh
memory provider configure --model <installed-model>
memory provider status
```

Defaults cap extraction at 12 jobs per hour, 48 KiB of event input, 2,048 output tokens, and eight memories per checkpoint. Every job retries at most five times.

Each extraction compares new events with up to 64 active memories from the same project. A direct replacement or contradiction creates a new active memory and atomically marks the old memory as superseded. Superseded and retracted memories remain available for audit but are excluded from normal search and hook recall.

Session and subagent start hooks inject a grouped digest of up to 12 active project memories. A successful SessionStart lookup with no project history injects an explicit empty-project marker. Prompt hooks use FTS5 to inject up to six query-relevant active memories. Both paths are local, bounded to 6,000 bytes, and make no model or network call.

For another OpenAI-compatible endpoint:

```sh
export MEMORY_PROVIDER_API_KEY=<key>
memory provider configure \
  --url https://provider.example/v1 \
  --model <model> \
  --api-key-env MEMORY_PROVIDER_API_KEY
```

The API key is never written to the config file. A background service must receive the named environment variable through its native service environment. Disable extraction without deleting captured events:

```sh
memory provider disable
```

After fixing a provider or model configuration, requeue permanently failed jobs with `memory provider retry`.

## Codex

From this repository root:

```sh
codex plugin marketplace add .
codex plugin add memory@memory-local
```

Start a new Codex session after installation. The plugin registers lifecycle hooks, the `memory` MCP server, and the usage skill.

## Claude Code

From this repository root:

```sh
claude plugin marketplace add .
claude plugin install memory@memory-local
```

Start a new Claude Code session after installation.

## CLI

```sh
memory remember "Use pnpm in this repository"
memory search pnpm
memory recent
memory inspect <id>
memory correct <id> "Use pnpm 10 in this repository"
memory forget <id>
memory export
```

Use `--global` with `remember` for preferences shared across projects. Project identity is derived from the canonical Git remote when present, so worktrees share memory.

Use an isolated data directory during development:

```sh
export MEMORY_HOME="$PWD/.tmp-memory"
bin/memory init
bin/memory remember "Use pnpm in this repository"
bin/memory search pnpm
```

PowerShell:

```powershell
$env:MEMORY_HOME = "$PWD\.tmp-memory"
bin\memory.exe init
```

## Data locations

- Windows: `%LOCALAPPDATA%\memory`
- macOS: `~/Library/Application Support/memory`
- Linux: `$XDG_DATA_HOME/memory` or `~/.local/share/memory`

See [PLAN.md](PLAN.md) for the product and delivery plan.

## Repository layout

- `cmd/memory`: CLI dispatch and command implementations
- `internal/store`: SQLite schema, events, extraction jobs, memory writes, search, and health checks
- `internal/extractor`: provider clients, structured-output validation, and bounded prompt rendering
- `internal/hook`: lifecycle capture, sanitization, and local context injection
- `internal/worker`: spool consumption and background extraction orchestration
- `internal/mcpserver`: MCP transport, tool registration, and API views
- `internal/service`: native per-user service integration
- `integrations`: Codex and Claude plugin packages
