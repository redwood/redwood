package utils

import (
	"github.com/pkg/errors"
)

func Annotate(err *error, msg string, args ...interface{}) {
	if *err != nil {
		*err = errors.Wrapf(*err, msg, args...)
	}
}

func WithStack(err *error) {
	if *err != nil {
		*err = errors.WithStack(*err)
	}
}
