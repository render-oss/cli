package views

import "strings"

func matchesServiceId(id string) bool {
	return strings.HasPrefix(id, "srv-") && len(id) == 24
}

func matchesPostgresId(id string) bool {
	return strings.HasPrefix(id, "dpg-") && (len(id) == 24 || len(id) == 26)
}

func matchesKeyValueId(id string) bool {
	return strings.HasPrefix(id, "red-") && len(id) == 24
}

func matchesCronJobId(id string) bool {
	return strings.HasPrefix(id, "crn-") && len(id) == 24
}

func matchesResourceId(id string) bool {
	return matchesServiceId(id) || matchesPostgresId(id) || matchesKeyValueId(id) || matchesCronJobId(id)
}
