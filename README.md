# Render CLI

## Installation

- [Homebrew](https://render.com/docs/cli#homebrew-macos-linux)
- [Direct Download](https://render.com/docs/cli#direct-download)

## Documentation

Documentation is hosted at https://render.com/docs/cli.

## Contributing

To create a new command, use the `cmd/template.go` template file as a starting point. Reference the [CLI Style Guide](docs/STYLE.md) to learn more about command naming, flags, arguments, and help text conventions.

### Dev setup

We use [prek](https://prek.j178.dev/) to run precommit-compatible checks locally and in CI.

- If you didn't install `prek` following the [dev setup guide](https://slab.render.com/posts/dev-setup-guide-ect5drdb), do so now (e.g., `brew install prek`)
- Set up prek git hooks with `prek install`.

Read [AGENTS.md](AGENTS.md) for common dev commands. It's written for humans too!
