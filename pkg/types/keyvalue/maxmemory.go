package keyvalue

import "github.com/render-oss/cli/pkg/client"

// MaxmemoryPolicy represents values accepted by the --memory-policy flag.
// It is a superset of client.MaxmemoryPolicy: the cache/queue shortcuts are
// CLI-only and must be normalized to their API equivalents before use.
type MaxmemoryPolicy string

const (
	MaxmemoryPolicyNoeviction     MaxmemoryPolicy = MaxmemoryPolicy(client.Noeviction)
	MaxmemoryPolicyAllkeysLfu     MaxmemoryPolicy = MaxmemoryPolicy(client.AllkeysLfu)
	MaxmemoryPolicyAllkeysLru     MaxmemoryPolicy = MaxmemoryPolicy(client.AllkeysLru)
	MaxmemoryPolicyAllkeysRandom  MaxmemoryPolicy = MaxmemoryPolicy(client.AllkeysRandom)
	MaxmemoryPolicyVolatileLfu    MaxmemoryPolicy = MaxmemoryPolicy(client.VolatileLfu)
	MaxmemoryPolicyVolatileLru    MaxmemoryPolicy = MaxmemoryPolicy(client.VolatileLru)
	MaxmemoryPolicyVolatileRandom MaxmemoryPolicy = MaxmemoryPolicy(client.VolatileRandom)
	MaxmemoryPolicyVolatileTtl    MaxmemoryPolicy = MaxmemoryPolicy(client.VolatileTtl)

	// Friendly shortcuts that normalize to their technical equivalents.
	MaxmemoryPolicyCache MaxmemoryPolicy = "cache" // → allkeys_lru
	MaxmemoryPolicyQueue MaxmemoryPolicy = "queue" // → noeviction
)

var maxmemoryPolicyValues = []MaxmemoryPolicy{
	MaxmemoryPolicyNoeviction,
	MaxmemoryPolicyAllkeysLfu,
	MaxmemoryPolicyAllkeysLru,
	MaxmemoryPolicyAllkeysRandom,
	MaxmemoryPolicyVolatileLfu,
	MaxmemoryPolicyVolatileLru,
	MaxmemoryPolicyVolatileRandom,
	MaxmemoryPolicyVolatileTtl,
}

func MaxmemoryPolicyValues() []string {
	values := make([]string, 0, len(maxmemoryPolicyValues))
	for _, v := range maxmemoryPolicyValues {
		values = append(values, string(v))
	}
	return values
}

// MemoryPolicyInputValues returns all accepted --memory-policy flag values,
// including the cache/queue shortcuts alongside the technical API values.
func MemoryPolicyInputValues() []string {
	return append(
		[]string{string(MaxmemoryPolicyCache), string(MaxmemoryPolicyQueue)},
		MaxmemoryPolicyValues()...,
	)
}

// NormalizeMemoryPolicy translates cache/queue shortcuts to their API equivalents.
func NormalizeMemoryPolicy(p MaxmemoryPolicy) MaxmemoryPolicy {
	switch p {
	case MaxmemoryPolicyCache:
		return MaxmemoryPolicyAllkeysLru
	case MaxmemoryPolicyQueue:
		return MaxmemoryPolicyNoeviction
	default:
		return p
	}
}
