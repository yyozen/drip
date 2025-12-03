package protocol

import (
	json "github.com/goccy/go-json"
	"errors"
)

// EncodeDataPayload encodes a data header and payload into a frame payload
// Uses binary encoding (optimized format)
func EncodeDataPayload(header DataHeader, data []byte) ([]byte, error) {
	return EncodeDataPayloadV2(header, data)
}

// EncodeDataPayloadV1 encodes using JSON (legacy)
// Format: JSON_HEADER\nDATA
func EncodeDataPayloadV1(header DataHeader, data []byte) ([]byte, error) {
	headerBytes, err := json.Marshal(header)
	if err != nil {
		return nil, err
	}

	// Combine: header + newline + data
	payload := make([]byte, 0, len(headerBytes)+1+len(data))
	payload = append(payload, headerBytes...)
	payload = append(payload, '\n')
	payload = append(payload, data...)

	return payload, nil
}

// EncodeDataPayloadV2 encodes using binary format (optimized)
// Format: BINARY_HEADER + DATA
func EncodeDataPayloadV2(header DataHeader, data []byte) ([]byte, error) {
	// Convert to binary header
	var h2 DataHeaderV2
	h2.FromDataHeader(header)

	// Encode header to binary
	headerBytes := h2.MarshalBinary()

	// Combine: binary header + data
	payload := make([]byte, 0, len(headerBytes)+len(data))
	payload = append(payload, headerBytes...)
	payload = append(payload, data...)

	return payload, nil
}

// DecodeDataPayload decodes a frame payload into header and data
// Auto-detects protocol version
func DecodeDataPayload(payload []byte) (DataHeader, []byte, error) {
	if len(payload) == 0 {
		return DataHeader{}, nil, errors.New("empty payload")
	}

	// Try to detect version:
	// - V1 (JSON): starts with '{'
	// - V2 (Binary): first byte is flags (0x00-0x1F typically)
	if payload[0] == '{' {
		// V1: JSON format
		return DecodeDataPayloadV1(payload)
	}

	// V2: Binary format
	return DecodeDataPayloadV2(payload)
}

// DecodeDataPayloadV1 decodes JSON format (legacy)
// Format: JSON_HEADER\nDATA
func DecodeDataPayloadV1(payload []byte) (DataHeader, []byte, error) {
	// Find newline separator
	sepIdx := -1
	for i, b := range payload {
		if b == '\n' {
			sepIdx = i
			break
		}
	}

	if sepIdx == -1 {
		return DataHeader{}, nil, errors.New("invalid v1 payload: no newline separator")
	}

	// Parse JSON header
	var header DataHeader
	if err := json.Unmarshal(payload[:sepIdx], &header); err != nil {
		return DataHeader{}, nil, err
	}

	// Extract data (after newline)
	data := payload[sepIdx+1:]

	return header, data, nil
}

// DecodeDataPayloadV2 decodes binary format (optimized)
// Format: BINARY_HEADER + DATA
func DecodeDataPayloadV2(payload []byte) (DataHeader, []byte, error) {
	if len(payload) < binaryHeaderMinSize {
		return DataHeader{}, nil, errors.New("invalid v2 payload: too short")
	}

	// Decode binary header
	var h2 DataHeaderV2
	if err := h2.UnmarshalBinary(payload); err != nil {
		return DataHeader{}, nil, err
	}

	// Extract data (after header)
	headerSize := h2.Size()
	if len(payload) < headerSize {
		return DataHeader{}, nil, errors.New("invalid v2 payload: data missing")
	}

	data := payload[headerSize:]

	// Convert to DataHeader
	header := h2.ToDataHeader()

	return header, data, nil
}

// GetPayloadHeaderSize returns the size of the header in the payload
// This is useful for pre-allocating buffers
func GetPayloadHeaderSize(header DataHeader) int {
	var h2 DataHeaderV2
	h2.FromDataHeader(header)
	return h2.Size()
}
