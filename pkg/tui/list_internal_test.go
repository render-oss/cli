package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testListItem string

func (i testListItem) Title() string       { return string(i) }
func (i testListItem) Description() string { return "" }
func (i testListItem) FilterValue() string { return string(i) }
func (i testListItem) Height() int         { return 1 }

// fakePages serves pages keyed by cursor and records the cursors the list asks
// it to load.
type fakePages struct {
	pages     map[string]LoadDataMsg[[]string]
	requested []string
}

func (f *fakePages) load(cursor string) TypedCmd[[]string] {
	f.requested = append(f.requested, cursor)
	return func() tea.Msg {
		msg := f.pages[cursor]
		msg.Cursor = cursor
		return msg
	}
}

// threePages is a list of a, b | c, d | e split across three pages.
func threePages() *fakePages {
	return &fakePages{pages: map[string]LoadDataMsg[[]string]{
		"":   {Data: []string{"a", "b"}, NextCursor: "c1"},
		"c1": {Data: []string{"c", "d"}, NextCursor: "c2"},
		"c2": {Data: []string{"e"}},
	}}
}

func newTestList(t *testing.T, pages *fakePages) *List[string] {
	t.Helper()
	l := NewList("", pages.load, func(s string) ListItem { return testListItem(s) })
	l.Update(StackSizeMsg{Width: 80, Height: 40})
	l.Update(pages.load("")())
	return l
}

func listTitles(l *List[string]) []string {
	var titles []string
	for _, item := range l.list.Items() {
		titles = append(titles, item.(ListItem).Title())
	}
	return titles
}

// pressDown sends a down keypress, running any page load it triggers.
func pressDown(l *List[string]) {
	_, cmd := l.Update(tea.KeyMsg{Type: tea.KeyDown})
	if cmd == nil {
		return
	}
	if msg, ok := cmd().(LoadDataMsg[[]string]); ok {
		l.Update(msg)
	}
}

func TestList_Paging(t *testing.T) {
	tests := []struct {
		name          string
		act           func(t *testing.T, l *List[string])
		wantTitles    []string
		wantIndex     int
		wantRequested []string
	}{
		{
			name: "scrolling past last item appends next page",
			act: func(t *testing.T, l *List[string]) {
				pressDown(l) // a -> b
				pressDown(l) // past b: loads page 2
			},
			wantTitles:    []string{"a", "b", "c", "d"},
			wantIndex:     2, // selection moves onto the first new item
			wantRequested: []string{"", "c1"},
		},
		{
			name: "scrolling back reaches earlier pages",
			act: func(t *testing.T, l *List[string]) {
				for range 4 {
					pressDown(l)
				}
				for range 4 {
					l.Update(tea.KeyMsg{Type: tea.KeyUp})
				}
			},
			wantTitles:    []string{"a", "b", "c", "d", "e"},
			wantIndex:     0,
			wantRequested: []string{"", "c1", "c2"},
		},
		{
			name: "stops loading after last page",
			act: func(t *testing.T, l *List[string]) {
				for range 6 {
					pressDown(l)
				}
			},
			wantTitles:    []string{"a", "b", "c", "d", "e"},
			wantIndex:     4,
			wantRequested: []string{"", "c1", "c2"}, // no request after the page without more data
		},
		{
			name: "init keeps loaded pages",
			act: func(t *testing.T, l *List[string]) {
				for range 4 {
					pressDown(l)
				}
				l.Init() // returning to the list in the stack re-runs Init
			},
			wantTitles:    []string{"a", "b", "c", "d", "e"},
			wantIndex:     4, // selection is kept
			wantRequested: []string{"", "c1", "c2"},
		},
		{
			name: "keeps selection moved during load",
			act: func(t *testing.T, l *List[string]) {
				pressDown(l) // a -> b
				_, cmd := l.Update(tea.KeyMsg{Type: tea.KeyDown})
				require.NotNil(t, cmd, "moving past the last item should load the next page")

				l.Update(tea.KeyMsg{Type: tea.KeyUp}) // move up while the page loads
				l.Update(cmd())
			},
			wantTitles:    []string{"a", "b", "c", "d"},
			wantIndex:     0, // selection stays where the user moved it
			wantRequested: []string{"", "c1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pages := threePages()
			l := newTestList(t, pages)

			tt.act(t, l)

			assert.Equal(t, tt.wantTitles, listTitles(l))
			assert.Equal(t, tt.wantIndex, l.list.Index())
			assert.Equal(t, tt.wantRequested, pages.requested)
		})
	}
}
