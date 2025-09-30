package logs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gorilla/websocket"

	"github.com/render-oss/cli/pkg/client"
	lclient "github.com/render-oss/cli/pkg/client/logs"
	"github.com/render-oss/cli/pkg/config"
)

func NewLogRepo(c *client.ClientWithResponses, apiConfig *config.APIConfig) *LogRepo {
	return &LogRepo{c: c, apiConfig: apiConfig}
}

type LogRepo struct {
	c         *client.ClientWithResponses
	apiConfig *config.APIConfig
}

func (l *LogRepo) ListLogs(ctx context.Context, params *client.ListLogsParams) (*client.Logs200Response, error) {
	logs, err := l.c.ListLogsWithResponse(ctx, params)
	if err != nil {
		return nil, err
	}

	if err := client.ErrorFromResponse(logs); err != nil {
		return nil, err
	}

	return logs.JSON200, nil
}

func (l *LogRepo) TailLogs(ctx context.Context, params *client.ListLogsParams) (<-chan *lclient.Log, error) {
	subscribeParams := client.SubscribeLogsParams(*params)
	req, err := client.NewSubscribeLogsRequest(l.apiConfig.Host, &subscribeParams)
	if err != nil {
		return nil, err
	}
	dialer := websocket.Dialer{}

	u := req.URL

	if u.Scheme == "https" {
		u.Scheme = "wss"
	} else {
		u.Scheme = "ws"
	}

	// Establish WebSocket connection using the custom dialer
	conn, resp, err := dialer.Dial(u.String(), client.AddHeaders(http.Header{}, l.apiConfig.Key))
	if err != nil {
		// Return the http error if it exists, fall back to the websocket error
		if resp != nil && resp.StatusCode != http.StatusSwitchingProtocols {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, err
			}

			return nil, fmt.Errorf("failed to tail logs: %s", body)
		}

		return nil, err
	}

	ch := make(chan *lclient.Log)

	// Read messages from the WebSocket connection
	go func(ctx context.Context) {
		defer conn.Close()
		defer close(ch)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				_, message, err := conn.ReadMessage()
				if err != nil {
					return
				}

				var log lclient.Log
				err = json.Unmarshal(message, &log)
				if err != nil {
					return
				}

				ch <- &log
			}
		}
	}(ctx)

	return ch, nil
}
