package keyvalue

import (
	"fmt"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/types"
	kvtypes "github.com/render-oss/cli/pkg/types/keyvalue"
)

var wellKnownPlanValues = []string{
	kvtypes.PlanFree,
	kvtypes.PlanStarter,
	kvtypes.PlanStandard,
	kvtypes.PlanPro,
	kvtypes.PlanProPlus,
}

// PlanValues returns common KV plan names for help text.
// The API accepts additional account-specific plan names that are not listed here.
// It should not be used for validation.
func PlanValues() []string {
	out := make([]string, len(wellKnownPlanValues))
	copy(out, wellKnownPlanValues)
	return out
}

func validateCreateRequestInput(input kvtypes.KeyValueCreateRequestInput) error {
	if input.Name == "" {
		return fmt.Errorf("name is required")
	}
	if input.OwnerID == "" {
		return fmt.Errorf("owner ID is required")
	}
	if input.Plan == "" {
		return fmt.Errorf("plan is required")
	}
	return nil
}

func BuildCreateRequest(input kvtypes.KeyValueCreateRequestInput) (client.CreateKeyValueJSONRequestBody, error) {
	if err := validateCreateRequestInput(input); err != nil {
		return client.CreateKeyValueJSONRequestBody{}, err
	}

	body := client.CreateKeyValueJSONRequestBody{
		Name:    input.Name,
		OwnerId: input.OwnerID,
		Plan:    client.KeyValuePlan(input.Plan),
	}

	if input.Region != nil {
		body.Region = input.Region
	}

	if input.MaxmemoryPolicy != nil {
		p := client.MaxmemoryPolicy(*input.MaxmemoryPolicy)
		body.MaxmemoryPolicy = &p
	}

	if input.EnvironmentID != nil {
		body.EnvironmentId = input.EnvironmentID
	}

	if len(input.IPAllowList) > 0 {
		entries, err := parseIPAllowList(input.IPAllowList)
		if err != nil {
			return client.CreateKeyValueJSONRequestBody{}, err
		}
		body.IpAllowList = &entries
	}

	return body, nil
}

func parseIPAllowList(raw []string) ([]client.CidrBlockAndDescription, error) {
	out := make([]client.CidrBlockAndDescription, 0, len(raw))
	for _, entry := range raw {
		cidr, desc, err := types.ParseIPAllowListEntry(entry)
		if err != nil {
			return nil, err
		}
		out = append(out, client.CidrBlockAndDescription{
			CidrBlock:   cidr,
			Description: desc,
		})
	}
	return out, nil
}
