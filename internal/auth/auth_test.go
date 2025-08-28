package auth

import (
	"testing"
)

func TestHashToken(t *testing.T) {
	token := "secret_token_value"
	expectedHash := "dfdd08345db4042bb40647747c75482e5a7d89c43a5085eae255385dd0675669"

	hash := HashToken(token)

	if hash != expectedHash {
		t.Errorf("HashToken(%s) = %s; want %s", token, hash, expectedHash)
	}
}
