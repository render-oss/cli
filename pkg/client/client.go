package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"

	"github.com/renderinc/render-cli/pkg/cfg"
)

func NewDefaultClient() (*ClientWithResponses, error) {
	apiKey := cfg.GetAPIKey()
	if apiKey == "" {
		return nil, fmt.Errorf("no API key set for env var RENDER_API_KEY")
	}

	return clientWithAuth(
		&http.Client{},
		cfg.GetHost(),
		cfg.GetAPIKey(),
	)
}

func AddHeaders(header http.Header, token string) http.Header {
	header.Add("user-agent", "render-cli")
	header.Add("authorization", fmt.Sprintf("Bearer %s", token))
	return header
}

func ErrorFromResponse(v any) error {
	responseErr := firstNonNilErrorField(v)
	if responseErr == nil {
		return nil
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

func clientWithAuth(httpClient *http.Client, server string, token string) (*ClientWithResponses, error) {
	insertAuth := func(ctx context.Context, req *http.Request) error {
		req.Header = AddHeaders(req.Header, token)
		return nil
	}

	return NewClientWithResponses(server, WithRequestEditorFn(insertAuth), WithHTTPClient(httpClient))
}
