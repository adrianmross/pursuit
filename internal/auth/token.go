// Package auth hashes and generates Pursuit API tokens.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strings"
)

// Hash returns the hex-encoded SHA-256 of a token secret.
func Hash(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// EqualHash reports whether a stored hex hash matches a secret.
func EqualHash(storedHex, secret string) bool {
	want, err := hex.DecodeString(storedHex)
	if err != nil {
		return false
	}
	sum := sha256.Sum256([]byte(secret))
	return subtle.ConstantTimeCompare(want, sum[:]) == 1
}

// Generate returns a new secret, its display prefix, and its hash.
func Generate() (secret, prefix, hash string, err error) {
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return "", "", "", err
	}
	secret = "pst_" + hex.EncodeToString(buf)
	prefix = Prefix(secret)
	hash = Hash(secret)
	return secret, prefix, hash, nil
}

// Prefix is the non-secret identifier stored beside the hash.
func Prefix(secret string) string {
	secret = strings.TrimSpace(secret)
	if len(secret) <= 12 {
		return secret
	}
	return secret[:12]
}

// NormalizeScopes accepts read, write, and approve.
// write implies read. approve implies read and does not imply write.
// An empty list defaults to read only. Agents that prepare applications
// must not be given approve; only a human approval token can move work
// from awaiting_review to approved, or reject that review.
func NormalizeScopes(in []string) ([]string, error) {
	set := map[string]bool{}
	for _, raw := range in {
		for _, part := range strings.Split(raw, ",") {
			s := strings.ToLower(strings.TrimSpace(part))
			if s == "" {
				continue
			}
			switch s {
			case "read", "write", "approve":
				set[s] = true
			default:
				return nil, errUnknownScope(s)
			}
		}
	}
	if len(set) == 0 {
		set["read"] = true
	}
	if set["write"] || set["approve"] {
		set["read"] = true
	}
	out := make([]string, 0, 3)
	if set["read"] {
		out = append(out, "read")
	}
	if set["write"] {
		out = append(out, "write")
	}
	if set["approve"] {
		out = append(out, "approve")
	}
	return out, nil
}

func errUnknownScope(s string) error {
	return &scopeError{s}
}

type scopeError struct{ s string }

func (e *scopeError) Error() string { return "unknown scope " + e.s }
