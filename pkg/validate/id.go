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
	var objectIDRegex = regexp.MustCompile(fmt.Sprintf(`%s-[a-z0-9]{20}$`, prefix))
	return objectIDRegex.MatchString(s)
}

// IsServiceID checks if the string is a valid service ID (srv-[a-z0-9]{20})
func IsServiceID(s string) bool {
	return IsObjectID("srv", s)
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
