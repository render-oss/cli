package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/client"
	sandboxclient "github.com/render-oss/cli/pkg/client/sandboxes"
)

const listTestWorkspace = "tea-workspace"

// newTestService wires a Service backed by a generated client pointed at h.
func newTestService(t *testing.T, h http.HandlerFunc) *Service {
	t.Helper()
	server := httptest.NewServer(h)
	t.Cleanup(server.Close)

	t.Setenv("RENDER_WORKSPACE", listTestWorkspace)

	c, err := client.NewClientWithResponses(server.URL)
	require.NoError(t, err)
	return NewService(NewRepo(c))
}

func TestServiceList_StatusQuery(t *testing.T) {
	tests := []struct {
		name       string
		statuses   []string
		all        bool
		wantStatus string // expected value of the status query param
		wantNoStat bool   // expect no status param at all
	}{
		{
			name:       "default sends no status filter",
			wantNoStat: true,
		},
		{
			name:       "--all requests every status",
			all:        true,
			wantStatus: "creating,running,suspending,suspended,resuming,errored,terminated",
		},
		{
			name:       "single status passes through",
			statuses:   []string{"running"},
			wantStatus: "running",
		},
		{
			name:       "multiple statuses pass through comma-joined",
			statuses:   []string{"running", "creating"},
			wantStatus: "running,creating",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotQuery url.Values
			svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
				gotQuery = r.URL.Query()
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode([]client.SandboxWithCursor{})
			})

			_, err := svc.List(context.Background(), ListInput{Statuses: tt.statuses, All: tt.all})
			require.NoError(t, err)

			assert.Equal(t, listTestWorkspace, gotQuery.Get("ownerId"))
			assert.Equal(t, "100", gotQuery.Get("limit"))
			assert.NotContains(t, gotQuery, "name")
			if tt.wantNoStat {
				assert.NotContains(t, gotQuery, "status")
			} else {
				assert.Equal(t, tt.wantStatus, gotQuery.Get("status"))
			}
		})
	}
}

func TestServiceList_FollowsCursorsAcrossPages(t *testing.T) {
	// A full first page (100 items) forces ListAll to request a second page
	// using the last item's cursor; the short second page ends pagination.
	const firstPageSize = 100

	var gotCursors []string
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		cursor := r.URL.Query().Get("cursor")
		gotCursors = append(gotCursors, cursor)

		w.Header().Set("Content-Type", "application/json")
		if cursor == "" {
			page := make([]client.SandboxWithCursor, firstPageSize)
			for i := range page {
				page[i] = client.SandboxWithCursor{
					Cursor:  fmt.Sprintf("cur-%d", i),
					Sandbox: sandboxclient.Sandbox{Id: fmt.Sprintf("sbx-%d", i)},
				}
			}
			_ = json.NewEncoder(w).Encode(page)
			return
		}
		_ = json.NewEncoder(w).Encode([]client.SandboxWithCursor{
			{Cursor: "cur-last", Sandbox: sandboxclient.Sandbox{Id: "sbx-last"}},
		})
	})

	got, err := svc.List(context.Background(), ListInput{})
	require.NoError(t, err)

	assert.Len(t, got, firstPageSize+1)
	// First request has no cursor; second carries the last cursor of page one.
	require.Equal(t, []string{"", fmt.Sprintf("cur-%d", firstPageSize-1)}, gotCursors)
	assert.Equal(t, "sbx-last", got[len(got)-1].Id)
}

// TestServiceListPage covers the TUI's page-at-a-time fetch: exactly one
// request per call, the cursor is forwarded, and a short page ends pagination.
func TestServiceListPage(t *testing.T) {
	page := func(n int) []client.SandboxWithCursor {
		out := make([]client.SandboxWithCursor, n)
		for i := range out {
			out[i] = client.SandboxWithCursor{
				Cursor:  fmt.Sprintf("cur-%d", i),
				Sandbox: sandboxclient.Sandbox{Id: fmt.Sprintf("sbx-%d", i)},
			}
		}
		return out
	}

	tests := []struct {
		name       string
		input      ListInput
		cursor     client.Cursor
		response   []client.SandboxWithCursor
		wantQuery  url.Values
		wantNext   client.Cursor
		wantLength int
	}{
		{
			name:       "full first page returns the last cursor",
			response:   page(ListPageSize),
			wantQuery:  url.Values{"ownerId": {listTestWorkspace}, "limit": {fmt.Sprint(ListPageSize)}},
			wantNext:   client.Cursor(fmt.Sprintf("cur-%d", ListPageSize-1)),
			wantLength: ListPageSize,
		},
		{
			name:       "short page is the last page",
			response:   page(3),
			wantQuery:  url.Values{"ownerId": {listTestWorkspace}, "limit": {fmt.Sprint(ListPageSize)}},
			wantNext:   "",
			wantLength: 3,
		},
		{
			name:       "empty page is the last page",
			response:   page(0),
			wantQuery:  url.Values{"ownerId": {listTestWorkspace}, "limit": {fmt.Sprint(ListPageSize)}},
			wantNext:   "",
			wantLength: 0,
		},
		{
			name:     "sends cursor and filters",
			input:    ListInput{Statuses: []string{"running"}, SandboxGroupID: "sbg-1"},
			cursor:   "cur-prev",
			response: page(1),
			wantQuery: url.Values{
				"ownerId":        {listTestWorkspace},
				"limit":          {fmt.Sprint(ListPageSize)},
				"cursor":         {"cur-prev"},
				"status":         {"running"},
				"sandboxGroupId": {"sbg-1"},
			},
			wantLength: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requests int
			var gotQuery url.Values
			svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
				requests++
				gotQuery = r.URL.Query()
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(tt.response)
			})

			next, got, err := svc.ListPage(context.Background(), tt.input, tt.cursor)
			require.NoError(t, err)

			assert.Equal(t, 1, requests, "ListPage makes exactly one request")
			assert.Equal(t, tt.wantQuery, gotQuery)
			assert.Equal(t, tt.wantNext, next)
			assert.Len(t, got, tt.wantLength)
		})
	}
}
