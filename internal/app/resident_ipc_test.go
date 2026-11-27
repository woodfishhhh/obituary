package app

import (
	"bytes"
	"context"
	"io"
	"log"
	"path/filepath"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/daemonipc"
	"github.com/woodfishhhh/obituary/internal/i18n"
	"github.com/woodfishhhh/obituary/internal/listing"
)

// The resident prints what the command prints from the same ledger, in the
// caller's language and format, however it came by the entries.
func TestResidentPrintsTheListTheCommandWould(t *testing.T) {
	home := t.TempDir()
	svc := &lifecycle.Service{Store: ledger.Store{Home: home}}
	soon := time.Now().Add(100*time.Hour + 30*time.Second) // far from a change of the remaining-time text
	if err := svc.Store.Update(func(st *ledger.State) error {
		st.Items = []ledger.Item{
			{ID: 1, Path: filepath.Join(home, "plain"), ExpiresAt: soon, Note: "scratch"},
			{ID: 2, Path: filepath.Join(home, "stuck"), ExpiresAt: soon, Attempts: 12, NextAttempt: ledger.RetryStopped, LastError: "in use"},
			{ID: 3, Source: "watch", Path: filepath.Join(home, "watched"), ExpiresAt: soon, Watch: &ledger.WatchTiming{StableSince: time.Now()}},
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	svc.Store = svc.Store.WithResidentReplay()
	stop := serveHookEndpoint(svc, log.New(io.Discard, "", 0))
	if stop == nil {
		t.Fatal("the endpoint did not start")
	}
	defer stop()

	for _, req := range []daemonipc.ListRequest{
		{Language: "en"}, {Language: "zh-CN"}, {Language: "en", JSON: true, StableWindow: time.Hour},
	} {
		got, err := daemonipc.List(context.Background(), home, req)
		if err != nil {
			t.Fatalf("%+v: %v", req, err)
		}
		state, err := svc.Store.ReadActive()
		if err != nil {
			t.Fatal(err)
		}
		var want bytes.Buffer
		err = listing.Write(&want, state.Items, time.Now(), listing.Options{Language: i18n.Language(req.Language), JSON: req.JSON, StableWindow: req.StableWindow})
		if err != nil || !bytes.Equal(got, want.Bytes()) {
			t.Fatalf("%+v: the resident printed\n%s\nthe command prints\n%s\n%v", req, got, want.Bytes(), err)
		}
	}
	// i18n.NewLocalizer panics on a language it has no catalog for; a request must not reach it.
	if _, err := daemonipc.List(context.Background(), home, daemonipc.ListRequest{Language: "xx"}); err == nil {
		t.Fatal("an unsupported language must be refused")
	}
}
