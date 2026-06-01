package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// ErrInvalidToken is returned by Parse when the token is malformed, has an
// unexpected signature, has expired, or is missing required claims.
var ErrInvalidToken = errors.New("invalid token")

// DefaultClockSkew is the tolerance applied to exp / nbf / iat validation
// when no Signer is configured explicitly. 30 seconds covers the realistic
// NTP drift between PgArachne and the issuing service in a single data
// centre while still keeping the window too small for meaningful replay.
const DefaultClockSkew = 30 * time.Second

// Claims is the subset of JWT claims PgArachne relies on for authorisation.
// Both fields are required; the service user (DB_USER) is granted membership
// to each role to enable SET LOCAL ROLE switching.
type Claims struct {
	DBRole string
	DBName string
}

// Options bundles the optional hardening parameters that production
// deployments want. The zero value is fine for development: it produces
// and accepts tokens equivalent to the bare Issue/Parse functions.
type Options struct {
	// Issuer, if non-empty, is written into the "iss" claim on Issue
	// and required to match exactly on Parse.
	Issuer string
	// Audience, if non-empty, is written into the "aud" claim on Issue
	// and required to match exactly on Parse.
	Audience string
	// Leeway is the tolerance applied to exp / nbf / iat. Zero means
	// use DefaultClockSkew.
	Leeway time.Duration
	// Now is the time source used in tests; nil means time.Now. Not
	// exposed via config — only the auth tests set it.
	Now func() time.Time
}

// Signer carries the configured Options so callers don't have to pass them
// in repeatedly. The zero value of Signer (with default Options) matches
// the original Issue/Parse behaviour.
type Signer struct {
	opts Options
}

// NewSigner returns a Signer that uses the given options. A nil pointer
// is treated as the zero Options.
func NewSigner(opts *Options) *Signer {
	if opts == nil {
		return &Signer{}
	}
	return &Signer{opts: *opts}
}

// Issue signs a short-lived HS256 JWT carrying the database role and database
// name. The token is intended to be returned from the JSON-RPC get_jwt (login)
// // method and presented as "Authorization: Bearer <token>" on subsequent calls.
func (s *Signer) Issue(secret, dbRole, dbName string, ttl time.Duration) (string, error) {
	if secret == "" {
		return "", fmt.Errorf("%w: empty secret", ErrInvalidToken)
	}
	if dbRole == "" || dbName == "" {
		return "", fmt.Errorf("%w: db_role and db_name are required", ErrInvalidToken)
	}
	if ttl <= 0 {
		return "", fmt.Errorf("%w: ttl must be positive", ErrInvalidToken)
	}

	now := s.now()
	claims := jwt.MapClaims{
		"db_role": dbRole,
		"db_name": dbName,
		"iat":     now.Unix(),
		"exp":     now.Add(ttl).Unix(),
	}
	if s.opts.Issuer != "" {
		claims["iss"] = s.opts.Issuer
	}
	if s.opts.Audience != "" {
		claims["aud"] = s.opts.Audience
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// Parse validates the JWT signature, the signing algorithm, expiration, and
// required claims. Only HS* (HMAC) algorithms are accepted to prevent the
// well-known "alg: none" attack. When the Signer was constructed with an
// Issuer or Audience, those claims are required to match. Clock skew
// tolerance is applied to exp / nbf / iat as configured via Options.Leeway
// (defaults to DefaultClockSkew = 30s).
func (s *Signer) Parse(secret, tokenString string) (*Claims, error) {
	if secret == "" {
		return nil, fmt.Errorf("%w: empty secret", ErrInvalidToken)
	}

	// jwt/v4's parser does not expose a leeway option, so we skip its
	// built-in exp / nbf checks and run our own below, with the configured
	// leeway applied uniformly. The library still verifies the signature
	// and rejects non-HMAC algorithms.
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	}, jwt.WithoutClaimsValidation())
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	if !token.Valid {
		return nil, fmt.Errorf("%w: token is not valid", ErrInvalidToken)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("%w: claims are not a map", ErrInvalidToken)
	}

	if err := s.checkTimeBounds(claims); err != nil {
		return nil, err
	}
	if err := s.checkIssuer(claims); err != nil {
		return nil, err
	}
	if err := s.checkAudience(claims); err != nil {
		return nil, err
	}

	dbRole, roleOk := claims["db_role"].(string)
	dbName, dbNameOk := claims["db_name"].(string)
	if !roleOk || !dbNameOk || dbRole == "" || dbName == "" {
		return nil, fmt.Errorf("%w: missing db_role or db_name claim", ErrInvalidToken)
	}

	return &Claims{DBRole: dbRole, DBName: dbName}, nil
}

