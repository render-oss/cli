package oauth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/render-oss/cli/pkg/cfg"
)

const cliOauthClientID = "429024F5E608930E2A65EF92591A25CC"
const authorizationPendingAPIMsg = "authorization_pending"

// invalidGrantAPIMsg is the RFC 6749 section 5.2 error code for a refresh token
// that is invalid, expired, revoked, or was issued to another client.
const invalidGrantAPIMsg = "invalid_grant"

var ErrAuthorizationPending = errors.New("authorization pending")

// ResponseError is returned when an OAuth endpoint responds with a non-200
// status. It keeps the HTTP status and OAuth error code so callers can tell a
// definitive grant refusal apart from a temporary failure.
type ResponseError struct {
	// StatusCode is the HTTP status code of the response.
	StatusCode int
	// Code is the OAuth "error" field of the response body, if present.
	Code string
	// Body is the raw response body when it does not contain an OAuth error code.
	Body string
	// RetryAfter is the delay from a delta-seconds Retry-After header, if present.
	RetryAfter time.Duration
}

func (e *ResponseError) Error() string {
	if e.Code != "" {
		return e.Code
	}
	if e.Body != "" {
		return e.Body
	}
	return fmt.Sprintf("create device grant failed with status %d", e.StatusCode)
}

// IsInvalidGrant reports whether err is a definitive refusal of the grant
// (for example, a revoked or expired refresh token). Other failures, such as
// network errors, rate limiting, server errors, and malformed responses, are
// not proof that the grant is unusable and may succeed on a later attempt.
func IsInvalidGrant(err error) bool {
	var respErr *ResponseError
	if !errors.As(err, &respErr) {
		return false
	}
	return respErr.Code == invalidGrantAPIMsg &&
		(respErr.StatusCode == http.StatusBadRequest || respErr.StatusCode == http.StatusUnauthorized)
}

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

func (c *Client) RevokeToken(ctx context.Context, accessToken string) error {
	host := strings.TrimSuffix(c.host, "/")
	req, err := http.NewRequest(http.MethodPost, host+"/oauth/revoke", http.NoBody)
	if err != nil {
		return err
	}

	req = req.WithContext(ctx)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("revoke token failed with status %d", resp.StatusCode)
	}

	return nil
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
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		respErr := &ResponseError{
			StatusCode: resp.StatusCode,
			RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After")),
		}
		if string(respBody) != "" {
			var errResp ErrorResponse
			err = json.Unmarshal(respBody, &errResp)
			if err == nil && errResp.Error != "" {
				respErr.Code = errResp.Error
			} else {
				respErr.Body = string(respBody)
			}
		}

		return respErr
	}

	return json.NewDecoder(resp.Body).Decode(v)
}

// parseRetryAfter returns the delay from a delta-seconds Retry-After header.
// HTTP-date values and malformed headers yield zero.
func parseRetryAfter(v string) time.Duration {
	seconds, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || seconds < 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}
