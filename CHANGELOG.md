# Changelog

## [2.21.0] - 2026-06-29

### Added

- Key Value commands are now generally available as top-level `render keyvalues` commands, with `render kv` as a shorthand alias
- Postgres commands are now generally available as top-level `render postgres` commands, with `render pg` as a shorthand alias
- `render services delete` command to delete services from the CLI
- `render ea sandbox` commands to create, list, stop, and run commands in sandboxes

### Changed

- Key Value JSON and YAML output now uses consistent `data` and `meta` envelopes with flattened resource fields across `get`, `create`, `list`, `update`, `delete`, `suspend`, and `resume`; `update` output includes a diff of changed fields
- Postgres JSON and YAML output now uses a slimmer resource-focused contract across `get`, `create`, `list`, `update`, `delete`, `suspend`, and `resume`; `update` output includes before/after details for changed fields
- Key Value and Postgres text detail output now includes workspace, project, and environment context when available
- Key Value and Postgres text detail output now explains when an empty IP allow list blocks external connections
- `render ea sandbox exec` now streams stdout and stderr by default and propagates the remote command exit code

### Fixed

- `render services update` now applies supported service updates instead of returning unchanged service data
- Workflow create and task run forms now validate required fields inline in interactive mode
- Updated `go-git` to address upstream security vulnerabilities

## [2.20.0] - 2026-06-04

### Added

- `render ea pg list` command to list Postgres databases in a workspace, project, or environment
- `render ea pg get` command to fetch details of a Postgres database
- `render ea pg update` command to update a Postgres database
- `render ea pg suspend` and `render ea pg resume` commands to suspend and resume a Postgres database
- `render ea pg create` now offers an interactive wizard when run without `--confirm` in interactive output mode

### Changed

- **Breaking:** `render ssh --ephemeral` now accepts `--plan` instead of `--size` to choose the instance type (e.g. `--plan standard`)

## [2.19.0] - 2026-05-28

### Added

- `render ea pg create` command to create a Postgres instance from the CLI
- `render ssh --ephemeral` accepts `--size` to choose the plan for the ephemeral instance (e.g. `--size standard`)

### Changed

- Runtime CLI command errors no longer print command usage/help text after the error

## [2.18.0] - 2026-05-21

### Added

- `render ea kv` commands to create, get, list, update, delete, suspend, and resume Key Values. Key Value itself is generally available. We will promote `kv` out of the `ea` namespace once we stabilize the interface.
- `workflows dev` loads environment variables from a `.env` file in the current directory by default, and supports `--env-file` (repeatable) to load specific files
- Workflow task input field now expands to 15 lines, supports opening in `$EDITOR` via `ctrl+e`, and validates JSON before submission

### Changed

- `render ssh --ephemeral` uses the per-shell id returned by the API as the SSH username, supporting single-use ephemeral shells

### Fixed

- Fixed users being prompted to log in again when authenticated but no active workspace was set

## [2.17.0] - 2026-05-13

### Added

- `render logout` command to revoke the stored OAuth token and clear local CLI credentials/settings

### Changed

- Local `workflows dev` now retries failed task runs according to task retry configuration
- Nest workflow task-run commands under `workflows tasks runs`, with `workflows start` and `workflows cancel` shortcuts for common operations
- Previous workflow task-run commands remain available as hidden deprecated aliases

## [2.16.0] - 2026-04-29

### Added

- `render workflows create` command to create new workflows from the CLI
- `render workflows runs cancel` command to cancel a running task
- `workflows create --repo` accepts a local directory path (e.g., `--repo .`)
- `workflows init` prints the dependency install command when the user skips running it
- `workflows init` next steps include a ready-to-copy `workflows create` command for deploying the scaffolded workflow

### Changed

- Aligned all commands with the new CLI style guide for consistent help text and output formatting
- Cleaner spacing in `objects`, `skills`, `workflows`, and `tasks` list output
- `workflows init` now installs dependencies and initializes a Git repo by default when run with `--confirm` or in non-interactive output (pass `--install-deps=false` or `--git=false` to opt out)
- Style and language tweaks to `workflows init` prompts
- Removed clickable URL from `workflows dev` startup output
- Sorted task list output in the local `workflows dev` server

### Fixed

- Interactive forms (e.g. `services create`) now scroll in terminals shorter than the form
- Fixed form views flashing leftover form data during the loading transition
- Fixed `Enter` submitting forms on first pass (now, advances between fields)
- Fixed forms appearing blank when navigating back after submitting
- Fixed workflow task logs getting cut off at start or end of task execution window

## [2.15.1] - 2026-03-27

### Fixed

- Fixed CLI flag parsing to handle undefined flags gracefully instead of failing
- Fixed a bug where workflow development mode was not properly filtering task runs

## [2.15.0] - 2026-03-23

### Added

- `services update` command to update service configuration (name, branch, image, build/start commands, auto-deploy, and more)
- Input validation for `postgres create` and `postgres update` commands

### Changed

- `workflows init` defaults the Git repository branch to `main`
- `workflows init` now generates `.gitignore` and `.env.example` files when the template doesn't include them

### Fixed

- Fixed object ID validation accepting IDs with extra characters prepended (e.g., `xsrv-...`)

## [2.14.0] - 2026-03-13

### Added

- Support for IP allow list, previews, and additional service fields in `services create`
- `workflows init` command for scaffolding new workflow projects from templates
- Added local workflows task output to local task server logs

### Changed
- Reformatted CLI help output with new visual styles

### Fixed
- Fixed flag parsing to preserve user intent by treating unset flags as nil
- Fixed local workflows task runs not being visible in interactive list

## [2.13.0] - 2026-03-9

### Added

- `services create` command to create services via the CLI

## [2.12.0] - 2026-03-05

### Added

- Support for paginated workflows task run listing
- Handle `succeeded` workflows task run status

### Changed

- Renamed "task identifier" / "task ID" to "task slug" in error messages and help text for workflows

## [2.11.0] - 2026-03-03

### Added

#### Workflows
- `render workflows list` interactive palette for browsing and managing workflows
- Support for named-parameter (object) input for task runs (Python workflows only)

#### Early Access
- `render ea objects delete` supports deleting multiple objects

### Changed

#### General
- Skip auth and workspace selection prompts for `--local` commands

#### Workflows
- **Breaking:** Promoted workflows commands from `render ea` to `render workflows`
- **Breaking:** Moved `taskruns start` to `tasks start`
- **Breaking:** Renamed `taskruns` command to `runs`
- Moved local development `dev` command from `workflows tasks` to `workflows`
- Skip version selection step in interactive task navigation (use most recent version)
- Use compact tables for workflows task and task run lists
- Improved `tasks dev` startup output

### Fixed

#### General
- Show loading spinner in content pane only, keeping header and footer visible

#### Workflows
- Fixed `--wait` on `versions release` to poll until completion
- Fixed `tasks dev` hang when start command is invalid or crashes
- Fixed local task run input display and interactive mode bugs
- Fixed local `taskruns list` when no task id specified or id is a slug
- Fixed local dev server generating UUIDs instead of XIDs for task IDs
- Fixed local dev server logs endpoint returning incorrect response format
- Fixed referencing local dev server tasks by slug only
- Fixed malformed format string in `taskruns show -o text`
- Fixed "service id" error typo when validating TaskRunInput
- Fixed missing parent and root task ids in local task runs
- Fixed local dev server returning task runs with `attempts: null`
- Fixed error message when starting a task run for a nonexistent task in local dev
