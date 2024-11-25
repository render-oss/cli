package cfg

import "os"

const RepoURL = "https://api.github.com/repos/render-oss/cli"
const InstallationInstructionsURL = "https://docs.render.com/cli#1-install"

var Version = "dev"

func GetHost() string {
	if host := os.Getenv("RENDER_HOST"); host != "" {
		return host
	}

	return "https://api.render.com/v1/"
}

func GetAPIKey() string {
	return os.Getenv("RENDER_API_KEY")
}
