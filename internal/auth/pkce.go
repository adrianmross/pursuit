package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
)

// RandomURLToken returns n random bytes, base64url without padding.
func RandomURLToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// PKCEChallenge is the RFC 7636 S256 code_challenge for a verifier.
func PKCEChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// PKCEVerify reports, in constant time, whether verifier matches an S256 challenge.
func PKCEVerify(challenge, verifier string) bool {
	return subtle.ConstantTimeCompare([]byte(PKCEChallenge(verifier)), []byte(challenge)) == 1
}

// ValidPKCEValue checks the RFC 7636 shape shared by verifiers and S256
// challenges: 43 to 128 characters of [A-Za-z0-9-._~].
func ValidPKCEValue(s string) bool {
	if len(s) < 43 || len(s) > 128 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '.' || c == '_' || c == '~') {
			return false
		}
	}
	return true
}
