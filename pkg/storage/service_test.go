package storage

import (
	"context"
	"os"
	"testing"

	"github.com/render-oss/cli/pkg/client"
	"github.com/stretchr/testify/require"
)

func TestNewService_ServiceModeSelection(t *testing.T) {
	// Create a mock client (we won't actually use it for local mode)
	mockClient := &client.ClientWithResponses{}

	tests := []struct {
		name     string
		cfg      ServiceConfig
		env      map[string]string
		wantType string // "local" or "cloud"
		wantErr  bool
	}{
		{
			name: "explicit local flag",
			cfg: ServiceConfig{
				Local:   true,
				Region:  "oregon",
				OwnerId: "usr-test",
			},
			wantType: "local",
		},
		{
			name: "RENDER_USE_LOCAL_DEV=true",
			cfg: ServiceConfig{
				Region:  "oregon",
				OwnerId: "usr-test",
			},
			env: map[string]string{
				"RENDER_USE_LOCAL_DEV": "true",
			},
			wantType: "local",
		},
		{
			name: "default to cloud",
			cfg: ServiceConfig{
				Region:  "oregon",
				OwnerId: "usr-test",
			},
			wantType: "cloud",
		},
		{
			name: "local flag takes precedence over env var",
			cfg: ServiceConfig{
				Local:   true,
				Region:  "oregon",
				OwnerId: "usr-test",
			},
			env: map[string]string{
				"RENDER_USE_LOCAL_DEV": "false",
			},
			wantType: "local",
		},
		{
			name:    "missing region",
			cfg:     ServiceConfig{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			svc, err := NewService(mockClient, tt.cfg)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			switch tt.wantType {
			case "local":
				_, ok := svc.(*LocalService)
				require.True(t, ok, "expected LocalService, got %T", svc)
			case "cloud":
				_, ok := svc.(*CloudService)
				require.True(t, ok, "expected CloudService, got %T", svc)
			}
		})
	}
}

func TestNewLocalServiceWithConfig(t *testing.T) {
	tests := []struct {
		name      string
		cfg       ServiceConfig
		wantErr   bool
		wantOwner string
	}{
		{
			name: "with ownerId",
			cfg: ServiceConfig{
				Region:  "oregon",
				OwnerId: "usr-123",
			},
			wantOwner: "usr-123",
		},
		{
			name: "missing region",
			cfg: ServiceConfig{
				OwnerId: "usr-123",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, err := newLocalServiceWithConfig(tt.cfg)

			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), "region is required")
				return
			}

			require.NoError(t, err)
			require.NotNil(t, svc)
			require.Equal(t, tt.wantOwner, svc.bucketName)
			require.Equal(t, tt.cfg.Region, svc.region)
		})
	}
}

func TestNewCloudServiceWithConfig(t *testing.T) {
	mockClient := &client.ClientWithResponses{}

	tests := []struct {
		name     string
		cfg      ServiceConfig
		wantErr  bool
		wantOwner string
		wantRegion string
	}{
		{
			name: "with ownerId and region",
			cfg: ServiceConfig{
				Region:  "oregon",
				OwnerId: "usr-123",
			},
			wantOwner:  "usr-123",
			wantRegion: "oregon",
		},
		{
			name: "missing region",
			cfg: ServiceConfig{
				OwnerId: "usr-123",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, err := newCloudServiceWithConfig(mockClient, tt.cfg)

			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), "region is required")
				return
			}

			require.NoError(t, err)
			require.NotNil(t, svc)
			require.Equal(t, tt.wantOwner, svc.ownerId)
			require.Equal(t, tt.wantRegion, svc.region)
		})
	}
}

func TestNewServiceFromContext_LocalMode(t *testing.T) {
	tests := []struct {
		name string
		cfg  ServiceConfig
		env  map[string]string
	}{
		{
			name: "explicit local flag",
			cfg: ServiceConfig{
				Local:   true,
				Region:  "oregon",
				OwnerId: "usr-test",
			},
		},
		{
			name: "RENDER_USE_LOCAL_DEV=true",
			cfg: ServiceConfig{
				Region:  "oregon",
				OwnerId: "usr-test",
			},
			env: map[string]string{
				"RENDER_USE_LOCAL_DEV": "true",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			svc, err := NewServiceFromContext(context.Background(), tt.cfg)
			require.NoError(t, err)

			_, ok := svc.(*LocalService)
			require.True(t, ok, "expected LocalService, got %T", svc)
		})
	}
}

// TestIsLocalMode tests the env var detection.
// Both CLI and SDK support RENDER_USE_LOCAL_DEV=true for local mode.
func TestIsLocalMode(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     bool
	}{
		{
			name:     "RENDER_USE_LOCAL_DEV=true",
			envValue: "true",
			want:     true,
		},
		{
			name:     "RENDER_USE_LOCAL_DEV=false",
			envValue: "false",
			want:     false,
		},
		{
			name:     "RENDER_USE_LOCAL_DEV not set",
			envValue: "",
			want:     false,
		},
		{
			name:     "RENDER_USE_LOCAL_DEV=1",
			envValue: "1",
			want:     false, // Only "true" is accepted
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				t.Setenv("RENDER_USE_LOCAL_DEV", tt.envValue)
			} else {
				os.Unsetenv("RENDER_USE_LOCAL_DEV")
			}

			got := IsLocalMode()
			require.Equal(t, tt.want, got)
		})
	}
}
