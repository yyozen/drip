package httputil

import (
	"fmt"
	"net/http"
)

// WriteJSON writes a JSON response with the appropriate headers.
func WriteJSON(w http.ResponseWriter, data []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	w.Write(data)
}

// WriteHTML writes an HTML response with the appropriate headers.
func WriteHTML(w http.ResponseWriter, data []byte) {
	w.Header().Set("Content-Type", "text/html")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	w.Write(data)
}

// WriteHTMLWithStatus writes an HTML response with a custom status code.
func WriteHTMLWithStatus(w http.ResponseWriter, data []byte, statusCode int) {
	w.Header().Set("Content-Type", "text/html")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	w.WriteHeader(statusCode)
	w.Write(data)
}

// SetContentLength sets the Content-Length header.
func SetContentLength(w http.ResponseWriter, length int64) {
	w.Header().Set("Content-Length", fmt.Sprintf("%d", length))
}

// SetCloseConnection sets the Connection: close header.
func SetCloseConnection(w http.ResponseWriter) {
	w.Header().Set("Connection", "close")
}
