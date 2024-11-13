package devicegrant

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/renderinc/cli/pkg/client"
)

const cliOauthClientID = "429024F5E608930E2A65EF92591A25CC"
const authorizationPendingAPIMsg = "authorization_pending"

var ErrAuthorizationPending = errors.New("authorization pending")

type DeviceGrant struct {
	DeviceCode      string `json:"deviceCode"`
	UserCode        string `json:"userCode"`
	VerificationUri string `json:"verificationUri"`
	ExpiresIn       int    `json:"expiresIn"`
	Interval        int    `json:"interval"`
}

type GrantRequestBody struct {
	ClientID string `json:"clientId"`
}

type DeviceToken struct {
	DeviceToken string `json:"deviceToken"`
}

type TokenRequestBody struct {
	ClientID   string `json:"clientId"`
	DeviceCode string `json:"deviceCode"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type Client struct {
	c    *http.Client
	host string
}

func NewClient(host string) *Client {
	return &Client{
		c:    http.DefaultClient,
		host: host,
	}
}

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	req.Header = client.AddUserAgent(req.Header)
	return c.c.Do(req)
}

func (c *Client) CreateGrant(ctx context.Context) (*DeviceGrant, error) {
	body := &GrantRequestBody{ClientID: cliOauthClientID}

	var grant DeviceGrant
	err := c.postFor(ctx, "/device-grant", body, &grant)
	if err != nil {
		return nil, err
	}

	return &grant, nil
}

func (c *Client) GetDeviceToken(ctx context.Context, dg *DeviceGrant) (string, error) {
	body := &TokenRequestBody{ClientID: cliOauthClientID, DeviceCode: dg.DeviceCode}

	var token DeviceToken
	err := c.postFor(ctx, "/device-token", body, &token)
	if err != nil {
		if err.Error() == authorizationPendingAPIMsg {
			return "", ErrAuthorizationPending
		}

		return "", err
	}

	return token.DeviceToken, nil
}

func (c *Client) postFor(ctx context.Context, path string, body any, v any) error {
	bs, err := json.Marshal(body)
	if err != nil {
		return err
	}

	host := strings.TrimSuffix(c.host, "/")
	req, err := http.NewRequest(http.MethodPost, host+path, bytes.NewBuffer(bs))
	if err != nil {
		return err
	}

	req = req.WithContext(ctx)
	resp, err := c.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		if string(respBody) != "" {
			var errResp ErrorResponse
			err = json.Unmarshal(respBody, &errResp)
			if err == nil && errResp.Error != "" {
				return errors.New(errResp.Error)
			}

			return errors.New(string(respBody))
		}

		return fmt.Errorf("create device grant failed with status %d", resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(v)
}
