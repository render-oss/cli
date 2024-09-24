package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const currentVersion = 1

var defaultConfigPath string

const configPathEnvKey = "RENDER_CLI_CONFIG_PATH"

type Config struct {
	Version   int    `yaml:"version"`
	Workspace string `yaml:"workspace"`
}

func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	defaultConfigPath = filepath.Join(home, ".render", "cli.yaml")
}

func getConfigPath() string {
	if path := os.Getenv(configPathEnvKey); path != "" {
		return path
	}
	return defaultConfigPath
}

func expandPath(path string) (string, error) {
	if path == "~" || len(path) >= 2 && path[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, path[1:])
	}
	return path, nil
}

func WorkspaceID() (string, error) {
	cfg, err := Load()
	if err != nil {
		return "", err
	}
	return cfg.Workspace, nil
}

func Load() (*Config, error) {
	path, err := expandPath(getConfigPath())
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{Version: currentVersion}, nil
		}
		return nil, err
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func (c *Config) Persist() error {
	path, err := expandPath(getConfigPath())
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	err = os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
