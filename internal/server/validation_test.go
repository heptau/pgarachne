package server

import "testing"

func TestIsSafeDatabaseName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"simple", "mydb", true},
		{"underscore", "_db1", true},
		{"dollar", "db$1", true},
		{"quoted_simple", "\"MyDb\"", true},
		{"quoted_space", "\"My Db\"", true},
		{"quoted_quote", "\"weird\"\"name\"", true},
		{"empty", "", false},
		{"dash", "my-db", false},
		{"semi", "db;drop", false},
		{"dot", "db.name", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSafeDatabaseName(tt.input); got != tt.want {
				t.Fatalf("isSafeDatabaseName(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsSafeFunctionName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"simple", "api.hello_world", true},
		{"quoted_schema", "\"Api\".hello_world", true},
		{"quoted_func", "api.\"Hello World\"", true},
		{"quoted_both", "\"Api\".\"Hello World\"", true},
		{"quoted_quote", "\"A\"\"B\".\"C\"\"D\"", true},
		{"empty", "", false},
		{"missing_dot", "hello_world", false},
		{"double_dot", "api..hello", false},
		{"bad_chars", "api.hello-world", false},
		{"semi", "api.hello;drop", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSafeFunctionName(tt.input); got != tt.want {
				t.Fatalf("isSafeFunctionName(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
