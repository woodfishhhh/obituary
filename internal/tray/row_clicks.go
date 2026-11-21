//go:build windows || ((darwin || linux) && desktop)

package tray

import "context"

// listen hands a row's clicks to the menu loop until the rows are rebuilt.
func (m *menu) listen(ctx context.Context, row menuItem, action func() error) {
	for {
		select {
		case <-ctx.Done():
			return
		case _, open := <-row.clicks():
			if !open { // Removing a native row closes its click channel.
				return
			}
			select {
			case <-ctx.Done():
				return
			case m.actions <- action:
			}
		}
	}
}
