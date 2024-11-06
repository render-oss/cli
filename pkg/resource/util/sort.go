package util

import (
	"sort"
	"strings"
)

type Resource interface {
	ID() string
	Name() string
	EnvironmentName() string
	ProjectName() string
	Type() string
}

// SortResources sorts the resources by Project, Environment, and Name,
// with empty values appearing last in their respective categories.
func SortResources[T Resource](resources []T) {
	// Helper function to handle empty strings
	emptyLast := func(a, b string) int {
		if a == "" && b == "" {
			return 0
		}
		if a == "" {
			return 1
		}
		if b == "" {
			return -1
		}
		return strings.Compare(strings.ToLower(a), strings.ToLower(b))
	}

	sort.Slice(resources, func(i, j int) bool {
		// Compare projects
		if cmp := emptyLast(resources[i].ProjectName(), resources[j].ProjectName()); cmp != 0 {
			return cmp < 0
		}

		// If projects are equal, compare environments
		if cmp := emptyLast(resources[i].EnvironmentName(), resources[j].EnvironmentName()); cmp != 0 {
			return cmp < 0
		}

		// If environments are equal, compare types
		if cmp := emptyLast(resources[i].Type(), resources[j].Type()); cmp != 0 {
			return cmp < 0
		}

		// If types are equal, compare names
		return strings.ToLower(resources[i].Name()) < strings.ToLower(resources[j].Name())
	})
}
