package command

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/tui"
)

// tui.List appends a page only when it carries the cursor it was requested
// with, so the loader must pass that cursor through.
func TestLoadPageMsg(t *testing.T) {
	var gotCursor client.Cursor
	loadData := func(_ context.Context, in string, cursor client.Cursor) (client.Cursor, []string, error) {
		gotCursor = cursor
		return "c2", []string{in}, nil
	}

	msg := loadPageMsg(context.Background(), loadData, "item", "c1")

	require.IsType(t, tui.LoadDataMsg[[]string]{}, msg)
	got := msg.(tui.LoadDataMsg[[]string])
	assert.Equal(t, "c1", gotCursor)
	assert.Equal(t, []string{"item"}, got.Data)
	assert.Equal(t, "c1", got.Cursor)
	assert.Equal(t, "c2", got.NextCursor)
}
