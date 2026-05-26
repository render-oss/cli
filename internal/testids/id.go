package testids

import (
	"strings"

	"github.com/rs/xid"
)

// WorkspaceID returns a syntactically valid workspace owner ID for tests.
func WorkspaceID(label string) string {
	return objectID("tea", label)
}

// RandomWorkspaceID returns a syntactically valid workspace owner ID for tests.
func RandomWorkspaceID() string {
	return WorkspaceID(xid.New().String())
}

// UserID returns a syntactically valid user owner ID for tests.
func UserID(label string) string {
	return objectID("usr", label)
}

// RandomUserID returns a syntactically valid user owner ID for tests.
func RandomUserID() string {
	return UserID(xid.New().String())
}

// ProjectID returns a syntactically valid project ID for tests.
func ProjectID(label string) string {
	return objectID("prj", label)
}

// RandomProjectID returns a syntactically valid project ID for tests.
func RandomProjectID() string {
	return ProjectID(xid.New().String())
}

// EnvironmentID returns a syntactically valid environment ID for tests.
func EnvironmentID(label string) string {
	return objectID("evm", label)
}

// RandomEnvironmentID returns a syntactically valid environment ID for tests.
func RandomEnvironmentID() string {
	return EnvironmentID(xid.New().String())
}

// KeyValueID returns a syntactically valid Key Value ID for tests.
func KeyValueID(label string) string {
	return objectID("red", label)
}

// RandomKeyValueID returns a syntactically valid Key Value ID for tests.
func RandomKeyValueID() string {
	return KeyValueID(xid.New().String())
}

// PostgresID returns a syntactically valid Postgres ID for tests.
func PostgresID(label string) string {
	return objectID("dpg", label)
}

// RandomPostgresID returns a syntactically valid Postgres ID for tests.
func RandomPostgresID() string {
	return PostgresID(xid.New().String())
}

// objectID returns a deterministic test ID in Render object ID form:
//
//	objectID("prj", "Project A!") == "prj-projecta000000000000"
//	objectID("evm", "!!!") == "evm-id000000000000000000"
//
// The body uses the first 20 lowercase ASCII letters and digits from label,
// then pads with zeroes to satisfy the required [a-z0-9]{20} shape. If the
// label contributes no usable characters, "id" keeps the generated ID readable.
// Padding is deterministic instead of random so fixtures and failure messages
// stay stable across test runs.
func objectID(prefix string, label string) string {
	var body strings.Builder
	for _, r := range strings.ToLower(label) {
		if body.Len() == 20 {
			break
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			body.WriteRune(r)
		}
	}
	if body.Len() == 0 {
		body.WriteString("id")
	}
	for body.Len() < 20 {
		body.WriteByte('0')
	}
	return prefix + "-" + body.String()
}
