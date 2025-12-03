package protocol

import json "github.com/goccy/go-json"

// RegisterRequest is sent by client to register a tunnel
type RegisterRequest struct {
	Token           string     `json:"token"`            // Authentication token
	CustomSubdomain string     `json:"custom_subdomain"` // Optional custom subdomain
	TunnelType      TunnelType `json:"tunnel_type"`      // http, tcp, udp
	LocalPort       int        `json:"local_port"`       // Local port to forward to
}

// RegisterResponse is sent by server after successful registration
type RegisterResponse struct {
	Subdomain string `json:"subdomain"`        // Assigned subdomain
	Port      int    `json:"port,omitempty"`   // Assigned TCP port (for TCP tunnels)
	URL       string `json:"url"`              // Full tunnel URL
	Message   string `json:"message"`          // Success message
}

// ErrorMessage represents an error
type ErrorMessage struct {
	Code    string `json:"code"`    // Error code
	Message string `json:"message"` // Error message
}

// DataHeader represents metadata for a data frame
type DataHeader struct {
	StreamID  string `json:"stream_id"`  // Unique stream identifier
	RequestID string `json:"request_id"` // Request identifier (for HTTP)
	Type      string `json:"type"`       // "data", "response", "close", "http_request", "http_response"
	IsLast    bool   `json:"is_last"`    // Is this the last frame for this stream
}

// TCPData represents TCP tunnel data
type TCPData struct {
	StreamID string `json:"stream_id"` // Stream identifier
	Data     []byte `json:"data"`      // Raw TCP data
	IsClose  bool   `json:"is_close"`  // Close this stream
}

// Marshal helpers
func MarshalJSON(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func UnmarshalJSON(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
