package logs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/renderinc/render-cli/pkg/cfg"
	"github.com/renderinc/render-cli/pkg/client"
	lclient "github.com/renderinc/render-cli/pkg/client/logs"
)

func NewLogRepo(c *client.ClientWithResponses) *LogRepo {
	return &LogRepo{c: c}
}

type LogRepo struct {
	c *client.ClientWithResponses
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
	req, err := client.NewSubscribeLogsRequest(cfg.GetHost(), &subscribeParams)
	dialer := websocket.Dialer{}

	u := req.URL
	u.Scheme = "wss"

	// Establish WebSocket connection using the custom dialer
	conn, _, err := dialer.Dial(u.String(), http.Header{
		"authorization": []string{fmt.Sprintf("Bearer %s", cfg.GetAPIKey())},
	})
	if err != nil {
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
