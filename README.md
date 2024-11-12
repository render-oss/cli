# Render CLI

This is the beta version of the Render CLI.

# Getting Started

## Installation

### Homebrew (Recommended for MacOS)

You can install the Render CLI using Homebrew by running the following commands:

```sh
brew tap render-oss/homebrew-render
brew update
brew install render
```

### Building from source

To build the Render CLI from source, you will need to have Go installed on your machine. You can install Go by following the [Go installation instructions](https://golang.org/doc/install).

Once you have Go installed, you can build the Render CLI by running the following commands:

```sh
git clone git@github.com:render-oss/cli.git
cd cli
go build -o render 
```

This will create a binary called `render` in the current directory. You can move this binary to a directory in your `PATH` to make it easier to use.

### Downloading a pre-built binary

Pre-built binaries for the Render CLI are available on the [Releases page](https://github.com/render-oss/cli/releases/) of this repository. You can download the binary for your platform and move it to a directory in your `PATH` to make it easier to use.

## Configuration

The CLI expects an API key to be set in the `RENDER_API_KEY` environment variable. You can generate an API key from your [user settings page](https://dashboard.render.com/u/settings#api-keys).

