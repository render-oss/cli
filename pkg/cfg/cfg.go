package cfg

import "os"

func GetHost() string {
	return os.Getenv("RENDER_HOST")
}

func GetAPIKey() string {
	return os.Getenv("RENDER_API_KEY")
}
