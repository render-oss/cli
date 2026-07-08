package postgres

import (
	pgclient "github.com/render-oss/cli/pkg/client/postgres"
)

// PlanMetadata describes the attributes of a Postgres plan that are not
// derivable from the plan value alone.
type PlanMetadata struct {
	// Label is the human-facing name shown in pickers and confirmations.
	Label string
	// HAEligible reports whether the plan supports high availability.
	HAEligible bool
}

// planMetadata carries the display label and HA eligibility for every modern
// Postgres plan. Legacy and custom plans are intentionally  absent (they never
// reach the interactive picker).
var planMetadata = map[string]PlanMetadata{
	string(pgclient.Free):       {Label: "Free"},
	string(pgclient.Basic256mb): {Label: "Basic 256MB"},
	string(pgclient.Basic1gb):   {Label: "Basic 1GB"},
	string(pgclient.Basic4gb):   {Label: "Basic 4GB"},

	string(pgclient.Pro4gb):   {Label: "Pro 4GB", HAEligible: true},
	string(pgclient.Pro8gb):   {Label: "Pro 8GB", HAEligible: true},
	string(pgclient.Pro16gb):  {Label: "Pro 16GB", HAEligible: true},
	string(pgclient.Pro32gb):  {Label: "Pro 32GB", HAEligible: true},
	string(pgclient.Pro64gb):  {Label: "Pro 64GB", HAEligible: true},
	string(pgclient.Pro128gb): {Label: "Pro 128GB", HAEligible: true},
	string(pgclient.Pro192gb): {Label: "Pro 192GB", HAEligible: true},
	string(pgclient.Pro256gb): {Label: "Pro 256GB", HAEligible: true},
	string(pgclient.Pro384gb): {Label: "Pro 384GB", HAEligible: true},
	string(pgclient.Pro512gb): {Label: "Pro 512GB", HAEligible: true},

	string(pgclient.Accelerated16gb):   {Label: "Accelerated 16GB", HAEligible: true},
	string(pgclient.Accelerated32gb):   {Label: "Accelerated 32GB", HAEligible: true},
	string(pgclient.Accelerated64gb):   {Label: "Accelerated 64GB", HAEligible: true},
	string(pgclient.Accelerated128gb):  {Label: "Accelerated 128GB", HAEligible: true},
	string(pgclient.Accelerated256gb):  {Label: "Accelerated 256GB", HAEligible: true},
	string(pgclient.Accelerated384gb):  {Label: "Accelerated 384GB", HAEligible: true},
	string(pgclient.Accelerated512gb):  {Label: "Accelerated 512GB", HAEligible: true},
	string(pgclient.Accelerated768gb):  {Label: "Accelerated 768GB", HAEligible: true},
	string(pgclient.Accelerated1024gb): {Label: "Accelerated 1024GB", HAEligible: true},
}

// PlanLabel returns the human-facing label for a plan value, falling back to the
// raw value for plans without metadata (e.g. account-specific custom plans).
func PlanLabel(plan string) string {
	if m, ok := planMetadata[plan]; ok {
		return m.Label
	}
	return plan
}

// IsHAEligible reports whether the plan supports high availability. Plans
// without metadata (e.g. custom) are treated as not eligible.
func IsHAEligible(plan string) bool {
	return planMetadata[plan].HAEligible
}
