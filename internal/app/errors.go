package app

import "fmt"

// ErrConnection represents a database connection error.
type ErrConnection struct {
	Cause error
}

func (e *ErrConnection) Error() string {
	return fmt.Sprintf("connection error: %v", e.Cause)
}

func (e *ErrConnection) Unwrap() error {
	return e.Cause
}

// ErrQuery represents a query execution error.
type ErrQuery struct {
	Query string
	Cause error
}

func (e *ErrQuery) Error() string {
	return fmt.Sprintf("query error: %v", e.Cause)
}

func (e *ErrQuery) Unwrap() error {
	return e.Cause
}

// ErrConfig represents a configuration error.
type ErrConfig struct {
	Cause error
}

func (e *ErrConfig) Error() string {
	return fmt.Sprintf("config error: %v", e.Cause)
}

func (e *ErrConfig) Unwrap() error {
	return e.Cause
}
