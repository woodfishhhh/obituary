//go:build windows || ((darwin || linux) && desktop)

package tray

import (
	"context"
	"testing"
)

type testRow struct {
	menuItem
	ch chan struct{}
}

func (r testRow) clicks() <-chan struct{} { return r.ch }

func TestRemovingTrayRowDoesNotClickIt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m := &menu{actions: make(chan func() error)}
	for _, selected := range []bool{false, true} {
		row := testRow{ch: make(chan struct{}, 1)}
		if selected {
			row.ch <- struct{}{}
		}
		close(row.ch) // systray.Remove closes ClickedCh during a menu refresh.
		done := make(chan struct{})
		go func() { m.listen(ctx, row, func() error { return nil }); close(done) }()
		if selected {
			select {
			case <-m.actions:
			case <-done:
				t.Fatal("lost the selected row click")
			}
		}
		select {
		case <-done:
		case <-m.actions:
			t.Fatal("removing a tray row dispatched an unrequested action")
		}
	}
}
