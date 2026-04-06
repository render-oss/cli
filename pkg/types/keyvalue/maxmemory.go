package keyvalue


type MaxmemoryPolicy string

const (
	MaxmemoryPolicyNoeviction     MaxmemoryPolicy = "noeviction"
	MaxmemoryPolicyAllkeysLfu     MaxmemoryPolicy = "allkeys_lfu"
	MaxmemoryPolicyAllkeysLru     MaxmemoryPolicy = "allkeys_lru"
	MaxmemoryPolicyAllkeysRandom  MaxmemoryPolicy = "allkeys_random"
	MaxmemoryPolicyVolatileLfu    MaxmemoryPolicy = "volatile_lfu"
	MaxmemoryPolicyVolatileLru    MaxmemoryPolicy = "volatile_lru"
	MaxmemoryPolicyVolatileRandom MaxmemoryPolicy = "volatile_random"
	MaxmemoryPolicyVolatileTtl    MaxmemoryPolicy = "volatile_ttl"
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
