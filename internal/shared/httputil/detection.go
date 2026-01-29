package httputil

import "strings"

// HTTPMethods contains common HTTP method prefixes for protocol detection.
var HTTPMethods = []string{
	"GET ", "POST", "PUT ", "DELE", "HEAD", "OPTI", "PATC", "CONN", "TRAC",
}

// IsHTTPRequest checks if the given bytes represent the start of an HTTP request.
// It checks for common HTTP method prefixes.
func IsHTTPRequest(data []byte) bool {
	if len(data) < 4 {
		return false
	}

	dataStr := string(data[:4])
	for _, method := range HTTPMethods {
		if strings.HasPrefix(dataStr, method) {
			return true
		}
	}
	return false
}

// DetectHTTPMethod returns the HTTP method if the data starts with one, or empty string.
func DetectHTTPMethod(data []byte) string {
	if len(data) < 4 {
		return ""
	}

	dataStr := string(data)
	for _, method := range HTTPMethods {
		if strings.HasPrefix(dataStr, method) {
			return strings.TrimSpace(method)
		}
	}
	return ""
}
