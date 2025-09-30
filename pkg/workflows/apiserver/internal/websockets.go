package internal

import "github.com/gorilla/websocket"

type WebSocketData struct {
	MessageType int
	Data        []byte
}

// WebsocketChannelWrapper wraps a websocket connection with a read and write channel.
// The read channel will receive messages from the websocket connection.
// The write channel can be used to send messages to the websocket connection.
// The websocket connection will be closed when the write channel is closed or
// when a close message is sent to the write channel.
func WebsocketChannelWrapper(ws *websocket.Conn) (readChannel <-chan WebSocketData, writeChannel chan<- WebSocketData) {
	rch := make(chan WebSocketData)
	wch := make(chan WebSocketData)

	go func() {
		defer close(rch)
		for {
			messageType, data, err := ws.ReadMessage()
			if err != nil {
				return
			}
			rch <- WebSocketData{MessageType: messageType, Data: data}
		}
	}()

	go func() {
		for data := range wch {
			err := ws.WriteMessage(data.MessageType, data.Data)
			if err != nil {
				return
			}

			if data.MessageType == websocket.CloseMessage {
				_ = ws.Close()
				return
			}
		}
		_ = closeGracefully(ws, "data transfer complete")
	}()

	return rch, wch
}

func closeGracefully(ws *websocket.Conn, reason string) error {
	err := ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, reason))
	if err != nil {
		return err
	}
	return ws.Close()
}
