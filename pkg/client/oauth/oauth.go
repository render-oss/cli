package oauth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/render-oss/cli/pkg/cfg"
)

const cliOauthClientID = "429024F5E608930E2A65EF92591A25CC"
const authorizationPendingAPIMsg = "authorization_pending"

var ErrAuthorizationPending = errors.New("authorization pending")

type DeviceGrant struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationUri         string `json:"verification_uri"`
	VerificationUriComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

type GrantRequestBody struct {
	ClientID string `json:"client_id"`
}

type DeviceToken struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

type TokenRequestBody struct {
	GrantType  string `json:"grant_type"`
	ClientID   string `json:"client_id"`
	DeviceCode string `json:"device_code"`
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
	req.Header = cfg.AddUserAgent(req.Header)
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

func (c *Client) GetDeviceTokenResponse(ctx context.Context, dg *DeviceGrant) (*DeviceToken, error) {
	body := &TokenRequestBody{
		ClientID: cliOauthClientID, DeviceCode: dg.DeviceCode,
		GrantType: "urn:ietf:params:oauth:grant-type:device_code",
	}

	var token DeviceToken
	err := c.postFor(ctx, "/device-token", body, &token)
	if err != nil {
		if err.Error() == authorizationPendingAPIMsg {
			return nil, ErrAuthorizationPending
		}

		return nil, err
	}

	return &token, nil
}

type RefreshTokenRequestBody struct {
	GrantType    string `json:"grant_type"`
	RefreshToken string `json:"refresh_token"`
}

func (c *Client) RefreshToken(ctx context.Context, refreshToken string) (*DeviceToken, error) {
	body := &RefreshTokenRequestBody{
		GrantType:    "refresh_token",
		RefreshToken: refreshToken,
	}

	var token DeviceToken
	err := c.postFor(ctx, "/token/refresh/", body, &token)
	if err != nil {
		return nil, err
	}

	return &token, nil
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
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
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
