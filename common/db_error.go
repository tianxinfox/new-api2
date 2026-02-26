package common

import "strings"

// IsDuplicateConstraintError checks whether an error is caused by unique-key conflicts
// across common databases (MySQL/PostgreSQL/SQLite).
func IsDuplicateConstraintError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "duplicate") ||
		strings.Contains(s, "unique constraint") ||
		strings.Contains(s, "unique failed") ||
		strings.Contains(s, "1062") ||
		strings.Contains(s, "23505")
}
