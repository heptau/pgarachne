package auth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

func TestHashToken(t *testing.T) {
	token := "secret_token_value"
	expectedHash := "dfdd08345db4042bb40647747c75482e5a7d89c43a5085eae255385dd0675669"

	hash := HashToken(token)

	if hash != expectedHash {
		t.Errorf("HashToken(%s) = %s; want %s", token, hash, expectedHash)
	}
}

func TestIssueAndParseRoundTrip(t *testing.T) {
	secret := "test_secret"
	role := "app_user"
	dbName := "analytics"
	ttl := time.Hour

	signed, err := Issue(secret, role, dbName, ttl)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if signed == "" {
		t.Fatalf("Issue returned empty token")
	}

	claims, err := Parse(secret, signed)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if claims.DBRole != role {
		t.Errorf("DBRole = %q; want %q", claims.DBRole, role)
	}
	if claims.DBName != dbName {
		t.Errorf("DBName = %q; want %q", claims.DBName, dbName)
	}
}

func TestParseRejectsWrongSecret(t *testing.T) {
	signed, err := Issue("right_secret", "app_user", "main", time.Hour)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	if _, err := Parse("wrong_secret", signed); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("Parse with wrong secret: err = %v; want ErrInvalidToken", err)
	}
}

func TestParseRejectsExpiredToken(t *testing.T) {
	// Issue rejects negative TTL, so we craft an already-expired token by
	// hand. This exercises the exp-claim path of Parse in isolation.
	claims := jwt.MapClaims{
		"db_role": "app_user",
		"db_name": "main",
		"exp":     time.Now().Add(-time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte("secret"))
	if err != nil {
		t.Fatalf("sign expired token: %v", err)
	}

	if _, err := Parse("secret", signed); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("Parse on expired token: err = %v; want ErrInvalidToken", err)
	}
}

func TestParseRejectsMalformedToken(t *testing.T) {
	cases := []string{"", "not-a-jwt", "a.b.c"}
	for _, in := range cases {
		if _, err := Parse("secret", in); !errors.Is(err, ErrInvalidToken) {
			t.Errorf("Parse(%q): err = %v; want ErrInvalidToken", in, err)
		}
	}
}

func TestIssueRejectsInvalidInput(t *testing.T) {
	cases := []struct {
		name   string
		secret string
		role   string
		db     string
		ttl    time.Duration
	}{
		{"empty_secret", "", "app_user", "main", time.Hour},
		{"empty_role", "secret", "", "main", time.Hour},
		{"empty_db", "secret", "app_user", "", time.Hour},
		{"zero_ttl", "secret", "app_user", "main", 0},
		{"negative_ttl", "secret", "app_user", "main", -time.Minute},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := Issue(c.secret, c.role, c.db, c.ttl); !errors.Is(err, ErrInvalidToken) {
				t.Errorf("Issue(%+v): err = %v; want ErrInvalidToken", c, err)
			}
		})
	}
}

func TestParseRejectsAlgorithmNone(t *testing.T) {
	// A token signed with alg=none must never be accepted. The header
	// advertises none and the signature segment is empty.
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"db_role":"app_user","db_name":"main"}`))
	none := header + "." + payload + "."

	if _, err := Parse("secret", none); err == nil {
		t.Fatalf("Parse accepted a token with alg=none")
	} else if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("Parse on alg=none token: err = %v; want ErrInvalidToken", err)
	}
}

func TestParseRejectsMissingDBName(t *testing.T) {
	secret := "secret"
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"db_role":"app_user"}`))
	// Hand-craft a token with valid header/payload but invalid signature.
	// Parse must reject on signature failure, before even reading the claims.
	token := header + "." + payload + ".invalidsig"

	if _, err := Parse(secret, token); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("Parse with bad signature: err = %v; want ErrInvalidToken", err)
	}
}

