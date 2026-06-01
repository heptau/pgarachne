package version

import (
	"strings"
	"testing"
)

func TestVersionNonEmpty(t *testing.T) {
	if Version == "" {
		t.Fatal("Version is empty; the build did not inject a value via -ldflags")
	}
}

func TestCommitNonEmpty(t *testing.T) {
	if Commit == "" {
		t.Fatal("Commit is empty; the build did not inject a value via -ldflags")
	}
}

func TestBuildDateNonEmpty(t *testing.T) {
	if BuildDate == "" {
		t.Fatal("BuildDate is empty; the build did not inject a value via -ldflags")
	}
}

func TestFullIncludesAllFields(t *testing.T) {
	full := Full()
	// Format is "Version (Commit, built BuildDate)" — no spaces around
	// the comma inside the parens, exactly one space before "built".
	// Downstream parsers may rely on this; keep it stable.
	if !strings.Contains(full, Version) {
		t.Errorf("Full() = %q does not contain Version %q", full, Version)
	}
	if !strings.Contains(full, Commit) {
		t.Errorf("Full() = %q does not contain Commit %q", full, Commit)
	}
	if !strings.Contains(full, BuildDate) {
		t.Errorf("Full() = %q does not contain BuildDate %q", full, BuildDate)
	}
	if !strings.HasSuffix(full, ")") {
		t.Errorf("Full() = %q does not end with )", full)
	}
}

func TestFullIsSingleLine(t *testing.T) {
	if strings.ContainsAny(Full(), "\r\n") {
		t.Errorf("Full() must be a single line, got %q", Full())
	}
}