func (s *Signer) checkIssuer(claims jwt.MapClaims) error {
	if s.opts.Issuer == "" {
		return nil
	}
	iss, _ := claims["iss"].(string)
	if iss != s.opts.Issuer {
		return fmt.Errorf("%w: iss = %q, want %q", ErrInvalidToken, iss, s.opts.Issuer)
	}
	return nil
}

// checkAudience accepts either a single string or a JSON array of strings,
// matching the JOSE convention where "aud" can be either. The Signer's
// Audience must appear as one of the entries.
func (s *Signer) checkAudience(claims jwt.MapClaims) error {
	if s.opts.Audience == "" {
		return nil
	}
	switch v := claims["aud"].(type) {
	case string:
		if v != s.opts.Audience {
			return fmt.Errorf("%w: aud = %q, want %q", ErrInvalidToken, v, s.opts.Audience)
		}
		return nil
	case []interface{}:
		for _, e := range v {
			if aud, ok := e.(string); ok && aud == s.opts.Audience {
				return nil
			}
		}
		return fmt.Errorf("%w: aud does not contain %q", ErrInvalidToken, s.opts.Audience)
	}
	return fmt.Errorf("%w: aud missing or wrong type", ErrInvalidToken)
}

// checkTimeBounds verifies exp / nbf / iat against the configured leeway.
// The library's own checks were disabled via jwt.WithoutClaimsValidation()
// so we own this validation here. Missing exp is treated as an error:
// tokens without an expiry are effectively eternal, which is never what
// the caller wants.
func (s *Signer) checkTimeBounds(claims jwt.MapClaims) error {
	leeway := s.opts.Leeway
	if leeway == 0 {
		leeway = DefaultClockSkew
	}
	now := s.now()

	exp, ok := numericClaim(claims, "exp")
	if !ok {
		return fmt.Errorf("%w: missing exp claim", ErrInvalidToken)
	}
	if now.After(time.Unix(exp, 0).Add(leeway)) {
		return fmt.Errorf("%w: token is expired", ErrInvalidToken)
	}
	if nbf, ok := numericClaim(claims, "nbf"); ok {
		if now.Add(leeway).Before(time.Unix(nbf, 0)) {
			return fmt.Errorf("%w: token not yet valid (nbf in the future)", ErrInvalidToken)
		}
	}
	if iat, ok := numericClaim(claims, "iat"); ok {
		// iat in the future is suspicious but not always a hard error.
		// Logically a clock-skew issue, so we apply the same leeway.
		if now.Add(leeway).Before(time.Unix(iat, 0)) {
			return fmt.Errorf("%w: iat in the future", ErrInvalidToken)
		}
	}
	return nil
}

// numericClaim reads a numeric JWT claim. The standard JSON decoder always
// produces float64 for numbers; int64 is kept as a defensive case for
// callers that pre-populate MapClaims programmatically.
func numericClaim(claims jwt.MapClaims, key string) (int64, bool) {
	switch v := claims[key].(type) {
	case float64:
		return int64(v), true
	case int64:
		return v, true
	}
	return 0, false
}

func (s *Signer) now() time.Time {
	if s.opts.Now != nil {
		return s.opts.Now()
	}
	return time.Now()
}

// Issue and Parse are package-level convenience wrappers that use the
// zero Options (no issuer/audience enforcement, default 30s clock skew).
// They preserve the original auth.Issue/Parse behaviour so existing
// callers and tests continue to work unchanged.
var defaultSigner = NewSigner(nil)

// Issue signs a short-lived HS256 JWT carrying the database role and database
// name. See Signer.Issue for full configuration; this is the bare default.
func Issue(secret, dbRole, dbName string, ttl time.Duration) (string, error) {
	return defaultSigner.Issue(secret, dbRole, dbName, ttl)
}

// Parse validates the JWT signature, the signing algorithm, expiration, and
// required claims. Only HS* (HMAC) algorithms are accepted to prevent the
// well-known "alg: none" attack. See Signer.Parse for full configuration.
func Parse(secret, tokenString string) (*Claims, error) {
	return defaultSigner.Parse(secret, tokenString)
}
