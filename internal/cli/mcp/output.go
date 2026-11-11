package mcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"
)

// SDK v1.8.0 mcp/transport.go caps inbound frames at 16 MiB; AddTool duplicates
// structured output in text (docs/server.md). Progressively shorten display
// data while preserving the ledger. At most 200 rows of
// 4 KiB keep both copies comfortably below the transport limit.
const mcpRowBytes = 4 << 10

// The SDK emits structured JSON and an escaped text copy. A 256 KiB JSON
// budget keeps their complete protocol representation below 1 MiB, even when
// every encoded character needs another escape in the text copy.
const mcpReplyBytes = 256 << 10

func mcpDisplayText(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	for limit > 0 && !utf8.RuneStart(s[limit]) {
		limit--
	}
	return s[:limit] + " [truncated]"
}

func boundMCPReply(reply mcpReply) mcpReply {
	errorText, warningText := mcpDisplayText(reply.Error, 512), mcpDisplayText(reply.Warning, 512)
	reply.Truncated = errorText != reply.Error || warningText != reply.Warning
	reply.Error, reply.Warning = errorText, warningText
	if reply.Data != nil {
		value, err := decodeMCPData(reply.Data)
		if err != nil {
			return mcpResult(nil, err)
		}
		boundMCPFields(value, &reply.Truncated)
		reply.Data = value
		if err := fitMCPReply(&reply, value); err != nil {
			return mcpResult(nil, err)
		}
	}
	if reply.Truncated {
		reply.Warning = strings.TrimSpace(reply.Warning + " Display data truncated; full metadata remains in the ledger and CLI output.")
	}
	return reply
}

func decodeMCPData(data any) (map[string]any, error) {
	encoded, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var value map[string]any
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber() // ledger IDs retain their full int64 precision
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return value, nil
}

func boundMCPFields(value map[string]any, truncated *bool) {
	for key, field := range value {
		rows, ok := field.([]any)
		if !ok {
			var trimmed bool
			value[key], trimmed = boundedMCPRow(field)
			*truncated = *truncated || trimmed
			continue
		}
		if len(rows) > 200 {
			if _, exists := value[key+"Total"]; !exists {
				value[key+"Total"] = len(rows)
			}
			rows = rows[:200]
			value[key] = rows
			*truncated = true
		}
		for i, row := range rows {
			var trimmed bool
			rows[i], trimmed = boundedMCPRow(row)
			*truncated = *truncated || trimmed
		}
	}
}

func fitMCPReply(reply *mcpReply, value map[string]any) error {
	// Preserve every top-level input row (tools accept at most 100), while
	// reducing strings/nested detail if several bounded arrays add up.
	for limit := 256; ; limit /= 2 {
		encoded, err := json.Marshal(reply)
		if err != nil {
			return err
		}
		if len(encoded) <= mcpReplyBytes-1024 {
			return nil
		}
		if limit == 0 {
			return errors.New("reply exceeds the display budget; use obit graveyard or a smaller page")
		}
		reply.Truncated = true
		for key, field := range value {
			if rows, ok := field.([]any); ok {
				for i, row := range rows {
					rows[i] = compactMCPValue(row, limit, &reply.Truncated)
				}
			} else {
				value[key] = compactMCPValue(field, limit, &reply.Truncated)
			}
		}
	}
}

func boundedMCPRow(value any) (any, bool) {
	for limit := 512; ; limit /= 2 {
		trimmed := false
		short := compactMCPValue(value, limit, &trimmed)
		encoded, _ := json.Marshal(short) // value has already been decoded from JSON
		if len(encoded) <= mcpRowBytes {
			return short, trimmed
		}
	}
}

func compactMCPValue(value any, limit int, trimmed *bool) any {
	switch v := value.(type) {
	case string:
		short := mcpDisplayText(v, limit)
		*trimmed = *trimmed || short != v
		return short
	case []any:
		n := min(len(v), max(1, limit/32))
		*trimmed = *trimmed || n < len(v)
		out := make([]any, n)
		for i := range out {
			out[i] = compactMCPValue(v[i], limit, trimmed)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, field := range v {
			// Parent/child identities are internal verification snapshots.
			if key == "parents" || key == "children" || key == "notified" {
				continue
			}
			if list, ok := field.([]any); ok && len(list) > max(1, limit/32) {
				if _, exists := v[key+"Total"]; !exists {
					out[key+"Total"] = len(list)
				}
			}
			out[key] = compactMCPValue(field, limit, trimmed)
		}
		return out
	default:
		return value
	}
}
