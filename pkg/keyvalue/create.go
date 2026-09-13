package keyvalue

import (
	"context"
	"fmt"
	"regexp"
	"slices"

	petname "github.com/dustinkirkland/golang-petname"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/resolve"
	"github.com/render-oss/cli/pkg/types"
	kvtypes "github.com/render-oss/cli/pkg/types/keyvalue"
)

// Create applies defaults, resolves the requested scope (workspace/project/
// environment), and calls the Key Value create endpoint.
func Create(ctx context.Context, input kvtypes.KeyValueCreateInput) (*client.KeyValueDetail, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, err
	}
	svc := NewService(NewRepo(c), nil, nil, resolve.NewFromClient(c))
	resolved, err := svc.Create(ctx, input)
	if err != nil {
		return nil, err
	}
	return resolved.KeyValue, nil
}

func (s *Service) create(ctx context.Context, input kvtypes.KeyValueCreateInput) (*ResolvedKeyValue, error) {
	input = kvtypes.NormalizeCreateInput(input)

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

	scope, err := s.resolver.ResolveScope(ctx, resolve.ScopeInput{
		WorkspaceIDOrName:   input.WorkspaceIDOrName,
		ProjectIDOrName:     input.ProjectIDOrName,
		EnvironmentIDOrName: input.EnvironmentIDOrName,
	})
	if err != nil {
		return nil, err
	}

	environmentID := scope.EnvironmentID()
	if environmentID == nil && input.ProjectIDOrName != nil && input.EnvironmentIDOrName == nil {
		environmentID, err = s.resolver.ResolveEnvironmentID(ctx, scope.Project, nil, scope.WorkspaceID)
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
		PersistenceMode: input.PersistenceMode,
		IPAllowList:     input.IPAllowList,
	})
	if err != nil {
		return nil, err
	}

	kv, err := s.repo.CreateKeyValue(ctx, body)
	if err != nil {
		return nil, err
	}
	return &ResolvedKeyValue{
		KeyValue:    kv,
		Project:     scope.Project,
		Environment: scope.Environment,
	}, nil
}

// specPlanName matches the spec-based Key Value plan names, e.g. "256mb" or "10g"
var specPlanName = regexp.MustCompile(`^[0-9.]+(mb|g)$`)

// wellKnownPlanValues lists the KV plan names suggested in --plan help text and
// offered in the interactive picker. It is derived from the generated
// client.KeyValuePlanValues() so newly added plans appear automatically, narrowed
// to the spec-based names (plus "free") and with the "custom"
// sentinel filtered out: custom is not a real plan name but a signal that
// account-specific plans exist. Names the CLI does not advertise remain valid
// --plan input that passes through to the API unchanged. The API accepts
// additional plan names not listed here, so this must not be used for validation.
var wellKnownPlanValues = wellKnownKeyValuePlanNames()

func wellKnownKeyValuePlanNames() []string {
	plans := slices.DeleteFunc(client.KeyValuePlanValues(), func(p client.KeyValuePlan) bool {
		if p == client.KeyValuePlanCustom {
			return true
		}
		return !specPlanName.MatchString(string(p)) && p != client.KeyValuePlanFree
	})
	out := make([]string, len(plans))
	for i, p := range plans {
		out[i] = string(p)
	}
	return out
}

// PlanValues returns common KV plan names for help text.
// The API accepts additional account-specific plan names that are not listed here.
// It should not be used for validation.
func PlanValues() []string {
	out := make([]string, len(wellKnownPlanValues))
	copy(out, wellKnownPlanValues)
	return out
}

// PlanOption pairs a KV plan value with its human-facing display label.
type PlanOption struct {
	Value string
	Label string
}

// planLabels maps each well-known KV plan value to its display label. Some labels
// are not derivable from the plan value alone (e.g. "pro_plus" → "Pro Plus"),
// so they are curated here.
var planLabels = map[string]string{
	string(client.KeyValuePlanFree): "Free",

	string(client.KeyValuePlanN256mb): "256mb",
	string(client.KeyValuePlanN1g):    "1g",
	string(client.KeyValuePlanN5g):    "5g",
	string(client.KeyValuePlanN10g):   "10g",
	string(client.KeyValuePlanN20g):   "20g",
	string(client.KeyValuePlanN40g):   "40g",

	string(client.KeyValuePlanStarter):  "Starter",
	string(client.KeyValuePlanStandard): "Standard",
	string(client.KeyValuePlanPro):      "Pro",
	string(client.KeyValuePlanProPlus):  "Pro Plus",
}

// PlanOptions returns the well-known KV plans with display labels for interactive
// selection. It is derived from wellKnownPlanValues so a newly added plan appears
// automatically. The API accepts additional custom plan names not listed here, so
// this must not be used for validation.
func PlanOptions() []PlanOption {
	out := make([]PlanOption, 0, len(wellKnownPlanValues))
	for _, v := range wellKnownPlanValues {
		label, ok := planLabels[v]
		if !ok {
			label = v
		}
		out = append(out, PlanOption{Value: v, Label: label})
	}
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

	if input.PersistenceMode != nil {
		body.PersistenceMode = input.PersistenceMode
	}

	if input.EnvironmentID != nil {
		body.EnvironmentId = input.EnvironmentID
	}

	if len(input.IPAllowList) > 0 {
		entries, err := types.ParseIPAllowList(input.IPAllowList)
		if err != nil {
			return client.CreateKeyValueJSONRequestBody{}, err
		}
		body.IpAllowList = &entries
	}

	return body, nil
}
