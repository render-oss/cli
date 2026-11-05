package cmd

import (
	"fmt"
	"io"
	"os"
	"unicode/utf8"

	"github.com/spf13/cobra"

	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/sandboxcredential"
)

// SandboxCredentialsWriteInput preserves whether each optional field was supplied.
// Create and update share flags, while their factories enforce distinct requirements.
type SandboxCredentialsWriteInput struct {
	CredentialID      string   `cli:"arg:0"`
	Group             string   `cli:"group"`
	Domain            *string  `cli:"domain"`
	Value             *string  `cli:"value"`
	ValueFile         *string  `cli:"value-file"`
	InjectionType     *string  `cli:"injection-type"`
	InjectionName     *string  `cli:"injection-name"`
	InjectionTemplate *string  `cli:"injection-template"`
	Enabled           *bool    `cli:"enabled"`
	Methods           []string `cli:"methods"`
	Paths             []string `cli:"paths"`
}

func (i *SandboxCredentialsWriteInput) injection() *sandboxesclient.SandboxCredentialInjection {
	if i.InjectionType == nil && i.InjectionName == nil && i.InjectionTemplate == nil {
		return nil
	}
	injection := &sandboxesclient.SandboxCredentialInjection{
		Name:     i.InjectionName,
		Template: i.InjectionTemplate,
	}
	if i.InjectionType != nil {
		injection.Type = sandboxesclient.SandboxCredentialInjectionType(*i.InjectionType)
	}
	return injection
}

func (i *SandboxCredentialsWriteInput) updateInput(cmd *cobra.Command, value *string) sandboxcredential.UpdateInput {
	input := sandboxcredential.UpdateInput{
		SandboxGroupID: i.Group,
		CredentialID:   i.CredentialID,
		Domain:         i.Domain,
		Value:          value,
		Enabled:        i.Enabled,
		Injection:      i.injection(),
	}
	if cmd.Flags().Changed("methods") {
		methods := i.Methods
		if methods == nil {
			methods = []string{}
		}
		input.Methods = &methods
	}
	if cmd.Flags().Changed("paths") {
		paths := i.Paths
		if paths == nil {
			paths = []string{}
		}
		input.Paths = &paths
	}
	return input
}

func readSandboxCredentialValue(cmd *cobra.Command, value, file *string) (*string, error) {
	if file != nil {
		var reader io.Reader
		if *file == "-" {
			reader = cmd.InOrStdin()
		} else {
			f, err := os.Open(*file)
			if err != nil {
				return nil, fmt.Errorf("read --value-file: %w", err)
			}
			defer func() { _ = f.Close() }()
			reader = f
		}
		data, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("read --value-file: %w", err)
		}
		secret := string(data)
		value = &secret
	}
	// JSON encoding would replace invalid UTF-8 and silently change the secret.
	if value != nil && !utf8.ValidString(*value) {
		return nil, fmt.Errorf("credential value must be valid UTF-8")
	}
	return value, nil
}

func addSandboxCredentialWriteFlags(cmd *cobra.Command) {
	cmd.Flags().String("group", "", "Use this sandbox group (defaults to the active workspace's default group)")
	cmd.Flags().String("domain", "", "Match this exact public DNS hostname")
	cmd.Flags().String("value", "", "Set the secret value (prefer --value-file to keep it out of shell history)")
	cmd.Flags().String("value-file", "", "Read the secret value from a file, or - for stdin")
	cmd.MarkFlagsMutuallyExclusive("value", "value-file")
	cmd.Flags().String("injection-type", "", "Inject using header, basic, cookie, or query")
	cmd.Flags().String("injection-name", "", "Set the header, cookie, or query parameter name")
	cmd.Flags().String("injection-template", "", "Format the value using 1-5 literal ${SECRET} placeholders")
	cmd.Flags().StringSlice("methods", nil, "Match HTTP methods (comma-separated; empty matches any)")
	cmd.Flags().StringSlice("paths", nil, "Match exact absolute paths (comma-separated; empty matches any)")
	cmd.Flags().Bool("enabled", true, "Enable credential injection")
	setAllFlagPlaceholders(cmd, map[string]string{
		"group":              "SANDBOX_GROUP_ID",
		"domain":             "DOMAIN",
		"value":              "VALUE",
		"value-file":         "PATH",
		"injection-type":     "TYPE",
		"injection-name":     "NAME",
		"injection-template": "TEMPLATE",
		"methods":            "METHODS",
		"paths":              "PATHS",
	})
}
