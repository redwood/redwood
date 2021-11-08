package errors

import (
	"github.com/pkg/errors"
)

var (
	Err403           = errors.New("403: nope")
	Err404           = errors.New("not found")
	ErrUnimplemented = errors.New("unimplemented")
	ErrConnection    = errors.New("connection failed")
	ErrClosed        = errors.New("closed")
	ErrUnsupported   = errors.New("unsupported")
)

var (
	New       = errors.New
	Errorf    = errors.Errorf
	Wrap      = errors.Wrap
	Wrapf     = errors.Wrapf
	WithStack = errors.WithStack
	Cause     = errors.Cause
)

func Annotate(err *error, msg string, args ...interface{}) {
	if *err != nil {
		*err = errors.Wrapf(*err, msg, args...)
	}
}

func AddStack(err *error) {
	if *err != nil {
		*err = errors.WithStack(*err)
	}
}
