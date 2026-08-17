package keyvalue

import "github.com/render-oss/cli/pkg/client"

// persistenceModeValues is the set of persistence modes the CLI advertises for
// the --persistence-mode flag and help text. It mirrors the
// client.PersistenceMode enum, ordered most-durable first.
//
// Unlike the plan enums, there is no generated client.PersistenceModeValues()
// to consume: the public-api-schema value generator only emits helpers for plan
// enums. So this list is curated by hand and guarded by
// TestPersistenceModeValues, which asserts every entry is a valid
// client.PersistenceMode. If the generator later learns to emit a
// PersistenceModeValues(), delete this and point PersistenceModeValues at it.
var persistenceModeValues = []client.PersistenceMode{
	client.PersistenceModeJournalSnapshot,
	client.PersistenceModeSnapshot,
	client.PersistenceModeOff,
}

// PersistenceModeValues returns the accepted --persistence-mode flag values.
func PersistenceModeValues() []string {
	out := make([]string, 0, len(persistenceModeValues))
	for _, v := range persistenceModeValues {
		out = append(out, string(v))
	}
	return out
}
