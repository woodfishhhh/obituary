package app

import (
	"log"
	"os"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/daemonipc"
)

// serveHookEndpoint lets hooks hand their writes to this resident. It returns
// the function that closes the endpoint, or nil when there is none: hooks then
// register directly, as they did before a resident existed, and the resident's
// other work is unaffected.
func serveHookEndpoint(svc *core.Service, logger *log.Logger) func() error {
	if os.Getenv("OBIT_TEST_NO_RESIDENT_IPC") == "1" { // tests of the direct-write fallback
		return nil
	}
	stop, err := daemonipc.Serve(svc.Store.Home, daemonipc.Batch(svc.Store.AcceptObservations))
	if err != nil {
		logger.Printf("hook endpoint unavailable, hooks register directly: %v", err)
		return nil
	}
	return stop
}
