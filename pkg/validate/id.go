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
