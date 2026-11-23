// Package daemonipc delivers verified hook observations to the resident
// process. The resident acknowledges only after the durable observation queue
// has accepted the request; Git and ledger processing happen afterwards.
package daemonipc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
)

const protocolVersion = 1

var ErrUnavailable = errors.New("obituary resident IPC unavailable")

type request struct {
	Version      int
	Observations []core.Observation
}

type response struct {
	Version  int
	Accepted int
	Error    string
}

type Handler func([]core.Observation) error

// Serve starts the local resident endpoint and returns its shutdown function.
func Serve(home string, handler Handler) (func() error, error) {
	listener, cleanup, err := listen(home)
	if err != nil {
		return nil, err
	}
	done := make(chan struct{})
	go serveListener(listener, handler, done)
	return func() error {
		err := listener.Close()
		<-done
		return errors.Join(err, cleanup())
	}, nil
}

// Submit waits only for durable queue acceptance. A missing or stopped
// resident is reported distinctly so callers can use their direct-write path.
func Submit(ctx context.Context, home string, observations []core.Observation) error {
	if len(observations) == 0 {
		return nil
	}
	conn, err := dial(ctx, home)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	defer conn.Close()
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(250 * time.Millisecond)
	}
	_ = conn.SetDeadline(deadline)
	if err := json.NewEncoder(conn).Encode(request{Version: protocolVersion, Observations: observations}); err != nil {
		return fmt.Errorf("resident IPC request: %w", err)
	}
	var reply response
	if err := json.NewDecoder(io.LimitReader(conn, 64<<10)).Decode(&reply); err != nil {
		return fmt.Errorf("resident IPC response: %w", err)
	}
	if reply.Version != protocolVersion {
		return fmt.Errorf("resident IPC protocol version %d", reply.Version)
	}
	if reply.Error != "" {
		return errors.New(reply.Error)
	}
	if reply.Accepted != len(observations) {
		return fmt.Errorf("resident IPC accepted %d of %d observations", reply.Accepted, len(observations))
	}
	return nil
}

func serveListener(listener net.Listener, handler Handler, done chan<- struct{}) {
	defer close(done)
	var clients sync.WaitGroup
	defer clients.Wait()
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		clients.Go(func() { serveConn(conn, handler) })
	}
}

func serveConn(conn net.Conn, handler Handler) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	var req request
	if err := json.NewDecoder(io.LimitReader(conn, 2<<20)).Decode(&req); err != nil {
		_ = writeResponse(conn, response{Version: protocolVersion, Error: "invalid resident IPC request"})
		return
	}
	if req.Version != protocolVersion || len(req.Observations) == 0 || len(req.Observations) > 128 {
		_ = writeResponse(conn, response{Version: protocolVersion, Error: "invalid resident IPC request"})
		return
	}
	if err := handler(req.Observations); err != nil {
		_ = writeResponse(conn, response{Version: protocolVersion, Error: err.Error()})
		return
	}
	_ = writeResponse(conn, response{Version: protocolVersion, Accepted: len(req.Observations)})
}

func writeResponse(conn net.Conn, reply response) error {
	return json.NewEncoder(conn).Encode(reply)
}
