package adminapp

import "errors"

var (
	ErrNotStarted          = errors.New("adminapp: app has not started")
	ErrAlreadyStarted      = errors.New("adminapp: app already started")
	ErrMissingSystemModule = errors.New("adminapp: system module is required")
	ErrNilContext          = errors.New("adminapp: context is nil")
	ErrShutdownTimeout     = errors.New("adminapp: shutdown timed out")
)
