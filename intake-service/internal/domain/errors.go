package domain

import "errors"

var (
	ErrNotFound = errors.New("record not found")
	ErrConflict = errors.New("record conflict")
)
