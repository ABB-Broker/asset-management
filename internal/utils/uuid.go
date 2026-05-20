// Package utils provides shared helpers used across handlers.
package utils

import "github.com/google/uuid"

func NewUUID() string {
	return uuid.NewString()
}
