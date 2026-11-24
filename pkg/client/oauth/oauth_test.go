package oauth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/client/oauth"
)

func TestClient_CreateGrant(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "POST", r.Method)
		require.Equal(t, "/device-grant", r.URL.Path)

		_, err := w.Write([]byte(deviceGrantResp))
		require.NoError(t, err)
	}))

	c := oauth.NewClient(s.URL)

	dg, err := c.CreateGrant(context.Background())
	require.NoError(t, err)

	assert.Equal(t, &oauth.DeviceGrant{
		DeviceCode:              "some device code",
		UserCode:                "some user code",
		VerificationUri:         "some verification uri",
		VerificationUriComplete: "some complete verification uri",
		ExpiresIn:               1,
		Interval:                2,
	}, dg)
}

func TestClient_GetDeviceToken(t *testing.T) {
	t.Run("it gets the device token", func(t *testing.T) {
		var gotBody oauth.TokenRequestBody
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "POST", r.Method)
			require.Equal(t, "/device-token", r.URL.Path)

			require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))

			_, err := w.Write([]byte(deviceTokenResp))
			require.NoError(t, err)
		}))

		c := oauth.NewClient(s.URL)

		token, err := c.GetDeviceTokenResponse(context.Background(), &oauth.DeviceGrant{
			DeviceCode: "some device code",
		})
		require.NoError(t, err)

		assert.Equal(t, "some device token", token.AccessToken)

		assert.Equal(t, "some device code", gotBody.DeviceCode)
		assert.NotZero(t, gotBody.ClientID)
	})

	t.Run("it returns an authorization pending error if grant is pending", func(t *testing.T) {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, err := w.Write([]byte(`{"error": "authorization_pending"}`))
			require.NoError(t, err)
		}))

		c := oauth.NewClient(s.URL)

		_, err := c.GetDeviceTokenResponse(context.Background(), &oauth.DeviceGrant{
			DeviceCode: "some device code",
		})
		require.ErrorIs(t, err, oauth.ErrAuthorizationPending)
	})
}

func TestClient_RefreshToken(t *testing.T) {
	t.Run("it returns the rotated token", func(t *testing.T) {
		var gotBody oauth.RefreshTokenRequestBody
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "POST", r.Method)
			require.Equal(t, "/token/refresh/", r.URL.Path)
			require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))

			_, err := w.Write([]byte(`{"access_token": "new access token", "refresh_token": "new refresh token", "expires_in": 60}`))
			require.NoError(t, err)
		}))
		defer s.Close()

		token, err := oauth.NewClient(s.URL).RefreshToken(context.Background(), "some refresh token")
		require.NoError(t, err)

		assert.Equal(t, "new access token", token.AccessToken)
		assert.Equal(t, "new refresh token", token.RefreshToken)
		assert.Equal(t, "refresh_token", gotBody.GrantType)
		assert.Equal(t, "some refresh token", gotBody.RefreshToken)
	})

	tests := []struct {
		name             string
		status           int
		header           map[string]string
		body             string
		wantErr          oauth.ResponseError
		wantInvalidGrant bool
	}{
		{
			name:    "server unavailable",
			status:  http.StatusServiceUnavailable,
			body:    `{"error":"temporarily_unavailable"}`,
			wantErr: oauth.ResponseError{StatusCode: http.StatusServiceUnavailable, Code: "temporarily_unavailable"},
		},
		{
			name:    "rate limited",
			status:  http.StatusTooManyRequests,
			header:  map[string]string{"Retry-After": "60"},
			body:    `{"error":"slow_down"}`,
			wantErr: oauth.ResponseError{StatusCode: http.StatusTooManyRequests, Code: "slow_down", RetryAfter: 60 * time.Second},
		},
		{
			name:    "non-JSON server error",
			status:  http.StatusBadGateway,
			body:    "bad gateway",
			wantErr: oauth.ResponseError{StatusCode: http.StatusBadGateway, Body: "bad gateway"},
		},
		{
			name:    "server error without body",
			status:  http.StatusInternalServerError,
			wantErr: oauth.ResponseError{StatusCode: http.StatusInternalServerError},
		},
		{
			name:             "invalid grant",
			status:           http.StatusBadRequest,
			body:             `{"error":"invalid_grant"}`,
			wantErr:          oauth.ResponseError{StatusCode: http.StatusBadRequest, Code: "invalid_grant"},
			wantInvalidGrant: true,
		},
		{
			name:    "invalid grant code on server error",
			status:  http.StatusInternalServerError,
			body:    `{"error":"invalid_grant"}`,
			wantErr: oauth.ResponseError{StatusCode: http.StatusInternalServerError, Code: "invalid_grant"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				for k, v := range tt.header {
					w.Header().Set(k, v)
				}
				w.WriteHeader(tt.status)
				_, err := w.Write([]byte(tt.body))
				require.NoError(t, err)
			}))
			defer s.Close()

			_, err := oauth.NewClient(s.URL).RefreshToken(context.Background(), "some refresh token")

			var respErr *oauth.ResponseError
			require.ErrorAs(t, err, &respErr)
			assert.Equal(t, tt.wantErr, *respErr)
			assert.Equal(t, tt.wantInvalidGrant, oauth.IsInvalidGrant(err))
		})
	}

	t.Run("a transport error is not an invalid grant", func(t *testing.T) {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		s.Close()

		_, err := oauth.NewClient(s.URL).RefreshToken(context.Background(), "some refresh token")
		require.Error(t, err)
		assert.False(t, oauth.IsInvalidGrant(err))
	})
}

const deviceGrantResp = `{
	"device_code": "some device code",
	"user_code": "some user code",
	"verification_uri": "some verification uri",
	"verification_uri_complete": "some complete verification uri",
	"expires_in": 1,
	"interval": 2
}`

const deviceTokenResp = `{"access_token": "some device token"}`
