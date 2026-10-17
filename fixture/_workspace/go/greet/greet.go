// Package greet builds greetings tagged with a request id.
package greet

import (
	"fmt"

	"github.com/google/uuid"
)

// Greeting returns a greeting for name with a fresh request id.
func Greeting(name string) string {
	return fmt.Sprintf("hello, %s (request %s)", name, RequestID())
}

// RequestID returns a random request id.
func RequestID() string {
	return uuid.NewString()
}
