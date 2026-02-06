package config

import "strings"

// QuoteConninfoValue safely quotes a lib/pq connection string value.
// It uses single quotes and escapes backslashes and single quotes.
func QuoteConninfoValue(value string) string {
	escaped := strings.ReplaceAll(value, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `'`, `\'`)
	return "'" + escaped + "'"
}
