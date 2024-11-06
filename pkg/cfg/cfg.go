package cfg

import "os"

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
