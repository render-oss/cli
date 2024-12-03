package oauth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/renderinc/cli/pkg/client/oauth"
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

const deviceGrantResp = `{
	"device_code": "some device code",
	"user_code": "some user code",
	"verification_uri": "some verification uri",
	"verification_uri_complete": "some complete verification uri",
	"expires_in": 1,
	"interval": 2
}`

const deviceTokenResp = `{"access_token": "some device token"}`
