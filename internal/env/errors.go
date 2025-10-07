package env

import "errors"

// ErrMissingVariable indicates a required environment variable was not provided.
var ErrMissingVariable = errors.New("required environment variable not set")

// ErrInvalidDuration indicates an environment variable contained an invalid duration value.
var ErrInvalidDuration = errors.New("invalid duration value")
