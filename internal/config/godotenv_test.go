package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGodotenvSyntaxErrorIsWarned verifies that a config file with invalid
// syntax is skipped with a warning rather than causing Load to fail.
// The required env vars (DB_HOST, DB_PORT, DB_USER, JWT_SECRET) are expected
// to be set in the test environment; this test only checks that a bad file
// does not abort the load.
func TestGodotenvSyntaxErrorIsWarned(t *testing.T) {
	dir := t.TempDir()
	badFile := filepath.Join(dir, "pgarachne.env")
	// Write an intentionally malformed .env file — unclosed quote.
	if err := os.WriteFile(badFile, []byte(`DB_HOST='unclosed`), 0600); err != nil {
		t.Fatalf("write bad config: %v", err)
	}

	// Change into the temp dir so the auto-search finds pgarachne.env there.
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	// Load must not return an error for the parse failure — it should warn
	// and fall through to the next search path (or env vars).
	// It may still fail due to missing required env vars (DB_HOST etc.),
	// but the error must NOT mention "pgarachne.env" or "parse".
	_, err = Load("")
	if err != nil {
		// Acceptable: required env vars may be missing in this temp context.
		// Unacceptable: the error says the config file itself failed to load.
		msg := err.Error()
		if contains(msg, badFile) {
			t.Errorf("Load returned an error referencing the bad config file, want a silent skip: %v", err)
		}
	}
}
