package types

import (
	"github.com/render-oss/cli/pkg/client"
)

type ServiceCreateInput struct {
	// Common fields
	Name    string               `cli:"name"`
	Type    client.ServiceType
	Repo    string               `cli:"repo"`
	Branch  string               `cli:"branch"`
	RootDir string               `cli:"root-dir"`

	// Static site specific
	PublishPath string `cli:"publish-path,omitempty"`

	// Server specific (web, private, worker)
	BuildCommand  string `cli:"build-command,omitempty"`
	StartCommand  string `cli:"start-command,omitempty"`
	Runtime       string `cli:"runtime,omitempty"`
	Env           string `cli:"env,omitempty"` // deprecated
	Plan          string `cli:"plan,omitempty"`
	NumInstances  int    `cli:"num-instances,omitempty"`
}

func (s ServiceCreateInput) String() []string {
	return []string{}
}