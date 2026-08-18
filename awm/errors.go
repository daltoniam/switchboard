package awm

import (
	"errors"
	"fmt"
)

// Stable domain error codes for work-model tools. Adapters must not
// string-match messages; map on Code instead.
const (
	CodeNotFound          = "not_found"
	CodeAlreadyExists     = "already_exists"
	CodeInvalidInput      = "invalid_input"
	CodeInvalidReference  = "invalid_reference"
	CodeInvalidTransition = "invalid_transition"
	CodeConflict          = "conflict"
	CodeReferenced        = "referenced"
	CodePolicyBroadening  = "policy_broadening"
	CodeUnavailable       = "unavailable"
	CodeInternal          = "internal_error"
	CodeLockTimeout       = "lock_timeout"
	CodeMissingDefault    = "missing_default_profile"
)

// Error is a typed AWM catalog error with stable Code and relevant IDs.
type Error struct {
	Code          string
	Message       string
	EntityKind    string
	EntityID      string
	ProjectID     string
	WorkProfileID string
	WorkSessionID string
	// Extra carries CAS / revision diagnostics when relevant.
	Expected string
	Current  string
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.EntityID != "" {
		return fmt.Sprintf("%s: %s (%s=%s)", e.Code, e.Message, e.EntityKind, e.EntityID)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Is(target error) bool {
	var other *Error
	if !errors.As(target, &other) {
		return false
	}
	return e.Code == other.Code
}

// AsError extracts a typed AWM error.
func AsError(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

// IsCode reports whether err is an AWM error with the given code.
func IsCode(err error, code string) bool {
	e, ok := AsError(err)
	return ok && e.Code == code
}

func errf(code, kind, id, msg string) *Error {
	return &Error{Code: code, Message: msg, EntityKind: kind, EntityID: id}
}

func notFound(kind, id string) *Error {
	return errf(CodeNotFound, kind, id, kind+" not found")
}

func alreadyExists(kind, id string) *Error {
	return errf(CodeAlreadyExists, kind, id, kind+" already exists")
}

func invalidRef(kind, id, msg string) *Error {
	return errf(CodeInvalidReference, kind, id, msg)
}

func invalidInput(msg string) *Error {
	return &Error{Code: CodeInvalidInput, Message: msg}
}
