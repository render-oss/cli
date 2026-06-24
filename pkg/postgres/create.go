package postgres

import (
	"fmt"
	"strconv"
	"strings"

	petname "github.com/dustinkirkland/golang-petname"

	"github.com/render-oss/cli/pkg/client"
	pgclient "github.com/render-oss/cli/pkg/client/postgres"
	"github.com/render-oss/cli/pkg/types"
	pgtypes "github.com/render-oss/cli/pkg/types/postgres"
)

// Default plan when --plan is not supplied.
const defaultPlan = pgclient.Free

// Default Postgres major version when --version is not supplied. Server-side
// validation is the source of truth for which versions are accepted; bump this
// when we want new databases to default to a newer version.
const DefaultPostgresVersion = 18

// CreateRequestInput is the resolved input to BuildCreateRequest.
// All client-side defaults have been applied and the scope has been resolved to an owner
// ID + optional environment ID by the time this struct is constructed.
type CreateRequestInput struct {
	Name             string
	OwnerID          string
	Plan             string
	Version          int
	Region           *string
	EnvironmentID    *string
	DatabaseName     *string
	DatabaseUser     *string
	HighAvailability *bool
	DiskSizeGB       *int
	DiskAutoscaling  *bool
	DatadogAPIKey    *string
	DatadogSite      *string
	IPAllowList      []string
	ReadReplicas     []string
}

// buildRequestInput applies defaults to CLI input and produces the resolved
// CreateRequestInput that BuildCreateRequest expects.
func buildRequestInput(in pgtypes.CreatePostgresInput, ownerID string, environmentID *string) CreateRequestInput {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = petname.Generate(2, "-")
	}
	plan := strings.TrimSpace(in.Plan)
	if plan == "" {
		plan = string(defaultPlan)
	}
	version := DefaultPostgresVersion
	if in.Version != nil {
		version = *in.Version
	}
	return CreateRequestInput{
		Name:             name,
		OwnerID:          ownerID,
		Plan:             plan,
		Version:          version,
		Region:           in.Region,
		EnvironmentID:    environmentID,
		DatabaseName:     in.DatabaseName,
		DatabaseUser:     in.DatabaseUser,
		HighAvailability: in.HighAvailability,
		DiskSizeGB:       in.DiskSizeGB,
		DiskAutoscaling:  in.DiskAutoscaling,
		DatadogAPIKey:    in.DatadogAPIKey,
		DatadogSite:      in.DatadogSite,
		IPAllowList:      in.IPAllowList,
		ReadReplicas:     in.ReadReplicas,
	}
}

// BuildCreateRequest converts a resolved CreateRequestInput into the API
// request body. The errors below should not be reachable through normal use;
// they exist to catch bugs in callers that skip the defaulting/resolution step.
func BuildCreateRequest(input CreateRequestInput) (client.CreatePostgresJSONRequestBody, error) {
	if input.OwnerID == "" {
		return client.CreatePostgresJSONRequestBody{}, fmt.Errorf("workspace is required")
	}
	if input.Name == "" {
		return client.CreatePostgresJSONRequestBody{}, fmt.Errorf("name is required")
	}
	if input.Plan == "" {
		return client.CreatePostgresJSONRequestBody{}, fmt.Errorf("plan is required")
	}
	if input.Version == 0 {
		return client.CreatePostgresJSONRequestBody{}, fmt.Errorf("version is required")
	}
	if err := pgtypes.ValidateDiskSizeGB(input.DiskSizeGB); err != nil {
		return client.CreatePostgresJSONRequestBody{}, err
	}

	body := client.CreatePostgresJSONRequestBody{
		Name:                   input.Name,
		OwnerId:                input.OwnerID,
		Plan:                   pgclient.PostgresPlans(input.Plan),
		Version:                client.PostgresVersion(strconv.Itoa(input.Version)),
		DatabaseName:           input.DatabaseName,
		DatabaseUser:           input.DatabaseUser,
		DatadogAPIKey:          input.DatadogAPIKey,
		DatadogSite:            input.DatadogSite,
		DiskSizeGB:             input.DiskSizeGB,
		EnableDiskAutoscaling:  input.DiskAutoscaling,
		EnableHighAvailability: input.HighAvailability,
		EnvironmentId:          input.EnvironmentID,
		ReadReplicas:           buildReadReplicas(input.ReadReplicas),
	}

	if input.Region != nil {
		r := client.Region(*input.Region)
		body.Region = &r
	}

	if len(input.IPAllowList) > 0 {
		entries, err := types.ParseIPAllowList(input.IPAllowList)
		if err != nil {
			return client.CreatePostgresJSONRequestBody{}, err
		}
		body.IpAllowList = &entries
	}

	return body, nil
}
