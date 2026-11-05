package text

import (
	"fmt"
	"strings"
	"time"

	"github.com/jedib0t/go-pretty/table"

	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
)

func SandboxCredentialTable(credentials []sandboxesclient.SandboxCredential) string {
	t := newTable()
	t.AppendHeader(table.Row{"ID", "Domain", "Enabled", "Injection", "Version"})
	for _, c := range credentials {
		version := "-"
		if c.Version != nil {
			version = fmt.Sprint(c.Version.Id)
		}
		t.AppendRow(table.Row{c.Id, c.Domain, c.Enabled, c.Injection.Type, version})
	}
	return FormatString(t.Render())
}

func SandboxCredentialDetail(c *sandboxesclient.SandboxCredential) string {
	lines := []string{
		fmt.Sprintf("ID:             %s", c.Id),
		fmt.Sprintf("Domain:         %s", c.Domain),
		fmt.Sprintf("Enabled:        %t", c.Enabled),
		fmt.Sprintf("Methods:        %s", strings.Join(c.Methods, ", ")),
		fmt.Sprintf("Paths:          %s", strings.Join(c.Paths, ", ")),
		fmt.Sprintf("Injection:      %s", c.Injection.Type),
	}
	if c.Injection.Name != nil {
		lines = append(lines, fmt.Sprintf("Name:           %s", *c.Injection.Name))
	}
	if c.Injection.Template != nil {
		lines = append(lines, fmt.Sprintf("Template:       %s", *c.Injection.Template))
	}
	if c.Version != nil {
		lines = append(lines,
			fmt.Sprintf("Version:        %d", c.Version.Id),
			fmt.Sprintf("Updated:        %s", c.Version.UpdatedAt.UTC().Format(time.RFC3339)),
			fmt.Sprintf("Updated by:     %s", c.Version.UpdatedBy),
			fmt.Sprintf("Fingerprint:    %s", c.Version.Fingerprint),
		)
		if c.Version.RestoredFromVersionId != nil {
			lines = append(lines, fmt.Sprintf("Restored from:  %d", *c.Version.RestoredFromVersionId))
		}
	}
	return FormatString(strings.Join(lines, "\n"))
}

func SandboxCredentialVersionTable(versions []sandboxesclient.SandboxCredentialVersion) string {
	t := newTable()
	t.AppendHeader(table.Row{"Version", "Updated", "Updated By", "Fingerprint", "Restored From"})
	for _, v := range versions {
		restored := "-"
		if v.RestoredFromVersionId != nil {
			restored = fmt.Sprint(*v.RestoredFromVersionId)
		}
		t.AppendRow(table.Row{v.Id, v.UpdatedAt.UTC().Format(time.RFC3339), v.UpdatedBy, v.Fingerprint, restored})
	}
	return FormatString(t.Render())
}
