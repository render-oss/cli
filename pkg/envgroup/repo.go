package envgroup

import (
	"context"
	"fmt"

	"github.com/render-oss/cli/pkg/client"
)

type Repo struct{ client *client.ClientWithResponses }

func NewRepo(c *client.ClientWithResponses) *Repo { return &Repo{client: c} }

func (r *Repo) Get(ctx context.Context, id string) ([]client.EnvVar, error) {
	resp, err := r.client.RetrieveEnvGroupWithResponse(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("get environment group: unexpected response %s", resp.Status())
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("get environment group: success response missing group body")
	}
	if resp.JSON200.EnvVars == nil {
		return nil, fmt.Errorf("environment variables were not returned; check your permission to read secret values")
	}
	return *resp.JSON200.EnvVars, nil
}

func (r *Repo) Set(ctx context.Context, id, key, value string) error {
	var body client.AddUpdateEnvVarInput
	if err := body.FromEnvVarValue(client.EnvVarValue{Value: value}); err != nil {
		return err
	}
	resp, err := r.client.UpdateEnvGroupEnvVarWithResponse(ctx, id, key, body)
	if err != nil {
		return err
	}
	if err := client.ErrorFromResponse(resp); err != nil {
		return err
	}
	if resp.StatusCode() != 200 {
		return fmt.Errorf("set environment group variable: unexpected response %s", resp.Status())
	}
	return nil
}