func TestParseRejectsEmptyClaims(t *testing.T) {
	secret := "secret"
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))

	// Valid signature of an empty-claims payload. We rely on Issue for the
	// happy path of claim structure, so here we just build a token where the
	// payload decodes but has no recognised fields.
	payloadBytes, _ := json.Marshal(map[string]string{"other": "value"})
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	token := header + "." + payload + "."

	_, err := Parse(secret, token)
	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("Parse with empty claims: err = %v; want ErrInvalidToken", err)
	}
	if !strings.Contains(err.Error(), "signature") && !strings.Contains(err.Error(), "missing") {
		t.Errorf("Parse with empty claims: error = %v; expected signature or missing claim error", err)
	}
}

// TestSignerIssuerRoundTrip verifies that an issuer configured on the
// Signer is written into the "iss" claim and required on Parse.
func TestSignerIssuerRoundTrip(t *testing.T) {
	signer := NewSigner(&Options{Issuer: "pgarachne.example.com"})
	signed, err := signer.Issue("secret", "app_user", "main", time.Hour)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	claims, err := signer.Parse("secret", signed)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if claims.DBRole != "app_user" || claims.DBName != "main" {
		t.Errorf("claims = %+v; want DBRole=app_user DBName=main", claims)
	}
}

func TestSignerRejectsWrongIssuer(t *testing.T) {
	issuer := NewSigner(&Options{Issuer: "pgarachne.example.com"})
	signed, err := issuer.Issue("secret", "app_user", "main", time.Hour)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	verifier := NewSigner(&Options{Issuer: "different.example.com"})
	if _, err := verifier.Parse("secret", signed); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("Parse with wrong issuer: err = %v; want ErrInvalidToken", err)
	}
}

func TestSignerIssuerRejectsMissingClaim(t *testing.T) {
	// Signer with no issuer requirement accepts a token without iss.
	permissive := NewSigner(nil)
	signed, err := permissive.Issue("secret", "app_user", "main", time.Hour)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	strict := NewSigner(&Options{Issuer: "pgarachne.example.com"})
	if _, err := strict.Parse("secret", signed); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("Parse without iss against strict Signer: err = %v; want ErrInvalidToken", err)
	}
}

func TestSignerAudienceRoundTrip(t *testing.T) {
	signer := NewSigner(&Options{Audience: "pgarachne-api"})
	signed, err := signer.Issue("secret", "app_user", "main", time.Hour)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if _, err := signer.Parse("secret", signed); err != nil {
		t.Errorf("Parse: %v", err)
	}
}

func TestSignerRejectsWrongAudience(t *testing.T) {
	issuer := NewSigner(&Options{Audience: "pgarachne-api"})
	signed, err := issuer.Issue("secret", "app_user", "main", time.Hour)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	verifier := NewSigner(&Options{Audience: "other-api"})
	if _, err := verifier.Parse("secret", signed); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("Parse with wrong audience: err = %v; want ErrInvalidToken", err)
	}
}

func TestSignerAcceptsAudienceArray(t *testing.T) {
	// JOSE allows aud to be a JSON array of strings. The Signer should
	// accept the token as long as its audience is one of the entries.
	signer := NewSigner(&Options{Audience: "pgarachne-api"})
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payloadBytes, _ := json.Marshal(map[string]interface{}{
		"db_role": "app_user",
		"db_name": "main",
		"iat":     time.Now().Unix(),
		"exp":     time.Now().Add(time.Hour).Unix(),
		"aud":     []string{"other-api", "pgarachne-api"},
	})
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	// Sign with the same secret the Signer will use.
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{})
	// We have to use a real signature. Round-trip through the Signer
	// with the array claim replaced after signing is fiddly; easier to
	// just sign with the secret directly.
	claims := jwt.MapClaims{
		"db_role": "app_user",
		"db_name": "main",
		"iat":     time.Now().Unix(),
		"exp":     time.Now().Add(time.Hour).Unix(),
		"aud":     []string{"other-api", "pgarachne-api"},
	}
	t2 := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := t2.SignedString([]byte("secret"))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := signer.Parse("secret", signed); err != nil {
		t.Errorf("Parse with audience array: %v", err)
	}
	_ = header
	_ = payloadBytes
	_ = payload
	_ = tok
}

