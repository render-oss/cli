package validate_test

import (
	"testing"

	"github.com/render-oss/cli/v2/pkg/validate"
)

func TestIsObjectID(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		input    string
		expected bool
	}{
		// Valid IDs
		{"valid service ID", "srv", "srv-12345678901234567890", true},
		{"valid postgres ID", "dpg", "dpg-12345678901234567890", true},

		// Anchoring — the core regression test
		{"prefix suffix not matched (no ^ anchor regression)", "dpg", "xdpg-12345678901234567890", false},
		{"prefix suffix not matched with srv", "srv", "xsrv-12345678901234567890", false},
		{"valid ID embedded in longer string", "dpg", "dpg-12345678901234567890extra", false},

		// Format violations
		{"uppercase letters rejected", "srv", "srv-1234567890ABCDEFGHIJ", false},
		{"too short", "srv", "srv-1234567890", false},
		{"too long", "srv", "srv-123456789012345678901", false},
		{"wrong prefix", "srv", "dpg-12345678901234567890", false},
		{"empty string", "srv", "", false},
		{"prefix only", "srv", "srv-", false},
		{"no prefix separator", "srv", "srv12345678901234567890", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := validate.IsObjectID(tc.prefix, tc.input)
			if got != tc.expected {
				t.Errorf("IsObjectID(%q, %q) = %v, want %v", tc.prefix, tc.input, got, tc.expected)
			}
		})
	}
}

func TestIsServiceID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid service ID", "srv-12345678901234567890", true},
		{"service instance ID rejected", "srv-12345678901234567890-abc", false},
		{"postgres ID rejected", "dpg-12345678901234567890", false},
		{"suffix match rejected (anchor regression)", "xsrv-12345678901234567890", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := validate.IsServiceID(tc.input)
			if got != tc.expected {
				t.Errorf("IsServiceID(%q) = %v, want %v", tc.input, got, tc.expected)
			}
		})
	}
}

func TestIsServiceInstanceID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid instance ID", "srv-12345678901234567890-abc", true},
		{"valid instance ID with longer suffix", "srv-12345678901234567890-abc123", true},
		{"bare service ID rejected", "srv-12345678901234567890", false},
		{"suffix match rejected", "xsrv-12345678901234567890-abc", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := validate.IsServiceInstanceID(tc.input)
			if got != tc.expected {
				t.Errorf("IsServiceInstanceID(%q) = %v, want %v", tc.input, got, tc.expected)
			}
		})
	}
}

func TestExtractServiceIDFromInstanceID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"extracts service ID from instance ID", "srv-12345678901234567890-abc", "srv-12345678901234567890"},
		{"returns empty for bare service ID", "srv-12345678901234567890", ""},
		{"returns empty for invalid input", "not-an-id", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := validate.ExtractServiceIDFromInstanceID(tc.input)
			if got != tc.expected {
				t.Errorf("ExtractServiceIDFromInstanceID(%q) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}
