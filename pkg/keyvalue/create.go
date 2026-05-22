package keyvalue

import (
	"context"
	"fmt"

	petname "github.com/dustinkirkland/golang-petname"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/resolve"
	"github.com/render-oss/cli/pkg/types"
	kvtypes "github.com/render-oss/cli/pkg/types/keyvalue"
)

// Create applies defaults, resolves the requested scope (workspace/project/
// environment), and calls the Key Value create endpoint. It is the shared
// entry point for both the non-interactive flag path and the interactive
// wizard's final submit step.
func Create(ctx context.Context, input kvtypes.KeyValueCreateInput) (*client.KeyValueDetail, error) {
	input = kvtypes.NormalizeCreateInput(input)

	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, err
	}
	resolver := resolve.NewFromClient(c)

	if input.Name == "" {
		input.Name = petname.Generate(2, "-")
	}
	if input.Plan == "" {
		input.Plan = kvtypes.PlanFree
	}
	if input.Region == nil {
		r := string(types.RegionOregon)
		input.Region = &r
	}
	if input.MaxmemoryPolicy == nil {
		p := kvtypes.MaxmemoryPolicyAllkeysLru
		input.MaxmemoryPolicy = &p
	}

	scope, err := resolver.ResolveScope(ctx, resolve.ScopeInput{
		WorkspaceIDOrName:   input.WorkspaceIDOrName,
		ProjectIDOrName:     input.ProjectIDOrName,
		EnvironmentIDOrName: input.EnvironmentIDOrName,
	})
	if err != nil {
		return nil, err
	}

	environmentID := scope.EnvironmentID()
	if environmentID == nil && input.ProjectIDOrName != nil && input.EnvironmentIDOrName == nil {
		environmentID, err = resolver.ResolveEnvironmentID(ctx, scope.Project, nil, scope.WorkspaceID)
		if err != nil {
			return nil, err
		}
	}

	body, err := BuildCreateRequest(kvtypes.KeyValueCreateRequestInput{
		Name:            input.Name,
		OwnerID:         scope.WorkspaceID,
		Plan:            input.Plan,
		Region:          input.Region,
		EnvironmentID:   environmentID,
		MaxmemoryPolicy: input.MaxmemoryPolicy,
		IPAllowList:     input.IPAllowList,
	})
	if err != nil {
		return nil, err
	}

	return NewRepo(c).CreateKeyValue(ctx, body)
}

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
		r := client.Region(*input.Region)
		body.Region = &r
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
