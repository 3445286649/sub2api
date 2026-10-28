package service

import (
	"bytes"
	"encoding/json"
	"math"
	"strconv"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// normalizeOpenAISSEDataPayload repairs only raw JSON control bytes that occur
// inside a JSON string. Upstream structural corruption is deliberately left as
// an error so it cannot be forwarded as an invalid SSE data frame.
func normalizeOpenAISSEDataPayload(payload []byte) ([]byte, bool, error) {
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("[DONE]")) || json.Valid(trimmed) {
		return payload, false, nil
	}

	repaired, changed := escapeOpenAIRawJSONStringControls(trimmed)
	if !changed {
		return payload, false, nil
	}
	if !json.Valid(repaired) {
		return payload, false, &openAIInvalidSSEJSONError{}
	}
	return repaired, true, nil
}

type openAIInvalidSSEJSONError struct{}

func (*openAIInvalidSSEJSONError) Error() string {
	return "upstream SSE data payload is invalid JSON"
}

func escapeOpenAIRawJSONStringControls(payload []byte) ([]byte, bool) {
	result := make([]byte, 0, len(payload))
	inString := false
	escaped := false
	changed := false
	const hex = "0123456789abcdef"

	for _, b := range payload {
		if !inString {
			result = append(result, b)
			if b == '"' {
				inString = true
			}
			continue
		}

		if escaped {
			result = append(result, b)
			escaped = false
			continue
		}
		switch b {
		case '\\':
			result = append(result, b)
			escaped = true
		case '"':
			result = append(result, b)
			inString = false
		case '\n':
			result = append(result, '\\', 'n')
			changed = true
		case '\r':
			result = append(result, '\\', 'r')
			changed = true
		case '\t':
			result = append(result, '\\', 't')
			changed = true
		default:
			if b < 0x20 {
				result = append(result, '\\', 'u', '0', '0', hex[b>>4], hex[b&0x0f])
				changed = true
			} else {
				result = append(result, b)
			}
		}
	}
	return result, changed
}

// normalizeOpenAIResponseCreatedAt converts integer-valued floating-point
// created_at fields from vendor gateways (for example 1790892388.0) to the
// integer form required by strict Responses clients.
func normalizeOpenAIResponseCreatedAt(payload []byte) ([]byte, bool) {
	if len(payload) == 0 || !gjson.ValidBytes(payload) {
		return payload, false
	}
	changed := false
	for _, path := range []string{"created_at", "response.created_at"} {
		value := gjson.GetBytes(payload, path)
		if value.Type != gjson.Number || !value.Exists() {
			continue
		}
		raw := value.Raw
		if _, err := strconv.ParseInt(raw, 10, 64); err == nil {
			continue
		}
		floatValue, err := strconv.ParseFloat(raw, 64)
		if err != nil || math.IsNaN(floatValue) || math.IsInf(floatValue, 0) || math.Trunc(floatValue) != floatValue {
			continue
		}
		if floatValue < math.MinInt64 || floatValue > math.MaxInt64 {
			continue
		}
		updated, err := sjson.SetRawBytes(payload, path, []byte(strconv.FormatInt(int64(floatValue), 10)))
		if err != nil {
			continue
		}
		payload = updated
		changed = true
	}
	return payload, changed
}