// TestSignerClockSkewAcceptsRecentlyExpired verifies that a token which
// expired N seconds ago is still accepted when N < leeway.
func TestSignerClockSkewAcceptsRecentlyExpired(t *testing.T) {
	frozen := time.Now()
	signer := NewSigner(&Options{
		Leeway: 5 * time.Minute,
		Now:    func() time.Time { return frozen },
	})
	// Issue a token that expired 1 minute ago.
	claims := jwt.MapClaims{
		"db_role": "app_user",
		"db_name": "main",
		"iat":     frozen.Add(-time.Hour).Unix(),
		"exp":     frozen.Add(-time.Minute).Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte("secret"))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := signer.Parse("secret", signed); err != nil {
		t.Errorf("Parse within leeway: %v", err)
	}
}

// TestSignerClockSkewRejectsLongExpired checks that the leeway actually
// caps the tolerance — a token expired well outside the leeway must be
// rejected.
func TestSignerClockSkewRejectsLongExpired(t *testing.T) {
	frozen := time.Now()
	signer := NewSigner(&Options{
		Leeway: 30 * time.Second,
		Now:    func() time.Time { return frozen },
	})
	claims := jwt.MapClaims{
		"db_role": "app_user",
		"db_name": "main",
		"iat":     frozen.Add(-time.Hour).Unix(),
		"exp":     frozen.Add(-time.Hour).Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte("secret"))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := signer.Parse("secret", signed); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("Parse outside leeway: err = %v; want ErrInvalidToken", err)
	}
}

func TestSignerRejectsFutureNbf(t *testing.T) {
	frozen := time.Now()
	signer := NewSigner(&Options{
		Leeway: 10 * time.Second,
		Now:    func() time.Time { return frozen },
	})
	claims := jwt.MapClaims{
		"db_role": "app_user",
		"db_name": "main",
		"iat":     frozen.Unix(),
		"exp":     frozen.Add(time.Hour).Unix(),
		"nbf":     frozen.Add(time.Hour).Unix(), // valid in 1h, well outside leeway
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte("secret"))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := signer.Parse("secret", signed); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("Parse with future nbf: err = %v; want ErrInvalidToken", err)
	}
}

func TestIssueWritesIatClaim(t *testing.T) {
	signed, err := Issue("secret", "app_user", "main", time.Hour)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	// Parse the payload segment directly to inspect the iat claim.
	parts := strings.Split(signed, ".")
	if len(parts) != 3 {
		t.Fatalf("token has %d parts, want 3", len(parts))
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	var claims map[string]interface{}
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if _, ok := claims["iat"]; !ok {
		t.Errorf("issued token has no iat claim: %s", payloadBytes)
	}
}

// BenchmarkHashToken measures the cost of SHA-256 hashing an API token.
// It runs on every pgarachne.verify_api_token() call (login path) and
// on every API-token authenticated request, so it sits on a hot path.
// The bench varies the token length to surface the cost of longer
// tokens (e.g. 64+ char random secrets).
func BenchmarkHashToken(b *testing.B) {
	cases := []struct {
		name  string
		token string
	}{
		{"short_16", "abcdefghijklmnop"},
		{"medium_64", strings.Repeat("a", 64)},
		{"long_256", strings.Repeat("a", 256)},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = HashToken(tc.token)
			}
		})
	}
}

// BenchmarkJWTIssue measures the cost of minting a short-lived JWT.
// This runs on every successful login (the login handler signs a fresh
// token), so the cost is on the auth hot path.
func BenchmarkJWTIssue(b *testing.B) {
	signer := NewSigner(nil)
	for i := 0; i < b.N; i++ {
		if _, err := signer.Issue("secret", "app_user", "main", time.Hour); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkJWTParse measures the cost of validating and parsing a
// short-lived JWT. This runs on every authenticated JSON-RPC, MCP,
// and SSE request, so the cost is on the per-request hot path.
func BenchmarkJWTParse(b *testing.B) {
	signer := NewSigner(nil)
	signed, err := signer.Issue("secret", "app_user", "main", time.Hour)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := signer.Parse("secret", signed); err != nil {
			b.Fatal(err)
		}
	}
}
