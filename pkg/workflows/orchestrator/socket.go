package orchestrator

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/google/uuid"
)

type SocketTracker struct {
	sockets map[string]bool
	dir     string
}

func NewSocketTracker(ctx context.Context) (*SocketTracker, error) {
	dir, err := os.MkdirTemp("/tmp", "render-sdk-sockets-")
	if err != nil {
		return nil, err
	}

	go func() {
		<-ctx.Done()
		os.RemoveAll(dir)
	}()

	return &SocketTracker{
		sockets: make(map[string]bool),
		dir:     dir,
	}, nil
}

func (s *SocketTracker) NewSocket() (net.Listener, error) {
	name := fmt.Sprintf("%s/%s.sock", s.dir, uuid.New().String())

	err := os.RemoveAll(name)
	if err != nil {
		return nil, err
	}

	ln, err := net.Listen("unix", name)
	if err != nil {
		return nil, err
	}
	return ln, nil
}

func (s *SocketTracker) DeleteSocket(socketPath string) {
	err := os.Remove(socketPath)
	if err != nil {
		log.Println("error removing socket", err)
	}
	delete(s.sockets, socketPath)
}

func (s *SocketTracker) GetSocket(socketPath string) bool {
	return s.sockets[socketPath]
}
