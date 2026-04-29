# Changelog

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
