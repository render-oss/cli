package keyvalue

// Plan is intentionally a string. The REST API resolves plan names server-side,
// including account-specific custom plans, so the OpenAPI enum is only a partial
// set of common values and should not be used for client-side validation.
type Plan = string

const (
	PlanFree     Plan = "free"
	PlanStarter  Plan = "starter"
	PlanStandard Plan = "standard"
	PlanPro      Plan = "pro"
	PlanProPlus  Plan = "pro_plus"
)
