// Package auth checks token scopes and makes the CLI's PKCE values.
package auth

import "strings"

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
