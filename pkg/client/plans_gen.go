// This file has been generated from our REST API schema. Do not edit it manually
// For more details, see public-api-schema/README.md.

// Code generated automatically. DO NOT EDIT.
package client

// PlanValues returns every valid Plan value defined in the
// public API schema.
func PlanValues() []Plan {
	return []Plan{
		Plan("starter"),
		Plan("starter_plus"),
		Plan("standard"),
		Plan("standard_plus"),
		Plan("pro"),
		Plan("pro_plus"),
		Plan("pro_max"),
		Plan("pro_ultra"),
		Plan("free"),
		Plan("custom"),
		Plan("starter_legacy"),
		Plan("standard_legacy"),
		Plan("standard_plus_legacy"),
		Plan("pro_legacy"),
		Plan("pro_plus_legacy"),
		Plan("0.5c-512mb"),
		Plan("1c-2g"),
		Plan("2c-4g"),
		Plan("2c-8g"),
		Plan("2c-16g"),
		Plan("4c-8g"),
		Plan("4c-16g"),
		Plan("4c-32g"),
		Plan("8c-16g"),
		Plan("8c-32g"),
		Plan("8c-64g"),
		Plan("12c-24g"),
		Plan("12c-48g"),
		Plan("12c-96g"),
	}
}

// PaidPlanValues returns every valid PaidPlan value defined in the
// public API schema.
func PaidPlanValues() []PaidPlan {
	return []PaidPlan{
		PaidPlan("starter"),
		PaidPlan("standard"),
		PaidPlan("pro"),
		PaidPlan("pro_plus"),
		PaidPlan("pro_max"),
		PaidPlan("pro_ultra"),
		PaidPlan("0.5c-512mb"),
		PaidPlan("1c-2g"),
		PaidPlan("2c-4g"),
		PaidPlan("2c-8g"),
		PaidPlan("2c-16g"),
		PaidPlan("4c-8g"),
		PaidPlan("4c-16g"),
		PaidPlan("4c-32g"),
		PaidPlan("8c-16g"),
		PaidPlan("8c-32g"),
		PaidPlan("8c-64g"),
		PaidPlan("12c-24g"),
		PaidPlan("12c-48g"),
		PaidPlan("12c-96g"),
	}
}

// KeyValuePlanValues returns every valid KeyValuePlan value defined in the
// public API schema.
func KeyValuePlanValues() []KeyValuePlan {
	return []KeyValuePlan{
		KeyValuePlan("free"),
		KeyValuePlan("starter"),
		KeyValuePlan("standard"),
		KeyValuePlan("pro"),
		KeyValuePlan("pro_plus"),
		KeyValuePlan("custom"),
		KeyValuePlan("256mb"),
		KeyValuePlan("1g"),
		KeyValuePlan("5g"),
		KeyValuePlan("10g"),
		KeyValuePlan("20g"),
		KeyValuePlan("40g"),
	}
}

// RedisPlanValues returns every valid RedisPlan value defined in the
// public API schema.
func RedisPlanValues() []RedisPlan {
	return []RedisPlan{
		RedisPlan("free"),
		RedisPlan("starter"),
		RedisPlan("standard"),
		RedisPlan("pro"),
		RedisPlan("pro_plus"),
		RedisPlan("custom"),
		RedisPlan("256mb"),
		RedisPlan("1g"),
		RedisPlan("5g"),
		RedisPlan("10g"),
		RedisPlan("20g"),
		RedisPlan("40g"),
	}
}
