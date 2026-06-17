package testrequire_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/render-oss/cli/internal/testrequire"
)

func TestSubMap(t *testing.T) {
	body := map[string]any{
		"data":   map[string]any{"id": "dpg-123"},
		"string": "this is a string",
	}

	t.Run("nested map passes", func(t *testing.T) {
		assert.Equal(t, map[string]any{"id": "dpg-123"}, testrequire.SubMap(t, body, "data"))
	})

	t.Run("missing key fails now", func(t *testing.T) {
		fake := &fakeT{}
		callAndRecover(func() {
			_ = testrequire.SubMap(fake, body, "missing")
		})
		assert.True(t, fake.failed)
		assert.True(t, fake.failNow)
	})

	t.Run("wrong type fails now", func(t *testing.T) {
		fake := &fakeT{}
		callAndRecover(func() {
			_ = testrequire.SubMap(fake, body, "string")
		})
		assert.True(t, fake.failed)
		assert.True(t, fake.failNow)
	})
}

func TestSubSlice(t *testing.T) {
	body := map[string]any{
		"data":   []any{float64(1), float64(2), float64(3)},
		"string": "this is a string",
	}

	t.Run("nested slice passes", func(t *testing.T) {
		assert.Equal(t, []any{float64(1), float64(2), float64(3)}, testrequire.SubSlice(t, body, "data"))
	})

	t.Run("missing key fails now", func(t *testing.T) {
		fake := &fakeT{}
		callAndRecover(func() {
			_ = testrequire.SubSlice(fake, body, "missing")
		})
		assert.True(t, fake.failed)
		assert.True(t, fake.failNow)
	})

	t.Run("wrong type fails now", func(t *testing.T) {
		fake := &fakeT{}
		callAndRecover(func() {
			_ = testrequire.SubSlice(fake, body, "string")
		})
		assert.True(t, fake.failed)
		assert.True(t, fake.failNow)
	})
}

func TestAsJSONMap(t *testing.T) {
	t.Run("round-trips value through JSON into map", func(t *testing.T) {
		type service struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		type response struct {
			Data service `json:"data"`
		}
		out := response{
			Data: service{
				ID:   "srv-123",
				Name: "my-api",
			},
		}

		body := testrequire.AsJSONMap(t, out)

		data := testrequire.SubMap(t, body, "data")
		assert.Equal(t, "srv-123", data["id"])
		assert.Equal(t, "my-api", data["name"])
	})

	t.Run("marshal error fails now", func(t *testing.T) {
		fake := &fakeT{}
		out := map[string]any{
			"bad": make(chan int),
		}

		callAndRecover(func() {
			testrequire.AsJSONMap(fake, out)
		})

		assert.True(t, fake.failed)
		assert.True(t, fake.failNow)
	})
}

func TestParseJSONMap(t *testing.T) {
	t.Run("parses JSON string into map", func(t *testing.T) {
		raw := `{"data":{"id":"srv-123","name":"my-api"}}`

		body := testrequire.ParseJSONMap(t, raw)

		data := testrequire.SubMap(t, body, "data")
		assert.Equal(t, "srv-123", data["id"])
		assert.Equal(t, "my-api", data["name"])
	})

	t.Run("invalid JSON fails now", func(t *testing.T) {
		fake := &fakeT{}

		callAndRecover(func() {
			testrequire.ParseJSONMap(fake, `{"data":`)
		})

		assert.True(t, fake.failed)
		assert.True(t, fake.failNow)
	})
}

func callAndRecover(fn func()) {
	defer func() {
		_ = recover()
	}()
	fn()
}

type fakeT struct {
	failed  bool
	failNow bool
}

func (f *fakeT) Helper() {}

func (f *fakeT) Errorf(string, ...any) {
	f.failed = true
}

func (f *fakeT) FailNow() {
	f.failNow = true
}
