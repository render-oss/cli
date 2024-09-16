package client

import (
	"context"
	"fmt"
	"net/http"
)

func ClientWithAuth(httpClient *http.Client, server string, token string) (*ClientWithResponses, error) {
	insertAuth := func(ctx context.Context, req *http.Request) error {
		req.Header.Add("authorization", fmt.Sprintf("Bearer %s", token))
		return nil
	}

	return NewClientWithResponses(server, WithRequestEditorFn(insertAuth), WithHTTPClient(httpClient))
}
