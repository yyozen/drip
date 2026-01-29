package utils

import "strings"

// IsNetworkError checks if an error message indicates a common network error
// that should be handled gracefully (not logged as severe errors).
func IsNetworkError(errStr string) bool {
	return strings.Contains(errStr, "EOF") ||
		strings.Contains(errStr, "connection reset by peer") ||
		strings.Contains(errStr, "broken pipe") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "use of closed network connection") ||
		strings.Contains(errStr, "websocket: close")
}

// IsProtocolError checks if an error message indicates a protocol-level error
// (invalid requests, malformed data, etc.).
func IsProtocolError(errStr string) bool {
	return strings.Contains(errStr, "payload too large") ||
		strings.Contains(errStr, "failed to read registration frame") ||
		strings.Contains(errStr, "expected register frame") ||
		strings.Contains(errStr, "failed to parse registration request") ||
		strings.Contains(errStr, "failed to parse HTTP request") ||
		strings.Contains(errStr, "tunnel type not allowed")
}

// ContainsAny checks if a string contains any of the given substrings.
func ContainsAny(s string, substrings ...string) bool {
	for _, substr := range substrings {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}
