# Repository Guidelines

> **Last verified**: 2026-02-10

Essential guidance for AI coding agents working in the Render CLI.

## Project Overview

**What**: Terminal interface for Render platform (Go + Bubble Tea TUI)
**Why**: Manage deployments, services, logs, and infrastructure from the command line

## Before You Code

1. Search existing code for similar implementations
2. Follow established patterns in `pkg/tui/`
3. Check if types exist in `pkg/client/` before defining new ones

## Boundaries

**Never do (without explicit approval)**:
- Edit files in `pkg/client/` (generated code)
- Modify `*_gen.go` files
- Remove existing tests
- Change navigation stack architecture in `pkg/tui/stack.go`

**Always ask first**:
- Major TUI architectural changes
- Adding new dependencies
- Changes to command flag APIs (breaking changes)

**Safe to do**:
- Add new views in `pkg/tui/views/`
- Add new commands in `cmd/`
- Write tests anywhere
- Refactor within a single package

## When Uncertain

- **Requirements unclear**: Ask before implementing
- **Architecture questions**: Propose minimal approach, get feedback
- **TUI interactions**: Describe intended flow, get confirmation

Prefer small, incremental changes over large speculative implementations.

## Essential Commands

```bash
# Building
go build -o render .              # Build binary

# Testing
go test ./...                     # All tests
go test -run TestName ./pkg/...   # Single test

# Linting & Formatting
golangci-lint run                 # Lint
prek run --all-files              # All hooks (see prek.toml)

# Type Generation (from public-api-schema)
export RENDER_API_PATH=/path/to/api
cd ../public-api-schema && ./generate-cli.sh
```

## Project Structure

- `cmd/` - Cobra command definitions
- `pkg/client/` - **Generated** API client (READ-ONLY)
- `pkg/tui/` - Bubble Tea TUI framework ([see AGENTS.md](pkg/tui/AGENTS.md))
- `pkg/config/` - User config file (`~/.render/cli.yaml`)
- `pkg/cfg/` - Environment defaults (different from config!)
- `pkg/command/` - Output formats, context utilities
- `pkg/dependencies/` - Dependency injection container
- `pkg/style/` - Lipgloss styling system

## Key Patterns

**TUI**: Elm Architecture (Message → Update → View). See [pkg/tui/AGENTS.md](pkg/tui/AGENTS.md).

**Data Access**: Service → Repo → Client (three-layer architecture):
- **Client** (`pkg/client/`): Generated HTTP client - never edit
- **Repo** (`pkg/*/repo.go`): Wraps client, handles pagination & error parsing
- **Service** (`pkg/*/service.go`): Business logic, orchestrates multiple repos, enriches data

```go
// Service combines data from multiple repos
svc, _ := s.repo.ListServices(ctx, params)    // calls client internally
proj, _ := s.projectRepo.ListProjects(ctx)    // different repo
return s.enrich(svc, proj)                    // business logic
```

**Output formats**: Use `command.IsInteractive(ctx)` for branching:
```go
if command.IsInteractive(ctx) {
    return runTUI(deps)
}
return runNonInteractive(ctx, deps)  // JSON, YAML, TEXT
```

**Naming**:
| Element | Pattern | Example |
|---------|---------|---------|
| Commands | `New{Action}Cmd()` | `NewServiceListCmd()` |
| Models | `{Entity}Model` | `ServiceListModel` |
| Messages | `{Action}Msg` | `LoadServicesMsg` |

## Testing Against Local Dev API

```bash
# Start API
tilt up api
curl -k https://api.localhost.render.com:8443/health

# Configure CLI
export RENDER_HOST="https://api.localhost.render.com:8443/v1/"
export RENDER_API_KEY="your-api-key"
./render services list
```

| Variable | Description |
|----------|-------------|
| `RENDER_HOST` | API base URL |
| `RENDER_API_KEY` | API key (skips OAuth) |
| `RENDER_WORKSPACE` | Workspace ID override |
| `RENDER_CLI_CONFIG_PATH` | Config file path override |

## Testing

- **Table-driven tests** with `stretchr/testify`
- **Manual fakes** in `pkg/tui/testhelper/`
- **Hooks** (`prek.toml`): golangci-lint, shellcheck, shfmt, yaml checks, large file detection

## Common Gotchas

- Don't block in `Update()` - use `tea.Cmd` for async work
- Handle `tea.KeyCtrlC` and `tea.KeyCtrlD` for proper exit
- Run `reset` if terminal breaks after a crash
- `Push()` returns a `tea.Cmd` that must be returned from `Update()`
- Generated types are read-only - regenerate via `generate-cli.sh`

## Package-Specific Guides

- `pkg/tui/`: [See AGENTS.md](pkg/tui/AGENTS.md) for Bubble Tea patterns and examples

## Reference

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) | [Cobra](https://github.com/spf13/cobra) | [Render CLI Docs](https://render.com/docs/cli)
