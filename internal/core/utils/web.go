package utils

import "strings"

// Constants for URL validation
const (
	HTTPPrefix  = "http://"
	HTTPSPrefix = "https://"
	SSHPrefix   = "git@"

	// Minimum URL length to check prefixes
	MinURLLength = 5
)

// IsURLString checks if a string is a URL
func IsURLString(s string) bool {
	return s != "" &&
		len(s) >= MinURLLength &&
		(strings.HasPrefix(s, HTTPPrefix) ||
			strings.HasPrefix(s, HTTPSPrefix) ||
			strings.HasPrefix(s, SSHPrefix))
}
