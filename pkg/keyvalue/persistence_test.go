package keyvalue_test

import (
	"testing"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/keyvalue"
	"github.com/stretchr/testify/assert"
)

// TestPersistenceModeValues guards the hand-maintained list against typos: every
// advertised value must be a real client.PersistenceMode. It can't catch a value
// newly added to the API (that's the gap a generated PersistenceModeValues()
// would close), but it prevents shipping an unrecognized string.
func TestPersistenceModeValues(t *testing.T) {
	values := keyvalue.PersistenceModeValues()
	assert.ElementsMatch(t, []string{"journal_snapshot", "snapshot", "off"}, values)
	for _, v := range values {
		assert.True(t, client.PersistenceMode(v).Valid(),
			"%q must be a valid client.PersistenceMode", v)
	}
}
