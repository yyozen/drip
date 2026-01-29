package httputil

import (
	"fmt"
	"net"
)

// HTTPErrorResponse represents a standard HTTP error response.
type HTTPErrorResponse struct {
	StatusCode int
	StatusText string
	Message    string
}

// Common HTTP error responses
var (
	ServiceUnavailable = &HTTPErrorResponse{
		StatusCode: 503,
		StatusText: "Service Unavailable",
		Message:    "Server busy, please retry later",
	}

	HandlerNotConfigured = &HTTPErrorResponse{
		StatusCode: 503,
		StatusText: "Service Unavailable",
		Message:    "HTTP handler not configured for this TCP port",
	}
)

// WriteErrorResponse writes an HTTP error response to the connection.
func WriteErrorResponse(conn net.Conn, resp *HTTPErrorResponse) error {
	response := fmt.Sprintf(
		"HTTP/1.1 %d %s\r\n"+
			"Content-Type: text/plain\r\n"+
			"Content-Length: %d\r\n"+
			"Connection: close\r\n"+
			"\r\n"+
			"%s\r\n",
		resp.StatusCode,
		resp.StatusText,
		len(resp.Message)+2,
		resp.Message,
	)
	_, err := conn.Write([]byte(response))
	return err
}

// WriteServiceUnavailable writes a 503 Service Unavailable response.
func WriteServiceUnavailable(conn net.Conn, message string) error {
	if message == "" {
		message = ServiceUnavailable.Message
	}
	return WriteErrorResponse(conn, &HTTPErrorResponse{
		StatusCode: 503,
		StatusText: "Service Unavailable",
		Message:    message,
	})
}
