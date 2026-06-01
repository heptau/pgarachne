package config

import "testing"

// setRequiredTestEnv pins all required variables so Load() succeeds without
// depending on the developer's personal pgarachne.env or the ambient
// environment. Individual tests override specific variables after calling it.
//
// Note that godotenv never overrides variables that are already set, so values
// set here always win over any config file found in the search paths.
func setRequiredTestEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_USER", "pgarachne")
	t.Setenv("JWT_SECRET", "unit-test-secret-0123456789abcdef-0123456789abcdef")
}
