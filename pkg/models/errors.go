package models

import "fmt"

type FieldError struct {
	Field   string
	Message string
}

func (e FieldError) Error() string {
	return fmt.Sprintf("field %s: %s", e.Field, e.Message)
}

type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error on %s: %s", e.Field, e.Message)
}

func NewValidationError(field, message string) ValidationError {
	return ValidationError{Field: field, Message: message}
}

type StageError struct {
	Stage   string
	Message string
	Err     error
}

func (e StageError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("stage %s: %s: %v", e.Stage, e.Message, e.Err)
	}
	return fmt.Sprintf("stage %s: %s", e.Stage, e.Message)
}

func (e StageError) Unwrap() error {
	return e.Err
}

type NotFoundError struct {
	Resource string
	ID       string
}

func (e NotFoundError) Error() string {
	return fmt.Sprintf("%s not found: %s", e.Resource, e.ID)
}
