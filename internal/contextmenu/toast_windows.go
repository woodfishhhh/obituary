package contextmenu

import (
	"bytes"
	"encoding/xml"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	iidDocumentIO   = windows.GUID{Data1: 0x6cd0e74e, Data2: 0xee65, Data3: 0x4489, Data4: [8]byte{0x9e, 0xbf, 0xca, 0x43, 0xe8, 0x7b, 0xa6, 0x37}}
	iidDocument     = windows.GUID{Data1: 0xf7f3a506, Data2: 0x1e87, Data3: 0x42d6, Data4: [8]byte{0xbc, 0xfb, 0xb8, 0xc8, 0x09, 0xfa, 0x54, 0x94}}
	iidToastFactory = windows.GUID{Data1: 0x04124b20, Data2: 0x82c6, Data3: 0x4229, Data4: [8]byte{0xb1, 0x09, 0xfd, 0x9e, 0xd4, 0x66, 0x2b, 0x53}}
	iidToastManager = windows.GUID{Data1: 0x50ac103f, Data2: 0xd235, Data3: 0x4598, Data4: [8]byte{0xbb, 0xef, 0x98, 0xfe, 0x4d, 0x1a, 0x3a, 0xd4}}
)

// Notify requests Windows' short toast duration (normally a few seconds).
// Accessibility/notification preferences remain controlled by Windows.
func Notify(title, message string) error {
	return withWinRT(func() error {
		toast, err := makeToast(toastXML(title, message))
		if err != nil {
			return err
		}
		defer toast.release()
		notifier, err := makeNotifier()
		if err != nil {
			return err
		}
		defer notifier.release()
		return winrtError(notifier.call(6, uintptr(unsafe.Pointer(toast))), "show toast")
	})
}

func toastXML(title, message string) string {
	var text bytes.Buffer
	text.WriteString(`<toast duration="short"><visual><binding template="ToastGeneric"><text>`)
	_ = xml.EscapeText(&text, []byte(shortText(title))) // bytes.Buffer writes cannot fail.
	text.WriteString(`</text><text>`)
	_ = xml.EscapeText(&text, []byte(shortText(message)))
	text.WriteString(`</text></binding></visual><audio silent="true"/></toast>`)
	return text.String()
}

func shortText(value string) string {
	runes := []rune(value)
	if len(runes) > 400 {
		return string(runes[:400]) + "…"
	}
	return value
}

func loadDocument(text string) (*runtimeObject, error) {
	instance, err := activate("Windows.Data.Xml.Dom.XmlDocument", nil)
	if err != nil {
		return nil, err
	}
	defer instance.release()
	io, err := instance.query(&iidDocumentIO)
	if err != nil {
		return nil, err
	}
	defer io.release()
	err = withHString(text, func(value uintptr) error {
		return winrtError(io.call(6, value), "load toast XML")
	})
	if err != nil {
		return nil, err
	}
	return instance.query(&iidDocument)
}

func makeToast(text string) (*runtimeObject, error) {
	document, err := loadDocument(text)
	if err != nil {
		return nil, err
	}
	defer document.release()
	factory, err := activate("Windows.UI.Notifications.ToastNotification", &iidToastFactory)
	if err != nil {
		return nil, err
	}
	defer factory.release()
	var toast *runtimeObject
	err = winrtError(factory.call(6, uintptr(unsafe.Pointer(document)), uintptr(unsafe.Pointer(&toast))), "create toast")
	return toast, err
}

func makeNotifier() (*runtimeObject, error) {
	manager, err := activate("Windows.UI.Notifications.ToastNotificationManager", &iidToastManager)
	if err != nil {
		return nil, err
	}
	defer manager.release()
	var notifier *runtimeObject
	err = withHString(AppID, func(id uintptr) error {
		return winrtError(manager.call(7, id, uintptr(unsafe.Pointer(&notifier))), "create toast notifier")
	})
	return notifier, err
}
