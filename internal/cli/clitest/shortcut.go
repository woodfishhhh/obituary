package clitest

// ShortcutCodec exercises installation ownership without activating COM.
type ShortcutCodec struct{}

func (ShortcutCodec) Encode(target string) ([]byte, error) { return []byte(target), nil }
func (ShortcutCodec) Target(data []byte) (string, error)   { return string(data), nil }
