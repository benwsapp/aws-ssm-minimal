// Package env supplies helpers for reading environment variables with validation.
package env

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// DurationSeconds reads an environment variable and returns its value in seconds, or a default.
func DurationSeconds(key string, defaultSeconds int) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return time.Duration(defaultSeconds) * time.Second, nil
	}

	secs, err := strconv.Atoi(value)
	if err != nil {
		return 0, errors.Join(ErrInvalidDuration, fmt.Errorf("parse %s (%q): %w", key, value, err))
	}

	return time.Duration(secs) * time.Second, nil
}

// MustGetNonEmpty fetches a non-empty environment variable value.
func MustGetNonEmpty(key string) (string, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return "", fmt.Errorf("%w: %s", ErrMissingVariable, key)
	}

	return value, nil
}

// GetString returns the trimmed environment variable value.
func GetString(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}
