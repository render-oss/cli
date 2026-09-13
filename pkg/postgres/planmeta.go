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

// planMetadata carries the display label and HA eligibility of every plan name the
// CLI recognizes. Legacy and custom plans are intentionally  absent (they never
// reach the interactive picker).
var planMetadata = map[string]PlanMetadata{
	string(pgclient.Free): {Label: "Free"},

	string(pgclient.N01c256mb):  {Label: "0.1c-256mb"},
	string(pgclient.N05c1g):     {Label: "0.5c-1g"},
	string(pgclient.N1c2g):      {Label: "1c-2g", HAEligible: true},
	string(pgclient.N1c4g):      {Label: "1c-4g", HAEligible: true},
	string(pgclient.N2c4g):      {Label: "2c-4g", HAEligible: true},
	string(pgclient.N2c8g):      {Label: "2c-8g", HAEligible: true},
	string(pgclient.N2c16g):     {Label: "2c-16g", HAEligible: true},
	string(pgclient.N4c16g):     {Label: "4c-16g", HAEligible: true},
	string(pgclient.N4c32g):     {Label: "4c-32g", HAEligible: true},
	string(pgclient.N8c32g):     {Label: "8c-32g", HAEligible: true},
	string(pgclient.N8c64g):     {Label: "8c-64g", HAEligible: true},
	string(pgclient.N16c64g):    {Label: "16c-64g", HAEligible: true},
	string(pgclient.N16c128g):   {Label: "16c-128g", HAEligible: true},
	string(pgclient.N32c128g):   {Label: "32c-128g", HAEligible: true},
	string(pgclient.N32c256g):   {Label: "32c-256g", HAEligible: true},
	string(pgclient.N48c192g):   {Label: "48c-192g", HAEligible: true},
	string(pgclient.N48c384g):   {Label: "48c-384g", HAEligible: true},
	string(pgclient.N64c256g):   {Label: "64c-256g", HAEligible: true},
	string(pgclient.N64c512g):   {Label: "64c-512g", HAEligible: true},
	string(pgclient.N96c384g):   {Label: "96c-384g", HAEligible: true},
	string(pgclient.N96c768g):   {Label: "96c-768g", HAEligible: true},
	string(pgclient.N128c512g):  {Label: "128c-512g", HAEligible: true},
	string(pgclient.N128c1024g): {Label: "128c-1024g", HAEligible: true},

	string(pgclient.Basic256mb): {Label: "Basic 256MB"},
	string(pgclient.Basic1gb):   {Label: "Basic 1GB"},
	string(pgclient.Basic4gb):   {Label: "Basic 4GB", HAEligible: true},

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
