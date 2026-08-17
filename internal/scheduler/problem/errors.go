package problem

import "errors"

var (
	ErrInvalidAssignment = errors.New("invalid assignment")
	ErrFacultyConflict   = errors.New("faculty conflict")
	ErrRoomConflict      = errors.New("room conflict")
	ErrGroupConflict     = errors.New("student group conflict")
)
