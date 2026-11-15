package jobresult

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

type Decoded struct {
	JSON          []byte
	FormatVersion int
	Output        string
}

func Decode(encoded string) (*Decoded, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return nil, fmt.Errorf("invalid base64 result: %w", err)
	}

	var envelope struct {
		FormatVersion int    `json:"format_version"`
		Output        string `json:"output"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("invalid structured result: %w", err)
	}
	if envelope.FormatVersion != 2 {
		return nil, fmt.Errorf("unsupported result format version %d", envelope.FormatVersion)
	}

	return &Decoded{
		JSON:          raw,
		FormatVersion: envelope.FormatVersion,
		Output:        envelope.Output,
	}, nil
}

func LegacyJSON(stored string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(stored))
	if err != nil {
		return nil, fmt.Errorf("invalid base64 result: %w", err)
	}
	return bytes.TrimSpace(raw), nil
}
