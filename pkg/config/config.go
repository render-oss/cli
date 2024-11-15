package config

import (
	"errors"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const currentVersion = 1

var defaultConfigPath string

const configPathEnvKey = "RENDER_CLI_CONFIG_PATH"
const workspaceEnvKey = "RENDER_WORKSPACE"

type Config struct {
	Version       int    `yaml:"version"`
	Workspace     string `yaml:"workspace"`
	WorkspaceName string `yaml:"workspace_name"`
	ProjectFilter string `yaml:"project_filter,omitempty"` // Project ID for filtering
	ProjectName   string `yaml:"project_name,omitempty"`   // Project name for display

	APIConfig `yaml:"api"`
}

type APIConfig struct {
	Key  string `yaml:"key,omitempty"`
	Host string `json:"host,omitempty"`
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
	if workspaceID := os.Getenv(workspaceEnvKey); workspaceID != "" {
		return workspaceID, nil
	}

	cfg, err := Load()
	if err != nil {
		return "", err
	}
	if cfg.Workspace == "" {
		return "", errors.New("no workspace set. Use `render workspace` to set a workspace")
	}
	return cfg.Workspace, nil
}

func WorkspaceName() (string, error) {
	if workspaceID := os.Getenv(workspaceEnvKey); workspaceID != "" {
		return workspaceID, nil
	}

	cfg, err := Load()
	if err != nil {
		return "", err
	}
	if cfg.WorkspaceName == "" {
		return "", errors.New("no workspace set. Use `render workspace` to set a workspace")
	}
	return cfg.WorkspaceName, nil
}

func GetProjectFilter() (projectID string, projectName string, err error) {
	cfg, err := Load()
	if err != nil {
		return "", "", err
	}
	return cfg.ProjectFilter, cfg.ProjectName, nil
}

func SetProjectFilter(projectID string, projectName string) error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	cfg.ProjectFilter = projectID
	cfg.ProjectName = projectName
	return cfg.Persist()
}

func ClearProjectFilter() error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	cfg.ProjectFilter = ""
	cfg.ProjectName = ""
	return cfg.Persist()
}

func GetAPIConfig() (APIConfig, error) {
	cfg, err := Load()
	if err != nil {
		return APIConfig{}, err
	}

	return cfg.APIConfig, nil
}

func SetAPIConfig(host, apiKey string) error {
	cfg, err := Load()
	if err != nil {
		return err
	}

	cfg.Host = host
	cfg.Key = apiKey
	return cfg.Persist()
}

func Load() (*Config, error) {
	path, err := expandPath(getConfigPath())
	if err != nil {
		return nil, err
	}

	// Ignore the error if we can't chmod try to continue
	_ = os.Chmod(filepath.Dir(path), 0755)

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

	// Ignore the error if we can't chmod try to continue
	_ = os.Chmod(filepath.Dir(path), 0755)

	err = os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
