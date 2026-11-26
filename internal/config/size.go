package config

import (
	"fmt"
	"math/big"
	"strings"
)

// ParseByteSize parses a graveyard capacity limit such as "500MB", "2GB",
// "10GiB", "1024M", a plain byte count, or "" / "0" / "unlimited" for no limit.
// Units use binary powers of 1024 to match Obituary's size display.
func ParseByteSize(raw string) (int64, error) {
	s := strings.TrimSpace(raw)
	if s == "" || strings.EqualFold(s, "unlimited") {
		return 0, nil
	}
	numText, unitText, err := splitByteSize(s)
	if err != nil {
		return 0, err
	}
	mult, err := byteUnitMultiplier(unitText)
	if err != nil {
		return 0, err
	}
	return evalByteSize(numText, mult, raw)
}

// NormalizeByteSize validates raw and returns a clean config string alongside
// the parsed byte count. Unlimited values normalize to "".
func NormalizeByteSize(raw string) (string, int64, error) {
	n, err := ParseByteSize(raw)
	if err != nil {
		return "", 0, err
	}
	if n == 0 {
		return "", 0, nil
	}
	numText, unitText, _ := splitByteSize(strings.TrimSpace(raw))
	return numText + canonicalByteUnit(unitText), n, nil
}

func splitByteSize(s string) (numText, unitText string, err error) {
	if strings.HasPrefix(s, "-") {
		return "", "", fmt.Errorf("must not be negative: %q", s)
	}
	if strings.HasPrefix(s, "+") {
		return "", "", fmt.Errorf("invalid byte size %q", s)
	}
	i, dots := 0, 0
	for i < len(s) {
		ch := s[i]
		if ch >= '0' && ch <= '9' {
			i++
			continue
		}
		if ch == '.' && dots == 0 {
			dots++
			i++
			continue
		}
		break
	}
	if i == 0 || s[:i] == "." {
		return "", "", fmt.Errorf("invalid byte size %q", s)
	}
	numText = s[:i]
	if strings.HasPrefix(numText, ".") || strings.HasSuffix(numText, ".") {
		return "", "", fmt.Errorf("invalid byte size %q", s)
	}
	unitText = strings.TrimSpace(s[i:])
	return numText, unitText, nil
}

func byteUnitMultiplier(unit string) (int64, error) {
	switch strings.ToLower(unit) {
	case "", "b":
		return 1, nil
	case "k", "kb", "kib":
		return 1 << 10, nil
	case "m", "mb", "mib":
		return 1 << 20, nil
	case "g", "gb", "gib":
		return 1 << 30, nil
	case "t", "tb", "tib":
		return 1 << 40, nil
	case "p", "pb", "pib":
		return 1 << 50, nil
	default:
		return 0, fmt.Errorf("unknown byte size unit %q", unit)
	}
}

func canonicalByteUnit(unit string) string {
	switch strings.ToLower(unit) {
	case "":
		return ""
	case "b":
		return "B"
	case "k", "kb":
		return "KB"
	case "kib":
		return "KiB"
	case "m", "mb":
		return "MB"
	case "mib":
		return "MiB"
	case "g", "gb":
		return "GB"
	case "gib":
		return "GiB"
	case "t", "tb":
		return "TB"
	case "tib":
		return "TiB"
	case "p", "pb":
		return "PB"
	case "pib":
		return "PiB"
	default:
		return unit
	}
}

func evalByteSize(numText string, mult int64, raw string) (int64, error) {
	r, ok := new(big.Rat).SetString(numText)
	if !ok || r.Sign() < 0 {
		return 0, fmt.Errorf("invalid byte size %q", raw)
	}
	if r.Sign() == 0 {
		return 0, nil
	}
	r.Mul(r, new(big.Rat).SetInt64(mult))
	q := new(big.Int).Quo(r.Num(), r.Denom())
	if q.Sign() <= 0 {
		return 0, fmt.Errorf("byte size %q is smaller than 1 byte", raw)
	}
	if !q.IsInt64() {
		return 0, fmt.Errorf("byte size %q overflows int64", raw)
	}
	return q.Int64(), nil
}
