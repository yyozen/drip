package utils

import "strings"

// AllowedList manages a list of allowed values with case-insensitive matching.
type AllowedList struct {
	items []string
}

// NewAllowedList creates a new AllowedList from the given items.
func NewAllowedList(items []string) *AllowedList {
	return &AllowedList{items: items}
}

// IsAllowed checks if a value is in the allowed list.
// If the list is empty, all values are allowed.
// Matching is case-insensitive.
func (a *AllowedList) IsAllowed(value string) bool {
	if len(a.items) == 0 {
		return true
	}
	for _, item := range a.items {
		if strings.EqualFold(item, value) {
			return true
		}
	}
	return false
}

// GetPreferred returns the first item in the list, or the default value if empty.
func (a *AllowedList) GetPreferred(defaultValue string) string {
	if len(a.items) == 0 {
		return defaultValue
	}
	if len(a.items) == 1 {
		return a.items[0]
	}
	return defaultValue
}

// Items returns the underlying slice of allowed items.
func (a *AllowedList) Items() []string {
	return a.items
}

// IsEmpty returns true if the allowed list is empty.
func (a *AllowedList) IsEmpty() bool {
	return len(a.items) == 0
}
