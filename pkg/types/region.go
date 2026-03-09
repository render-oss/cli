package types

import (
	"fmt"
	"strings"
)

type Region string

const (
	RegionFrankfurt Region = "frankfurt"
	RegionOhio      Region = "ohio"
	RegionOregon    Region = "oregon"
	RegionSingapore Region = "singapore"
	RegionVirginia  Region = "virginia"
)

var regionValues = []Region{
	RegionFrankfurt,
	RegionOhio,
	RegionOregon,
	RegionSingapore,
	RegionVirginia,
}

func RegionValues() []string {
	values := make([]string, 0, len(regionValues))
	for _, value := range regionValues {
		values = append(values, string(value))
	}
	return values
}

func ParseRegion(value string) (Region, error) {
	normalized := strings.TrimSpace(value)
	for _, region := range regionValues {
		if normalized == string(region) {
			return region, nil
		}
	}

	return "", fmt.Errorf("region must be one of: %s", strings.Join(RegionValues(), ", "))
}

func OptionalRegion(value *string) (*Region, error) {
	return ParseOptional(value, ParseRegion)
}
