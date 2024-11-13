package devicegrant_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/renderinc/cli/pkg/client/devicegrant"
)

func TestClient_CreateGrant(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "POST", r.Method)
		require.Equal(t, "/device-grant", r.URL.Path)

		_, err := w.Write([]byte(deviceGrantResp))
		require.NoError(t, err)
	}))

	c := devicegrant.NewClient(s.URL)

	dg, err := c.CreateGrant(context.Background())
	require.NoError(t, err)

	assert.Equal(t, &devicegrant.DeviceGrant{
		DeviceCode:      "some device code",
		UserCode:        "some user code",
		VerificationUri: "some verification uri",
		ExpiresIn:       1,
		Interval:        2,
	}, dg)
}

func TestClient_GetDeviceToken(t *testing.T) {
	t.Run("it gets the device token", func(t *testing.T) {
		var gotBody devicegrant.TokenRequestBody
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "POST", r.Method)
			require.Equal(t, "/device-token", r.URL.Path)

			require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))

			_, err := w.Write([]byte(deviceTokenResp))
			require.NoError(t, err)
		}))

		c := devicegrant.NewClient(s.URL)

		token, err := c.GetDeviceToken(context.Background(), &devicegrant.DeviceGrant{
			DeviceCode: "some device code",
		})
		require.NoError(t, err)

		assert.Equal(t, "some device token", token)

		assert.Equal(t, "some device code", gotBody.DeviceCode)
		assert.NotZero(t, gotBody.ClientID)
	})

	t.Run("it returns an authorization pending error if grant is pending", func(t *testing.T) {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, err := w.Write([]byte(`{"error": "authorization_pending"}`))
			require.NoError(t, err)
		}))

		c := devicegrant.NewClient(s.URL)

		_, err := c.GetDeviceToken(context.Background(), &devicegrant.DeviceGrant{
			DeviceCode: "some device code",
		})
		require.ErrorIs(t, err, devicegrant.ErrAuthorizationPending)
	})
}

const deviceGrantResp = `{
	"deviceCode": "some device code",
	"userCode": "some user code",
	"verificationUri": "some verification uri",
	"expiresIn": 1,
	"interval": 2
}`

const deviceTokenResp = `{"deviceToken": "some device token"}`
