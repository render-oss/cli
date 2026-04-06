package types

import (
	"fmt"
	"net"
	"strings"
)

// ParseIPAllowListEntry parses an AWS-style composite flag value into a CIDR block and description.
// Format: "cidr=CIDR[,description=DESC]"
// The description is optional and defaults to empty string.
// IPv6 CIDRs (containing colons) are supported.
func ParseIPAllowListEntry(raw string) (cidrBlock string, description string, err error) {
	if !strings.HasPrefix(raw, "cidr=") {
		return "", "", fmt.Errorf("invalid --ip-allow-list %q: must start with cidr=", raw)
	}

	// Split on ",description=" boundary to handle IPv6 CIDRs containing colons and slashes
	rest := strings.TrimPrefix(raw, "cidr=")
	if before, after, found := strings.Cut(rest, ",description="); found {
		cidrBlock = before
		description = after
	} else {
		cidrBlock = rest
	}

	if cidrBlock == "" {
		return "", "", fmt.Errorf("invalid --ip-allow-list %q: cidr value is empty", raw)
	}

	_, ipNet, err := net.ParseCIDR(cidrBlock)
	if err != nil {
		return "", "", fmt.Errorf("invalid --ip-allow-list %q: %w", raw, err)
	}

	cidrBlock = ipNet.String()

	const maxDescriptionLength = 255
	if len(description) > maxDescriptionLength {
		return "", "", fmt.Errorf("invalid --ip-allow-list %q: description exceeds %d characters", raw, maxDescriptionLength)
	}

	return cidrBlock, description, nil
}

// FormatIPAllowListEntry formats a CIDR block and description into the composite flag format.
func FormatIPAllowListEntry(cidrBlock, description string) string {
	if description != "" {
		return fmt.Sprintf("cidr=%s,description=%s", cidrBlock, description)
	}
	return fmt.Sprintf("cidr=%s", cidrBlock)
}
