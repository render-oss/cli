// This file has been generated from our REST API schema. Do not edit it manually
// For more details, see public-api-schema/README.md.

// Code generated automatically. DO NOT EDIT.
package client

// SandboxNetworkPolicyTypeValues returns every valid SandboxNetworkPolicyType value defined in the
// public API schema.
func SandboxNetworkPolicyTypeValues() []SandboxNetworkPolicyType {
	return []SandboxNetworkPolicyType{
		SandboxNetworkPolicyType("allow-all"),
		SandboxNetworkPolicyType("deny-all"),
		SandboxNetworkPolicyType("allow-list"),
	}
}
