package pointers

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEqual(t *testing.T) {
	value := "value"
	same := "value"
	other := "other"

	tests := []struct {
		name string
		a    *string
		b    *string
		want bool
	}{
		{name: "both nil", want: true},
		{name: "left nil", b: &value, want: false},
		{name: "right nil", a: &value, want: false},
		{name: "same value", a: &value, b: &same, want: true},
		{name: "different value", a: &value, b: &other, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, Equal(tt.a, tt.b))
		})
	}
}
