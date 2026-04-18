package service

import (
	"fmt"
	"strings"

	types "github.com/render-oss/cli/v2/pkg/types"
)

type ServiceRuntime string

const (
	ServiceRuntimeDocker ServiceRuntime = "docker"
	ServiceRuntimeElixir ServiceRuntime = "elixir"
	ServiceRuntimeGo     ServiceRuntime = "go"
	ServiceRuntimeImage  ServiceRuntime = "image"
	ServiceRuntimeNode   ServiceRuntime = "node"
	ServiceRuntimePython ServiceRuntime = "python"
	ServiceRuntimeRuby   ServiceRuntime = "ruby"
	ServiceRuntimeRust   ServiceRuntime = "rust"
)

var serviceRuntimeValues = []ServiceRuntime{
	ServiceRuntimeDocker,
	ServiceRuntimeElixir,
	ServiceRuntimeGo,
	ServiceRuntimeImage,
	ServiceRuntimeNode,
	ServiceRuntimePython,
	ServiceRuntimeRuby,
	ServiceRuntimeRust,
}

func ServiceRuntimeValues() []string {
	values := make([]string, 0, len(serviceRuntimeValues))
	for _, value := range serviceRuntimeValues {
		values = append(values, string(value))
	}
	return values
}

func ParseServiceRuntime(value string) (ServiceRuntime, error) {
	normalized := strings.TrimSpace(value)
	for _, runtime := range serviceRuntimeValues {
		if normalized == string(runtime) {
			return runtime, nil
		}
	}

	return "", fmt.Errorf("runtime must be one of: %s", strings.Join(ServiceRuntimeValues(), ", "))
}

func OptionalServiceRuntime[S ~string](value *S) (*ServiceRuntime, error) {
	return types.ParseOptionalString(value, ParseServiceRuntime)
}

func (r ServiceRuntime) IsNative() bool {
	return r != "" && r != ServiceRuntimeDocker && r != ServiceRuntimeImage
}
