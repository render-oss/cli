package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"

	"github.com/renderinc/render-cli/pkg/cfg"
	"github.com/renderinc/render-cli/pkg/config"
)

var ErrUnauthorized = errors.New("unauthorized")
var ErrForbidden = errors.New("forbidden")

var ErrLogin = errors.New("run `render login` to authenticate")

func NewDefaultClient() (*ClientWithResponses, error) {
	apiCfg := config.APIConfig{
		Key:  cfg.GetAPIKey(),
		Host: cfg.GetHost(),
	}

	var err error
	if apiCfg.Key == "" {
		apiCfg, err = config.GetAPIConfig()
		if err != nil || apiCfg.Key == "" {
			return nil, ErrLogin
		}
	}

	if apiCfg.Host == "" {
		apiCfg.Host = cfg.GetHost()
	}

	return clientWithAuth(&http.Client{}, apiCfg)
}

func AddHeaders(header http.Header, token string) http.Header {
	header.Add("user-agent", "render-cli/"+cfg.Version)
	header.Add("authorization", fmt.Sprintf("Bearer %s", token))
	return header
}

func ErrorFromResponse(v any) error {
	responseErr := firstNonNilErrorField(v)
	if responseErr == nil {
		return nil
	}

	if responseErr.Code == http.StatusUnauthorized {
		return ErrUnauthorized
	}
	if responseErr.Code == http.StatusForbidden {
		return ErrForbidden
	}

	if responseErr.Message != nil && *responseErr.Message != "" {
		return fmt.Errorf("received response code %d: %s", responseErr.Code, *responseErr.Message)
	}

	return fmt.Errorf("unknown error")
}

type ErrorWithCode struct {
	Error
	Code int
}

func firstNonNilErrorField(response any) *ErrorWithCode {
	if reflect.TypeOf(response).Kind() == reflect.Ptr {
		return firstNonNilErrorField(reflect.ValueOf(response).Elem().Interface())
	}

	v := reflect.ValueOf(response)

	httpRespField := v.FieldByName("HTTPResponse")
	if !httpRespField.IsValid() {
		return nil
	}
	httpResponse, ok := httpRespField.Interface().(*http.Response)
	if !ok {
		couldNotReadResponse := "could not read HTTP response"
		return &ErrorWithCode{Error: Error{Message: &couldNotReadResponse}}
	}

	if httpResponse.StatusCode < 400 {
		return nil
	}

	body, ok := v.FieldByName("Body").Interface().([]byte)
	if !ok {
		couldNotReadBody := "could not read response body"
		return &ErrorWithCode{Error: Error{Message: &couldNotReadBody}}
	}

	var httpError Error
	if err := json.Unmarshal(body, &httpError); err != nil {
		stringBody := string(body)
		return &ErrorWithCode{Error: Error{Message: &stringBody}, Code: httpResponse.StatusCode}
	}

	return &ErrorWithCode{Error: httpError, Code: httpResponse.StatusCode}
}

func clientWithAuth(httpClient *http.Client, apiCfg config.APIConfig) (*ClientWithResponses, error) {
	insertAuth := func(ctx context.Context, req *http.Request) error {
		req.Header = AddHeaders(req.Header, apiCfg.Key)
		return nil
	}

	return NewClientWithResponses(apiCfg.Host, WithRequestEditorFn(insertAuth), WithHTTPClient(httpClient))
}

type paginationParams interface {
	SetCursor(cursor *Cursor)
	SetLimit(int)
}

func ListAll[T any, P paginationParams](ctx context.Context, params P, listPage func(ctx context.Context, params P) ([]T, *Cursor, error)) ([]T, error) {
	limit := 100
	params.SetLimit(limit)

	var res []T
	for {
		page, cursor, err := listPage(ctx, params)
		if err != nil {
			return nil, err
		}

		if len(page) == 0 {
			return res, nil
		}

		res = append(res, page...)

		if len(page) < limit {
			return res, nil
		}
		params.SetCursor(cursor)
	}
}
