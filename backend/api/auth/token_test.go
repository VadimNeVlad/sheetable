package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestCreatedTokenRetainsUserIDAndLifetime(t *testing.T) {
	const secret = "test-only-token-secret"
	before := time.Now()
	tokenString, err := CreateToken(42, secret)
	if err != nil {
		t.Fatal(err)
	}
	if err := TokenValid(tokenString, secret); err != nil {
		t.Fatalf("created token rejected: %v", err)
	}
	userID, err := ExtractTokenID(tokenString, secret)
	if err != nil || userID != 42 {
		t.Fatalf("user ID = %d, error = %v; want 42", userID, err)
	}
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	expires, err := token.Claims.GetExpirationTime()
	if err != nil || expires == nil {
		t.Fatalf("missing expiration: %v", err)
	}
	// JWT timestamps have second precision; preserve the existing seven-day lifetime.
	if expires.Time.Before(before.Add(168*time.Hour-time.Second)) || expires.Time.After(time.Now().Add(168*time.Hour)) {
		t.Fatalf("unexpected expiration: %v", expires.Time)
	}
}

func TestTokenValidationRejectsInvalidTokens(t *testing.T) {
	const secret = "test-only-token-secret"
	validClaims := func() jwt.MapClaims {
		return jwt.MapClaims{"authorized": true, "user_id": 42, "exp": time.Now().Add(time.Hour).Unix()}
	}
	sign := func(method jwt.SigningMethod, claims jwt.MapClaims, key interface{}) string {
		t.Helper()
		token, err := jwt.NewWithClaims(method, claims).SignedString(key)
		if err != nil {
			t.Fatal(err)
		}
		return token
	}
	expired := validClaims()
	expired["exp"] = time.Now().Add(-time.Hour).Unix()
	missingExpiration := validClaims()
	delete(missingExpiration, "exp")
	notYetValid := validClaims()
	notYetValid["nbf"] = time.Now().Add(time.Hour).Unix()

	tests := []struct {
		name  string
		token string
	}{
		{"wrong signature", sign(jwt.SigningMethodHS256, validClaims(), []byte("wrong-test-secret"))},
		{"expired", sign(jwt.SigningMethodHS256, expired, []byte(secret))},
		{"missing expiration", sign(jwt.SigningMethodHS256, missingExpiration, []byte(secret))},
		{"not yet valid", sign(jwt.SigningMethodHS256, notYetValid, []byte(secret))},
		{"different HMAC algorithm", sign(jwt.SigningMethodHS384, validClaims(), []byte(secret))},
		{"unsigned", sign(jwt.SigningMethodNone, validClaims(), jwt.UnsafeAllowNoneSignatureType)},
		{"malformed", "not.a.valid-token"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := TokenValid(test.token, secret); err == nil {
				t.Fatal("invalid token accepted")
			}
			if userID, err := ExtractTokenID(test.token, secret); err == nil || userID != 0 {
				t.Fatalf("invalid token returned user ID %d, error %v", userID, err)
			}
		})
	}
}
