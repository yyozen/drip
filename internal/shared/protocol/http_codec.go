package protocol

import (
	json "github.com/goccy/go-json"
	"errors"

	"github.com/vmihailenco/msgpack/v5"
)

// EncodeHTTPRequest encodes HTTPRequest using msgpack encoding (optimized)
func EncodeHTTPRequest(req *HTTPRequest) ([]byte, error) {
	return msgpack.Marshal(req)
}

// DecodeHTTPRequest decodes HTTPRequest with automatic version detection
// Detects based on first byte: '{' = JSON, else = msgpack
func DecodeHTTPRequest(data []byte) (*HTTPRequest, error) {
	if len(data) == 0 {
		return nil, errors.New("empty data")
	}

	var req HTTPRequest

	// Auto-detect: JSON starts with '{', msgpack starts with 0x80-0x8f (fixmap)
	if data[0] == '{' {
		// v1: JSON
		if err := json.Unmarshal(data, &req); err != nil {
			return nil, err
		}
	} else {
		// v2: msgpack
		if err := msgpack.Unmarshal(data, &req); err != nil {
			return nil, err
		}
	}

	return &req, nil
}

// EncodeHTTPResponse encodes HTTPResponse using msgpack encoding (optimized)
func EncodeHTTPResponse(resp *HTTPResponse) ([]byte, error) {
	return msgpack.Marshal(resp)
}

// DecodeHTTPResponse decodes HTTPResponse with automatic version detection
// Detects based on first byte: '{' = JSON, else = msgpack
func DecodeHTTPResponse(data []byte) (*HTTPResponse, error) {
	if len(data) == 0 {
		return nil, errors.New("empty data")
	}

	var resp HTTPResponse

	// Auto-detect: JSON starts with '{', msgpack starts with 0x80-0x8f (fixmap)
	if data[0] == '{' {
		// v1: JSON
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, err
		}
	} else {
		// v2: msgpack
		if err := msgpack.Unmarshal(data, &resp); err != nil {
			return nil, err
		}
	}

	return &resp, nil
}
