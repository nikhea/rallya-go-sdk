// Package rallya is the Go SDK for the Rallya event platform API (/api/v1).
// Port of @rallya/sdk (TypeScript): zero third-party dependencies.
package rallya

import "fmt"

// RallyaError is the typed error for every API failure.
// Server envelopes are {error} / {error, code}; outsiders get stealth 404s.
type RallyaError struct {
	Status       int
	Code         string
	Message      string
	Details      any
	RetryAfterMs *int64
}

func (e *RallyaError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("rallya: request failed with status %d (%s): %s", e.Status, e.Code, e.Message)
	}
	return fmt.Sprintf("rallya: request failed with status %d: %s", e.Status, e.Message)
}

func (e *RallyaError) IsUnauthorized() bool { return e.Status == 401 }
func (e *RallyaError) IsForbidden() bool    { return e.Status == 403 }
func (e *RallyaError) IsNotFound() bool     { return e.Status == 404 }
func (e *RallyaError) IsRateLimited() bool  { return e.Status == 429 }

// IsNotFoundOrForbidden reports stealth 404s ("not found OR no access" for
// orgs, events, and other people's orders). Use it instead of treating 404
// as pure absence.
func IsNotFoundOrForbidden(err error) bool {
	var re *RallyaError
	if !asRallyaError(err, &re) {
		return false
	}
	return re.Status == 404 || re.Status == 403
}

func asRallyaError(err error, target **RallyaError) bool {
	if err == nil {
		return false
	}
	type causer interface{ Unwrap() error }
	// Direct type assert first, then unwrap chain manually to avoid importing errors here.
	if re, ok := err.(*RallyaError); ok {
		*target = re
		return true
	}
	if u, ok := err.(causer); ok {
		return asRallyaError(u.Unwrap(), target)
	}
	return false
}
