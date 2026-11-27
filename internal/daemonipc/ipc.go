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

	"github.com/woodfishhhh/obituary/internal/core/ledger"
)

const protocolVersion = 1

var ErrUnavailable = errors.New("obituary resident IPC unavailable")

type request struct {
	Version      int
	Observations []ledger.Observation
	// List asks for the output of `obit list` instead of delivering observations.
	// A resident that predates it finds no observations and refuses the request,
	// which the caller answers by reading the ledger itself.
	List *ListRequest `json:",omitempty"`
}

// ListRequest carries what the caller's own configuration decides about its output.
type ListRequest struct {
	Language     string
	JSON         bool
	StableWindow time.Duration
}

// ListHandler returns the bytes `obit list` prints for the request.
type ListHandler func(ListRequest) ([]byte, error)

type response struct {
	Version  int
	Accepted int
	Error    string
	// Quiet: the ledger holds nothing to tell an agent, so the hook need not read
	// it. Absent (false) means "read it", which is what an older resident implies.
	Quiet bool `json:",omitempty"`
	// Output is the reply to a list request.
	Output []byte `json:",omitempty"`
}

type Handler func([]ledger.Observation) error

// Receipt is what a hook learns from an accepted request besides the fact.
type Receipt struct{ Quiet bool }

type server struct {
	handler Handler
	quiet   func() bool
	list    ListHandler
}

// Option adds a capability to the endpoint.
type Option func(*server)

// WithList lets the endpoint print `obit list` for its callers.
func WithList(list ListHandler) Option { return func(s *server) { s.list = list } }

// Serve starts the local resident endpoint and returns its shutdown function.
// quiet, if not nil, is asked after each accepted request.
func Serve(home string, handler Handler, quiet func() bool, options ...Option) (func() error, error) {
	listener, cleanup, err := listen(home)
	if err != nil {
		return nil, err
	}
	srv := &server{handler: handler, quiet: quiet}
	for _, option := range options {
		option(srv)
	}
	done := make(chan struct{})
	go srv.serve(listener, done)
	return func() error {
		err := listener.Close()
		<-done
		return errors.Join(err, cleanup())
	}, nil
}

// Submit waits only for durable queue acceptance. A missing or stopped
// resident is reported distinctly so callers can use their direct-write path.
func Submit(ctx context.Context, home string, observations []ledger.Observation) (Receipt, error) {
	if len(observations) == 0 {
		return Receipt{}, nil
	}
	conn, err := dial(ctx, home)
	if err != nil {
		return Receipt{}, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	defer conn.Close()
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(250 * time.Millisecond)
	}
	_ = conn.SetDeadline(deadline)
	if err := json.NewEncoder(conn).Encode(request{Version: protocolVersion, Observations: observations}); err != nil {
		return Receipt{}, fmt.Errorf("resident IPC request: %w", err)
	}
	var reply response
	if err := json.NewDecoder(io.LimitReader(conn, 64<<10)).Decode(&reply); err != nil {
		return Receipt{}, fmt.Errorf("resident IPC response: %w", err)
	}
	if reply.Version != protocolVersion {
		return Receipt{}, fmt.Errorf("resident IPC protocol version %d", reply.Version)
	}
	if reply.Error != "" {
		return Receipt{}, errors.New(reply.Error)
	}
	if reply.Accepted != len(observations) {
		return Receipt{}, fmt.Errorf("resident IPC accepted %d of %d observations", reply.Accepted, len(observations))
	}
	return Receipt{Quiet: reply.Quiet}, nil
}

func (srv *server) serve(listener net.Listener, done chan<- struct{}) {
	defer close(done)
	var clients sync.WaitGroup
	defer clients.Wait()
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		clients.Go(func() { srv.serveConn(conn) })
	}
}

func (srv *server) serveConn(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	var req request
	if err := json.NewDecoder(io.LimitReader(conn, 2<<20)).Decode(&req); err != nil {
		_ = writeResponse(conn, response{Version: protocolVersion, Error: "invalid resident IPC request"})
		return
	}
	if req.Version == protocolVersion && req.List != nil && len(req.Observations) == 0 && srv.list != nil {
		srv.serveList(conn, *req.List)
		return
	}
	if req.Version != protocolVersion || len(req.Observations) == 0 || len(req.Observations) > 128 || req.List != nil {
		_ = writeResponse(conn, response{Version: protocolVersion, Error: "invalid resident IPC request"})
		return
	}
	if err := srv.handler(req.Observations); err != nil {
		_ = writeResponse(conn, response{Version: protocolVersion, Error: err.Error()})
		return
	}
	_ = writeResponse(conn, response{Version: protocolVersion, Accepted: len(req.Observations), Quiet: srv.quiet != nil && srv.quiet()})
}

func (srv *server) serveList(conn net.Conn, req ListRequest) {
	out, err := srv.list(req)
	if err != nil {
		_ = writeResponse(conn, response{Version: protocolVersion, Error: err.Error()})
		return
	}
	_ = writeResponse(conn, response{Version: protocolVersion, Output: out})
}

func writeResponse(conn net.Conn, reply response) error {
	return json.NewEncoder(conn).Encode(reply)
}

// maxListReply bounds what a caller will buffer for one list (10,000 entries print in about 2 MiB).
const maxListReply = 64 << 20

// List asks the resident for the output of `obit list`. Any error, including a
// resident that is absent, older or busy, means the caller prints the list itself;
// nothing has been written by then.
func List(ctx context.Context, home string, req ListRequest) ([]byte, error) {
	conn, err := dial(ctx, home)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	defer conn.Close()
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(2 * time.Second)
	}
	_ = conn.SetDeadline(deadline)
	if err := json.NewEncoder(conn).Encode(request{Version: protocolVersion, List: &req}); err != nil {
		return nil, fmt.Errorf("resident IPC request: %w", err)
	}
	var reply response
	if err := json.NewDecoder(io.LimitReader(conn, maxListReply)).Decode(&reply); err != nil {
		return nil, fmt.Errorf("resident IPC response: %w", err)
	}
	switch {
	case reply.Version != protocolVersion:
		return nil, fmt.Errorf("resident IPC protocol version %d", reply.Version)
	case reply.Error != "":
		return nil, errors.New(reply.Error)
	case len(reply.Output) == 0:
		return nil, errors.New("resident IPC list reply is empty")
	}
	return reply.Output, nil
}
