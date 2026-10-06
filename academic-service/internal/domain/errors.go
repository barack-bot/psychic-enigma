package domain

import "errors"

var (
	ErrNotFound      = errors.New("record not found")
	ErrUnauthorized  = errors.New("not authorized")
	ErrInvalidStatus = errors.New("invalid status")
	ErrInvalidScore  = errors.New("invalid score")
)
