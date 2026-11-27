package app

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/daemonipc"
	"github.com/woodfishhhh/obituary/internal/i18n"
	"github.com/woodfishhhh/obituary/internal/listing"
)

// serveHookEndpoint lets hooks hand their writes to this resident, and lets
// `obit list` have its output printed from the ledger this resident already holds.
// It returns the function that closes the endpoint, or nil when there is none:
// hooks then register directly and the list reads the ledger itself, as they did
// before a resident existed, and the resident's other work is unaffected.
func serveHookEndpoint(svc *lifecycle.Service, logger *log.Logger) func() error {
	if os.Getenv("OBIT_TEST_NO_RESIDENT_IPC") == "1" { // tests of the direct-write fallback
		return nil
	}
	stop, err := daemonipc.Serve(svc.Store.Home, daemonipc.Batch(svc.Store.AcceptObservations), svc.Store.NoticesQuiet,
		daemonipc.WithList(func(req daemonipc.ListRequest) ([]byte, error) { return printList(svc, req) }))
	if err != nil {
		logger.Printf("hook endpoint unavailable, hooks register directly: %v", err)
		return nil
	}
	return stop
}

// printList is the output of `obit list` for the caller's settings. The ledger is
// read as the command reads it (committed, never waiting for a writer), but the
// decoded copy is kept until store.json changes.
func printList(svc *lifecycle.Service, req daemonipc.ListRequest) ([]byte, error) {
	if language := i18n.Language(req.Language); language != i18n.English && language != i18n.Chinese {
		return nil, fmt.Errorf("unsupported language %q", req.Language)
	}
	state, err := svc.Store.ReadActiveView()
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	err = listing.Write(&out, state.Items, time.Now(), listing.Options{Language: i18n.Language(req.Language), JSON: req.JSON, StableWindow: req.StableWindow})
	return out.Bytes(), err
}
