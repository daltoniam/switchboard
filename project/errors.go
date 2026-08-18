package project

import (
	"errors"
	"fmt"
)

// Stable domain error codes frozen in contracts.md.
const (
	CodeProjectNotFound      = "project_not_found"
	CodeProjectAlreadyExists = "project_already_exists"
	CodeInvalidDefinition    = "invalid_definition"
	CodeRevisionConflict     = "revision_conflict"
	CodeAmbiguousProject     = "ambiguous_project"
	CodeInvalidRoot          = "invalid_root"
	CodeRootProjectMismatch  = "root_project_mismatch"
	CodeWriteDisabled        = "write_disabled"
	CodePermissionDenied     = "permission_denied"
	CodeInternalError        = "internal_error"
	CodeLockTimeout          = "lock_timeout"
	CodeDuplicateProjectID   = "duplicate_project_id"
	// CodeReferenced is returned when a Project cannot be deleted because a
	// retained WorkSession still pins it.
	CodeReferenced = "referenced"
)

// Error is a typed catalog error. Adapters must not string-match messages.
type Error struct {
	Code                   string
	Message                string
	ProjectID              ProjectID
	ExpectedSourceRevision Revision
	CurrentSourceRevision  Revision
	RootURI                string
	PathHint               string
	Diagnostics            []Diagnostic
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.ProjectID != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.ProjectID)
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

func errorWithProject(code, message string, id ProjectID) *Error {
	return &Error{Code: code, Message: message, ProjectID: id}
}

// AsError extracts a typed catalog error.
func AsError(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

// IsCode reports whether err is a catalog error with the given code.
func IsCode(err error, code string) bool {
	e, ok := AsError(err)
	return ok && e.Code == code
}
