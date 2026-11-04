package identity

import "strings"

// NormalizeIdentifier preserves program-key case and interior whitespace.
func NormalizeIdentifier(value string) string { return strings.TrimSpace(value) }

// ASCIICaseKey is the stored and lookup identity for usernames and product numbers.
func ASCIICaseKey(value string) string {
	value = NormalizeIdentifier(value)
	b := []byte(value)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}
