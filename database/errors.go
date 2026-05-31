package database

import "errors"

var (
	ErrNotFound         = errors.New("record not found")
	ErrDuplicate        = errors.New("duplicate record")
	ErrInvalidStatus    = errors.New("invalid status transition")
	ErrMaterialNotFound = errors.New("material not found")
	ErrLineNotFound     = errors.New("line not found")
)
