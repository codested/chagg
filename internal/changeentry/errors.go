package changeentry

import "fmt"

const (
	ExitCodeGeneral    = 1
	ExitCodeValidation = 2
	ExitCodeConflict   = 3
)

type CodedError interface {
	error
	ExitCode() int
}

type ValidationError struct {
	Field   string
	Message string
}

func NewValidationError(field string, message string) *ValidationError {
	return &ValidationError{Field: field, Message: message}
}

func (e *ValidationError) Error() string {
	if e.Field == "" {
		return e.Message
	}

	return fmt.Sprintf("invalid %s: %s", e.Field, e.Message)
}

func (e *ValidationError) ExitCode() int {
	return ExitCodeValidation
}

type ConflictError struct {
	Message string
}

func NewConflictError(message string) *ConflictError {
	return &ConflictError{Message: message}
}

func (e *ConflictError) Error() string {
	return e.Message
}

func (e *ConflictError) ExitCode() int {
	return ExitCodeConflict
}
