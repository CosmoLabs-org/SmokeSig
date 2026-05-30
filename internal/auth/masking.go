package auth

import "strings"

// knownCredentialPrefixes are token/key prefixes that should never appear in output.
var knownCredentialPrefixes = []string{
	"AKIA", // AWS long-lived access key
	"ASIA", // AWS temporary access key
	"ya29.", // GCP access token
	"eyJ",  // JWT (base64 of {"alg":...)
}

// MaskCredentials replaces known credential patterns in a string with ***redacted***.
func MaskCredentials(s string) string {
	for _, prefix := range knownCredentialPrefixes {
		for {
			idx := strings.Index(s, prefix)
			if idx == -1 {
				break
			}
			// Find the end of the credential (next whitespace, quote, or end of string)
			end := idx + len(prefix)
			for end < len(s) && !isCredentialBoundary(s[end]) {
				end++
			}
			s = s[:idx] + "***redacted***" + s[end:]
		}
	}
	return s
}

func isCredentialBoundary(b byte) bool {
	return b == ' ' || b == '"' || b == '\'' || b == '\n' || b == '\r' || b == '\t' || b == ',' || b == '}' || b == ']'
}
