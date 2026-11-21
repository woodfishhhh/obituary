package i18n

import "strings"

// Host adapters own the English metadata used by JSON and generated reference
// documents. Look up only complete metadata strings, never user paths or commands.
var hostMessageIDs = func() map[string]string {
	ids := map[string]string{}
	for id, m := range hostMessages {
		if strings.HasPrefix(id, "cli.host.") {
			ids[m.Other] = id
		}
	}
	return ids
}()

func (l *Localizer) HostText(source string) string {
	if l != nil && l.Language() != English {
		if id := hostMessageIDs[source]; id != "" {
			return l.Text(id, nil)
		}
	}
	return source
}
