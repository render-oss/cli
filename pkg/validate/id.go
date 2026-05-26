package validate

import (
	"fmt"
	"regexp"
)

type ObjectIDPrefix string

const (
	ServiceIDPrefix ObjectIDPrefix = "srv"
)

func IsObjectID(prefix, s string) bool {
	var objectIDRegex = regexp.MustCompile(fmt.Sprintf(`^%s-[a-z0-9]{20}$`, prefix))
	return objectIDRegex.MatchString(s)
}

// IsServiceID checks if the string is a valid service ID (srv-[a-z0-9]{20})
func IsServiceID(s string) bool {
	return IsObjectID("srv", s)
}

// IsWorkspaceID checks if the string is a valid workspace owner ID. Workspaces
// are represented by owner IDs, so both team IDs (tea-) and user IDs (usr-) are
// accepted.
func IsWorkspaceID(s string) bool {
	return IsObjectID("tea", s) || IsObjectID("usr", s)
}

// IsProjectID checks if the string is a valid project ID (prj-[a-z0-9]{20}).
func IsProjectID(s string) bool {
	return IsObjectID("prj", s)
}

// IsEnvironmentID checks if the string is a valid environment ID (evm-[a-z0-9]{20}).
func IsEnvironmentID(s string) bool {
	return IsObjectID("evm", s)
}

// IsKeyValueID checks if the string is a valid Key Value ID (red-[a-z0-9]{20}).
func IsKeyValueID(s string) bool {
	return IsObjectID("red", s)
}

// IsPostgresID checks if the string is a valid Postgres ID (dpg-[a-z0-9]{20}).
func IsPostgresID(s string) bool {
	return IsObjectID("dpg", s)
}

// IsServiceInstanceID checks if the string is a valid service instance ID (srv-[a-z0-9]{20}-[a-z0-9]+)
func IsServiceInstanceID(s string) bool {
	var instanceIDRegex = regexp.MustCompile(`^srv-[a-z0-9]{20}-[a-z0-9]+$`)
	return instanceIDRegex.MatchString(s)
}

// ExtractServiceIDFromInstanceID extracts the service ID from an instance ID
// e.g., "srv-123abc456def789-asdf" -> "srv-123abc456def789"
func ExtractServiceIDFromInstanceID(instanceID string) string {
	if !IsServiceInstanceID(instanceID) {
		return ""
	}
	// Extract the first 24 characters: srv- (4) + 20 chars
	return instanceID[:24]
}
